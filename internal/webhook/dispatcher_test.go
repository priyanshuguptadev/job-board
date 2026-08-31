package webhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWebhookRepo struct {
	mu   sync.Mutex
	subs map[string]*domain.WebhookSubscription
}

func newMockWebhookRepo() *mockWebhookRepo {
	return &mockWebhookRepo{
		subs: make(map[string]*domain.WebhookSubscription),
	}
}

func (m *mockWebhookRepo) Create(_ context.Context, sub *domain.WebhookSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *sub
	m.subs[sub.ID] = &cp
	return nil
}

func (m *mockWebhookRepo) GetByID(_ context.Context, id string) (*domain.WebhookSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *sub
	return &cp, nil
}

func (m *mockWebhookRepo) List(_ context.Context) ([]*domain.WebhookSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []*domain.WebhookSubscription
	for _, s := range m.subs {
		cp := *s
		list = append(list, &cp)
	}
	return list, nil
}

func (m *mockWebhookRepo) ListActiveByEvent(_ context.Context, event string) ([]*domain.WebhookSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []*domain.WebhookSubscription
	for _, s := range m.subs {
		if !s.IsActive {
			continue
		}
		for _, e := range s.Events {
			if e == "*" || e == event {
				cp := *s
				list = append(list, &cp)
				break
			}
		}
	}
	return list, nil
}

func (m *mockWebhookRepo) Update(_ context.Context, sub *domain.WebhookSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *sub
	m.subs[sub.ID] = &cp
	return nil
}

func (m *mockWebhookRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subs, id)
	return nil
}

func TestDispatcher_DeliverySuccess(t *testing.T) {
	receivedCh := make(chan *domain.WebhookPayload, 1)
	sigCh := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigCh <- r.Header.Get(webhook.HeaderSignature)

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var payload domain.WebhookPayload
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivedCh <- &payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := newMockWebhookRepo()
	sub := &domain.WebhookSubscription{
		ID:          "sub-1",
		TargetURL:   server.URL,
		SecretToken: "whsec_test_secret_123",
		Events:      []string{domain.EventJobPublished},
		IsActive:    true,
	}
	require.NoError(t, repo.Create(context.Background(), sub))

	dispatcher := webhook.NewDispatcher(repo, &webhook.DispatcherConfig{
		HTTPTimeout: 2 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher.Start(ctx)
	defer dispatcher.Stop()

	err := dispatcher.Dispatch(ctx, domain.EventJobPublished, map[string]string{"title": "Backend Engineer"})
	require.NoError(t, err)

	select {
	case payload := <-receivedCh:
		assert.Equal(t, domain.EventJobPublished, payload.Event)
		assert.NotEmpty(t, payload.ID)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for webhook payload")
	}

	select {
	case sig := <-sigCh:
		assert.NotEmpty(t, sig)
		assert.Contains(t, sig, "sha256=")
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for signature header")
	}
}

func TestDispatcher_DeactivateOn410Gone(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusGone) // 410 Gone
	}))
	defer server.Close()

	repo := newMockWebhookRepo()
	sub := &domain.WebhookSubscription{
		ID:          "sub-gone",
		TargetURL:   server.URL,
		SecretToken: "whsec_test_secret_gone",
		Events:      []string{domain.EventApplicationCreated},
		IsActive:    true,
	}
	require.NoError(t, repo.Create(context.Background(), sub))

	dispatcher := webhook.NewDispatcher(repo, &webhook.DispatcherConfig{
		HTTPTimeout: 2 * time.Second,
		RetryDelays: []time.Duration{10 * time.Millisecond},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher.Start(ctx)
	defer dispatcher.Stop()

	err := dispatcher.Dispatch(ctx, domain.EventApplicationCreated, map[string]string{"candidate": "Alice"})
	require.NoError(t, err)

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Verify subscriber was deactivated
	updatedSub, err := repo.GetByID(ctx, "sub-gone")
	require.NoError(t, err)
	assert.False(t, updatedSub.IsActive)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "should not retry on 410 Gone")
}

func TestDispatcher_DeactivateAfterMaxFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := newMockWebhookRepo()
	sub := &domain.WebhookSubscription{
		ID:          "sub-fail",
		TargetURL:   server.URL,
		SecretToken: "whsec_test_secret_fail",
		Events:      []string{domain.EventApplicationCreated},
		IsActive:    true,
	}
	require.NoError(t, repo.Create(context.Background(), sub))

	// Configure with max 2 failures for fast testing
	dispatcher := webhook.NewDispatcher(repo, &webhook.DispatcherConfig{
		HTTPTimeout:            2 * time.Second,
		MaxConsecutiveFailures: 2,
		RetryDelays:            []time.Duration{10 * time.Millisecond},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher.Start(ctx)
	defer dispatcher.Stop()

	err := dispatcher.Dispatch(ctx, domain.EventApplicationCreated, map[string]string{"candidate": "Bob"})
	require.NoError(t, err)

	// Initial attempt + 1 retry will equal 2 failures
	time.Sleep(100 * time.Millisecond)

	updatedSub, err := repo.GetByID(ctx, "sub-fail")
	require.NoError(t, err)
	assert.False(t, updatedSub.IsActive)
}

func TestDispatcher_SendDirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get(webhook.HeaderSignature))
		assert.NotEmpty(t, r.Header.Get(webhook.HeaderTimestamp))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received": true}`))
	}))
	defer server.Close()

	repo := newMockWebhookRepo()
	sub := &domain.WebhookSubscription{
		ID:          "sub-test",
		TargetURL:   server.URL,
		SecretToken: "whsec_test_secret_direct",
		Events:      []string{domain.EventWebhookPing},
		IsActive:    true,
	}

	dispatcher := webhook.NewDispatcher(repo, nil)

	payload := &domain.WebhookPayload{
		ID:        "evt_ping_1",
		Event:     domain.EventWebhookPing,
		CreatedAt: time.Now(),
		Data: domain.WebhookPingData{
			Message:   "Ping test",
			Timestamp: time.Now(),
		},
	}

	res, err := dispatcher.SendDirect(context.Background(), sub, payload)
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}
