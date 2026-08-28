package api_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/api"
	"github.com/priyanshuguptadev/job-board/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter(db *sql.DB) http.Handler {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:               8080,
			Env:                "test",
			LogLevel:           "error",
			CORSAllowedOrigins: []string{"*"},
			RateLimitRPS:       20,
			RateLimitBurst:     50,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return api.NewRouter(api.RouterConfig{
		Config: cfg,
		Logger: logger,
		DB:     db,
	})
}

func TestHealthCheck(t *testing.T) {
	router := newTestRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var body map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "disabled", body["database"])
}

func TestNotFoundHandler(t *testing.T) {
	router := newTestRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	var resp api.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeNotFound, resp.Error.Code)
	assert.NotEmpty(t, resp.Error.Message)
}

func TestRespondErrorWithDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	api.RespondError(rec, http.StatusUnprocessableEntity, api.ErrCodeValidationError, "Validation failed", api.ErrorDetail{
		Field: "resume",
		Issue: "File exceeds 10MB limit",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	var resp api.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeValidationError, resp.Error.Code)
	assert.Equal(t, "Validation failed", resp.Error.Message)
	require.Len(t, resp.Error.Details, 1)
	assert.Equal(t, "resume", resp.Error.Details[0].Field)
	assert.Equal(t, "File exceeds 10MB limit", resp.Error.Details[0].Issue)
}
