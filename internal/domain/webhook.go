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
	EventAll                     = "*"
)

// WebhookPayload represents the standard outbound JSON payload structure.
type WebhookPayload struct {
	ID        string      `json:"id"`
	Event     string      `json:"event"`
	CreatedAt time.Time   `json:"created_at"`
	Data      interface{} `json:"data"`
}

// ApplicationCreatedData represents data payload for application.created event.
type ApplicationCreatedData struct {
	ApplicationID  string                 `json:"application_id"`
	JobID          string                 `json:"job_id"`
	JobTitle       string                 `json:"job_title"`
	JobSlug        string                 `json:"job_slug"`
	CandidateName  string                 `json:"candidate_name"`
	CandidateEmail string                 `json:"candidate_email"`
	Stage          ApplicationStage       `json:"stage"`
	CustomAnswers  map[string]interface{} `json:"custom_answers,omitempty"`
}

// ApplicationStageUpdatedData represents data payload for application.stage_updated event.
type ApplicationStageUpdatedData struct {
	ApplicationID string           `json:"application_id"`
	JobID         string           `json:"job_id,omitempty"`
	OldStage      ApplicationStage `json:"old_stage"`
	NewStage      ApplicationStage `json:"new_stage"`
	NotesCount    int              `json:"notes_count"`
}

// ApplicationRejectedData represents data payload for application.rejected event.
type ApplicationRejectedData struct {
	ApplicationID  string  `json:"application_id"`
	RejectedReason *string `json:"rejected_reason,omitempty"`
}

// WebhookPingData represents data payload for webhook.ping event.
type WebhookPingData struct {
	Message        string    `json:"message"`
	Timestamp      time.Time `json:"timestamp"`
	SubscriptionID string    `json:"subscription_id,omitempty"`
}

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

// AllWebhookEvents returns all known webhook event topics.
func AllWebhookEvents() []string {
	return []string{
		EventJobPublished,
		EventJobArchived,
		EventApplicationCreated,
		EventApplicationStageUpdated,
		EventApplicationRejected,
		EventWebhookPing,
	}
}

// IsValidWebhookEvent returns true if the event name is recognized or is wildcard "*".
func IsValidWebhookEvent(event string) bool {
	if event == EventAll {
		return true
	}
	for _, e := range AllWebhookEvents() {
		if e == event {
			return true
		}
	}
	return false
}
