package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dinks/internal/httpx"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response body is not JSON: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func TestProblemReturnsErrorBodyWithoutLeakingCause(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()

	httpx.Problem(req, rec, http.StatusInternalServerError, "unable to load account",
		errTest("connection refused to 10.0.0.5:27017"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := decodeBody(t, rec)
	if body["error"] != "unable to load account" {
		t.Errorf("error = %v, want the safe message", body["error"])
	}
	// The internal cause must never reach the client.
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Errorf("internal detail leaked to client: %s", rec.Body.String())
	}
}

func TestProblemWithNilErrorStillReturnsMessage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/symptoms", nil)
	rec := httptest.NewRecorder()

	httpx.Problem(req, rec, http.StatusBadRequest, "recorded_on, kind, and severity (1-5) are required", nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if body := decodeBody(t, rec); body["error"] != "recorded_on, kind, and severity (1-5) are required" {
		t.Errorf("error = %v", body["error"])
	}
}

func TestDecodeAcceptsValidBody(t *testing.T) {
	var got struct {
		StartedOn string `json:"started_on"`
		Flow      string `json:"flow"`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/periods",
		strings.NewReader(`{"started_on":"2026-01-05","flow":"light"}`))
	rec := httptest.NewRecorder()

	if !httpx.Decode(rec, req, &got) {
		t.Fatal("Decode returned false for a valid body")
	}
	if got.StartedOn != "2026-01-05" || got.Flow != "light" {
		t.Errorf("decoded = %+v", got)
	}
}

func TestDecodeRejectsMalformedBody(t *testing.T) {
	var got struct{}
	req := httptest.NewRequest(http.MethodPost, "/api/periods", strings.NewReader(`{"started_on":`))
	rec := httptest.NewRecorder()

	if httpx.Decode(rec, req, &got) {
		t.Fatal("Decode accepted a malformed body")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if body := decodeBody(t, rec); body["error"] != "invalid request body" {
		t.Errorf("error = %v, want %q", body["error"], "invalid request body")
	}
}

func TestDecodeRejectsOversizedBody(t *testing.T) {
	var got map[string]any
	req := httptest.NewRequest(http.MethodPost, "/api/periods",
		strings.NewReader(`{"notes":"`+strings.Repeat("a", 1<<20)+`"}`))
	rec := httptest.NewRecorder()

	if httpx.Decode(rec, req, &got) {
		t.Fatal("Decode accepted a body beyond the size cap")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRecoverPanicReturnsJSONError(t *testing.T) {
	h := httpx.RecoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if body := decodeBody(t, rec); body["error"] != "internal error" {
		t.Errorf("error = %v, want %q", body["error"], "internal error")
	}
	// The panic value is for the log, not the response.
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("panic value leaked to client: %s", rec.Body.String())
	}
}

func TestRecoverPanicPassesThroughNormalRequests(t *testing.T) {
	h := httpx.RecoverPanic(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/periods", nil))

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if rec.Body.String() != "created" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "created")
	}
}

func TestJSONSetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.JSON(rec, http.StatusOK, map[string]any{"display_name": "Member"})

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if body := decodeBody(t, rec); body["display_name"] != "Member" {
		t.Errorf("body = %v", body)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
