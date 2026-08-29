package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

type webhookSubscriptionRepository struct {
	db *sql.DB
}

// NewWebhookSubscriptionRepository creates a new PostgreSQL Webhook Subscription repository.
func NewWebhookSubscriptionRepository(db *sql.DB) domain.WebhookSubscriptionRepository {
	return &webhookSubscriptionRepository{db: db}
}

func (r *webhookSubscriptionRepository) Create(ctx context.Context, sub *domain.WebhookSubscription) error {
	query := `
		INSERT INTO webhook_subscriptions (
			id, target_url, secret_token, events, is_active, created_at, updated_at
		)
		VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2, $3, $4, $5, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	events := sub.Events
	if events == nil {
		events = []string{}
	}

	var id string
	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(
		ctx,
		query,
		sub.ID,
		sub.TargetURL,
		sub.SecretToken,
		TextArray(events),
		sub.IsActive,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		return mapDBError(fmt.Errorf("failed to create webhook subscription: %w", err))
	}

	sub.ID = id
	sub.Events = events
	sub.CreatedAt = createdAt
	sub.UpdatedAt = updatedAt
	return nil
}

func (r *webhookSubscriptionRepository) scanSubscription(scanner interface {
	Scan(dest ...interface{}) error
}) (*domain.WebhookSubscription, error) {
	var sub domain.WebhookSubscription
	var events TextArray

	err := scanner.Scan(
		&sub.ID,
		&sub.TargetURL,
		&sub.SecretToken,
		&events,
		&sub.IsActive,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if events == nil {
		sub.Events = []string{}
	} else {
		sub.Events = []string(events)
	}

	return &sub, nil
}

func (r *webhookSubscriptionRepository) GetByID(ctx context.Context, id string) (*domain.WebhookSubscription, error) {
	query := `
		SELECT id, target_url, secret_token, events, is_active, created_at, updated_at
		FROM webhook_subscriptions
		WHERE id = $1
	`

	sub, err := r.scanSubscription(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get webhook subscription by id: %w", err)
	}

	return sub, nil
}

func (r *webhookSubscriptionRepository) List(ctx context.Context) ([]*domain.WebhookSubscription, error) {
	query := `
		SELECT id, target_url, secret_token, events, is_active, created_at, updated_at
		FROM webhook_subscriptions
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhook subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*domain.WebhookSubscription
	for rows.Next() {
		sub, err := r.scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook subscription: %w", err)
		}
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return subs, nil
}

func (r *webhookSubscriptionRepository) ListActiveByEvent(ctx context.Context, event string) ([]*domain.WebhookSubscription, error) {
	query := `
		SELECT id, target_url, secret_token, events, is_active, created_at, updated_at
		FROM webhook_subscriptions
		WHERE is_active = TRUE AND ($1 = ANY(events) OR '*' = ANY(events))
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, event)
	if err != nil {
		return nil, fmt.Errorf("failed to list active webhook subscriptions by event: %w", err)
	}
	defer rows.Close()

	var subs []*domain.WebhookSubscription
	for rows.Next() {
		sub, err := r.scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook subscription: %w", err)
		}
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return subs, nil
}

func (r *webhookSubscriptionRepository) Update(ctx context.Context, sub *domain.WebhookSubscription) error {
	events := sub.Events
	if events == nil {
		events = []string{}
	}

	query := `
		UPDATE webhook_subscriptions
		SET
			target_url = $1,
			secret_token = $2,
			events = $3,
			is_active = $4,
			updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
	`

	var updatedAt time.Time
	err := r.db.QueryRowContext(
		ctx,
		query,
		sub.TargetURL,
		sub.SecretToken,
		TextArray(events),
		sub.IsActive,
		sub.ID,
	).Scan(&updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return mapDBError(fmt.Errorf("failed to update webhook subscription: %w", err))
	}

	sub.UpdatedAt = updatedAt
	return nil
}

func (r *webhookSubscriptionRepository) Delete(ctx context.Context, id string) error {
	query := `
		DELETE FROM webhook_subscriptions
		WHERE id = $1
	`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete webhook subscription: %w", err)
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
