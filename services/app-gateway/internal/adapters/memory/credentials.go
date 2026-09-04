package memory

import (
	"context"
	"fmt"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListCredentials(_ context.Context, organizationID, appID string) ([]domain.AppCredential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, domain.ErrNotFound
	}
	items := make([]domain.AppCredential, 0, len(r.credentials[appID]))
	for _, credential := range r.credentials[appID] {
		items = append(items, cloneCredential(credential))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func (r *Repository) CreateCredential(_ context.Context, organizationID string, credential domain.AppCredential, meta ports.MutationMeta) (domain.AppCredential, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppCredential{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[credential.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppCredential{}, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+credential.AppID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneCredential(r.credentials[credential.AppID][existingID]), nil
	}
	if r.credentials[credential.AppID] == nil {
		r.credentials[credential.AppID] = make(map[string]domain.AppCredential)
	}
	for _, existing := range r.credentials[credential.AppID] {
		if existing.ClientID == credential.ClientID {
			return domain.AppCredential{}, fmt.Errorf("%w: client id already exists", domain.ErrConflict)
		}
	}
	r.credentials[credential.AppID][credential.ID] = cloneCredential(credential)
	r.idempotency[key] = credential.ID
	r.appendAuditLocked(auditID, organizationID, meta, "app_credential", credential.ID, map[string]any{"environment": credential.Environment})
	return cloneCredential(credential), nil
}

func (r *Repository) RotateCredential(_ context.Context, organizationID, appID, credentialID string, ciphertext []byte, fingerprint string, keyVersion int, meta ports.MutationMeta) (domain.AppCredential, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppCredential{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppCredential{}, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+credentialID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneCredential(r.credentials[appID][existingID]), nil
	}
	credential, ok := r.credentials[appID][credentialID]
	if !ok {
		return domain.AppCredential{}, domain.ErrNotFound
	}
	if credential.Status != domain.CredentialStatusActive || (credential.ExpiresAt != nil && !credential.ExpiresAt.After(r.now().UTC())) {
		return domain.AppCredential{}, fmt.Errorf("%w: only active, unexpired credentials can be rotated", domain.ErrConflict)
	}
	now := r.now().UTC()
	credential.SecretCiphertext = append([]byte(nil), ciphertext...)
	credential.SecretFingerprint = fingerprint
	credential.EncryptionKeyVersion = keyVersion
	credential.RotatedAt = &now
	r.credentials[appID][credentialID] = credential
	r.idempotency[key] = credentialID
	r.appendAuditLocked(auditID, organizationID, meta, "app_credential", credentialID, map[string]any{"fingerprint": fingerprint})
	return cloneCredential(credential), nil
}

func (r *Repository) RevokeCredential(_ context.Context, organizationID, appID, credentialID string, meta ports.MutationMeta) error {
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
	key := idempotencyKey(organizationID, meta.Action+":"+credentialID, meta.IdempotencyKey)
	if r.idempotency[key] != "" {
		return nil
	}
	credential, ok := r.credentials[appID][credentialID]
	if !ok {
		return domain.ErrNotFound
	}
	if credential.Status != domain.CredentialStatusRevoked {
		now := r.now().UTC()
		credential.Status = domain.CredentialStatusRevoked
		credential.RevokedAt = &now
		r.credentials[appID][credentialID] = credential
		r.revokeOAuthTokensLocked("", credentialID, now)
		r.appendAuditLocked(auditID, organizationID, meta, "app_credential", credentialID, nil)
	}
	r.idempotency[key] = credentialID
	return nil
}

func cloneCredential(credential domain.AppCredential) domain.AppCredential {
	credential.SecretCiphertext = append([]byte(nil), credential.SecretCiphertext...)
	if credential.LastUsedAt != nil {
		value := *credential.LastUsedAt
		credential.LastUsedAt = &value
	}
	if credential.ExpiresAt != nil {
		value := *credential.ExpiresAt
		credential.ExpiresAt = &value
	}
	if credential.RotatedAt != nil {
		value := *credential.RotatedAt
		credential.RotatedAt = &value
	}
	if credential.RevokedAt != nil {
		value := *credential.RevokedAt
		credential.RevokedAt = &value
	}
	return credential
}
