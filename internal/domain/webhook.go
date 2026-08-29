package domain

import (
	"context"
	"time"
)

// Webhook event topic constants.
const (
	EventJobPublished            = "job.published"
	EventJobArchived             = "job.archived"
	EventApplicationCreated      = "application.created"
	EventApplicationStageUpdated = "application.stage_updated"
	EventApplicationRejected     = "application.rejected"
	EventWebhookPing             = "webhook.ping"
)

// WebhookSubscription represents an outbound webhook registration.
type WebhookSubscription struct {
	ID          string    `json:"id"`
	TargetURL   string    `json:"target_url"`
	SecretToken string    `json:"secret_token"`
	Events      []string  `json:"events"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookSubscriptionRepository defines persistence operations for webhook subscriptions.
type WebhookSubscriptionRepository interface {
	Create(ctx context.Context, sub *WebhookSubscription) error
	GetByID(ctx context.Context, id string) (*WebhookSubscription, error)
	List(ctx context.Context) ([]*WebhookSubscription, error)
	ListActiveByEvent(ctx context.Context, event string) ([]*WebhookSubscription, error)
	Update(ctx context.Context, sub *WebhookSubscription) error
	Delete(ctx context.Context, id string) error
}
