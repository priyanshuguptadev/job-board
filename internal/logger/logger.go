package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New initializes and returns a configured slog.Logger instance based on environment and log level.
func New(env, levelStr string) *slog.Logger {
	return NewWithWriter(env, levelStr, os.Stdout)
}

// NewWithWriter initializes a slog.Logger with a custom io.Writer (useful for testing).
func NewWithWriter(env, levelStr string, w io.Writer) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "info":
		fallthrough
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(env) == "development" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(handler)
}
