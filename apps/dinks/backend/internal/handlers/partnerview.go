package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/middleware"
	"dinks/internal/model"
	"dinks/internal/repository"
)

// sexKindPrefixes and libidoKindPrefixes identify the check-in kinds that carry
// sex and libido. They are matched by prefix because the kinds are free-form
// strings that the Flo importer and the UI both write as "Sex — x" and
// "Libido — x"; a hard-coded list of exact kinds would silently start sharing
// nothing the moment a kind was renamed.
var (
	sexKindPrefixes    = []string{"Sex — ", "Sex - "}
	libidoKindPrefixes = []string{"Libido — ", "Libido - "}
)

func isSexKind(kind string) bool { return hasAnyPrefix(kind, sexKindPrefixes) }
func isLibidoKind(kind string) bool {
	return hasAnyPrefix(kind, libidoKindPrefixes)
}

// noteKind is the kind dinks uses to carry a day's free text. It is matched
// case-insensitively because the Flo importer writes "note" while the web app
// writes "note" and older data may hold either.
func isNoteKind(kind string) bool { return strings.EqualFold(kind, "note") }

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// UpdateShare sets which fields one partner may see. Every field is validated
// and unknown ones rejected, so a stale client cannot smuggle a field name the
// projection does not understand into the stored share.
func (h *Handler) UpdateShare(w http.ResponseWriter, r *http.Request) {
	var in dto.ShareInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Share == nil {
		httpx.Problem(r, w, http.StatusBadRequest, "share is required — send an array, empty to share nothing", nil)
		return
	}
	share := make([]model.ShareField, 0, len(*in.Share))
	for _, f := range *in.Share {
		field := model.ShareField(f)
		if !model.ValidShareField(field) {
			httpx.Problem(r, w, http.StatusBadRequest, "unknown share field: "+string(f), nil)
			return
		}
		if !slices.Contains(share, field) {
			share = append(share, field)
		}
	}
	owner, partner := middleware.Subject(r.Context()), r.PathValue("subject")
	if err := h.repo.UpdateShare(r.Context(), owner, partner, share); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to save share", err)
		return
	}
	link, err := h.repo.LinkBetween(r.Context(), owner, partner)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Problem(r, w, http.StatusNotFound, "partner not found", err)
		return
	}
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load partner", err)
		return
	}
	httpx.JSON(w, http.StatusOK, shareDTO(link))
}

// PartnerView returns everything one owner shares with the caller. The shape is
// the same for every field: the client learns which fields it may render from
// `shared`, and absent fields are simply absent — so a client cannot display
// data the projection did not include by assuming a field is present.
func (h *Handler) PartnerView(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("subject")
	link, err := h.repo.LinkBetween(r.Context(), owner, middleware.Subject(r.Context()))
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Problem(r, w, http.StatusNotFound, "nothing shared with you by this person", err)
		return
	}
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load share", err)
		return
	}
	member, err := h.repo.GetMember(r.Context(), owner)
	if err != nil {
		httpx.Problem(r, w, http.StatusNotFound, "unknown person", err)
		return
	}

	// The owner who revoked a link should not still be readable through a stale
	// route, so the link is re-checked against the live collection.
	ps, err := h.repo.SharedPeriods(r.Context(), owner, maxSharedDayReads)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load shared records", err)
		return
	}
	from := time.Now().AddDate(0, 0, -maxSharedDayReads)
	ss, err := h.repo.SharedSymptoms(r.Context(), owner, from)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load shared records", err)
		return
	}

	status := h.projectStatus(link, member.DisplayName, ps, h.preferencesOf(r.Context(), owner))
	returned := h.projectDays(link, ps, ss)
	httpx.JSON(w, http.StatusOK, dto.PartnerView{
		PartnerStatus: status,
		Shared:        sharedFieldNames(link),
		Days:          returned,
	})
}

// projectStatus derives the summary block, honouring per-field choices. Every
// field is derived once and then filtered, so the status a partner sees is
// always one consistent decision rather than two partially-applied ones.
func (h *Handler) projectStatus(link model.PartnerLink, displayName string, ps []model.Period, prefs model.Preferences) dto.PartnerStatus {
	out := dto.PartnerStatus{Subject: link.OwnerSubject, DisplayName: displayName}
	if !link.Allows(model.ShareOnPeriod) {
		return out
	}
	onPeriod, next, reminder := cycleStatus(ps, prefs)
	out.OnPeriod = &onPeriod
	if link.Allows(model.ShareNextPeriod) {
		out.NextPeriod = next
	}
	if link.Allows(model.ShareReminders) {
		out.Reminder = reminder
	}
	return out
}

// projectDays builds the per-day view, applying the share to every field.
func (h *Handler) projectDays(link model.PartnerLink, ps []model.Period, ss []model.Symptom) []dto.PartnerDay {
	// A period's existence is itself information: without on_period, a partner
	// who shared symptoms must not be able to infer a period from the shape of
	// the rows, so no period contributes a row or a flag. Flow is only meaningful
	// on a period day, so it is gated on on_period too — otherwise the shape of
	// the data would leak the period that on_period was meant to hide.
	sharePeriod := link.Allows(model.ShareOnPeriod)
	shareFlow := link.Allows(model.ShareFlow) && sharePeriod
	shareSymptoms := link.Allows(model.ShareSymptoms)
	shareNotes := link.Allows(model.ShareNotes)
	shareSex := link.Allows(model.ShareSex)
	shareLibido := link.Allows(model.ShareLibido)

	// flow per day, resolved through the period's own summary fallback
	flowByDate := map[string]string{}
	onPeriodDate := map[string]bool{}
	started := map[string]bool{}
	ended := map[string]bool{}
	if sharePeriod {
		for _, p := range ps {
			// Bounded so a period left open from years ago cannot make this loop
			// unbounded; the oldest days fall outside the shared window anyway.
			days := 0
			for d := p.StartedOn; !d.After(p.LastDay()) && days < maxSharedDayReads; d = d.AddDate(0, 0, 1) {
				key := d.Format(dateLayout)
				onPeriodDate[key] = true
				flowByDate[key] = p.FlowOn(d)
				days++
			}
			started[p.StartedOn.Format(dateLayout)] = true
			if p.EndedOn != nil {
				ended[p.EndedOn.Format(dateLayout)] = true
			}
		}
	}

	// Check-ins are sorted into the field that matches what they are, so a
	// partner never has sex or libido turn up in a symptom list, and a day's
	// free text is never presented as a symptom name.
	symptomsByDate := map[string][]string{}
	sexByDate := map[string][]string{}
	libidoByDate := map[string][]string{}
	notesByDate := map[string][]string{}
	for _, s := range ss {
		key := s.RecordedOn.Format(dateLayout)
		switch {
		case isSexKind(s.Kind):
			if shareSex {
				sexByDate[key] = append(sexByDate[key], s.Kind)
			}
		case isLibidoKind(s.Kind):
			if shareLibido {
				libidoByDate[key] = append(libidoByDate[key], s.Kind)
			}
		case isNoteKind(s.Kind):
			if shareNotes {
				if text := strings.TrimSpace(s.Notes); text != "" {
					notesByDate[key] = append(notesByDate[key], text)
				}
			}
		case shareSymptoms:
			symptomsByDate[key] = append(symptomsByDate[key], s.Kind)
		}
	}

	// A day is interesting if it has a period, a symptom, a note, or a sex
	// record — anything else the owner shared is not worth a row.
	type dayRow struct {
		on     bool
		flow   string
		starts bool
		ends   bool
		notes  string
		syms   []string
		sex    []string
		libido []string
	}
	rows := map[string]*dayRow{}
	get := func(key string) *dayRow {
		if rows[key] == nil {
			rows[key] = &dayRow{}
		}
		return rows[key]
	}
	if sharePeriod {
		for key, on := range onPeriodDate {
			r := get(key)
			r.on = on
			if shareFlow {
				r.flow = flowByDate[key]
			}
		}
		for key := range started {
			get(key).starts = true
		}
		for key := range ended {
			get(key).ends = true
		}
	}
	for key, syms := range symptomsByDate {
		get(key).syms = syms
	}
	for key, v := range sexByDate {
		get(key).sex = v
	}
	for key, v := range libidoByDate {
		get(key).libido = v
	}
	for key, v := range notesByDate {
		get(key).notes = strings.Join(v, " ")
	}

	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if len(keys) > maxSharedDayReads {
		keys = keys[len(keys)-maxSharedDayReads:]
	}
	out := make([]dto.PartnerDay, 0, len(keys))
	for _, k := range keys {
		r := rows[k]
		out = append(out, dto.PartnerDay{
			Date:          k,
			OnPeriod:      r.on,
			Flow:          r.flow,
			Symptoms:      r.syms,
			Sex:           r.sex,
			Libido:        r.libido,
			Notes:         r.notes,
			PeriodStarted: r.starts,
			PeriodEnded:   r.ends,
		})
	}
	return out
}

// maxSharedDayReads bounds how much history one partner view may return. The
// owner may share a field, but never an unbounded amount of history.
const maxSharedDayReads = 30

// shareDTO renders a link's current share for the owner's own settings screen.
func shareDTO(link model.PartnerLink) dto.Partner {
	return dto.Partner{
		Subject:     link.PartnerSubject,
		DisplayName: "",
		LinkedAt:    link.CreatedAt.Format(time.RFC3339),
		Share:       effectiveShare(link),
	}
}

// effectiveShare is the share as it will actually be applied, so the owner sees
// the default status-only scope on a link created before shares existed rather
// than an empty list that would suggest nothing is shared at all.
func effectiveShare(link model.PartnerLink) []dto.ShareField {
	if link.Share == nil {
		return []dto.ShareField{
			dto.ShareField(model.ShareOnPeriod),
			dto.ShareField(model.ShareNextPeriod),
			dto.ShareField(model.ShareReminders),
		}
	}
	out := make([]dto.ShareField, 0, len(link.Share))
	for _, f := range link.Share {
		out = append(out, dto.ShareField(f))
	}
	return out
}

func sharedFieldNames(link model.PartnerLink) []string {
	names := effectiveShare(link)
	out := make([]string, 0, len(names))
	for _, f := range names {
		out = append(out, string(f))
	}
	return out
}
