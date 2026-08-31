package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
	"github.com/priyanshuguptadev/job-board/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWebhookRepo struct {
	subs map[string]*domain.WebhookSubscription
}

func newMockWebhookRepo() *mockWebhookRepo {
	return &mockWebhookRepo{
		subs: make(map[string]*domain.WebhookSubscription),
	}
}

func (m *mockWebhookRepo) Create(_ context.Context, sub *domain.WebhookSubscription) error {
	m.subs[sub.ID] = sub
	return nil
}

func (m *mockWebhookRepo) GetByID(_ context.Context, id string) (*domain.WebhookSubscription, error) {
	sub, ok := m.subs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return sub, nil
}

func (m *mockWebhookRepo) List(_ context.Context) ([]*domain.WebhookSubscription, error) {
	var list []*domain.WebhookSubscription
	for _, s := range m.subs {
		list = append(list, s)
	}
	return list, nil
}

func (m *mockWebhookRepo) ListActiveByEvent(_ context.Context, event string) ([]*domain.WebhookSubscription, error) {
	var list []*domain.WebhookSubscription
	for _, s := range m.subs {
		if !s.IsActive {
			continue
		}
		for _, e := range s.Events {
			if e == "*" || e == event {
				list = append(list, s)
				break
			}
		}
	}
	return list, nil
}

func (m *mockWebhookRepo) Update(_ context.Context, sub *domain.WebhookSubscription) error {
	m.subs[sub.ID] = sub
	return nil
}

func (m *mockWebhookRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.subs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.subs, id)
	return nil
}

type mockDispatcher struct {
	dispatchedEvents []string
	directResult     *webhook.DeliveryResult
	directErr        error
}

func (m *mockDispatcher) Start(_ context.Context) {}
func (m *mockDispatcher) Stop()                   {}
func (m *mockDispatcher) Dispatch(_ context.Context, event string, _ interface{}) error {
	m.dispatchedEvents = append(m.dispatchedEvents, event)
	return nil
}
func (m *mockDispatcher) SendDirect(_ context.Context, _ *domain.WebhookSubscription, _ *domain.WebhookPayload) (*webhook.DeliveryResult, error) {
	return m.directResult, m.directErr
}

func TestWebhookService(t *testing.T) {
	repo := newMockWebhookRepo()
	disp := &mockDispatcher{
		directResult: &webhook.DeliveryResult{
			StatusCode: 200,
			Success:    true,
			Duration:   10 * time.Millisecond,
		},
	}
	svc := service.NewWebhookService(repo, disp)

	t.Run("CreateSubscription with auto-generated secret", func(t *testing.T) {
		input := service.CreateWebhookSubscriptionInput{
			TargetURL: "https://example.com/webhook",
			Events:    []string{domain.EventJobPublished, domain.EventApplicationCreated},
		}

		sub, err := svc.CreateSubscription(context.Background(), input)
		require.NoError(t, err)
		assert.NotEmpty(t, sub.ID)
		assert.Equal(t, "https://example.com/webhook", sub.TargetURL)
		assert.True(t, strings.HasPrefix(sub.SecretToken, "whsec_"))
		assert.True(t, sub.IsActive)
		assert.ElementsMatch(t, []string{domain.EventJobPublished, domain.EventApplicationCreated}, sub.Events)
	})

	t.Run("CreateSubscription validation failure", func(t *testing.T) {
		input := service.CreateWebhookSubscriptionInput{
			TargetURL: "not-a-valid-url",
			Events:    []string{},
		}

		_, err := svc.CreateSubscription(context.Background(), input)
		require.Error(t, err)
		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.NotEmpty(t, valErr.Details)
	})

	t.Run("GetSubscription and ListSubscriptions", func(t *testing.T) {
		list, err := svc.ListSubscriptions(context.Background())
		require.NoError(t, err)
		assert.Len(t, list, 1)

		got, err := svc.GetSubscription(context.Background(), list[0].ID)
		require.NoError(t, err)
		assert.Equal(t, list[0].ID, got.ID)
	})

	t.Run("TestSubscription executes direct ping", func(t *testing.T) {
		list, err := svc.ListSubscriptions(context.Background())
		require.NoError(t, err)

		res, err := svc.TestSubscription(context.Background(), list[0].ID)
		require.NoError(t, err)
		assert.True(t, res.Success)
		assert.Equal(t, 200, res.StatusCode)
	})

	t.Run("DeleteSubscription removes subscription", func(t *testing.T) {
		list, err := svc.ListSubscriptions(context.Background())
		require.NoError(t, err)

		err = svc.DeleteSubscription(context.Background(), list[0].ID)
		require.NoError(t, err)

		_, err = svc.GetSubscription(context.Background(), list[0].ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
