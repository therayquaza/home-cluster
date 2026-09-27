package logging_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"dinks/internal/logging"
)

// capture runs fn with the process logger redirected to a pipe and returns
// everything it wrote. LOG_LEVEL / LOG_FORMAT are read by logging.Init, so call
// t.Setenv before this.
func capture(t *testing.T, fn func()) string {
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

// captureJSON returns the decoded JSON log lines produced by fn.
func captureJSON(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	raw := capture(t, fn)
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(raw), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is not JSON: %q (%v)", l, err)
		}
		lines = append(lines, m)
	}
	return lines
}

func find(lines []map[string]any, msg string) map[string]any {
	for _, l := range lines {
		if l["msg"] == msg {
			return l
		}
	}
	return nil
}

func TestRequestLogCapturesMethodPathStatusBytesDuration(t *testing.T) {
	h := logging.RequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	}))

	lines := captureJSON(t, func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/symptoms", nil))
	})

	req := find(lines, "request")
	if req == nil {
		t.Fatal("no request log line emitted")
	}
	if req["method"] != "POST" {
		t.Errorf("method = %v, want POST", req["method"])
	}
	if req["path"] != "/api/symptoms" {
		t.Errorf("path = %v, want /api/symptoms", req["path"])
	}
	if req["status"] != float64(http.StatusTeapot) {
		t.Errorf("status = %v, want 418", req["status"])
	}
	if req["bytes"] != float64(5) {
		t.Errorf("bytes = %v, want 5", req["bytes"])
	}
	if d, _ := req["duration"].(string); d == "" {
		t.Error("duration missing from request log")
	}
}

func TestRequestLogRecordsImplicit200(t *testing.T) {
	// A handler that never calls WriteHeader must still log 200, not 0.
	h := logging.RequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	lines := captureJSON(t, func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
	})

	req := find(lines, "request")
	if req == nil {
		t.Fatal("no request log line emitted")
	}
	if req["status"] != float64(http.StatusOK) {
		t.Errorf("status = %v, want 200", req["status"])
	}
}

func TestRequestLogExcludesHealthChecks(t *testing.T) {
	h := logging.RequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	lines := captureJSON(t, func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	})

	if find(lines, "request") != nil {
		t.Error("probe traffic on /healthz must not be logged")
	}
}

// The authorization code and OIDC state arrive as query parameters on
// /auth/callback. Logging the raw request would leak both.
func TestRequestLogDoesNotLeakOIDCQuerySecrets(t *testing.T) {
	const code = "super-secret-authorization-code"
	const state = "super-secret-oauth-state"

	h := logging.RequestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	lines := captureJSON(t, func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
			"/auth/callback?code="+code+"&state="+state, nil))
	})

	raw, err := json.Marshal(lines)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{code, state} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("log output contains OIDC secret %q", secret)
		}
	}
	req := find(lines, "request")
	if req == nil {
		t.Fatal("expected /auth/callback to still be logged")
	}
	if req["path"] != "/auth/callback" {
		t.Errorf("path = %v, want /auth/callback", req["path"])
	}
}

func TestLevelFiltering(t *testing.T) {
	for _, tc := range []struct {
		level  string
		absent []string
		kept   string
	}{
		{level: "warn", absent: []string{"debug line", "info line"}, kept: "warn line"},
		{level: "error", absent: []string{"debug line", "info line", "warn line"}, kept: "error line"},
		{level: "debug", absent: nil, kept: "debug line"},
	} {
		t.Run(tc.level, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.level)
			out := capture(t, func() {
				logging.Log.Debug("debug line")
				logging.Log.Info("info line")
				logging.Log.Warn("warn line")
				logging.Log.Error("error line")
			})
			for _, a := range tc.absent {
				if strings.Contains(out, a) {
					t.Errorf("LOG_LEVEL=%s should suppress %q; got %q", tc.level, a, out)
				}
			}
			if !strings.Contains(out, tc.kept) {
				t.Errorf("LOG_LEVEL=%s should keep %q; got %q", tc.level, tc.kept, out)
			}
		})
	}
}

func TestTextFormat(t *testing.T) {
	t.Setenv("LOG_FORMAT", "text")
	out := capture(t, func() {
		logging.Log.Info("text mode", "component", "dinks")
	})
	if !strings.Contains(out, "level=INFO") {
		t.Errorf("text handler missing level=INFO: %q", out)
	}
	if !strings.Contains(out, "component=dinks") {
		t.Errorf("text handler missing key=value: %q", out)
	}
}

func TestHeadersSetsSecurityHeaders(t *testing.T) {
	h := logging.Headers(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))

	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "same-origin",
		"X-Frame-Options":        "DENY",
		// Health data must never be cached by an intermediary.
		"Cache-Control": "no-store",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}
}

func TestRedactURIHidesPassword(t *testing.T) {
	got := logging.RedactURI("mongodb://dinks:supersecret@mongo:27017/dinks?replicaSet=rs0")
	if strings.Contains(got, "supersecret") {
		t.Errorf("password leaked: %s", got)
	}
	if !strings.Contains(got, "mongo:27017") {
		t.Errorf("host lost: %s", got)
	}
	if !strings.Contains(got, "replicaSet=rs0") {
		t.Errorf("options lost: %s", got)
	}
}

func TestRedactURILeavesCredentialFreeURIIntact(t *testing.T) {
	const uri = "mongodb://mongo:27017"
	if got := logging.RedactURI(uri); got != uri {
		t.Errorf("RedactURI(%q) = %q, want unchanged", uri, got)
	}
}

func TestSetNeverRevealsSecret(t *testing.T) {
	if got := logging.Set("hunter2"); got != "<set>" {
		t.Errorf("Set(value) = %q, want <set>", got)
	}
	if got := logging.Set(""); got != "<empty>" {
		t.Errorf("Set(empty) = %q, want <empty>", got)
	}
	if got := logging.Set("   "); got != "<empty>" {
		t.Errorf("Set(blank) = %q, want <empty>", got)
	}
}
