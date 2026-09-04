package memory

import (
	"context"
	"fmt"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListWebhooks(_ context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.WebhookSubscription, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, ports.PageMeta{}, domain.ErrNotFound
	}
	items := make([]domain.WebhookSubscription, 0, len(r.webhooks[appID]))
	for _, subscription := range r.webhooks[appID] {
		if subscription.Status != domain.WebhookStatusDisabled {
			items = append(items, cloneWebhook(subscription))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return paginate(items, filter)
}

func (r *Repository) CreateWebhook(_ context.Context, organizationID string, subscription domain.WebhookSubscription, meta ports.MutationMeta) (domain.WebhookSubscription, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.WebhookSubscription{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[subscription.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.WebhookSubscription{}, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+subscription.AppID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneWebhook(r.webhooks[subscription.AppID][existingID]), nil
	}
	if r.webhooks[subscription.AppID] == nil {
		r.webhooks[subscription.AppID] = make(map[string]domain.WebhookSubscription)
	}
	for _, existing := range r.webhooks[subscription.AppID] {
		if existing.Status != domain.WebhookStatusDisabled && existing.Event == subscription.Event && existing.EndpointURL == subscription.EndpointURL {
			return domain.WebhookSubscription{}, fmt.Errorf("%w: webhook subscription already exists", domain.ErrConflict)
		}
	}
	r.webhooks[subscription.AppID][subscription.ID] = cloneWebhook(subscription)
	r.idempotency[key] = subscription.ID
	r.appendAuditLocked(auditID, organizationID, meta, "webhook_subscription", subscription.ID, map[string]any{"event": subscription.Event})
	return cloneWebhook(subscription), nil
}

func (r *Repository) GetWebhook(_ context.Context, organizationID, appID, webhookID string) (domain.WebhookSubscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.WebhookSubscription{}, domain.ErrNotFound
	}
	subscription, ok := r.webhooks[appID][webhookID]
	if !ok || subscription.Status == domain.WebhookStatusDisabled {
		return domain.WebhookSubscription{}, domain.ErrNotFound
	}
	return cloneWebhook(subscription), nil
}

func (r *Repository) UpdateWebhook(_ context.Context, organizationID string, subscription domain.WebhookSubscription, expectedRevision int64, meta ports.MutationMeta) (domain.WebhookSubscription, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.WebhookSubscription{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[subscription.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.WebhookSubscription{}, domain.ErrNotFound
	}
	current, ok := r.webhooks[subscription.AppID][subscription.ID]
	if !ok || current.Status == domain.WebhookStatusDisabled {
		return domain.WebhookSubscription{}, domain.ErrNotFound
	}
	if current.Revision != expectedRevision {
		return domain.WebhookSubscription{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	for _, existing := range r.webhooks[subscription.AppID] {
		if existing.ID != subscription.ID && existing.Status != domain.WebhookStatusDisabled && existing.Event == subscription.Event && existing.EndpointURL == subscription.EndpointURL {
			return domain.WebhookSubscription{}, fmt.Errorf("%w: webhook subscription already exists", domain.ErrConflict)
		}
	}
	subscription.Revision = current.Revision + 1
	subscription.UpdatedAt = r.now().UTC()
	r.webhooks[subscription.AppID][subscription.ID] = cloneWebhook(subscription)
	r.appendAuditLocked(auditID, organizationID, meta, "webhook_subscription", subscription.ID, map[string]any{"revision": subscription.Revision, "status": subscription.Status})
	return cloneWebhook(subscription), nil
}

func (r *Repository) DisableWebhook(_ context.Context, organizationID, appID, webhookID string, meta ports.MutationMeta) error {
	auditID, err := r.id()
	if err != nil {
		return fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.ErrNotFound
	}
	subscription, ok := r.webhooks[appID][webhookID]
	if !ok || subscription.Status == domain.WebhookStatusDisabled {
		return domain.ErrNotFound
	}
	subscription.Status = domain.WebhookStatusDisabled
	subscription.Revision++
	subscription.UpdatedAt = r.now().UTC()
	r.webhooks[appID][webhookID] = subscription
	r.appendAuditLocked(auditID, organizationID, meta, "webhook_subscription", webhookID, nil)
	return nil
}

func cloneWebhook(subscription domain.WebhookSubscription) domain.WebhookSubscription {
	subscription.SigningSecretCiphertext = append([]byte(nil), subscription.SigningSecretCiphertext...)
	return subscription
}
