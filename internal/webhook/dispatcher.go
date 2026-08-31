package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// Default delivery policy values.
var DefaultRetryDelays = []time.Duration{
	30 * time.Second,
	5 * time.Minute,
	30 * time.Minute,
}

const (
	DefaultHTTPTimeout            = 10 * time.Second
	DefaultQueueCapacity          = 1024
	DefaultMaxConsecutiveFailures = 50
)

// HTTPClient interface allows mocking HTTP calls in unit tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DeliveryResult represents the outcome of a webhook HTTP delivery attempt.
type DeliveryResult struct {
	StatusCode   int           `json:"status_code"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	ErrorMessage string        `json:"error_message,omitempty"`
	ResponseBody string        `json:"response_body,omitempty"`
}

// DispatcherConfig contains configuration options for the webhook dispatcher.
type DispatcherConfig struct {
	HTTPClient             HTTPClient
	RetryDelays            []time.Duration
	HTTPTimeout            time.Duration
	QueueCapacity          int
	MaxConsecutiveFailures int
	Logger                 *slog.Logger
}

// Dispatcher defines operations for dispatching outbound webhooks.
type Dispatcher interface {
	Start(ctx context.Context)
	Stop()
	Dispatch(ctx context.Context, event string, data interface{}) error
	SendDirect(ctx context.Context, sub *domain.WebhookSubscription, payload *domain.WebhookPayload) (*DeliveryResult, error)
}

type deliveryTask struct {
	subscription *domain.WebhookSubscription
	payload      *domain.WebhookPayload
	rawBody      []byte
	attempt      int // 0-based attempt count (0 is initial attempt)
}

type dispatcher struct {
	repo                   domain.WebhookSubscriptionRepository
	client                 HTTPClient
	retryDelays            []time.Duration
	httpTimeout            time.Duration
	maxConsecutiveFailures int
	logger                 *slog.Logger

	queue    chan deliveryTask
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	failures map[string]int // subscription ID -> consecutive failure count
}

// NewDispatcher creates a new outbound webhook dispatcher and worker.
func NewDispatcher(repo domain.WebhookSubscriptionRepository, cfg *DispatcherConfig) Dispatcher {
	if cfg == nil {
		cfg = &DispatcherConfig{}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.HTTPTimeout
		if timeout == 0 {
			timeout = DefaultHTTPTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	retryDelays := cfg.RetryDelays
	if retryDelays == nil {
		retryDelays = DefaultRetryDelays
	}

	queueCap := cfg.QueueCapacity
	if queueCap <= 0 {
		queueCap = DefaultQueueCapacity
	}

	maxFailures := cfg.MaxConsecutiveFailures
	if maxFailures <= 0 {
		maxFailures = DefaultMaxConsecutiveFailures
	}

	return &dispatcher{
		repo:                   repo,
		client:                 httpClient,
		retryDelays:            retryDelays,
		httpTimeout:            cfg.HTTPTimeout,
		maxConsecutiveFailures: maxFailures,
		logger:                 cfg.Logger,
		queue:                  make(chan deliveryTask, queueCap),
		stopCh:                 make(chan struct{}),
		failures:               make(map[string]int),
	}
}

// Start launches the background delivery worker.
func (d *dispatcher) Start(ctx context.Context) {
	d.wg.Add(1)
	go d.workerLoop(ctx)
}

// Stop signals the background worker to stop and waits for ongoing deliveries to finish.
func (d *dispatcher) Stop() {
	d.mu.Lock()
	select {
	case <-d.stopCh:
		d.mu.Unlock()
		return
	default:
		close(d.stopCh)
	}
	d.mu.Unlock()

	d.wg.Wait()
}

// Dispatch queries active subscriptions for the event topic and enqueues tasks for delivery.
func (d *dispatcher) Dispatch(ctx context.Context, event string, data interface{}) error {
	if d.repo == nil {
		return nil
	}

	queryCtx := ctx
	if queryCtx == nil || queryCtx.Err() != nil {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	subs, err := d.repo.ListActiveByEvent(queryCtx, event)
	if err != nil {
		if d.logger != nil {
			d.logger.Error("failed to list active webhook subscriptions", "event", event, "error", err)
		}
		return fmt.Errorf("failed to list active webhook subscriptions: %w", err)
	}

	if len(subs) == 0 {
		return nil
	}

	payload := &domain.WebhookPayload{
		ID:        "evt_" + domain.NewID(),
		Event:     event,
		CreatedAt: time.Now().UTC(),
		Data:      data,
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	for _, sub := range subs {
		task := deliveryTask{
			subscription: sub,
			payload:      payload,
			rawBody:      rawBody,
			attempt:      0,
		}

		select {
		case d.queue <- task:
		default:
			if d.logger != nil {
				d.logger.Warn("webhook delivery queue full, dropping task", "subscription_id", sub.ID, "event", event)
			}
		}
	}

	return nil
}

// SendDirect synchronously executes delivery of a webhook payload to a subscription.
func (d *dispatcher) SendDirect(ctx context.Context, sub *domain.WebhookSubscription, payload *domain.WebhookPayload) (*DeliveryResult, error) {
	rawBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	return d.executeDelivery(ctx, sub, rawBody)
}

func (d *dispatcher) workerLoop(ctx context.Context) {
	defer d.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case task, ok := <-d.queue:
			if !ok {
				return
			}
			d.processTask(ctx, task)
		}
	}
}

func (d *dispatcher) processTask(ctx context.Context, task deliveryTask) {
	result, err := d.executeDelivery(ctx, task.subscription, task.rawBody)
	subID := task.subscription.ID

	if err == nil && result.Success {
		// Delivery succeeded: reset failure count
		d.mu.Lock()
		d.failures[subID] = 0
		d.mu.Unlock()
		return
	}

	// Delivery failed
	if result != nil && result.StatusCode == http.StatusGone {
		// 410 Gone: Deactivate immediately, do not retry
		if d.logger != nil {
			d.logger.Warn("webhook endpoint returned 410 Gone, deactivating subscription", "subscription_id", subID)
		}
		d.deactivateSubscription(ctx, task.subscription)
		return
	}

	d.mu.Lock()
	d.failures[subID]++
	consecutiveFailures := d.failures[subID]
	d.mu.Unlock()

	if consecutiveFailures >= d.maxConsecutiveFailures {
		if d.logger != nil {
			d.logger.Warn("webhook endpoint failed consecutive attempts threshold, deactivating subscription",
				"subscription_id", subID,
				"failures", consecutiveFailures,
			)
		}
		d.deactivateSubscription(ctx, task.subscription)
		return
	}

	// Check if retry attempts remain
	if task.attempt < len(d.retryDelays) {
		delay := d.retryDelays[task.attempt]
		nextAttempt := task.attempt + 1

		if d.logger != nil {
			d.logger.Info("scheduling webhook retry",
				"subscription_id", subID,
				"attempt", nextAttempt,
				"delay", delay,
			)
		}

		go func(t deliveryTask, delay time.Duration) {
			select {
			case <-ctx.Done():
				return
			case <-d.stopCh:
				return
			case <-time.After(delay):
				t.attempt = nextAttempt
				select {
				case d.queue <- t:
				case <-ctx.Done():
				case <-d.stopCh:
				}
			}
		}(task, delay)
	}
}

func (d *dispatcher) executeDelivery(ctx context.Context, sub *domain.WebhookSubscription, rawBody []byte) (*DeliveryResult, error) {
	timestamp := time.Now().UTC().Unix()
	signature := ComputeSignature(sub.SecretToken, timestamp, rawBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.TargetURL, bytes.NewReader(rawBody))
	if err != nil {
		return &DeliveryResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderSignature, signature)
	req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", timestamp))

	start := time.Now()
	resp, err := d.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return &DeliveryResult{
			Success:      false,
			Duration:     duration,
			ErrorMessage: err.Error(),
		}, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	respBody := string(bodyBytes)

	isSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300
	var errMsg string
	if !isSuccess {
		errMsg = fmt.Sprintf("HTTP status %d", resp.StatusCode)
	}

	return &DeliveryResult{
		StatusCode:   resp.StatusCode,
		Success:      isSuccess,
		Duration:     duration,
		ErrorMessage: errMsg,
		ResponseBody: respBody,
	}, nil
}

func (d *dispatcher) deactivateSubscription(ctx context.Context, sub *domain.WebhookSubscription) {
	if d.repo == nil {
		return
	}
	updated := *sub
	updated.IsActive = false
	if err := d.repo.Update(ctx, &updated); err != nil {
		if d.logger != nil {
			d.logger.Error("failed to deactivate webhook subscription in store", "subscription_id", sub.ID, "error", err)
		}
	}
}
