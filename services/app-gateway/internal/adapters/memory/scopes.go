package memory

import (
	"context"
	"fmt"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListScopes(_ context.Context, organizationID, appID string) ([]domain.AppScope, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, domain.ErrNotFound
	}
	return sortedScopes(r.scopes[appID]), nil
}

func (r *Repository) ReplaceScopes(_ context.Context, organizationID, appID string, scopes []domain.AppScope, meta ports.MutationMeta) ([]domain.AppScope, error) {
	auditID, err := r.id()
	if err != nil {
		return nil, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, domain.ErrNotFound
	}
	replacement := make(map[string]domain.AppScope, len(scopes))
	for _, scope := range scopes {
		replacement[scope.Scope] = scope
	}
	r.scopes[appID] = replacement
	r.appendAuditLocked(auditID, organizationID, meta, "app", appID, map[string]interface{}{"scopeCount": len(scopes)})
	return sortedScopes(replacement), nil
}

func sortedScopes(items map[string]domain.AppScope) []domain.AppScope {
	result := make([]domain.AppScope, 0, len(items))
	for _, scope := range items {
		result = append(result, scope)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Access == result[j].Access {
			return result[i].Scope < result[j].Scope
		}
		return result[i].Access < result[j].Access
	})
	return result
}
