package domain

import (
	"context"
	"time"
)

// ApiKeyScope defines the access level for an API key.
type ApiKeyScope string

const (
	ApiKeyScopePublic ApiKeyScope = "public"
	ApiKeyScopeAdmin  ApiKeyScope = "admin"
)

// ApiKey represents an API key entity for authenticating requests.
type ApiKey struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	KeyHash    string      `json:"key_hash"`
	KeyPrefix  string      `json:"key_prefix"`
	Scope      ApiKeyScope `json:"scope"`
	CreatedAt  time.Time   `json:"created_at"`
	LastUsedAt *time.Time  `json:"last_used_at,omitempty"`
}

// ApiKeyRepository defines persistence operations for API keys.
type ApiKeyRepository interface {
	Create(ctx context.Context, key *ApiKey) error
	GetByHash(ctx context.Context, hash string) (*ApiKey, error)
	GetByID(ctx context.Context, id string) (*ApiKey, error)
	List(ctx context.Context) ([]*ApiKey, error)
	UpdateLastUsed(ctx context.Context, id string, lastUsed time.Time) error
	Delete(ctx context.Context, id string) error
}
