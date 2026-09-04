package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListInstallations(_ context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.AppInstallation, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, ports.PageMeta{}, domain.ErrNotFound
	}
	items := make([]domain.AppInstallation, 0, len(r.installations[appID]))
	for _, installation := range r.installations[appID] {
		items = append(items, cloneInstallation(installation))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return paginate(items, filter)
}

func (r *Repository) CreateInstallation(_ context.Context, organizationID string, installation domain.AppInstallation, expectedActiveVersionID string, meta ports.MutationMeta) (domain.AppInstallation, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[installation.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppInstallation{}, domain.ErrNotFound
	}
	if app.ActiveVersionID == nil || *app.ActiveVersionID != expectedActiveVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: active version changed", domain.ErrConflict)
	}
	key := idempotencyKey(organizationID, meta.Action+":"+installation.AppID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneInstallation(r.installations[installation.AppID][existingID]), nil
	}
	if r.installations[installation.AppID] == nil {
		r.installations[installation.AppID] = make(map[string]domain.AppInstallation)
	}
	for _, existing := range r.installations[installation.AppID] {
		if existing.MerchantID == installation.MerchantID && existing.Environment == installation.Environment && existing.Status != domain.InstallationStatusUninstalled {
			return domain.AppInstallation{}, fmt.Errorf("%w: merchant already has an installation in this environment", domain.ErrConflict)
		}
	}
	r.installations[installation.AppID][installation.ID] = cloneInstallation(installation)
	r.idempotency[key] = installation.ID
	r.appendAuditLocked(auditID, organizationID, meta, "app_installation", installation.ID, map[string]any{"merchantId": installation.MerchantID, "environment": installation.Environment, "versionId": installation.InstalledVersionID})
	return cloneInstallation(installation), nil
}

func (r *Repository) GetInstallation(_ context.Context, organizationID, appID, installationID string) (domain.AppInstallation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppInstallation{}, domain.ErrNotFound
	}
	installation, ok := r.installations[appID][installationID]
	if !ok {
		return domain.AppInstallation{}, domain.ErrNotFound
	}
	return cloneInstallation(installation), nil
}

func (r *Repository) UpdateInstallation(_ context.Context, organizationID string, installation domain.AppInstallation, expectedRevision int64, meta ports.MutationMeta) (domain.AppInstallation, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[installation.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppInstallation{}, domain.ErrNotFound
	}
	current, ok := r.installations[installation.AppID][installation.ID]
	if !ok {
		return domain.AppInstallation{}, domain.ErrNotFound
	}
	if current.Revision != expectedRevision {
		return domain.AppInstallation{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	installation.Revision = current.Revision + 1
	installation.UpdatedAt = r.now().UTC()
	r.installations[installation.AppID][installation.ID] = cloneInstallation(installation)
	r.appendAuditLocked(auditID, organizationID, meta, "app_installation", installation.ID, map[string]any{"status": installation.Status, "revision": installation.Revision})
	return cloneInstallation(installation), nil
}

func (r *Repository) UpgradeInstallation(_ context.Context, organizationID, appID, installationID, targetVersionID, expectedInstalledVersionID string, grantedScopes []string, expectedRevision int64, meta ports.MutationMeta) (domain.AppInstallation, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppInstallation{}, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+installationID, meta.IdempotencyKey)
	if r.idempotency[key] != "" {
		return cloneInstallation(r.installations[appID][installationID]), nil
	}
	if app.ActiveVersionID == nil || *app.ActiveVersionID != targetVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: target version is no longer active", domain.ErrConflict)
	}
	installation, ok := r.installations[appID][installationID]
	if !ok {
		return domain.AppInstallation{}, domain.ErrNotFound
	}
	if installation.Status == domain.InstallationStatusUninstalled {
		return domain.AppInstallation{}, fmt.Errorf("%w: uninstalled records cannot be upgraded", domain.ErrConflict)
	}
	if installation.Revision != expectedRevision {
		return domain.AppInstallation{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, installation.Revision)
	}
	if installation.InstalledVersionID != expectedInstalledVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: installed version changed", domain.ErrConflict)
	}
	if installation.InstalledVersionID == targetVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: installation already uses the active version", domain.ErrConflict)
	}
	fromVersionID := installation.InstalledVersionID
	installation.InstalledVersionID = targetVersionID
	installation.GrantedScopes = append([]string(nil), grantedScopes...)
	installation.Revision++
	installation.UpdatedAt = r.now().UTC()
	r.installations[appID][installationID] = cloneInstallation(installation)
	for id, token := range r.oauthTokens {
		if token.InstallationID == installationID && token.RevokedAt == nil {
			token.Scopes = append([]string(nil), grantedScopes...)
			r.oauthTokens[id] = token
		}
	}
	r.idempotency[key] = installationID
	r.appendAuditLocked(auditID, organizationID, meta, "app_installation", installationID, map[string]any{
		"fromVersionId": fromVersionID,
		"toVersionId":   targetVersionID,
		"grantedScopes": append([]string(nil), grantedScopes...),
		"revision":      installation.Revision,
	})
	return cloneInstallation(installation), nil
}

func (r *Repository) UninstallInstallation(_ context.Context, organizationID, appID, installationID string, meta ports.MutationMeta) error {
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
	key := idempotencyKey(organizationID, meta.Action+":"+installationID, meta.IdempotencyKey)
	if r.idempotency[key] != "" {
		return nil
	}
	installation, ok := r.installations[appID][installationID]
	if !ok {
		return domain.ErrNotFound
	}
	if installation.Status != domain.InstallationStatusUninstalled {
		now := r.now().UTC()
		if err := r.enqueueAppUninstalledEventLocked(installation, now); err != nil {
			return err
		}
		installation.Status = domain.InstallationStatusUninstalled
		installation.UninstalledAt = &now
		installation.UpdatedAt = now
		installation.Revision++
		r.installations[appID][installationID] = installation
		r.revokeOAuthTokensLocked(installationID, "", now)
		r.cancelAppBillingLocked(installation)
		r.appendAuditLocked(auditID, organizationID, meta, "app_installation", installationID, nil)
	}
	r.idempotency[key] = installationID
	return nil
}

func (r *Repository) enqueueAppUninstalledEventLocked(installation domain.AppInstallation, occurredAt time.Time) error {
	version, ok := r.versions[installation.AppID][installation.InstalledVersionID]
	if !ok {
		return domain.ErrNotFound
	}
	subscriptionIDs := make([]string, 0)
	for _, snapshot := range version.Snapshot.WebhookSubscriptions {
		if snapshot.Event != "app/uninstalled" {
			continue
		}
		subscription, ok := r.webhooks[installation.AppID][snapshot.SubscriptionID]
		if ok && (subscription.Status == domain.WebhookStatusActive || subscription.Status == domain.WebhookStatusFailing) {
			subscriptionIDs = append(subscriptionIDs, subscription.ID)
		}
	}
	if len(subscriptionIDs) == 0 {
		return nil
	}
	eventID, err := r.id()
	if err != nil {
		return fmt.Errorf("generate app uninstall event id: %w", err)
	}
	merchantID := installation.MerchantID
	installationID := installation.ID
	event := domain.WebhookEvent{
		ID: eventID, AppID: installation.AppID, Event: "app/uninstalled", Source: domain.WebhookEventSourceAppPlatform,
		MerchantID: &merchantID, InstallationID: &installationID,
		Payload: map[string]any{"appId": installation.AppID, "uninstalledAt": occurredAt}, CreatedAt: occurredAt,
	}
	for _, subscriptionID := range subscriptionIDs {
		subscription := r.webhooks[installation.AppID][subscriptionID]
		deliveryID, err := r.id()
		if err != nil {
			return fmt.Errorf("generate app uninstall delivery id: %w", err)
		}
		next := occurredAt
		r.webhookDeliveries[deliveryID] = domain.WebhookDelivery{
			ID: deliveryID, SubscriptionID: subscriptionID, EventID: eventID, Event: event.Event,
			EventSource: event.Source, MerchantID: &merchantID, InstallationID: &installationID, EventCreatedAt: occurredAt,
			Attempt: 1, Status: domain.WebhookDeliveryStatusPending, AttemptedAt: occurredAt, NextAttemptAt: &next,
			EndpointURL: subscription.EndpointURL, Payload: cloneMap(event.Payload),
			SigningCiphertext: append([]byte(nil), subscription.SigningSecretCiphertext...), EncryptionVersion: subscription.EncryptionKeyVersion,
		}
	}
	r.webhookEvents[eventID] = event
	return nil
}

func cloneInstallation(installation domain.AppInstallation) domain.AppInstallation {
	installation.GrantedScopes = append([]string(nil), installation.GrantedScopes...)
	if installation.MerchantDomain != nil {
		value := *installation.MerchantDomain
		installation.MerchantDomain = &value
	}
	if installation.UninstalledAt != nil {
		value := *installation.UninstalledAt
		installation.UninstalledAt = &value
	}
	return installation
}
