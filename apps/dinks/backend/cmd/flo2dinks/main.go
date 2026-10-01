// Command flo2dinks converts a Flo account export into a dinks import file.
//
// Flo's export is a wide, self-describing dump of every table the app has ever
// written; dinks stores two things — periods and daily check-ins. This walks
// the parts of the export that carry a date the user actually entered and
// projects them onto that shape, writing a file POST /api/import accepts
// directly:
//
//	go run ./cmd/flo2dinks -in flo-export.json -out dinks-import.json
//
// Days come from the date part of Flo's local timestamps: Flo records the
// calendar day in the member's own zone, so the leading YYYY-MM-DD is the day
// the user logged and the time component carries no further meaning.
//
// Anything this drops is counted and reported on stderr, so a conversion is
// never silently lossy.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"dinks/internal/dto"
)

const (
	source = "flo"
	// defaultSeverity is used for every converted check-in. Flo records that a
	// symptom happened but never how bad it was, so the neutral midpoint of
	// dinks' 1-5 scale is the only value that claims nothing.
	defaultSeverity = 3
	// noteKind is the kind dinks already uses to carry free text for a day
	// (see web/src/pages/Today.tsx, which writes moods and notes this way).
	noteKind = "note"
	// unknownFlow is what dinks stores when a period has no recorded flow.
	unknownFlow = "unknown"
)

func main() {
	in := flag.String("in", "", "path to the Flo export JSON (required)")
	out := flag.String("out", "", "path to write the dinks import file (default: stdout)")
	flag.Parse()

	if *in == "" {
		fail("usage: flo2dinks -in flo-export.json [-out dinks-import.json]")
	}
	payload, rep, err := convertFile(*in, time.Now())
	if err != nil {
		fail(err.Error())
	}
	if *out == "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fail(err.Error())
		}
	} else if err := writeFile(*out, payload); err != nil {
		fail(err.Error())
	}
	rep.print(*out)
}

// report accumulates what a conversion produced and what it could not carry
// across, so the user sees the lossy parts rather than discovering them later.
type report struct {
	periods   int
	checkins  int
	symptoms  int
	moodNotes int
	textNotes int
	// flowDays counts per-day flow entries written, and varyingPeriods the
	// periods that have at least one — the summary line the user cares about
	// when checking whether day-to-day flow made it across.
	flowDays       int
	varyingPeriods int
	skipped        []string
	clamped        []string
	dropped        map[string]int
}

func newReport() *report { return &report{dropped: map[string]int{}} }

func (rep *report) skip(format string, args ...any) {
	rep.skipped = append(rep.skipped, fmt.Sprintf(format, args...))
}

func (rep *report) drop(category, sub string) {
	rep.dropped[category+"/"+sub]++
}

func (rep *report) print(out string) {
	w := os.Stderr
	say := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, format, args...)
	}
	say("periods:   %d\n", rep.periods)
	say("check-ins: %d (%d symptom, %d mood notes, %d text notes)\n",
		rep.checkins, rep.symptoms, rep.moodNotes, rep.textNotes)
	if rep.varyingPeriods > 0 {
		say("per-day flow: %d day(s) across %d period(s) — the rest had a single level throughout\n",
			rep.flowDays, rep.varyingPeriods)
	} else {
		say("per-day flow: none recorded in the export\n")
	}
	if out != "" {
		say("written to %s\n", out)
	}
	if len(rep.skipped) > 0 {
		say("\n%d cycle(s) skipped:\n", len(rep.skipped))
		for _, s := range rep.skipped {
			say("  - %s\n", s)
		}
	}
	if len(rep.clamped) > 0 {
		say("\n%d period end date(s) clamped to today — dinks treats a period end as past:\n", len(rep.clamped))
		for _, s := range rep.clamped {
			say("  - %s\n", s)
		}
	}
	if len(rep.dropped) > 0 {
		keys := make([]string, 0, len(rep.dropped))
		for k := range rep.dropped {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		say("\n%d event(s) with no dinks equivalent, left out of the file:\n", countDropped(rep.dropped))
		for _, k := range keys {
			say("  - %s x%d\n", k, rep.dropped[k])
		}
	}
}

func countDropped(d map[string]int) int {
	n := 0
	for _, v := range d {
		n += v
	}
	return n
}

// --- Flo export shape -------------------------------------------------------
//
// Only the fields the conversion reads are declared; the rest of the export
// (social data, wearables, sessions, PII) is ignored, and encoding/json drops
// whatever it does not recognise.

type floExport struct {
	OperationalData       floOperational `json:"operationalData"`
	PointEventsManualData floPointEvents `json:"pointEventsManualData"`
}

type floOperational struct {
	Cycles []floCycle `json:"cycles"`
	Notes  []floNote  `json:"notes"`
}

type floCycle struct {
	ID string `json:"id"`
	// Flo writes local timestamps as "YYYY-MM-DD HH:MM:SS.fff". The date part
	// is the day; the rest is a zone artefact.
	PeriodStartDate string `json:"period_start_date"`
	PeriodEndDate   string `json:"period_end_date"`
	// PeriodIntensity is itself a JSON document, stored as a string, mapping a
	// zero-based day index within the period to the flow recorded that day.
	PeriodIntensity string `json:"period_intensity"`
	Pregnant        bool   `json:"pregnant"`
}

type floNote struct {
	Text string `json:"text"`
	Date string `json:"date"`
}

type floPointEvents struct {
	Events []floEvent `json:"point_events_manual_v2"`
}

type floEvent struct {
	// LocalDate is the member's own calendar day; Date is the same instant
	// rendered in UTC and can fall on a different day.
	LocalDate   string `json:"local_date"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
}

// --- conversion -------------------------------------------------------------

func convertFile(path string, now time.Time) (dto.ImportPayload, *report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return dto.ImportPayload{}, nil, err
	}
	return convert(raw, now)
}

func convert(raw []byte, now time.Time) (dto.ImportPayload, *report, error) {
	var src floExport
	if err := json.Unmarshal(raw, &src); err != nil {
		return dto.ImportPayload{}, nil, fmt.Errorf("reading the Flo export: %w", err)
	}
	today := now.Format(dateLayout)
	rep := newReport()

	periods := convertCycles(src.OperationalData.Cycles, today, rep)
	symptoms := convertEvents(src.PointEventsManualData.Events, src.OperationalData.Notes, rep)
	rep.periods = len(periods)
	rep.checkins = len(symptoms)

	return dto.ImportPayload{
		Format:     dto.ImportFormat,
		Version:    dto.ImportVersion,
		Source:     source,
		ExportedAt: now.UTC().Format(time.RFC3339),
		Periods:    periods,
		Symptoms:   symptoms,
	}, rep, nil
}

const dateLayout = "2006-01-02"

// convertCycles maps each Flo cycle to one dinks period. A cycle that is still
// open (no end date) becomes a period with no end, which is exactly how dinks
// represents the current one.
func convertCycles(cycles []floCycle, today string, rep *report) []dto.PeriodInput {
	out := make([]dto.PeriodInput, 0, len(cycles))
	for _, c := range cycles {
		start, ok := dayOf(c.PeriodStartDate)
		if !ok {
			rep.skip("cycle %s: unreadable start date %q", shortID(c.ID), c.PeriodStartDate)
			continue
		}
		p := dto.PeriodInput{StartedOn: start, Flow: flowOf(c.PeriodIntensity), Notes: ""}
		if c.Pregnant {
			// dinks has no pregnancy flag; keeping a cycle the user recorded
			// while pregnant would be their own data to re-check, not one to
			// silently drop, so it is reported and left out.
			rep.skip("cycle starting %s: recorded during pregnancy, dinks has no equivalent", start)
			continue
		}
		if end, ok := dayOf(c.PeriodEndDate); ok && end != "" {
			if end < start {
				rep.skip("cycle starting %s: ends %s, before it starts", start, end)
				continue
			}
			if end > today {
				rep.clamped = append(rep.clamped,
					fmt.Sprintf("%s: end %s is in the future, clamped to %s", start, end, today))
				end = today
			}
			p.EndedOn = end
		}
		p.Days = dayFlows(start, p.EndedOn, c.PeriodIntensity, today)
		rep.flowDays += len(p.Days)
		if len(p.Days) > 0 {
			rep.varyingPeriods++
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedOn < out[j].StartedOn })
	return out
}

// dayOf takes the calendar day out of one of Flo's local timestamps. The second
// return is false when the field is absent or unparseable.
func dayOf(ts string) (string, bool) {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return "", false
	}
	if _, err := time.Parse(dateLayout, ts[:min(len(ts), len(dateLayout))]); err != nil {
		return "", false
	}
	return ts[:len(dateLayout)], true
}

// flowOf reduces Flo's per-day flow to the single value dinks keeps as a
// period's summary. The peak is the honest choice for that summary: it is the
// heaviest day the user recorded for the period. The per-day detail is not lost
// — it becomes the period's `days` list via dayFlows.
func flowOf(encoded string) string {
	intensity := parseIntensity(encoded)
	if len(intensity) == 0 {
		return unknownFlow
	}
	peak := 0
	for _, v := range intensity {
		if v > peak {
			peak = v
		}
	}
	return flowForIntensity(peak)
}

// dayFlows expands Flo's day-index -> intensity map into dinks' per-day flow
// list, which is keyed by calendar date. Only days that differ from the
// period's summary flow are emitted: dinks treats the summary as the fallback
// for any unlisted day, so writing the peak for every day would be redundant
// and would misrepresent the lighter days. An index with no value, or a date
// past the period's end, is dropped — the export can carry intensity for a day
// the period record itself does not cover.
func dayFlows(start, end, encoded, today string) []dto.FlowDay {
	intensity := parseIntensity(encoded)
	if len(intensity) == 0 {
		return nil
	}
	summary := flowOf(encoded)

	last := end
	if last == "" {
		last = today // an open-ended period only has days up to today
	}
	base, err := time.Parse(dateLayout, start)
	if err != nil {
		return nil
	}
	var out []dto.FlowDay
	for idx, v := range intensity {
		if idx < 0 {
			continue
		}
		day := base.AddDate(0, 0, idx)
		iso := day.Format(dateLayout)
		if iso > last {
			continue
		}
		flow := flowForIntensity(v)
		if flow == summary {
			continue
		}
		out = append(out, dto.FlowDay{Date: iso, Flow: flow})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func parseIntensity(encoded string) map[int]int {
	intensity := map[int]int{}
	if err := json.Unmarshal([]byte(encoded), &intensity); err != nil {
		return nil
	}
	return intensity
}

// flowForIntensity maps Flo's flow scale onto dinks' three levels. Flo records
// 1 light, 2 medium, 3 heavy; 0 and anything above the known range are treated
// as the nearest level rather than rejected, so an unfamiliar value still lands
// somewhere sensible.
func flowForIntensity(v int) string {
	switch {
	case v <= 1:
		return "light"
	case v == 2:
		return "medium"
	default:
		return "heavy"
	}
}

// day groups everything Flo recorded for one calendar day, so the conversion
// can emit the same shape the app itself writes: one check-in per symptom kind,
// and a single note holding that day's moods and free text.
type day struct {
	symptoms map[string]bool
	moods    []string
	seenMood map[string]bool
	text     []string
}

func newDay() *day {
	return &day{symptoms: map[string]bool{}, seenMood: map[string]bool{}}
}

func convertEvents(events []floEvent, notes []floNote, rep *report) []dto.SymptomInput {
	days := map[string]*day{}
	get := func(d string) *day {
		if days[d] == nil {
			days[d] = newDay()
		}
		return days[d]
	}

	for _, e := range events {
		d, ok := dayOf(e.LocalDate)
		if !ok {
			rep.skip("event on %q: unreadable date", e.LocalDate)
			continue
		}
		switch e.Category {
		case "Symptom":
			kind, ok := symptomKind(e.Subcategory)
			if !ok {
				rep.drop(e.Category, e.Subcategory)
				continue
			}
			if get(d).symptoms[kind] {
				continue // already recorded for this day
			}
			get(d).symptoms[kind] = true
		case "Mood":
			mood, ok := moodName(e.Subcategory)
			if !ok {
				rep.drop(e.Category, e.Subcategory)
				continue
			}
			if g := get(d); !g.seenMood[mood] {
				g.seenMood[mood] = true
				g.moods = append(g.moods, mood)
			}
		case "Sex":
			// Flo records intercourse and libido as separate vocabularies under
			// one category. dinks has neither field, but a symptom kind is a
			// free-form string that already round-trips through import, export,
			// the calendar and stats — so a day's sex and libido are recorded as
			// ordinary check-ins rather than discarded.
			if kind, ok := sexKind(e.Subcategory); ok {
				if get(d).symptoms[kind] {
					continue // already recorded for this day
				}
				get(d).symptoms[kind] = true
				continue
			}
			rep.drop(e.Category, e.Subcategory)
		default:
			// Discharge, pregnancy tests, sport and the rest have no field in
			// dinks. They are reported rather than squeezed into the notes,
			// which would invent a meaning the app does not have.
			rep.drop(e.Category, e.Subcategory)
		}
	}

	for _, n := range notes {
		d, ok := dayOf(n.Date)
		if !ok {
			rep.skip("note: unreadable date %q", n.Date)
			continue
		}
		if text := strings.TrimSpace(n.Text); text != "" {
			get(d).text = append(get(d).text, text)
		}
	}

	dates := make([]string, 0, len(days))
	for d := range days {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	out := make([]dto.SymptomInput, 0, len(dates)*2)
	for _, d := range dates {
		g := days[d]
		for kind := range g.symptoms {
			out = append(out, dto.SymptomInput{
				RecordedOn: d, Kind: kind, Severity: defaultSeverity, Notes: "",
			})
			rep.symptoms++
		}
		if note := noteText(g); note != "" {
			out = append(out, dto.SymptomInput{
				RecordedOn: d, Kind: noteKind, Severity: defaultSeverity, Notes: note,
			})
			if len(g.moods) > 0 {
				rep.moodNotes++
			}
			if len(g.text) > 0 {
				rep.textNotes++
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RecordedOn != out[j].RecordedOn {
			return out[i].RecordedOn < out[j].RecordedOn
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// noteText renders a day's moods and free text in the form the dinks UI itself
// writes, so imported notes read exactly like notes the user typed.
func noteText(g *day) string {
	var b strings.Builder
	if len(g.moods) > 0 {
		b.WriteString("Mood: ")
		b.WriteString(strings.Join(g.moods, ", "))
		b.WriteString(".")
	}
	if len(g.text) > 0 {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(strings.Join(g.text, " "))
	}
	return b.String()
}

// symptomKind maps Flo's symptom vocabulary onto the labels dinks uses, so the
// existing emoji and analytics keep working. A kind Flo does not know about
// falls back to a humanised name rather than being lost, since dinks stores
// kinds as free-form strings and will happily display a new one.
func symptomKind(sub string) (string, bool) {
	if v, ok := knownSymptoms[sub]; ok {
		return v, true
	}
	return humanize(sub), sub != ""
}

var knownSymptoms = map[string]string{
	"DrawingPain":        "Cramps",
	"AbdominalPain":      "Abdominal pain",
	"TenderBreasts":      "Tender breasts",
	"IncreasedAppetite":  "Increased appetite",
	"Backache":           "Backache",
	"VaginalItching":     "Vaginal itching",
	"Insomnia":           "Insomnia",
	"Diarrhea":           "Diarrhea",
	"Nausea":             "Nausea",
	"Fatigue":            "Fatigue",
	"Bloating":           "Bloating",
	"Acne":               "Acne",
	"Headache":           "Headache",
	"MoodSwings":         "Mood swings",
	"VaginalDryness":     "Vaginal dryness",
	"BreastTenderness":   "Tender breasts",
	"HeavyPeriod":        "Heavy period",
	"IrregularPeriod":    "Irregular period",
	"Spotting":           "Spotting",
	"LowerAbdominalPain": "Abdominal pain",
}

// moodName maps Flo's mood vocabulary onto the labels dinks offers. Several
// Flo moods describe shades of the same dinks mood, so they collapse together.
func moodName(sub string) (string, bool) {
	if v, ok := knownMoods[sub]; ok {
		return v, true
	}
	return humanize(sub), sub != ""
}

var knownMoods = map[string]string{
	"Neutral":           "Calm",
	"Depressed":         "Sad",
	"Apathetic":         "Tired",
	"LowEnergy":         "Tired",
	"VerySelfCritical":  "Sensitive",
	"FeelingGuilty":     "Sensitive",
	"ObsessiveThoughts": "Anxious",
	"Panic":             "Anxious",
	"Confused":          "Confused",
	"Swings":            "Mood swings",
	"Angry":             "Irritable",
	"Calm":              "Calm",
	"Happy":             "Happy",
	"Sad":               "Sad",
	"Stressed":          "Stressed",
	"Grateful":          "Grateful",
	"Confident":         "Confident",
	"Energetic":         "Energetic",
	"Sensitive":         "Sensitive",
	"Anxious":           "Anxious",
	"Irritable":         "Irritable",
	"Tired":             "Tired",
}

// sexKind maps Flo's Sex category onto dinks check-in kinds. Flo mixes two
// unrelated things under it — what happened (intercourse) and how much desire
// there was (libido) — so the libido levels are kept distinct from the
// intercourse kinds rather than collapsed into one "sex" entry.
func sexKind(sub string) (string, bool) {
	if v, ok := knownSex[sub]; ok {
		return v, true
	}
	// "SexNone" means no sex that day and "Orgasm" is not a kind of sex, so
	// neither gets a generic fallback the way a symptom would.
	return "", false
}

var knownSex = map[string]string{
	"SexProtected":   "Sex — protected",
	"SexUnprotected": "Sex — unprotected",
	"SexOral":        "Sex — oral",
	"SexToys":        "Sex — toys",
	"Orgasm":         "Orgasm",
	"HighDrive":      "Libido — high",
	"NeutralDrive":   "Libido — moderate",
	"LowDrive":       "Libido — low",
	"NoneDrive":      "Libido — none",
}

// humanize turns a PascalCase enum into a readable label, so an unrecognised
// value still arrives in the app as something a person can read.
func humanize(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if prev := rune(s[i-1]); prev < 'A' || prev > 'Z' {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func writeFile(path string, payload dto.ImportPayload) error {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "flo2dinks:", msg)
	os.Exit(1)
}
