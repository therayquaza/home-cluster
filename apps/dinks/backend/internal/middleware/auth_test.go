package middleware_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"dinks/internal/logging"
	"dinks/internal/middleware"
)

const testSubject = "keycloak-subject-1234"

// fakeSessions stands in for the scs session manager.
type fakeSessions struct {
	values map[string]string
}

func (f fakeSessions) GetString(_ context.Context, key string) string { return f.values[key] }

func withLog(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	// Swap stdout before Init: the handler binds os.Stdout when it is built.
	orig := os.Stdout
	os.Stdout = w
	logging.Init()
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()
	fn()
	os.Stdout = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

func TestAuthRejectsRequestWithoutSession(t *testing.T) {
	auth := middleware.NewAuth(fakeSessions{}, nil, "dinks-mobile", nil)
	called := false
	h := auth.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rec := httptest.NewRecorder()
	withLog(t, func() {
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
	})

	if called {
		t.Error("handler ran without a session")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "sign in required") {
		t.Errorf("body = %q, want a sign-in error", body)
	}
}

func TestAuthAcceptsSessionAndExposesSubject(t *testing.T) {
	auth := middleware.NewAuth(fakeSessions{values: map[string]string{"subject": testSubject}}, nil, "dinks-mobile", nil)

	var got string
	h := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.Subject(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got != testSubject {
		t.Errorf("Subject() = %q, want %q", got, testSubject)
	}
}

func TestAuthIgnoresNonBearerAuthorizationHeader(t *testing.T) {
	// A "Basic" or empty-scheme header must fall through to the session check
	// rather than being treated as a bearer token.
	auth := middleware.NewAuth(fakeSessions{values: map[string]string{"subject": testSubject}}, nil, "dinks-mobile", nil)

	var got string
	h := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.Subject(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want the session path to be used (204)", rec.Code)
	}
	if got != testSubject {
		t.Errorf("Subject() = %q, want the session subject %q", got, testSubject)
	}
}

func TestAuthRejectsInvalidBearerToken(t *testing.T) {
	var upsertCalled bool
	auth := middleware.NewAuth(
		fakeSessions{},
		nil,
		"dinks-mobile",
		func(*http.Request, string, string) error { upsertCalled = true; return nil },
	)
	called := false
	h := auth.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()

	out := withLog(t, func() {
		h.ServeHTTP(rec, req)
	})

	if called {
		t.Error("handler ran with an invalid bearer token")
	}
	if upsertCalled {
		t.Error("member must not be upserted for a rejected token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(out, "bearer auth rejected") {
		t.Errorf("rejection was not logged: %q", out)
	}
}

func TestSubjectIsEmptyWithoutAuth(t *testing.T) {
	if got := middleware.Subject(context.Background()); got != "" {
		t.Errorf("Subject on a bare context = %q, want empty", got)
	}
}

func TestWithSubjectRoundTrips(t *testing.T) {
	ctx := middleware.WithSubject(context.Background(), "abc")
	if got := middleware.Subject(ctx); got != "abc" {
		t.Errorf("Subject = %q, want abc", got)
	}
}
