// Package middleware provides the request-scoped concerns that wrap handlers:
// authentication and the subject it puts in the request context.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"dinks/internal/httpx"
	"dinks/internal/logging"
)

type subjectKey struct{}

// bearerPrefix marks a mobile API token; the web UI uses a session cookie.
const bearerPrefix = "Bearer "

// Auth accepts either a mobile bearer token or a web session, and puts the
// authenticated subject in the request context for Subject to read.
type Auth struct {
	sessions       SessionGetter
	mobileVerifier *oidc.IDTokenVerifier
	mobileAudience string
	// upsertDisplayName is called with the subject and a display name derived
	// from the bearer token, so a mobile-only user exists before its first query.
	upsertDisplayName func(r *http.Request, subject, name string) error
}

// SessionGetter is the part of the session manager this package needs.
type SessionGetter interface {
	GetString(ctx context.Context, key string) string
}

// NewAuth wires the authenticator.
func NewAuth(sessions SessionGetter, verifier *oidc.IDTokenVerifier, mobileAudience string, upsert func(r *http.Request, subject, name string) error) *Auth {
	return &Auth{
		sessions:          sessions,
		mobileVerifier:    verifier,
		mobileAudience:    mobileAudience,
		upsertDisplayName: upsert,
	}
}

// Handler rejects unauthenticated requests and passes an authenticated one
// through with its subject in the context.
func (a *Auth) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := bearerToken(r); ok {
			a.authenticateBearer(w, r, token, next)
			return
		}
		sub := a.sessions.GetString(r.Context(), "subject")
		if sub == "" {
			logging.Log.Warn("session auth rejected: no session subject", "method", r.Method, "path", r.URL.Path)
			httpx.Problem(r, w, http.StatusUnauthorized, "sign in required", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithSubject(r.Context(), sub)))
	})
}

// bearerToken extracts an Authorization: Bearer header, reporting false when
// the header is absent or uses a different scheme.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" || !strings.HasPrefix(header, bearerPrefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, bearerPrefix)
	if token == "" {
		return "", false
	}
	return token, true
}

func (a *Auth) authenticateBearer(w http.ResponseWriter, r *http.Request, token string, next http.Handler) {
	if a.mobileVerifier == nil {
		// Dev-auth builds skip OIDC discovery, so no bearer token can be
		// verified. Reject cleanly rather than dereferencing a nil verifier.
		logging.Log.Warn("bearer auth rejected: no mobile verifier configured",
			"hint", "mobile tokens require OIDC; set OIDC_ISSUER and disable DEV_AUTH")
		httpx.Problem(r, w, http.StatusUnauthorized, "sign in required", nil)
		return
	}
	id, err := a.mobileVerifier.Verify(r.Context(), token)
	if err != nil {
		// A mobile token that fails here is usually issued for the wrong
		// audience or signed by a stale key.
		logging.Log.Warn("bearer auth rejected", "audience", a.mobileAudience,
			"hint", "token was issued for another audience or signed with a stale key", "err", err)
		httpx.Problem(r, w, http.StatusUnauthorized, "sign in required", err)
		return
	}
	if id.Subject == "" {
		logging.Log.Warn("bearer auth rejected: token carried no subject", "audience", a.mobileAudience)
		httpx.Problem(r, w, http.StatusUnauthorized, "sign in required", nil)
		return
	}
	if err := a.upsertDisplayName(r, id.Subject, displayName(id)); err != nil {
		logging.Log.Error("bearer auth: could not upsert member", "subject", id.Subject, "err", err)
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to sign in", err)
		return
	}
	next.ServeHTTP(w, r.WithContext(WithSubject(r.Context(), id.Subject)))
}

// displayName prefers the full name, then the username, then a neutral default.
func displayName(id *oidc.IDToken) string {
	var c struct {
		Name      string `json:"name"`
		Preferred string `json:"preferred_username"`
	}
	if err := id.Claims(&c); err != nil {
		logging.Log.Warn("bearer auth: could not read profile claims", "subject", id.Subject, "err", err)
	}
	for _, v := range []string{c.Name, c.Preferred} {
		if v != "" {
			return v
		}
	}
	return "Member"
}

// WithSubject returns ctx carrying the authenticated subject.
func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

// Subject returns the authenticated subject. It is only valid behind Auth, which
// rejects requests that carry none.
func Subject(ctx context.Context) string {
	s, _ := ctx.Value(subjectKey{}).(string)
	return s
}
