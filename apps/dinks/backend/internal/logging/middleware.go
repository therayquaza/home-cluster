package logging

import (
	"net/http"
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

// Headers applies the baseline security headers to every response. Responses
// carry personal health data, so nothing is cacheable.
func Headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
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
