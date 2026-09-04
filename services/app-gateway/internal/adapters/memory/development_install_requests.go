package memory

import (
	"context"
	"fmt"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListDevelopmentInstallRequests(_ context.Context, organizationID, appID string) ([]domain.DevelopmentInstallRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, domain.ErrNotFound
	}
	items := make([]domain.DevelopmentInstallRequest, 0, len(r.developmentInstallRequests[appID]))
	for _, request := range r.developmentInstallRequests[appID] {
		items = append(items, cloneDevelopmentInstallRequest(request))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func (r *Repository) CreateDevelopmentInstallRequest(_ context.Context, organizationID string, request domain.DevelopmentInstallRequest, meta ports.MutationMeta) (domain.DevelopmentInstallRequest, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[request.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.DevelopmentInstallRequest{}, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+request.AppID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneDevelopmentInstallRequest(r.developmentInstallRequests[request.AppID][existingID]), nil
	}
	if app.ActiveVersionID == nil || *app.ActiveVersionID != request.VersionID {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: active version changed", domain.ErrConflict)
	}
	if r.developmentInstallRequests[request.AppID] == nil {
		r.developmentInstallRequests[request.AppID] = make(map[string]domain.DevelopmentInstallRequest)
	}
	for _, existing := range r.developmentInstallRequests[request.AppID] {
		if existing.MerchantID == request.MerchantID && existing.Status == domain.DevelopmentInstallRequestStatusPending {
			if !existing.ExpiresAt.After(request.CreatedAt) {
				existing.Status = domain.DevelopmentInstallRequestStatusExpired
				existing.UpdatedAt = request.CreatedAt
				r.developmentInstallRequests[request.AppID][existing.ID] = existing
				continue
			}
			return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: a pending test request already exists for this merchant", domain.ErrConflict)
		}
	}
	r.developmentInstallRequests[request.AppID][request.ID] = cloneDevelopmentInstallRequest(request)
	r.idempotency[key] = request.ID
	r.appendAuditLocked(auditID, organizationID, meta, "development_install_request", request.ID, map[string]any{
		"appId": request.AppID, "merchantId": request.MerchantID, "versionId": request.VersionID,
	})
	return cloneDevelopmentInstallRequest(request), nil
}

func (r *Repository) GetDevelopmentInstallRequest(_ context.Context, organizationID, appID, requestID string) (domain.DevelopmentInstallRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.DevelopmentInstallRequest{}, domain.ErrNotFound
	}
	request, ok := r.developmentInstallRequests[appID][requestID]
	if !ok {
		return domain.DevelopmentInstallRequest{}, domain.ErrNotFound
	}
	return cloneDevelopmentInstallRequest(request), nil
}

func cloneDevelopmentInstallRequest(request domain.DevelopmentInstallRequest) domain.DevelopmentInstallRequest {
	request.MerchantDomain = cloneString(request.MerchantDomain)
	if request.AuthorizedAt != nil {
		value := *request.AuthorizedAt
		request.AuthorizedAt = &value
	}
	if request.AuthorizationID != nil {
		value := *request.AuthorizationID
		request.AuthorizationID = &value
	}
	return request
}
