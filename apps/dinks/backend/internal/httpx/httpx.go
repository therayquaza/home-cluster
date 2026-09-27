// Package httpx holds the JSON conventions shared by every API handler: the
// error body, the body decoder, the JSON writer, and panic recovery. Keeping
// them here means the wire format is defined once instead of in every handler.
package httpx

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	"dinks/internal/dto"
	"dinks/internal/logging"
)

// maxBodyBytes caps request bodies. The largest payload is a handful of short
// strings, so 64 KiB is generous and keeps a hostile client from filling memory.
const maxBodyBytes = 64 << 10

// JSON writes v as the response body with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Problem emits the API error body and records the cause server-side. The
// underlying error is never returned to the client, so without this record a
// failure like a rejected OIDC token exchange is indistinguishable from a
// validation error. Pass a nil err for expected client mistakes.
func Problem(r *http.Request, w http.ResponseWriter, status int, msg string, err error) {
	attrs := []any{"status", status, "method", r.Method, "path", r.URL.Path, "msg", msg}
	switch {
	case err != nil:
		logging.Log.Error("request failed", append(attrs, "err", err)...)
	case status >= 500:
		logging.Log.Error("request failed", attrs...)
	default:
		logging.Log.Warn("request rejected", attrs...)
	}
	JSON(w, status, dto.Problem{Error: msg})
}

// Decode reads a JSON request body into v, reporting a 400 and returning false
// if the body is malformed or oversized.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		Problem(r, w, http.StatusBadRequest, "invalid request body", err)
		return false
	}
	return true
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
				logging.Log.Error("panic recovered", "method", r.Method, "path", r.URL.Path,
					"panic", v, "stack", string(debug.Stack()))
				Problem(r, w, http.StatusInternalServerError, "internal error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
