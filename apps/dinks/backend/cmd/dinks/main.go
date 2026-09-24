package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"dinks/internal/domain"
	"dinks/internal/dto"
	"dinks/internal/model"
	"dinks/internal/repository"
	"dinks/internal/service"
)

type config struct {
	MongoURI, MongoDB, Port, BaseURL, OIDCIssuer, OIDCClientID, OIDCMobileClientID, OIDCClientSecret, OIDCRedirectURL string
	DevAuth                                                                                                           bool
}
type app struct {
	repo                     *repository.Cycle
	statsService             *service.Stats
	sessions                 *scs.SessionManager
	oauth                    *oauth2.Config
	verifier, mobileVerifier *oidc.IDTokenVerifier
	devAuth                  bool
}
type subjectKey struct{}

func main() {
	cfg := loadConfig()
	ctx := context.Background()
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}
	repo := repository.New(client, cfg.MongoDB)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	sessions := scs.New()
	sessions.Store = repository.NewSessionStore(client, cfg.MongoDB)
	sessions.Lifetime = 7 * 24 * time.Hour
	sessions.Cookie.Name = "dinks_session"
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.Secure = strings.HasPrefix(cfg.BaseURL, "https://")
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	a := &app{repo: repo, statsService: service.NewStats(), sessions: sessions, devAuth: cfg.DevAuth}
	if !cfg.DevAuth {
		provider, err := oidc.NewProvider(context.Background(), cfg.OIDCIssuer)
		if err != nil {
			log.Fatal(err)
		}
		a.oauth = &oauth2.Config{ClientID: cfg.OIDCClientID, ClientSecret: cfg.OIDCClientSecret, RedirectURL: cfg.OIDCRedirectURL, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}
		a.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
		a.mobileVerifier = provider.Verifier(&oidc.Config{ClientID: cfg.OIDCMobileClientID})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /auth/login", a.login)
	mux.HandleFunc("GET /auth/callback", a.callback)
	mux.HandleFunc("POST /auth/logout", a.logout)
	mux.HandleFunc("POST /auth/register", a.register)
	mux.HandleFunc("POST /auth/login", a.loginPassword)
	mux.Handle("GET /api/me", a.auth(http.HandlerFunc(a.me)))
	mux.Handle("GET /api/dashboard", a.auth(http.HandlerFunc(a.dashboard)))
	mux.Handle("GET /api/prediction", a.auth(http.HandlerFunc(a.prediction)))
	mux.Handle("GET /api/stats", a.auth(http.HandlerFunc(a.stats)))
	mux.Handle("POST /api/stats/query", a.auth(http.HandlerFunc(a.statsQuery)))
	mux.Handle("POST /api/periods", a.auth(http.HandlerFunc(a.createPeriod)))
	mux.Handle("PATCH /api/periods/{id}", a.auth(http.HandlerFunc(a.updatePeriod)))
	mux.Handle("POST /api/symptoms", a.auth(http.HandlerFunc(a.createSymptom)))
	mux.Handle("PATCH /api/symptoms/{id}", a.auth(http.HandlerFunc(a.updateSymptom)))
	mux.Handle("DELETE /api/symptoms/{id}", a.auth(http.HandlerFunc(a.deleteSymptom)))
	mux.Handle("GET /api/export", a.auth(http.HandlerFunc(a.export)))
	mux.Handle("DELETE /api/me", a.auth(http.HandlerFunc(a.deleteMe)))
	mux.Handle("POST /api/partners/invite", a.auth(http.HandlerFunc(a.createInvite)))
	mux.Handle("POST /api/partners/link", a.auth(http.HandlerFunc(a.redeemInvite)))
	mux.Handle("GET /api/partners", a.auth(http.HandlerFunc(a.listPartners)))
	mux.Handle("DELETE /api/partners/{subject}", a.auth(http.HandlerFunc(a.revokePartner)))
	mux.Handle("GET /api/partners/status", a.auth(http.HandlerFunc(a.partnerStatuses)))
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: a.sessions.LoadAndSave(headers(mux)), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("dinks backend listening on :%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
func loadConfig() config {
	get := func(k, d string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return d
	}
	return config{MongoURI: get("MONGO_URI", "mongodb://dinks:dinks@localhost:27017/dinks?replicaSet=rs0"), MongoDB: get("MONGO_DB", "dinks"), Port: get("PORT", "8080"), BaseURL: get("BASE_URL", "http://localhost:8080"), OIDCIssuer: os.Getenv("OIDC_ISSUER"), OIDCClientID: os.Getenv("OIDC_CLIENT_ID"), OIDCMobileClientID: get("OIDC_MOBILE_CLIENT_ID", "dinks-mobile"), OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"), OIDCRedirectURL: os.Getenv("OIDC_REDIRECT_URL"), DevAuth: get("DEV_AUTH", "") == "true"}
}
func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if a.devAuth {
		_ = a.repo.UpsertMember(r.Context(), "local-dev-user", "Local developer")
		_ = a.sessions.RenewToken(r.Context())
		a.sessions.Put(r.Context(), "subject", "local-dev-user")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	state := random()
	verifier := random()
	a.sessions.Put(r.Context(), "oidc_state", state)
	a.sessions.Put(r.Context(), "oidc_verifier", verifier)
	http.Redirect(w, r, a.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}
func (a *app) callback(w http.ResponseWriter, r *http.Request) {
	if a.sessions.GetString(r.Context(), "oidc_state") != r.URL.Query().Get("state") {
		problem(w, 400, "invalid login state")
		return
	}
	verifier := a.sessions.GetString(r.Context(), "oidc_verifier")
	a.sessions.Remove(r.Context(), "oidc_state")
	a.sessions.Remove(r.Context(), "oidc_verifier")
	tok, err := a.oauth.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		problem(w, 401, "authentication failed")
		return
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		problem(w, 401, "authentication failed")
		return
	}
	id, err := a.verifier.Verify(r.Context(), raw)
	if err != nil {
		problem(w, 401, "authentication failed")
		return
	}
	var c struct {
		Name      string `json:"name"`
		Preferred string `json:"preferred_username"`
		Email     string `json:"email"`
	}
	_ = id.Claims(&c)
	name := c.Name
	if name == "" {
		name = c.Preferred
	}
	if name == "" {
		name = c.Email
	}
	if name == "" {
		name = "Member"
	}
	if err := a.repo.UpsertMember(r.Context(), id.Subject, name); err != nil {
		problem(w, 500, "unable to sign in")
		return
	}
	if err := a.sessions.RenewToken(r.Context()); err != nil {
		problem(w, 500, "unable to sign in")
		return
	}
	a.sessions.Put(r.Context(), "subject", id.Subject)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (a *app) register(w http.ResponseWriter, r *http.Request) {
	var in dto.RegisterInput
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	name := strings.TrimSpace(in.DisplayName)
	if !strings.Contains(email, "@") || len(in.Password) < 8 {
		problem(w, 400, "valid email and a password of at least 8 characters are required")
		return
	}
	if name == "" {
		name = email
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		problem(w, 500, "unable to register")
		return
	}
	subject := "local:" + email
	if err := a.repo.CreateMemberWithPassword(r.Context(), subject, email, name, string(hash)); errors.Is(err, repository.ErrConflict) {
		problem(w, 409, "an account with that email already exists")
		return
	} else if err != nil {
		problem(w, 500, "unable to register")
		return
	}
	if err := a.sessions.RenewToken(r.Context()); err != nil {
		problem(w, 500, "unable to register")
		return
	}
	a.sessions.Put(r.Context(), "subject", subject)
	w.WriteHeader(201)
}
func (a *app) loginPassword(w http.ResponseWriter, r *http.Request) {
	var in dto.LoginInput
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	member, err := a.repo.FindMemberByEmail(r.Context(), email)
	if errors.Is(err, repository.ErrNotFound) {
		problem(w, 401, "invalid email or password")
		return
	} else if err != nil {
		problem(w, 500, "unable to sign in")
		return
	}
	if member.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(member.PasswordHash), []byte(in.Password)) != nil {
		problem(w, 401, "invalid email or password")
		return
	}
	if err := a.sessions.RenewToken(r.Context()); err != nil {
		problem(w, 500, "unable to sign in")
		return
	}
	a.sessions.Put(r.Context(), "subject", member.Subject)
	w.WriteHeader(204)
}
func (a *app) me(w http.ResponseWriter, r *http.Request) {
	member, err := a.repo.GetMember(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load account")
		return
	}
	jsonOut(w, 200, dto.Me{DisplayName: member.DisplayName})
}
func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	_ = a.sessions.Destroy(r.Context())
	w.WriteHeader(204)
}
func (a *app) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); token != "" && token != r.Header.Get("Authorization") {
			id, err := a.mobileVerifier.Verify(r.Context(), token)
			if err == nil && id.Subject != "" {
				var c struct {
					Name      string `json:"name"`
					Preferred string `json:"preferred_username"`
				}
				_ = id.Claims(&c)
				name := c.Name
				if name == "" {
					name = c.Preferred
				}
				if name == "" {
					name = "Member"
				}
				if a.repo.UpsertMember(r.Context(), id.Subject, name) == nil {
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), subjectKey{}, id.Subject)))
					return
				}
			}
			problem(w, 401, "sign in required")
			return
		}
		sub := a.sessions.GetString(r.Context(), "subject")
		if sub == "" {
			problem(w, 401, "sign in required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), subjectKey{}, sub)))
	})
}
func sub(r *http.Request) string { return r.Context().Value(subjectKey{}).(string) }
func (a *app) dashboard(w http.ResponseWriter, r *http.Request) {
	ps, err := a.repo.Periods(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load records")
		return
	}
	ss, err := a.repo.Symptoms(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load records")
		return
	}
	out := dto.Dashboard{Periods: make([]dto.Period, 0, len(ps)), Symptoms: make([]dto.Symptom, 0, len(ss))}
	for _, p := range ps {
		out.Periods = append(out.Periods, periodDTO(p))
	}
	for _, s := range ss {
		out.Symptoms = append(out.Symptoms, symptomDTO(s))
	}
	_, out.NextPeriod, out.Reminder = cycleStatus(ps)
	jsonOut(w, 200, out)
}

// cycleStatus derives on-period/next-period/reminder from a subject's periods —
// shared by the owner's own dashboard and the status-only view a partner sees.
func cycleStatus(ps []model.Period) (onPeriod bool, nextPeriod *string, reminder string) {
	for _, p := range ps {
		if p.EndedOn == nil {
			onPeriod = true
			break
		}
	}
	domainPeriods := make([]domain.Period, 0, len(ps))
	for _, p := range ps {
		domainPeriods = append(domainPeriods, domain.Period{StartedOn: p.StartedOn.Format("2006-01-02")})
	}
	if next := domain.EstimateNextPeriod(domainPeriods); next != nil && next.After(time.Now()) {
		v := next.Format("2006-01-02")
		nextPeriod = &v
		if next.Sub(time.Now()) < 7*24*time.Hour {
			reminder = "Your next period may be approaching. This is a non-medical estimate."
		}
	}
	return
}
func (a *app) prediction(w http.ResponseWriter, r *http.Request) {
	ps, err := a.repo.Periods(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load prediction")
		return
	}
	_, next, _ := cycleStatus(ps)
	jsonOut(w, 200, map[string]any{"predicted_period_start": next, "method": "average of recent plausible cycle lengths", "disclaimer": "Not medical advice."})
}

// stats accepts only allowlisted metrics (not raw user queries) so frontend-customizable
// dashboards cannot execute arbitrary database operations.
func (a *app) stats(w http.ResponseWriter, r *http.Request) {
	ps, err := a.repo.Periods(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load statistics")
		return
	}
	ss, err := a.repo.Symptoms(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load statistics")
		return
	}
	lengths := []int{}
	for i := 0; i < len(ps)-1; i++ {
		d := int(ps[i].StartedOn.Sub(ps[i+1].StartedOn).Hours() / 24)
		if d >= 15 && d <= 60 {
			lengths = append(lengths, d)
		}
	}
	avg := 0
	for _, d := range lengths {
		avg += d
	}
	if len(lengths) > 0 {
		avg /= len(lengths)
	}
	bySymptom := map[string]int{}
	for _, s := range ss {
		bySymptom[s.Kind]++
	}
	metric := r.URL.Query().Get("metric")
	payload := map[string]any{"period_count": len(ps), "checkin_count": len(ss), "average_cycle_days": avg, "symptom_counts": bySymptom, "cycles_sampled": len(lengths)}
	if metric != "" {
		if _, ok := payload[metric]; !ok {
			problem(w, 400, "unsupported metric")
			return
		}
		jsonOut(w, 200, map[string]any{"metric": metric, "value": payload[metric]})
		return
	}
	jsonOut(w, 200, payload)
}

// statsQuery is the frontend-customizable "advanced stats" endpoint: users compose
// a query from allowlisted metrics/group_by (service.AllowedMetrics/AllowedGroupBy)
// instead of sending a raw database query.
func (a *app) statsQuery(w http.ResponseWriter, r *http.Request) {
	var in dto.StatsQuery
	if !decode(w, r, &in) {
		return
	}
	ps, err := a.repo.Periods(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load statistics")
		return
	}
	ss, err := a.repo.Symptoms(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load statistics")
		return
	}
	out, err := a.statsService.Query(ps, ss, in)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, out)
}

func (a *app) createPeriod(w http.ResponseWriter, r *http.Request) {
	var in dto.PeriodInput
	if !decode(w, r, &in) {
		return
	}
	p, err := periodModel(sub(r), in)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	if err = a.repo.CreatePeriod(r.Context(), &p); err != nil {
		problem(w, 500, "unable to save period")
		return
	}
	w.WriteHeader(201)
}
func (a *app) updatePeriod(w http.ResponseWriter, r *http.Request) {
	var in dto.PeriodInput
	if !decode(w, r, &in) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		problem(w, 400, "invalid period id")
		return
	}
	p, err := periodModel(sub(r), in)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	if err = a.repo.UpdatePeriod(r.Context(), sub(r), id, p); errors.Is(err, repository.ErrNotFound) {
		problem(w, 404, "period not found")
	} else if err != nil {
		problem(w, 500, "unable to update period")
	} else {
		w.WriteHeader(204)
	}
}
func (a *app) createSymptom(w http.ResponseWriter, r *http.Request) {
	var in dto.SymptomInput
	if !decode(w, r, &in) {
		return
	}
	day, err := time.Parse("2006-01-02", in.RecordedOn)
	if err != nil || strings.TrimSpace(in.Kind) == "" || in.Severity < 1 || in.Severity > 5 {
		problem(w, 400, "recorded_on, kind, and severity (1-5) are required")
		return
	}
	if err = a.repo.CreateSymptom(r.Context(), &model.Symptom{Subject: sub(r), RecordedOn: day, Kind: strings.TrimSpace(in.Kind), Severity: in.Severity, Notes: strings.TrimSpace(in.Notes)}); err != nil {
		problem(w, 500, "unable to save symptom")
		return
	}
	w.WriteHeader(201)
}
func (a *app) updateSymptom(w http.ResponseWriter, r *http.Request) {
	var in dto.SymptomInput
	if !decode(w, r, &in) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		problem(w, 400, "invalid symptom id")
		return
	}
	day, err := time.Parse("2006-01-02", in.RecordedOn)
	if err != nil || strings.TrimSpace(in.Kind) == "" || in.Severity < 1 || in.Severity > 5 {
		problem(w, 400, "recorded_on, kind, and severity (1-5) are required")
		return
	}
	s := model.Symptom{RecordedOn: day, Kind: strings.TrimSpace(in.Kind), Severity: in.Severity, Notes: strings.TrimSpace(in.Notes)}
	if err = a.repo.UpdateSymptom(r.Context(), sub(r), id, s); errors.Is(err, repository.ErrNotFound) {
		problem(w, 404, "symptom not found")
	} else if err != nil {
		problem(w, 500, "unable to update symptom")
	} else {
		w.WriteHeader(204)
	}
}
func (a *app) deleteSymptom(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		problem(w, 400, "invalid symptom id")
		return
	}
	if err = a.repo.DeleteSymptom(r.Context(), sub(r), id); errors.Is(err, repository.ErrNotFound) {
		problem(w, 404, "symptom not found")
	} else if err != nil {
		problem(w, 500, "unable to delete symptom")
	} else {
		w.WriteHeader(204)
	}
}
func (a *app) createInvite(w http.ResponseWriter, r *http.Request) {
	inv, err := a.repo.CreateInvite(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to create invite")
		return
	}
	jsonOut(w, 201, dto.Invite{Code: inv.Code, ExpiresAt: inv.ExpiresAt.Format(time.RFC3339)})
}
func (a *app) redeemInvite(w http.ResponseWriter, r *http.Request) {
	var in dto.RedeemInput
	if !decode(w, r, &in) {
		return
	}
	_, err := a.repo.RedeemInvite(r.Context(), strings.TrimSpace(in.Code), sub(r))
	if errors.Is(err, repository.ErrNotFound) {
		problem(w, 404, "invite code not found or expired")
		return
	}
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	w.WriteHeader(204)
}
func (a *app) listPartners(w http.ResponseWriter, r *http.Request) {
	links, err := a.repo.PartnersOf(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load partners")
		return
	}
	out := make([]dto.Partner, 0, len(links))
	for _, l := range links {
		member, err := a.repo.GetMember(r.Context(), l.PartnerSubject)
		if err != nil {
			continue
		}
		out = append(out, dto.Partner{Subject: l.PartnerSubject, DisplayName: member.DisplayName, LinkedAt: l.CreatedAt.Format(time.RFC3339)})
	}
	jsonOut(w, 200, out)
}
func (a *app) revokePartner(w http.ResponseWriter, r *http.Request) {
	if err := a.repo.RevokePartner(r.Context(), sub(r), r.PathValue("subject")); errors.Is(err, repository.ErrNotFound) {
		problem(w, 404, "partner not found")
	} else if err != nil {
		problem(w, 500, "unable to revoke partner")
	} else {
		w.WriteHeader(204)
	}
}
func (a *app) partnerStatuses(w http.ResponseWriter, r *http.Request) {
	links, err := a.repo.SharedWithMe(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to load shared status")
		return
	}
	out := make([]dto.PartnerStatus, 0, len(links))
	for _, l := range links {
		member, err := a.repo.GetMember(r.Context(), l.OwnerSubject)
		if err != nil {
			continue
		}
		ps, err := a.repo.Periods(r.Context(), l.OwnerSubject)
		if err != nil {
			continue
		}
		onPeriod, nextPeriod, reminder := cycleStatus(ps)
		out = append(out, dto.PartnerStatus{Subject: l.OwnerSubject, DisplayName: member.DisplayName, OnPeriod: onPeriod, NextPeriod: nextPeriod, Reminder: reminder})
	}
	jsonOut(w, 200, out)
}
func (a *app) export(w http.ResponseWriter, r *http.Request) {
	ps, err := a.repo.Periods(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to export")
		return
	}
	ss, err := a.repo.Symptoms(r.Context(), sub(r))
	if err != nil {
		problem(w, 500, "unable to export")
		return
	}
	periods := make([]dto.Period, 0, len(ps))
	symptoms := make([]dto.Symptom, 0, len(ss))
	for _, p := range ps {
		periods = append(periods, periodDTO(p))
	}
	for _, s := range ss {
		symptoms = append(symptoms, symptomDTO(s))
	}
	w.Header().Set("Content-Disposition", `attachment; filename="dinks-export.json"`)
	jsonOut(w, 200, map[string]any{"periods": periods, "symptoms": symptoms, "exported_at": time.Now().UTC()})
}
func (a *app) deleteMe(w http.ResponseWriter, r *http.Request) {
	if err := a.repo.DeleteMember(r.Context(), sub(r)); err != nil {
		problem(w, 500, "unable to delete data")
		return
	}
	_ = a.sessions.Destroy(r.Context())
	w.WriteHeader(204)
}
func periodModel(subject string, in dto.PeriodInput) (model.Period, error) {
	start, err := time.Parse("2006-01-02", in.StartedOn)
	if err != nil {
		return model.Period{}, errors.New("started_on must be YYYY-MM-DD")
	}
	var end *time.Time
	if in.EndedOn != "" {
		v, err := time.Parse("2006-01-02", in.EndedOn)
		if err != nil || v.Before(start) {
			return model.Period{}, errors.New("ended_on must be on or after started_on")
		}
		end = &v
	}
	if in.Flow == "" {
		in.Flow = "unknown"
	}
	return model.Period{Subject: subject, StartedOn: start, EndedOn: end, Flow: in.Flow, Notes: strings.TrimSpace(in.Notes)}, nil
}
func periodDTO(p model.Period) dto.Period {
	out := dto.Period{ID: p.ID, StartedOn: p.StartedOn.Format("2006-01-02"), Flow: p.Flow, Notes: p.Notes}
	if p.EndedOn != nil {
		v := p.EndedOn.Format("2006-01-02")
		out.EndedOn = &v
	}
	return out
}
func symptomDTO(s model.Symptom) dto.Symptom {
	return dto.Symptom{ID: s.ID, RecordedOn: s.RecordedOn.Format("2006-01-02"), Kind: s.Kind, Severity: s.Severity, Notes: s.Notes}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		problem(w, 400, "invalid request body")
		return false
	}
	return true
}
func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, dto.Problem{Error: msg})
}
func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
