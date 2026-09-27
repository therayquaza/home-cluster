package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/logging"
	"dinks/internal/middleware"
	"dinks/internal/repository"
)

// devSubject is the fixed subject used when DEV_AUTH bypasses OIDC.
const devSubject = "local-dev-user"

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if h.devAuth {
		_ = h.repo.UpsertMember(r.Context(), devSubject, "Local developer")
		_ = h.sessions.RenewToken(r.Context())
		h.sessions.Put(r.Context(), sessionSubject, devSubject)
		logging.Log.Info("login ok", "subject", devSubject, "method", "dev_auth")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	state := random()
	verifier := random()
	h.sessions.Put(r.Context(), sessionState, state)
	h.sessions.Put(r.Context(), sessionPKCE, verifier)
	logging.Log.Info("login started", "client_id", h.oauth.ClientID)
	http.Redirect(w, r, h.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// The provider reports refusals and misconfiguration here instead of a code.
	if e := q.Get("error"); e != "" {
		logging.Log.Error("oidc callback: provider returned an error", "error", e, "description", q.Get("error_description"))
		httpx.Problem(r, w, http.StatusUnauthorized, "authentication failed", nil)
		return
	}
	if q.Get("code") == "" {
		logging.Log.Error("oidc callback: no authorization code in query", "query_keys", queryKeys(q))
		httpx.Problem(r, w, http.StatusBadRequest, "invalid login state", nil)
		return
	}
	if h.sessions.GetString(r.Context(), sessionState) != q.Get("state") {
		logging.Log.Error("oidc callback: state mismatch", "hint", "session cookie lost, or a stale/duplicate tab")
		httpx.Problem(r, w, http.StatusBadRequest, "invalid login state", nil)
		return
	}

	verifier := h.sessions.GetString(r.Context(), sessionPKCE)
	h.sessions.Remove(r.Context(), sessionState)
	h.sessions.Remove(r.Context(), sessionPKCE)

	tok, err := h.oauth.Exchange(r.Context(), q.Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		// invalid_client here almost always means OIDC_CLIENT_SECRET is empty or
		// does not match the client registered for cfg.OIDCClientID.
		logging.Log.Error("oidc callback: token exchange failed", "client_id", h.oauth.ClientID,
			"hint", "invalid_client means OIDC_CLIENT_SECRET is empty or does not match the registered client", "err", err)
		httpx.Problem(r, w, http.StatusUnauthorized, "authentication failed", err)
		return
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		logging.Log.Error("oidc callback: token response carried no id_token", "scopes", h.oauth.Scopes)
		httpx.Problem(r, w, http.StatusUnauthorized, "authentication failed", nil)
		return
	}
	id, err := h.verifier.Verify(r.Context(), raw)
	if err != nil {
		logging.Log.Error("oidc callback: id_token verification failed", "audience", h.webAudience, "err", err)
		httpx.Problem(r, w, http.StatusUnauthorized, "authentication failed", err)
		return
	}

	name := profileName(id)
	if err := h.repo.UpsertMember(r.Context(), id.Subject, name); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to sign in", err)
		return
	}
	if err := h.sessions.RenewToken(r.Context()); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to sign in", err)
		return
	}
	h.sessions.Put(r.Context(), sessionSubject, id.Subject)
	logging.Log.Info("login ok", "subject", id.Subject, "display_name", name, "method", "oidc")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// profileName picks the best available display name from the token claims.
func profileName(id *oidc.IDToken) string {
	var c struct {
		Name      string `json:"name"`
		Preferred string `json:"preferred_username"`
		Email     string `json:"email"`
	}
	if err := id.Claims(&c); err != nil {
		logging.Log.Warn("oidc callback: could not read profile claims", "subject", id.Subject, "err", err)
	}
	for _, v := range []string{c.Name, c.Preferred, c.Email} {
		if v != "" {
			return v
		}
	}
	return fallbackName
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if s := h.sessions.GetString(r.Context(), sessionSubject); s != "" {
		logging.Log.Info("logout", "subject", s)
	} else {
		logging.Log.Warn("logout with no active session")
	}
	_ = h.sessions.Destroy(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var in dto.RegisterInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	name := strings.TrimSpace(in.DisplayName)
	if !strings.Contains(email, "@") || len(in.Password) < 8 {
		httpx.Problem(r, w, http.StatusBadRequest, "valid email and a password of at least 8 characters are required", nil)
		return
	}
	if name == "" {
		name = email
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to register", err)
		return
	}
	subject := "local:" + email
	err = h.repo.CreateMemberWithPassword(r.Context(), subject, email, name, string(hash))
	if errors.Is(err, repository.ErrConflict) {
		httpx.Problem(r, w, http.StatusConflict, "an account with that email already exists", err)
		return
	}
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to register", err)
		return
	}
	if err := h.sessions.RenewToken(r.Context()); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to register", err)
		return
	}
	h.sessions.Put(r.Context(), sessionSubject, subject)
	logging.Log.Info("account registered", "subject", subject)
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) LoginPassword(w http.ResponseWriter, r *http.Request) {
	var in dto.LoginInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	member, err := h.repo.FindMemberByEmail(r.Context(), email)
	if errors.Is(err, repository.ErrNotFound) {
		logging.Log.Warn("password login failed", "reason", "no such account", "email", email)
		httpx.Problem(r, w, http.StatusUnauthorized, "invalid email or password", err)
		return
	}
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to sign in", err)
		return
	}
	if member.PasswordHash == "" {
		logging.Log.Warn("password login failed", "reason", "account has no password set", "email", email,
			"hint", "account was probably created through OIDC")
		httpx.Problem(r, w, http.StatusUnauthorized, "invalid email or password", nil)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(member.PasswordHash), []byte(in.Password)) != nil {
		logging.Log.Warn("password login failed", "reason", "wrong password", "email", email)
		httpx.Problem(r, w, http.StatusUnauthorized, "invalid email or password", nil)
		return
	}
	if err := h.sessions.RenewToken(r.Context()); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to sign in", err)
		return
	}
	h.sessions.Put(r.Context(), sessionSubject, member.Subject)
	logging.Log.Info("login ok", "subject", member.Subject, "method", "password")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	member, err := h.repo.GetMember(r.Context(), middleware.Subject(r.Context()))
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load account", err)
		return
	}
	httpx.JSON(w, http.StatusOK, dto.Me{DisplayName: member.DisplayName})
}

// queryKeys lists the parameter names present on a callback so a malformed or
// unexpected redirect can be diagnosed without logging their values, which
// include the authorization code.
func queryKeys(q url.Values) []string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
