package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListExtensions(_ context.Context, organizationID, appID string) ([]domain.AppExtension, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, domain.ErrNotFound
	}
	items := make([]domain.AppExtension, 0, len(r.extensions[appID]))
	for _, extension := range r.extensions[appID] {
		items = append(items, cloneExtension(extension))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func (r *Repository) CreateExtension(_ context.Context, organizationID string, extension domain.AppExtension, meta ports.MutationMeta) (domain.AppExtension, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppExtension{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[extension.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppExtension{}, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+extension.AppID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneExtension(r.extensions[extension.AppID][existingID]), nil
	}
	if r.extensions[extension.AppID] == nil {
		r.extensions[extension.AppID] = make(map[string]domain.AppExtension)
	}
	for _, existing := range r.extensions[extension.AppID] {
		if existing.Name == extension.Name {
			return domain.AppExtension{}, fmt.Errorf("%w: extension name already exists", domain.ErrConflict)
		}
	}
	r.extensions[extension.AppID][extension.ID] = cloneExtension(extension)
	r.idempotency[key] = extension.ID
	r.appendAuditLocked(auditID, organizationID, meta, "app_extension", extension.ID, map[string]interface{}{"type": extension.Type})
	return cloneExtension(extension), nil
}

func (r *Repository) GetExtension(_ context.Context, organizationID, appID, extensionID string) (domain.AppExtension, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppExtension{}, domain.ErrNotFound
	}
	extension, ok := r.extensions[appID][extensionID]
	if !ok {
		return domain.AppExtension{}, domain.ErrNotFound
	}
	return cloneExtension(extension), nil
}

func (r *Repository) UpdateExtension(_ context.Context, organizationID string, extension domain.AppExtension, expectedRevision int64, meta ports.MutationMeta) (domain.AppExtension, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppExtension{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[extension.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppExtension{}, domain.ErrNotFound
	}
	current, ok := r.extensions[extension.AppID][extension.ID]
	if !ok {
		return domain.AppExtension{}, domain.ErrNotFound
	}
	if current.Revision != expectedRevision {
		return domain.AppExtension{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	for _, existing := range r.extensions[extension.AppID] {
		if existing.ID != extension.ID && existing.Name == extension.Name {
			return domain.AppExtension{}, fmt.Errorf("%w: extension name already exists", domain.ErrConflict)
		}
	}
	extension.CreatedAt = current.CreatedAt
	extension.Revision = current.Revision + 1
	extension.UpdatedAt = r.now().UTC()
	r.extensions[extension.AppID][extension.ID] = cloneExtension(extension)
	r.appendAuditLocked(auditID, organizationID, meta, "app_extension", extension.ID, map[string]interface{}{"revision": extension.Revision})
	return cloneExtension(extension), nil
}

func (r *Repository) DisableExtension(_ context.Context, organizationID, appID, extensionID string, meta ports.MutationMeta) error {
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
	extension, ok := r.extensions[appID][extensionID]
	if !ok {
		return domain.ErrNotFound
	}
	if extension.Status == domain.ExtensionStatusDisabled {
		return nil
	}
	extension.Status = domain.ExtensionStatusDisabled
	extension.Revision++
	extension.UpdatedAt = r.now().UTC()
	r.extensions[appID][extensionID] = extension
	r.appendAuditLocked(auditID, organizationID, meta, "app_extension", extensionID, nil)
	return nil
}

func cloneExtension(extension domain.AppExtension) domain.AppExtension {
	if extension.RuntimeURL != nil {
		value := *extension.RuntimeURL
		extension.RuntimeURL = &value
	}
	if extension.Configuration == nil {
		extension.Configuration = map[string]interface{}{}
		return extension
	}
	encoded, err := json.Marshal(extension.Configuration)
	if err == nil {
		var configuration map[string]interface{}
		if json.Unmarshal(encoded, &configuration) == nil {
			extension.Configuration = configuration
		}
	}
	return extension
}
