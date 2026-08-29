package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/api"
	"github.com/priyanshuguptadev/job-board/internal/auth"
	"github.com/priyanshuguptadev/job-board/internal/config"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockApiKeyRepo struct {
	mu         sync.Mutex
	keysByHash map[string]*domain.ApiKey
}

func newMockApiKeyRepo() *mockApiKeyRepo {
	return &mockApiKeyRepo{
		keysByHash: make(map[string]*domain.ApiKey),
	}
}

func (m *mockApiKeyRepo) Create(_ context.Context, key *domain.ApiKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keysByHash[key.KeyHash] = key
	return nil
}

func (m *mockApiKeyRepo) GetByHash(_ context.Context, hash string) (*domain.ApiKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key, ok := m.keysByHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return key, nil
}

func (m *mockApiKeyRepo) GetByID(_ context.Context, _ string) (*domain.ApiKey, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApiKeyRepo) List(_ context.Context) ([]*domain.ApiKey, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApiKeyRepo) UpdateLastUsed(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (m *mockApiKeyRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func newTestRouter(db *sql.DB, apiKeyRepo domain.ApiKeyRepository, rps, burst int) http.Handler {
	if rps <= 0 {
		rps = 20
	}
	if burst <= 0 {
		burst = 50
	}
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:               8080,
			Env:                "test",
			LogLevel:           "error",
			CORSAllowedOrigins: []string{"*"},
			RateLimitRPS:       rps,
			RateLimitBurst:     burst,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return api.NewRouter(api.RouterConfig{
		Config:     cfg,
		Logger:     logger,
		DB:         db,
		ApiKeyRepo: apiKeyRepo,
	})
}

func TestHealthCheck(t *testing.T) {
	router := newTestRouter(nil, nil, 0, 0)

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
	router := newTestRouter(nil, nil, 0, 0)

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

func TestMethodNotAllowedHandler(t *testing.T) {
	router := newTestRouter(nil, nil, 0, 0)

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	var resp api.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeNotFound, resp.Error.Code)
}

func TestRouterAuthAndScoping(t *testing.T) {
	repo := newMockApiKeyRepo()

	adminRaw, adminKey, err := auth.GenerateKey("Admin", domain.ApiKeyScopeAdmin)
	require.NoError(t, err)
	adminKey.ID = "admin-1"
	require.NoError(t, repo.Create(context.Background(), adminKey))

	pubRaw, pubKey, err := auth.GenerateKey("Public", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	pubKey.ID = "pub-1"
	require.NoError(t, repo.Create(context.Background(), pubKey))

	router := newTestRouter(nil, repo, 20, 50)

	t.Run("public ping with valid public key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("public ping with admin key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("admin ping with admin key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("admin ping with public key returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		var resp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeForbidden, resp.Error.Code)
	})

	t.Run("admin ping without key returns 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var resp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeInvalidAPIKey, resp.Error.Code)
	})
}

func TestRouterRateLimiting(t *testing.T) {
	repo := newMockApiKeyRepo()
	pubRaw, pubKey, err := auth.GenerateKey("Public", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	pubKey.ID = "pub-1"
	require.NoError(t, repo.Create(context.Background(), pubKey))

	// Burst of 2, 1 RPS
	router := newTestRouter(nil, repo, 1, 2)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// 3rd request should hit rate limit
	req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("X-API-Key", pubRaw)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	var resp api.ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeRateLimitExceeded, resp.Error.Code)
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
