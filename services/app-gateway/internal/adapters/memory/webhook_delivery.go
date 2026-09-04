package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) EnqueueWebhookEvent(_ context.Context, organizationID string, event domain.WebhookEvent, subscriptionIDs []string, meta ports.MutationMeta) ([]domain.WebhookDelivery, error) {
	auditID, err := r.id()
	if err != nil {
		return nil, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[event.AppID]
	if !ok || app.OrganizationID != organizationID {
		return nil, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+event.AppID, meta.IdempotencyKey)
	if existingEventID := r.idempotency[key]; existingEventID != "" {
		return r.deliveriesForEventLocked(existingEventID), nil
	}
	deliveries := make([]domain.WebhookDelivery, 0, len(subscriptionIDs))
	for _, subscriptionID := range subscriptionIDs {
		subscription, ok := r.webhooks[event.AppID][subscriptionID]
		if !ok || subscription.Status != domain.WebhookStatusActive || subscription.Event != event.Event {
			continue
		}
		deliveryID, err := r.id()
		if err != nil {
			return nil, fmt.Errorf("generate delivery id: %w", err)
		}
		next := event.CreatedAt
		delivery := domain.WebhookDelivery{
			ID: deliveryID, SubscriptionID: subscriptionID, EventID: event.ID, Event: event.Event, EventCreatedAt: event.CreatedAt,
			EventSource: event.Source, MerchantID: cloneStringPointer(event.MerchantID), InstallationID: cloneStringPointer(event.InstallationID),
			Attempt: 1, Status: domain.WebhookDeliveryStatusPending, AttemptedAt: event.CreatedAt,
			NextAttemptAt: &next, EndpointURL: subscription.EndpointURL, Payload: cloneMap(event.Payload),
			SigningCiphertext: append([]byte(nil), subscription.SigningSecretCiphertext...), EncryptionVersion: subscription.EncryptionKeyVersion,
		}
		r.webhookDeliveries[delivery.ID] = cloneWebhookDelivery(delivery)
		deliveries = append(deliveries, cloneWebhookDelivery(delivery))
	}
	if len(deliveries) == 0 {
		return nil, fmt.Errorf("%w: no released webhook subscription is currently active", domain.ErrConflict)
	}
	event.Payload = cloneMap(event.Payload)
	r.webhookEvents[event.ID] = event
	r.idempotency[key] = event.ID
	r.appendAuditLocked(auditID, organizationID, meta, "webhook_event", event.ID, map[string]any{"appId": event.AppID, "event": event.Event, "deliveries": len(deliveries)})
	return deliveries, nil
}

func (r *Repository) deliveriesForEventLocked(eventID string) []domain.WebhookDelivery {
	items := make([]domain.WebhookDelivery, 0)
	for _, delivery := range r.webhookDeliveries {
		if delivery.EventID == eventID {
			items = append(items, cloneWebhookDelivery(delivery))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AttemptedAt.Before(items[j].AttemptedAt) })
	return items
}

func (r *Repository) ListWebhookDeliveries(_ context.Context, organizationID, appID, webhookID string, filter ports.AppFilter) ([]domain.WebhookDelivery, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, ports.PageMeta{}, domain.ErrNotFound
	}
	if _, ok := r.webhooks[appID][webhookID]; !ok {
		return nil, ports.PageMeta{}, domain.ErrNotFound
	}
	items := make([]domain.WebhookDelivery, 0)
	for _, delivery := range r.webhookDeliveries {
		if delivery.SubscriptionID == webhookID {
			items = append(items, cloneWebhookDelivery(delivery))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AttemptedAt.After(items[j].AttemptedAt) })
	return paginate(items, filter)
}

func (r *Repository) ClaimWebhookDeliveries(_ context.Context, limit int, now, leaseExpiredBefore time.Time) ([]domain.WebhookDelivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]domain.WebhookDelivery, 0)
	for id, delivery := range r.webhookDeliveries {
		if delivery.Status != domain.WebhookDeliveryStatusPending || delivery.NextAttemptAt == nil || delivery.NextAttemptAt.After(now) {
			continue
		}
		if claimedAt, claimed := r.webhookClaims[id]; claimed && !claimedAt.Before(leaseExpiredBefore) {
			continue
		}
		active := false
		for _, webhooks := range r.webhooks {
			if subscription, ok := webhooks[delivery.SubscriptionID]; ok && (subscription.Status == domain.WebhookStatusActive || subscription.Status == domain.WebhookStatusFailing) {
				active = true
				break
			}
		}
		if !active {
			continue
		}
		r.webhookClaims[id] = now
		items = append(items, cloneWebhookDelivery(delivery))
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *Repository) CompleteWebhookDelivery(_ context.Context, delivery domain.WebhookDelivery, retry *domain.WebhookDelivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.webhookDeliveries[delivery.ID]
	if !ok || current.Status != domain.WebhookDeliveryStatusPending {
		return fmt.Errorf("%w: webhook delivery is no longer pending", domain.ErrConflict)
	}
	current.Status = delivery.Status
	current.ResponseStatus = delivery.ResponseStatus
	current.ResponseTimeMS = delivery.ResponseTimeMS
	current.ErrorCode = delivery.ErrorCode
	current.CompletedAt = delivery.CompletedAt
	current.NextAttemptAt = nil
	r.webhookDeliveries[current.ID] = current
	delete(r.webhookClaims, current.ID)
	if retry != nil {
		event := r.webhookEvents[retry.EventID]
		for _, webhooks := range r.webhooks {
			if subscription, ok := webhooks[retry.SubscriptionID]; ok {
				retry.Event = event.Event
				retry.EventSource = event.Source
				retry.MerchantID = cloneStringPointer(event.MerchantID)
				retry.InstallationID = cloneStringPointer(event.InstallationID)
				retry.EventCreatedAt = event.CreatedAt
				retry.EndpointURL = subscription.EndpointURL
				retry.Payload = cloneMap(event.Payload)
				retry.SigningCiphertext = append([]byte(nil), subscription.SigningSecretCiphertext...)
				retry.EncryptionVersion = subscription.EncryptionKeyVersion
				break
			}
		}
		r.webhookDeliveries[retry.ID] = cloneWebhookDelivery(*retry)
	}
	for appID, webhooks := range r.webhooks {
		if subscription, ok := webhooks[delivery.SubscriptionID]; ok && (subscription.Status == domain.WebhookStatusActive || subscription.Status == domain.WebhookStatusFailing) {
			if delivery.Status == domain.WebhookDeliveryStatusDelivered {
				subscription.Status = domain.WebhookStatusActive
			} else {
				subscription.Status = domain.WebhookStatusFailing
			}
			subscription.Revision++
			subscription.UpdatedAt = r.now().UTC()
			r.webhooks[appID][delivery.SubscriptionID] = subscription
			break
		}
	}
	return nil
}

func cloneWebhookDelivery(delivery domain.WebhookDelivery) domain.WebhookDelivery {
	delivery.Payload = cloneMap(delivery.Payload)
	delivery.SigningCiphertext = append([]byte(nil), delivery.SigningCiphertext...)
	delivery.MerchantID = cloneStringPointer(delivery.MerchantID)
	delivery.InstallationID = cloneStringPointer(delivery.InstallationID)
	return delivery
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	copy := make(map[string]any, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
