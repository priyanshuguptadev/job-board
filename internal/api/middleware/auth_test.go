package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/auth"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockApiKeyRepository is an in-memory test repository.
type mockApiKeyRepository struct {
	mu           sync.Mutex
	keysByHash   map[string]*domain.ApiKey
	keysByID     map[string]*domain.ApiKey
	lastUsedByID map[string]time.Time
	forceDBErr   bool
}

func newMockApiKeyRepo() *mockApiKeyRepository {
	return &mockApiKeyRepository{
		keysByHash:   make(map[string]*domain.ApiKey),
		keysByID:     make(map[string]*domain.ApiKey),
		lastUsedByID: make(map[string]time.Time),
	}
}

func (m *mockApiKeyRepository) Create(_ context.Context, key *domain.ApiKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forceDBErr {
		return errors.New("db failure")
	}
	m.keysByHash[key.KeyHash] = key
	m.keysByID[key.ID] = key
	return nil
}

func (m *mockApiKeyRepository) GetByHash(_ context.Context, hash string) (*domain.ApiKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forceDBErr {
		return nil, errors.New("db failure")
	}
	key, exists := m.keysByHash[hash]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return key, nil
}

func (m *mockApiKeyRepository) GetByID(_ context.Context, id string) (*domain.ApiKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forceDBErr {
		return nil, errors.New("db failure")
	}
	key, exists := m.keysByID[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return key, nil
}

func (m *mockApiKeyRepository) List(_ context.Context) ([]*domain.ApiKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]*domain.ApiKey, 0, len(m.keysByID))
	for _, k := range m.keysByID {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockApiKeyRepository) UpdateLastUsed(_ context.Context, id string, lastUsed time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forceDBErr {
		return errors.New("db failure")
	}
	m.lastUsedByID[id] = lastUsed
	return nil
}

func (m *mockApiKeyRepository) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.keysByID, id)
	return nil
}

func TestApiKeyAuthMiddleware(t *testing.T) {
	repo := newMockApiKeyRepo()

	// Seed an Admin key
	adminRaw, adminKey, err := auth.GenerateKey("Admin Key", domain.ApiKeyScopeAdmin)
	require.NoError(t, err)
	adminKey.ID = "admin-key-id-1"
	require.NoError(t, repo.Create(context.Background(), adminKey))

	// Seed a Public key
	pubRaw, pubKey, err := auth.GenerateKey("Public Key", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	pubKey.ID = "pub-key-id-2"
	require.NoError(t, repo.Create(context.Background(), pubKey))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := auth.GetApiKey(r.Context())
		if ok && key != nil {
			w.Header().Set("X-Auth-Key-ID", key.ID)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	t.Run("admin endpoint with valid admin key via X-API-Key", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopeAdmin, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, adminKey.ID, rec.Header().Get("X-Auth-Key-ID"))
	})

	t.Run("admin endpoint with valid admin key via Authorization Bearer", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopeAdmin, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+adminRaw)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, adminKey.ID, rec.Header().Get("X-Auth-Key-ID"))
	})

	t.Run("admin endpoint with public key returns 403 Forbidden", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopeAdmin, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		var resp map[string]map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "FORBIDDEN", resp["error"]["code"])
	})

	t.Run("public endpoint with valid public key", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopePublic, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, pubKey.ID, rec.Header().Get("X-Auth-Key-ID"))
	})

	t.Run("public endpoint with valid admin key (admin has all access)", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopePublic, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, adminKey.ID, rec.Header().Get("X-Auth-Key-ID"))
	})

	t.Run("missing API key returns 401 Unauthorized", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopePublic, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var resp map[string]map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_API_KEY", resp["error"]["code"])
	})

	t.Run("malformed API key prefix returns 401 Unauthorized", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopePublic, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.Header.Set("X-API-Key", "random_unrecognized_key_123")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var resp map[string]map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_API_KEY", resp["error"]["code"])
	})

	t.Run("unrecognized valid format API key returns 401 Unauthorized", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopePublic, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.Header.Set("X-API-Key", "jb_pub_00000000000000000000000000000000")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var resp map[string]map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_API_KEY", resp["error"]["code"])
	})

	t.Run("database error returns 500 Internal Server Error", func(t *testing.T) {
		errRepo := newMockApiKeyRepo()
		errRepo.forceDBErr = true

		handler := ApiKeyAuth(errRepo, domain.ApiKeyScopePublic, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var resp map[string]map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "INTERNAL_SERVER_ERROR", resp["error"]["code"])
	})

	t.Run("updates last_used_at on successful auth", func(t *testing.T) {
		handler := ApiKeyAuth(repo, domain.ApiKeyScopePublic, nil)(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Give async goroutine a brief moment to update
		assert.Eventually(t, func() bool {
			repo.mu.Lock()
			defer repo.mu.Unlock()
			lastUsed, ok := repo.lastUsedByID[pubKey.ID]
			return ok && !lastUsed.IsZero()
		}, 1*time.Second, 10*time.Millisecond)
	})
}
