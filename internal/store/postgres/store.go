package postgres

import (
	"database/sql"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// Store aggregates all domain repositories for PostgreSQL.
type Store struct {
	DB               *sql.DB
	APIKeys          domain.ApiKeyRepository
	Jobs             domain.JobRepository
	Applications     domain.ApplicationRepository
	ApplicationNotes domain.ApplicationNoteRepository
	Webhooks         domain.WebhookSubscriptionRepository
}

// NewStore initializes a new Store instance wrapping the provided sql.DB pool.
func NewStore(db *sql.DB) *Store {
	return &Store{
		DB:               db,
		APIKeys:          NewApiKeyRepository(db),
		Jobs:             NewJobRepository(db),
		Applications:     NewApplicationRepository(db),
		ApplicationNotes: NewApplicationNoteRepository(db),
		Webhooks:         NewWebhookSubscriptionRepository(db),
	}
}
