package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// API key prefix constants.
const (
	PrefixPublic = "jb_pub_"
	PrefixAdmin  = "jb_sec_"
)

// Context key for storing authenticated API key in request context.
type contextKey string

const (
	apiKeyCtxKey contextKey = "auth_api_key"
)

var (
	ErrInvalidScope  = errors.New("invalid API key scope")
	ErrInvalidKey    = errors.New("invalid API key format")
	ErrKeyTooShort   = errors.New("API key token is too short")
	ErrMissingAPIKey = errors.New("missing API key")
)

// GenerateKey creates a cryptographically secure random API key for the specified scope and name.
// It returns the raw token string (to be shown to user once) and the domain.ApiKey model populated with the SHA-256 hash.
func GenerateKey(name string, scope domain.ApiKeyScope) (string, *domain.ApiKey, error) {
	var prefix string
	switch scope {
	case domain.ApiKeyScopeAdmin:
		prefix = PrefixAdmin
	case domain.ApiKeyScopePublic:
		prefix = PrefixPublic
	default:
		return "", nil, fmt.Errorf("%w: must be 'admin' or 'public'", ErrInvalidScope)
	}

	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	rawKey := prefix + hex.EncodeToString(randomBytes)
	keyHash := HashKey(rawKey)

	apiKey := &domain.ApiKey{
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: prefix,
		Scope:     scope,
		CreatedAt: time.Now().UTC(),
	}

	return rawKey, apiKey, nil
}

// HashKey computes the SHA-256 hex-encoded hash of the raw API key.
func HashKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

// ValidateKeyFormat checks if the raw API key starts with an expected prefix and has adequate length.
func ValidateKeyFormat(rawKey string) (domain.ApiKeyScope, error) {
	trimmed := strings.TrimSpace(rawKey)
	if trimmed == "" {
		return "", ErrMissingAPIKey
	}

	if strings.HasPrefix(trimmed, PrefixAdmin) {
		if len(trimmed) <= len(PrefixAdmin) {
			return "", ErrKeyTooShort
		}
		return domain.ApiKeyScopeAdmin, nil
	}

	if strings.HasPrefix(trimmed, PrefixPublic) {
		if len(trimmed) <= len(PrefixPublic) {
			return "", ErrKeyTooShort
		}
		return domain.ApiKeyScopePublic, nil
	}

	return "", fmt.Errorf("%w: key must start with %q or %q", ErrInvalidKey, PrefixPublic, PrefixAdmin)
}

// ExtractAPIKey extracts the API key from the request headers:
// 1. Checks `X-API-Key` header.
// 2. Checks `Authorization: Bearer <key>` header.
func ExtractAPIKey(r *http.Request) string {
	if apiKey := strings.TrimSpace(r.Header.Get("X-API-Key")); apiKey != "" {
		return apiKey
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

// HasRequiredScope determines if a key with actualScope is authorized for requiredScope.
// Admin keys ('jb_sec_...') have access to both admin and public endpoints.
// Public keys ('jb_pub_...') have access only to public endpoints.
func HasRequiredScope(actualScope, requiredScope domain.ApiKeyScope) bool {
	if actualScope == domain.ApiKeyScopeAdmin {
		return true
	}
	if actualScope == domain.ApiKeyScopePublic && requiredScope == domain.ApiKeyScopePublic {
		return true
	}
	return false
}

// ContextWithApiKey stores the authenticated API key into the context.
func ContextWithApiKey(ctx context.Context, apiKey *domain.ApiKey) context.Context {
	return context.WithValue(ctx, apiKeyCtxKey, apiKey)
}

// GetApiKey retrieves the authenticated API key from the context, if present.
func GetApiKey(ctx context.Context) (*domain.ApiKey, bool) {
	val := ctx.Value(apiKeyCtxKey)
	if val == nil {
		return nil, false
	}
	key, ok := val.(*domain.ApiKey)
	return key, ok
}

// MustGetApiKey retrieves the authenticated API key from context or returns nil.
func MustGetApiKey(ctx context.Context) *domain.ApiKey {
	key, _ := GetApiKey(ctx)
	return key
}
