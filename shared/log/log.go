// Package log provides a thin structured logging wrapper over the standard
// library's log/slog package, with a configurable level.
package log

import (
	"log/slog"
	"os"
	"strings"
)

// Level names accepted by ParseLevel.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// New constructs a slog.Logger writing to stderr at the given level. Unknown
// levels fall back to info.
func New(level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case LevelDebug:
		lv = slog.LevelDebug
	case LevelWarn:
		lv = slog.LevelWarn
	case LevelError:
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lv})
	return slog.New(h)
}
