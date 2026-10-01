package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/middleware"
	"dinks/internal/model"
	"dinks/internal/repository"
)

// validFlowLevels are the levels a member may store as their default. It is a
// superset of the levels the picker offers: "unknown" is accepted so an imported
// file can round-trip, but it is not something a member can choose.
var validFlowLevels = map[string]bool{
	model.FlowNone:    true,
	model.FlowUnknown: true,
	model.FlowLight:   true,
	model.FlowMedium:  true,
	model.FlowHeavy:   true,
}

// trackerKeyPattern constrains a tracker key to a slug: lowercase, digits and
// underscores. The key is embedded in the check-in Kind ("track:<key>"), so it
// must not be able to contain a separator or be unbounded in length.
var trackerKeyPattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// maxTrackers bounds the custom indicator list. Each tracker adds a field to
// every day's editor, so an unbounded list would make logging unusable.
const maxTrackers = 12

// GetPreferences returns the member's settings, with defaults filled in.
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	m, err := h.repo.GetMember(r.Context(), middleware.Subject(r.Context()))
	if errors.Is(err, repository.ErrNotFound) {
		// A bearer-only member may not have a member row yet; report defaults
		// rather than a 404, since preferences are always readable.
		httpx.JSON(w, http.StatusOK, preferencesDTO(model.Preferences{}))
		return
	}
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load preferences", err)
		return
	}
	httpx.JSON(w, http.StatusOK, preferencesDTO(m.Preferences))
}

// UpdatePreferences applies a partial update. Only the fields the client sent are
// changed, so a client that knows about one preference never clobbers another.
func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var in dto.PreferencesInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	sub := middleware.Subject(r.Context())
	m, err := h.repo.GetMember(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load preferences", err)
		return
	}
	p := m.Preferences

	if in.DefaultFlow != nil {
		if !validFlowLevels[*in.DefaultFlow] {
			httpx.Problem(r, w, http.StatusBadRequest, "default_flow is not a known flow level", nil)
			return
		}
		p.DefaultFlow = *in.DefaultFlow
	}
	if in.ReminderEnabled != nil {
		p.ReminderEnabled = in.ReminderEnabled
	}
	if in.ReminderLeadDays != nil {
		if *in.ReminderLeadDays < 0 || *in.ReminderLeadDays > 30 {
			httpx.Problem(r, w, http.StatusBadRequest, "reminder_lead_days must be between 0 and 30", nil)
			return
		}
		p.ReminderLeadDays = *in.ReminderLeadDays
	}
	if in.Trackers != nil {
		trackers, err := parseTrackers(*in.Trackers)
		if err != nil {
			httpx.Problem(r, w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		p.Trackers = trackers
	}

	if err := h.repo.UpdatePreferences(r.Context(), sub, p); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to save preferences", err)
		return
	}
	httpx.JSON(w, http.StatusOK, preferencesDTO(p))
}

// parseTrackers validates a whole tracker list, since the client sends the
// complete set each time: there is no meaningful partial state for "which
// indicators exist".
func parseTrackers(in []dto.Tracker) ([]model.Tracker, error) {
	if len(in) > maxTrackers {
		return nil, errors.New("too many trackers")
	}
	out := make([]model.Tracker, 0, len(in))
	seen := make(map[string]bool, len(in))
	for i, t := range in {
		key := strings.ToLower(strings.TrimSpace(t.Key))
		if key == "" {
			// Derive a key from the label so a client that only knows the name the
			// user typed still gets a stable one.
			key = slugify(t.Label)
		}
		if !trackerKeyPattern.MatchString(key) {
			return nil, errors.New("tracker key must be lowercase letters, digits and underscores")
		}
		if seen[key] {
			return nil, errors.New("two trackers share the key " + key)
		}
		seen[key] = true
		label := strings.TrimSpace(t.Label)
		if label == "" {
			return nil, errors.New("tracker needs a label")
		}
		if len([]rune(label)) > 40 {
			return nil, errors.New("tracker label is too long")
		}
		// An emoji is one grapheme but several codepoints (variation selectors,
		// skin-tone modifiers, ZWJ sequences), so it is stored whole and only
		// length-bounded. Truncating by rune count would cut "⚖️" in half and
		// change how it renders.
		emoji := strings.TrimSpace(t.Emoji)
		if utf8.RuneCountInString(emoji) > maxEmojiRunes {
			emoji = ""
		}
		unit := strings.TrimSpace(t.Unit)
		if len(unit) > 12 {
			return nil, errors.New("tracker unit is too long")
		}
		if i >= maxTrackers {
			return nil, errors.New("too many trackers")
		}
		out = append(out, model.Tracker{Key: key, Label: label, Emoji: emoji, Unit: unit})
	}
	return out, nil
}

// maxEmojiRunes is generous enough for any single emoji (a flag is two regional
// indicators, a skin-tone modifier is one more, a ZWJ family is several) while
// still bounding the stored string.
const maxEmojiRunes = 12

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a user-typed label into a key, e.g. "Body weight" -> "body_weight".
func slugify(s string) string {
	s = nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "_")
	return strings.Trim(s, "_")
}

func preferencesDTO(p model.Preferences) dto.Preferences {
	out := dto.Preferences{
		// Resolved, not stored raw: a member who never picked one still gets a
		// level the picker can preselect.
		DefaultFlow:      p.DefaultFlowLevel(),
		ReminderEnabled:  p.ReminderOn(),
		ReminderLeadDays: p.LeadDays(),
		Trackers:         make([]dto.Tracker, 0, len(p.Trackers)),
	}
	for _, t := range p.Trackers {
		out.Trackers = append(out.Trackers, dto.Tracker{Key: t.Key, Label: t.Label, Emoji: t.Emoji, Unit: t.Unit})
	}
	return out
}
