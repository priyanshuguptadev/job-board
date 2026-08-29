package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKey(t *testing.T) {
	t.Run("generate admin key", func(t *testing.T) {
		rawKey, apiKey, err := GenerateKey("Test Admin", domain.ApiKeyScopeAdmin)
		require.NoError(t, err)
		assert.NotEmpty(t, rawKey)
		assert.True(t, strings.HasPrefix(rawKey, PrefixAdmin))
		assert.Equal(t, "Test Admin", apiKey.Name)
		assert.Equal(t, domain.ApiKeyScopeAdmin, apiKey.Scope)
		assert.Equal(t, PrefixAdmin, apiKey.KeyPrefix)
		assert.Equal(t, HashKey(rawKey), apiKey.KeyHash)
		assert.WithinDuration(t, time.Now().UTC(), apiKey.CreatedAt, 2*time.Second)
	})

	t.Run("generate public key", func(t *testing.T) {
		rawKey, apiKey, err := GenerateKey("Public Frontend", domain.ApiKeyScopePublic)
		require.NoError(t, err)
		assert.NotEmpty(t, rawKey)
		assert.True(t, strings.HasPrefix(rawKey, PrefixPublic))
		assert.Equal(t, "Public Frontend", apiKey.Name)
		assert.Equal(t, domain.ApiKeyScopePublic, apiKey.Scope)
		assert.Equal(t, PrefixPublic, apiKey.KeyPrefix)
		assert.Equal(t, HashKey(rawKey), apiKey.KeyHash)
	})

	t.Run("generate invalid scope", func(t *testing.T) {
		rawKey, apiKey, err := GenerateKey("Invalid", domain.ApiKeyScope("superadmin"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidScope)
		assert.Empty(t, rawKey)
		assert.Nil(t, apiKey)
	})
}

func TestHashKey(t *testing.T) {
	rawKey := "jb_sec_0123456789abcdef0123456789abcdef"
	hash1 := HashKey(rawKey)
	hash2 := HashKey(rawKey)
	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 64) // SHA-256 is 64 hex characters
}

func TestValidateKeyFormat(t *testing.T) {
	tests := []struct {
		name          string
		rawKey        string
		expectedScope domain.ApiKeyScope
		expectedErr   error
	}{
		{
			name:          "valid admin key",
			rawKey:        "jb_sec_1234567890abcdef",
			expectedScope: domain.ApiKeyScopeAdmin,
			expectedErr:   nil,
		},
		{
			name:          "valid public key",
			rawKey:        "jb_pub_1234567890abcdef",
			expectedScope: domain.ApiKeyScopePublic,
			expectedErr:   nil,
		},
		{
			name:          "valid key with whitespace",
			rawKey:        "  jb_pub_1234567890abcdef  ",
			expectedScope: domain.ApiKeyScopePublic,
			expectedErr:   nil,
		},
		{
			name:          "empty key",
			rawKey:        "",
			expectedScope: "",
			expectedErr:   ErrMissingAPIKey,
		},
		{
			name:          "admin prefix only (too short)",
			rawKey:        "jb_sec_",
			expectedScope: "",
			expectedErr:   ErrKeyTooShort,
		},
		{
			name:          "public prefix only (too short)",
			rawKey:        "jb_pub_",
			expectedScope: "",
			expectedErr:   ErrKeyTooShort,
		},
		{
			name:          "invalid prefix",
			rawKey:        "sk_live_1234567890",
			expectedScope: "",
			expectedErr:   ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope, err := ValidateKeyFormat(tt.rawKey)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedScope, scope)
			}
		})
	}
}

func TestExtractAPIKey(t *testing.T) {
	t.Run("from X-API-Key header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "jb_pub_123456")
		key := ExtractAPIKey(req)
		assert.Equal(t, "jb_pub_123456", key)
	})

	t.Run("from Authorization Bearer header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer jb_sec_abcdef")
		key := ExtractAPIKey(req)
		assert.Equal(t, "jb_sec_abcdef", key)
	})

	t.Run("from Authorization lowercase bearer header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "bearer jb_sec_abcdef")
		key := ExtractAPIKey(req)
		assert.Equal(t, "jb_sec_abcdef", key)
	})

	t.Run("X-API-Key takes precedence", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "jb_pub_111")
		req.Header.Set("Authorization", "Bearer jb_sec_222")
		key := ExtractAPIKey(req)
		assert.Equal(t, "jb_pub_111", key)
	})

	t.Run("no auth headers present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		key := ExtractAPIKey(req)
		assert.Empty(t, key)
	})

	t.Run("malformed Authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		key := ExtractAPIKey(req)
		assert.Empty(t, key)
	})
}

func TestHasRequiredScope(t *testing.T) {
	tests := []struct {
		actual   domain.ApiKeyScope
		required domain.ApiKeyScope
		expected bool
	}{
		{domain.ApiKeyScopeAdmin, domain.ApiKeyScopeAdmin, true},
		{domain.ApiKeyScopeAdmin, domain.ApiKeyScopePublic, true},
		{domain.ApiKeyScopePublic, domain.ApiKeyScopePublic, true},
		{domain.ApiKeyScopePublic, domain.ApiKeyScopeAdmin, false},
		{domain.ApiKeyScope("unknown"), domain.ApiKeyScopePublic, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.actual)+"->"+string(tt.required), func(t *testing.T) {
			res := HasRequiredScope(tt.actual, tt.required)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	key, ok := GetApiKey(ctx)
	assert.False(t, ok)
	assert.Nil(t, key)
	assert.Panics(t, func() {
		MustGetApiKey(ctx)
	})

	testKey := &domain.ApiKey{
		ID:    "key-123",
		Name:  "Test Key",
		Scope: domain.ApiKeyScopeAdmin,
	}

	ctx = ContextWithApiKey(ctx, testKey)
	key, ok = GetApiKey(ctx)
	assert.True(t, ok)
	assert.Equal(t, testKey, key)
	assert.Equal(t, testKey, MustGetApiKey(ctx))
}
