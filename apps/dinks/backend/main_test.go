package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"dinks/internal/config"
	"dinks/internal/handlers"
	"dinks/internal/logging"
	"dinks/internal/middleware"
	"dinks/internal/repository"
	"dinks/internal/service"
)

// newTestServer builds the real route table in dev-auth mode. The mongo client
// is created but never dialled, so no database is required: Connect performs no
// I/O. A short server-selection timeout keeps the one dev-auth path that does
// touch the database (the login upsert) from stalling the test.
func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	logging.Init()

	client, err := mongo.Connect(options.Client().
		ApplyURI("mongodb://127.0.0.1:27017").
		SetServerSelectionTimeout(100 * time.Millisecond))
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(t.Context()) })

	cfg := config.Config{DevAuth: true, MongoDB: "dinks"}
	sessions := scs.New()
	repo := repository.New(client, cfg.MongoDB)
	h, err := handlers.New(repo, service.NewStats(), sessions, cfg)
	if err != nil {
		t.Fatalf("handlers.New: %v", err)
	}
	auth := middleware.NewAuth(sessions, h.MobileVerifier(), cfg.OIDCMobileClientID,
		func(*http.Request, string, string) error { return nil })
	return routes(h, auth, sessions)
}

// TestRoutesRejectsUnauthenticated guards the wiring after the refactor: every
// API route must sit behind the auth middleware.
func TestRoutesRejectsUnauthenticated(t *testing.T) {
	srv := newTestServer(t)

	protected := []struct{ method, path string }{
		{"GET", "/api/me"},
		{"DELETE", "/api/me"},
		{"GET", "/api/dashboard"},
		{"GET", "/api/prediction"},
		{"GET", "/api/export"},
		{"GET", "/api/stats"},
		{"POST", "/api/stats/query"},
		{"POST", "/api/periods"},
		{"PATCH", "/api/periods/1"},
		{"POST", "/api/symptoms"},
		{"PATCH", "/api/symptoms/1"},
		{"DELETE", "/api/symptoms/1"},
		{"POST", "/api/partners/invite"},
		{"POST", "/api/partners/link"},
		{"GET", "/api/partners"},
		{"DELETE", "/api/partners/other-subject"},
		{"GET", "/api/partners/status"},
	}
	for _, tc := range protected {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401 (route must be authenticated)", rec.Code)
			}
		})
	}
}

func TestHealthzIsPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// The login redirect is public, so it must not be blocked by auth. In dev-auth
// mode it signs the caller in and redirects to the app.
func TestLoginRedirectIsNotBlockedByAuth(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
}

// Every response must carry the baseline security headers.
func TestRoutesApplySecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))

	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Cache-Control":          "no-store",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}
}
