package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/webhook"
)

// CreateWebhookSubscriptionInput contains parameters for creating a webhook subscription.
type CreateWebhookSubscriptionInput struct {
	TargetURL   string
	SecretToken *string
	Events      []string
}

// WebhookTestResult contains the output of a test ping dispatch.
type WebhookTestResult struct {
	Success      bool   `json:"success"`
	StatusCode   int    `json:"status_code"`
	DurationMs   int64  `json:"duration_ms"`
	Message      string `json:"message"`
	ResponseBody string `json:"response_body,omitempty"`
}

// WebhookService defines business operations for managing and dispatching webhooks.
type WebhookService interface {
	CreateSubscription(ctx context.Context, input CreateWebhookSubscriptionInput) (*domain.WebhookSubscription, error)
	GetSubscription(ctx context.Context, id string) (*domain.WebhookSubscription, error)
	ListSubscriptions(ctx context.Context) ([]*domain.WebhookSubscription, error)
	DeleteSubscription(ctx context.Context, id string) error
	TestSubscription(ctx context.Context, id string) (*WebhookTestResult, error)
	DispatchEvent(ctx context.Context, event string, data interface{}) error
}

type webhookService struct {
	repo       domain.WebhookSubscriptionRepository
	dispatcher webhook.Dispatcher
}

// NewWebhookService creates a new WebhookService instance.
func NewWebhookService(repo domain.WebhookSubscriptionRepository, dispatcher webhook.Dispatcher) WebhookService {
	return &webhookService{
		repo:       repo,
		dispatcher: dispatcher,
	}
}

// CreateSubscription validates and registers a new outbound webhook subscriber.
func (s *webhookService) CreateSubscription(ctx context.Context, input CreateWebhookSubscriptionInput) (*domain.WebhookSubscription, error) {
	details := domain.ValidateWebhookSubscriptionInput(input.TargetURL, input.Events, input.SecretToken)
	if len(details) > 0 {
		return nil, &domain.ValidationError{
			Message: "The provided input failed validation checks.",
			Details: details,
		}
	}

	secret := ""
	if input.SecretToken != nil && strings.TrimSpace(*input.SecretToken) != "" {
		secret = strings.TrimSpace(*input.SecretToken)
	} else {
		var err error
		secret, err = webhook.GenerateSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
		}
	}

	sub := &domain.WebhookSubscription{
		ID:          domain.NewID(),
		TargetURL:   strings.TrimSpace(input.TargetURL),
		SecretToken: secret,
		Events:      input.Events,
		IsActive:    true,
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, sub); err != nil {
			return nil, err
		}
	}

	return sub, nil
}

// GetSubscription retrieves a webhook subscription by ID.
func (s *webhookService) GetSubscription(ctx context.Context, id string) (*domain.WebhookSubscription, error) {
	if s.repo == nil {
		return nil, domain.ErrNotFound
	}

	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, domain.ErrNotFound
	}
	return sub, nil
}

// ListSubscriptions retrieves all registered webhook subscriptions.
func (s *webhookService) ListSubscriptions(ctx context.Context) ([]*domain.WebhookSubscription, error) {
	if s.repo == nil {
		return []*domain.WebhookSubscription{}, nil
	}

	subs, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if subs == nil {
		subs = []*domain.WebhookSubscription{}
	}
	return subs, nil
}

// DeleteSubscription deletes a webhook subscription by ID.
func (s *webhookService) DeleteSubscription(ctx context.Context, id string) error {
	if s.repo == nil {
		return domain.ErrNotFound
	}
	return s.repo.Delete(ctx, id)
}

// TestSubscription sends a synchronous test ping event to a webhook endpoint.
func (s *webhookService) TestSubscription(ctx context.Context, id string) (*WebhookTestResult, error) {
	if s.repo == nil {
		return nil, domain.ErrNotFound
	}

	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, domain.ErrNotFound
	}

	if s.dispatcher == nil {
		return nil, errors.New("webhook dispatcher not configured")
	}

	payload := &domain.WebhookPayload{
		ID:        "evt_" + domain.NewID(),
		Event:     domain.EventWebhookPing,
		CreatedAt: time.Now().UTC(),
		Data: domain.WebhookPingData{
			Message:        "Job Board webhook connectivity ping test",
			Timestamp:      time.Now().UTC(),
			SubscriptionID: sub.ID,
		},
	}

	delResult, err := s.dispatcher.SendDirect(ctx, sub, payload)
	if err != nil {
		durMs := int64(0)
		if delResult != nil {
			durMs = delResult.Duration.Milliseconds()
		}
		return &WebhookTestResult{
			Success:    false,
			StatusCode: 0,
			DurationMs: durMs,
			Message:    fmt.Sprintf("Failed to reach webhook endpoint: %v", err),
		}, nil
	}

	msg := "Webhook ping delivered successfully"
	if !delResult.Success {
		msg = fmt.Sprintf("Webhook endpoint responded with HTTP %d", delResult.StatusCode)
	}

	return &WebhookTestResult{
		Success:      delResult.Success,
		StatusCode:   delResult.StatusCode,
		DurationMs:   delResult.Duration.Milliseconds(),
		Message:      msg,
		ResponseBody: delResult.ResponseBody,
	}, nil
}

// DispatchEvent enqueues an event to the background dispatcher.
func (s *webhookService) DispatchEvent(ctx context.Context, event string, data interface{}) error {
	if s.dispatcher == nil {
		return nil
	}
	return s.dispatcher.Dispatch(ctx, event, data)
}
