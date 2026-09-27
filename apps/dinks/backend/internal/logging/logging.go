// Package logging provides the process-wide structured logger and the generic
// HTTP middleware every package logs through. It emits JSON on stdout for the
// cluster log collector; verbosity is controlled with LOG_LEVEL.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Log is the process logger. It is initialised by Init, which must be called
// before any other package logs.
var Log *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// Init configures Log from the environment. LOG_LEVEL selects the threshold
// (debug, info, warn, error; default info) and LOG_FORMAT=text switches from
// JSON to a human-readable handler for local runs.
func Init() {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_FORMAT")), "text") {
		Log = slog.New(slog.NewTextHandler(os.Stdout, opts))
		return
	}
	Log = slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// Fatal records an unrecoverable failure and exits non-zero so the container
// restarts rather than serving without its dependencies.
func Fatal(msg string, args ...any) {
	Log.Error(msg, args...)
	os.Exit(1)
}
