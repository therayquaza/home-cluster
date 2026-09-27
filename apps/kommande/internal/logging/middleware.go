package logging

import (
	"net/http"
	"runtime/debug"
	"time"
)

// RequestLog records one line per request: method, path, status, size and
// duration. The query string is deliberately omitted — /auth/callback carries
// the OIDC authorization code there. Probe traffic on /healthz is dropped so it
// cannot drown the log.
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			Log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
				"bytes", rec.written, "duration", time.Since(start).Round(time.Millisecond).String())
		}()
		next.ServeHTTP(rec, r)
	})
}

// RecoverPanic keeps one bad request from taking the process down and records
// the stack before the connection is torn down.
func RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				Log.Error("panic recovered", "method", r.Method, "path", r.URL.Path,
					"panic", v, "stack", string(debug.Stack()))
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Headers applies the baseline security headers to every response.
func Headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status  int
	written int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	n, err := s.ResponseWriter.Write(b)
	s.written += n
	return n, err
}
