package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoggerDevelopment(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter("development", "debug", &buf)
	require.NotNil(t, l)

	l.DebugContext(context.Background(), "debug message", "key", "value")
	assert.Contains(t, buf.String(), "debug message")
	assert.Contains(t, buf.String(), "key=value")
}

func TestNewLoggerProductionJSON(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter("production", "info", &buf)
	require.NotNil(t, l)

	l.InfoContext(context.Background(), "production log", "app", "job-board")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "production log", logEntry["msg"])
	assert.Equal(t, "job-board", logEntry["app"])
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter("production", "warn", &buf)
	require.NotNil(t, l)

	l.InfoContext(context.Background(), "info message should be ignored")
	assert.Empty(t, buf.String())

	l.WarnContext(context.Background(), "warn message should appear")
	assert.NotEmpty(t, buf.String())
}

func TestLogLevelParsing(t *testing.T) {
	levels := []struct {
		input string
		pass  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
	}

	for _, tc := range levels {
		var buf bytes.Buffer
		l := NewWithWriter("production", tc.input, &buf)
		assert.True(t, l.Enabled(context.Background(), tc.pass))
	}
}
