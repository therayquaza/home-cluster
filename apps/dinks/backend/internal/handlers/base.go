// Package handlers implements the HTTP surface of the dinks backend: the
// authentication endpoints, the cycle and symptom records, statistics, partner
// sharing, and account management. Handlers only translate between HTTP and the
// repository/service layer; wire conventions live in httpx.
package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"dinks/internal/config"
	"dinks/internal/domain"
	"dinks/internal/dto"
	"dinks/internal/logging"
	"dinks/internal/model"
	"dinks/internal/repository"
	"dinks/internal/service"
)

// Session keys and the fallback display name for a member with no usable name.
const (
	sessionSubject = "subject"
	sessionState   = "oidc_state"
	sessionPKCE    = "oidc_verifier"

	fallbackName = "Member"
)

// dateLayout is the calendar-day format every date crosses the API as. Parsing
// and formatting through it keeps a day in the server's own zone instead of
// letting it drift with the process's location settings.
const dateLayout = "2006-01-02"

// Handler holds the dependencies every HTTP handler needs.
type Handler struct {
	repo         *repository.Cycle
	statsService *service.Stats
	sessions     *scs.SessionManager
	oauth        *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	mobile       *oidc.IDTokenVerifier
	webAudience  string
	devAuth      bool
}

// New builds the handler set and performs OIDC discovery. In dev-auth mode
// OIDC is skipped entirely and the verifiers are left nil.
func New(repo *repository.Cycle, statsService *service.Stats, sessions *scs.SessionManager, cfg config.Config) (*Handler, error) {
	h := &Handler{
		repo:         repo,
		statsService: statsService,
		sessions:     sessions,
		webAudience:  cfg.OIDCClientID,
		devAuth:      cfg.DevAuth,
	}
	if cfg.DevAuth {
		logging.Log.Warn("DEV_AUTH is enabled: every request is accepted as local-dev-user and OIDC is disabled")
		return h, nil
	}

	provider, err := oidc.NewProvider(context.Background(), cfg.OIDCIssuer)
	if err != nil {
		return nil, err
	}
	h.oauth = &oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	h.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
	h.mobile = provider.Verifier(&oidc.Config{ClientID: cfg.OIDCMobileClientID})

	if strings.TrimSpace(cfg.OIDCClientSecret) == "" {
		logging.Log.Warn("OIDC_CLIENT_SECRET is empty: token exchange will fail with invalid_client",
			"hint", "check the dinks-web-client-secret property of the keycloak secret in Vault")
	}
	logging.Log.Info("oidc ready", "issuer", cfg.OIDCIssuer, "web_client", cfg.OIDCClientID,
		"mobile_client", cfg.OIDCMobileClientID, "redirect_url", cfg.OIDCRedirectURL)
	return h, nil
}

// MobileVerifier returns the verifier for mobile bearer tokens, or nil when
// running with dev auth. The caller uses it to build the auth middleware.
func (h *Handler) MobileVerifier() *oidc.IDTokenVerifier { return h.mobile }

// random returns a URL-safe random string for OIDC state and PKCE verifiers.
func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// A failure here means the kernel entropy source is unavailable; there
		// is no safe way to continue issuing login redirects.
		panic(err)
	}
	return hex.EncodeToString(b)
}

// cycleStatus derives on-period/next-period/reminder from a subject's periods —
// shared by the owner's own dashboard and the status-only view a partner sees.
// reminderOn and leadDays come from the owner's preferences, so one member can
// silence the reminder or change its lead time without affecting anyone else.
func cycleStatus(ps []model.Period, prefs model.Preferences) (onPeriod bool, nextPeriod *string, reminder string) {
	reminderOn, leadDays := prefs.ReminderOn(), prefs.LeadDays()
	for _, p := range ps {
		if p.EndedOn == nil {
			onPeriod = true
			break
		}
	}
	domainPeriods := make([]domain.Period, 0, len(ps))
	for _, p := range ps {
		domainPeriods = append(domainPeriods, domain.Period{StartedOn: p.StartedOn.Format(dateLayout)})
	}
	if next := domain.EstimateNextPeriod(domainPeriods); next != nil && next.After(time.Now()) {
		v := next.Format(dateLayout)
		nextPeriod = &v
		if reminderOn && time.Until(*next) < time.Duration(leadDays)*24*time.Hour {
			reminder = "Your next period may be approaching. This is a non-medical estimate."
		}
	}
	return
}

// periodModel validates the input DTO and converts it to the persistence model.
func periodModel(subject string, in dto.PeriodInput) (model.Period, error) {
	start, err := time.Parse(dateLayout, in.StartedOn)
	if err != nil {
		return model.Period{}, errors.New("started_on must be YYYY-MM-DD")
	}
	var end *time.Time
	if in.EndedOn != "" {
		v, err := time.Parse(dateLayout, in.EndedOn)
		if err != nil || v.Before(start) {
			return model.Period{}, errors.New("ended_on must be on or after started_on")
		}
		end = &v
	}
	if in.Flow == "" {
		in.Flow = model.FlowUnknown
	}
	if !validFlowLevels[in.Flow] {
		return model.Period{}, fmt.Errorf("flow must be one of %s", strings.Join(model.FlowLevels, ", "))
	}
	days, err := flowDayModels(start, end, in.Days)
	if err != nil {
		return model.Period{}, err
	}
	return model.Period{Subject: subject, StartedOn: start, EndedOn: end, Flow: in.Flow, Days: days, Notes: strings.TrimSpace(in.Notes)}, nil
}

// flowDayModels converts the per-day flow entries, rejecting a day outside the
// period it belongs to. An open-ended period accepts any day from its start
// onwards, since it has no upper bound yet.
func flowDayModels(start time.Time, end *time.Time, in []dto.FlowDay) ([]model.FlowDay, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]model.FlowDay, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, d := range in {
		day, err := time.Parse(dateLayout, d.Date)
		if err != nil {
			return nil, fmt.Errorf("days: date %q must be YYYY-MM-DD", d.Date)
		}
		if day.Before(start) || (end != nil && day.After(*end)) {
			return nil, fmt.Errorf("days: %s is outside the period %s..%s", d.Date, start.Format(dateLayout), endOf(end))
		}
		if seen[d.Date] {
			return nil, fmt.Errorf("days: %s appears twice", d.Date)
		}
		seen[d.Date] = true
		flow := d.Flow
		if flow == "" {
			flow = model.FlowUnknown
		}
		if !validFlowLevels[flow] {
			return nil, fmt.Errorf("days: %s has flow %q, want one of %s", d.Date, d.Flow, strings.Join(model.FlowLevels, ", "))
		}
		out = append(out, model.FlowDay{Date: day, Flow: flow})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

func endOf(end *time.Time) string {
	if end == nil {
		return "ongoing"
	}
	return end.Format(dateLayout)
}

func periodDTO(p model.Period) dto.Period {
	out := dto.Period{ID: p.ID, StartedOn: p.StartedOn.Format(dateLayout), Flow: p.Flow, Notes: p.Notes}
	if p.EndedOn != nil {
		v := p.EndedOn.Format(dateLayout)
		out.EndedOn = &v
	}
	for _, d := range p.Days {
		out.Days = append(out.Days, dto.FlowDay{Date: d.Date.Format(dateLayout), Flow: d.Flow})
	}
	return out
}

func symptomDTO(s model.Symptom) dto.Symptom {
	return dto.Symptom{ID: s.ID, RecordedOn: s.RecordedOn.Format(dateLayout), Kind: s.Kind, Severity: s.Severity, Notes: s.Notes}
}
