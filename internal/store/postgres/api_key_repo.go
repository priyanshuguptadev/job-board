package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/priyanshuguptadev/job-board/internal/domain"
)

type apiKeyRepository struct {
	db *sql.DB
}

// NewApiKeyRepository creates a new PostgreSQL API key repository.
func NewApiKeyRepository(db *sql.DB) domain.ApiKeyRepository {
	return &apiKeyRepository{db: db}
}

func (r *apiKeyRepository) Create(ctx context.Context, key *domain.ApiKey) error {
	query := `
		INSERT INTO api_keys (id, name, key_hash, key_prefix, scope, created_at, last_used_at)
		VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2, $3, $4, $5,
			COALESCE(NULLIF($6, '0001-01-01 00:00:00+00'::timestamptz), NOW()),
			$7
		)
		RETURNING id, created_at
	`

	var createdAt time.Time
	var id string
	var createdAtParam interface{}
	if key.CreatedAt.IsZero() {
		createdAtParam = time.Time{}
	} else {
		createdAtParam = key.CreatedAt
	}

	err := r.db.QueryRowContext(
		ctx,
		query,
		key.ID,
		key.Name,
		key.KeyHash,
		key.KeyPrefix,
		key.Scope,
		createdAtParam,
		key.LastUsedAt,
	).Scan(&id, &createdAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConflict
		}
		return fmt.Errorf("failed to create api key: %w", err)
	}

	key.ID = id
	key.CreatedAt = createdAt
	return nil
}

func (r *apiKeyRepository) GetByHash(ctx context.Context, hash string) (*domain.ApiKey, error) {
	query := `
		SELECT id, name, key_hash, key_prefix, scope, created_at, last_used_at
		FROM api_keys
		WHERE key_hash = $1
	`

	var key domain.ApiKey
	err := r.db.QueryRowContext(ctx, query, hash).Scan(
		&key.ID,
		&key.Name,
		&key.KeyHash,
		&key.KeyPrefix,
		&key.Scope,
		&key.CreatedAt,
		&key.LastUsedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get api key by hash: %w", err)
	}

	return &key, nil
}

func (r *apiKeyRepository) GetByID(ctx context.Context, id string) (*domain.ApiKey, error) {
	query := `
		SELECT id, name, key_hash, key_prefix, scope, created_at, last_used_at
		FROM api_keys
		WHERE id = $1
	`

	var key domain.ApiKey
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&key.ID,
		&key.Name,
		&key.KeyHash,
		&key.KeyPrefix,
		&key.Scope,
		&key.CreatedAt,
		&key.LastUsedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get api key by id: %w", err)
	}

	return &key, nil
}

func (r *apiKeyRepository) List(ctx context.Context) ([]*domain.ApiKey, error) {
	query := `
		SELECT id, name, key_hash, key_prefix, scope, created_at, last_used_at
		FROM api_keys
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.ApiKey
	for rows.Next() {
		var key domain.ApiKey
		if err := rows.Scan(
			&key.ID,
			&key.Name,
			&key.KeyHash,
			&key.KeyPrefix,
			&key.Scope,
			&key.CreatedAt,
			&key.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan api key row: %w", err)
		}
		keys = append(keys, &key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return keys, nil
}

func (r *apiKeyRepository) UpdateLastUsed(ctx context.Context, id string, lastUsed time.Time) error {
	query := `
		UPDATE api_keys
		SET last_used_at = $1
		WHERE id = $2
	`

	res, err := r.db.ExecContext(ctx, query, lastUsed, id)
	if err != nil {
		return fmt.Errorf("failed to update api key last_used_at: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *apiKeyRepository) Delete(ctx context.Context, id string) error {
	query := `
		DELETE FROM api_keys
		WHERE id = $1
	`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete api key: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}
