package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

const maxWebhookPayloadBytes = 256 << 10

type PublishWebhookCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	AppID          string
	Event          string
	Payload        map[string]any
}

type PublishWebhookResponse struct {
	Event      domain.WebhookEvent      `json:"event"`
	Deliveries []domain.WebhookDelivery `json:"deliveries"`
}

func (s *WebhookService) Publish(ctx context.Context, command PublishWebhookCommand) (PublishWebhookResponse, error) {
	eventName, err := validateWebhookEvent(command.Event)
	if err != nil {
		return PublishWebhookResponse{}, err
	}
	if _, err := s.availableEventDefinition(ctx, eventName); err != nil {
		return PublishWebhookResponse{}, err
	}
	if command.Payload == nil {
		return PublishWebhookResponse{}, fmt.Errorf("%w: payload must be a JSON object", domain.ErrValidation)
	}
	encoded, err := json.Marshal(command.Payload)
	if err != nil || len(encoded) > maxWebhookPayloadBytes {
		return PublishWebhookResponse{}, fmt.Errorf("%w: payload must be a JSON object no larger than 256 KiB", domain.ErrValidation)
	}
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return PublishWebhookResponse{}, err
	}
	if app.Status != domain.AppStatusActive || app.ActiveVersionID == nil {
		return PublishWebhookResponse{}, fmt.Errorf("%w: app must have an active version before publishing events", domain.ErrConflict)
	}
	version, err := s.repository.GetVersion(ctx, command.OrganizationID, command.AppID, *app.ActiveVersionID)
	if err != nil {
		return PublishWebhookResponse{}, err
	}
	subscriptionIDs := make([]string, 0)
	for _, subscription := range version.Snapshot.WebhookSubscriptions {
		if subscription.Event == eventName {
			subscriptionIDs = append(subscriptionIDs, subscription.SubscriptionID)
		}
	}
	if len(subscriptionIDs) == 0 {
		return PublishWebhookResponse{}, fmt.Errorf("%w: active app version has no subscription for this event", domain.ErrConflict)
	}
	eventID, err := s.id()
	if err != nil {
		return PublishWebhookResponse{}, fmt.Errorf("generate webhook event id: %w", err)
	}
	event := domain.WebhookEvent{ID: eventID, AppID: command.AppID, Event: eventName, Source: domain.WebhookEventSourceTest, Payload: command.Payload, CreatedAt: s.now().UTC()}
	deliveries, err := s.repository.EnqueueWebhookEvent(ctx, command.OrganizationID, event, subscriptionIDs, ports.MutationMeta{ActorID: command.ActorID, Action: "webhook.event.enqueued", IdempotencyKey: command.IdempotencyKey})
	if err != nil {
		return PublishWebhookResponse{}, err
	}
	return PublishWebhookResponse{Event: event, Deliveries: deliveries}, nil
}

func (s *WebhookService) ListDeliveries(ctx context.Context, organizationID, appID, webhookID string, filter ports.AppFilter) ([]domain.WebhookDelivery, ports.PageMeta, error) {
	return s.repository.ListWebhookDeliveries(ctx, organizationID, appID, webhookID, filter)
}

type WebhookHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type WebhookDispatcher struct {
	repository  ports.Repository
	cipher      SecretCipher
	client      WebhookHTTPClient
	id          ids.Generator
	now         func() time.Time
	logger      *slog.Logger
	batchSize   int
	maxAttempts int
	baseRetry   time.Duration
	maxRetry    time.Duration
}

type WebhookDispatcherOptions struct {
	BatchSize   int
	MaxAttempts int
	BaseRetry   time.Duration
	MaxRetry    time.Duration
}

func NewWebhookDispatcher(repository ports.Repository, cipher SecretCipher, client WebhookHTTPClient, id ids.Generator, now func() time.Time, logger *slog.Logger, options WebhookDispatcherOptions) *WebhookDispatcher {
	return &WebhookDispatcher{repository: repository, cipher: cipher, client: client, id: id, now: now, logger: logger, batchSize: options.BatchSize, maxAttempts: options.MaxAttempts, baseRetry: options.BaseRetry, maxRetry: options.MaxRetry}
}

func (d *WebhookDispatcher) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := d.ProcessBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Error("process webhook delivery batch", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *WebhookDispatcher) ProcessBatch(ctx context.Context) (int, error) {
	now := d.now().UTC()
	deliveries, err := d.repository.ClaimWebhookDeliveries(ctx, d.batchSize, now, now.Add(-2*time.Minute))
	if err != nil {
		return 0, err
	}
	for _, delivery := range deliveries {
		d.process(ctx, delivery)
	}
	return len(deliveries), nil
}

func (d *WebhookDispatcher) process(ctx context.Context, delivery domain.WebhookDelivery) {
	completed := d.now().UTC()
	delivery.CompletedAt = &completed
	retryable := false
	secret, err := d.cipher.Decrypt(delivery.SigningCiphertext)
	if err != nil {
		code := "signing_secret_unavailable"
		delivery.ErrorCode = &code
	} else {
		envelope := map[string]any{
			"id": delivery.EventID, "event": delivery.Event, "apiVersion": "2026-09-01",
			"source": delivery.EventSource, "createdAt": delivery.EventCreatedAt, "data": delivery.Payload,
		}
		if delivery.MerchantID != nil {
			envelope["merchantId"] = *delivery.MerchantID
		}
		if delivery.InstallationID != nil {
			envelope["installationId"] = *delivery.InstallationID
		}
		body, marshalErr := json.Marshal(envelope)
		if marshalErr != nil {
			code := "payload_encoding_failed"
			delivery.ErrorCode = &code
		} else {
			started := time.Now()
			request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, delivery.EndpointURL, strings.NewReader(string(body)))
			if requestErr != nil {
				code := "invalid_endpoint"
				delivery.ErrorCode = &code
			} else {
				timestamp := strconv.FormatInt(completed.Unix(), 10)
				mac := hmac.New(sha256.New, secret)
				_, _ = mac.Write([]byte(timestamp + "."))
				_, _ = mac.Write(body)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("User-Agent", "Emisell-Webhooks/1.0")
				request.Header.Set("X-Emisell-Event", delivery.Event)
				request.Header.Set("X-Emisell-Event-Id", delivery.EventID)
				request.Header.Set("X-Emisell-Delivery-Id", delivery.ID)
				request.Header.Set("X-Emisell-Webhook-Signature", "t="+timestamp+",v1="+hex.EncodeToString(mac.Sum(nil)))
				response, requestErr := d.client.Do(request)
				elapsed := time.Since(started).Milliseconds()
				delivery.ResponseTimeMS = &elapsed
				if requestErr != nil {
					code := "network_error"
					if errors.Is(requestErr, context.DeadlineExceeded) || errors.Is(requestErr, context.Canceled) {
						code = "timeout"
					}
					delivery.ErrorCode = &code
					retryable = true
				} else {
					status := response.StatusCode
					delivery.ResponseStatus = &status
					_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
					_ = response.Body.Close()
					if status >= 200 && status < 300 {
						delivery.Status = domain.WebhookDeliveryStatusDelivered
					} else {
						code := "http_3xx"
						if status >= 400 {
							code = "http_4xx"
						}
						if status == http.StatusTooManyRequests {
							code, retryable = "http_429", true
						} else if status >= 500 {
							code, retryable = "http_5xx", true
						}
						delivery.ErrorCode = &code
					}
				}
			}
		}
	}
	if delivery.Status != domain.WebhookDeliveryStatusDelivered {
		delivery.Status = domain.WebhookDeliveryStatusFailed
	}
	var retry *domain.WebhookDelivery
	if retryable && delivery.Attempt < d.maxAttempts {
		retryID, idErr := d.id()
		if idErr == nil {
			nextAt := completed.Add(d.retryDelay(delivery.Attempt))
			retry = &domain.WebhookDelivery{ID: retryID, SubscriptionID: delivery.SubscriptionID, EventID: delivery.EventID, Event: delivery.Event, Attempt: delivery.Attempt + 1, Status: domain.WebhookDeliveryStatusPending, AttemptedAt: nextAt, NextAttemptAt: &nextAt}
		}
	}
	if err := d.repository.CompleteWebhookDelivery(ctx, delivery, retry); err != nil {
		d.logger.Error("complete webhook delivery", "delivery_id", delivery.ID, "error", err)
	}
}

func (d *WebhookDispatcher) retryDelay(attempt int) time.Duration {
	delay := d.baseRetry
	for current := 1; current < attempt; current++ {
		delay *= 2
		if delay >= d.maxRetry {
			return d.maxRetry
		}
	}
	return delay
}

func NewSafeWebhookHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       60 * time.Second,
		MaxIdleConns:          50,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid webhook address: %w", err)
			}
			resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(resolved) == 0 {
				return nil, fmt.Errorf("resolve webhook endpoint")
			}
			for _, candidate := range resolved {
				if unsafeWebhookIP(candidate.IP) {
					return nil, fmt.Errorf("webhook endpoint resolves to a restricted network")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(resolved[0].IP.String(), port))
		},
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func unsafeWebhookIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
