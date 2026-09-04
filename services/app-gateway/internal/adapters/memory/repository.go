package memory

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type Repository struct {
	appPlans                    map[string][]domain.AppPlan
	appBilling                  map[string]ports.AppBillingState
	extensionConnections        map[string]domain.ExtensionConnection
	extensionCredentialAccesses []string
	mu                          sync.RWMutex
	apps                        map[string]domain.App
	versions                    map[string]map[string]domain.AppVersion
	extensions                  map[string]map[string]domain.AppExtension
	scopes                      map[string]map[string]domain.AppScope
	credentials                 map[string]map[string]domain.AppCredential
	webhooks                    map[string]map[string]domain.WebhookSubscription
	webhookEventDefinitions     map[string]domain.WebhookEventDefinition
	installations               map[string]map[string]domain.AppInstallation
	developmentInstallRequests  map[string]map[string]domain.DevelopmentInstallRequest
	oauthAuthorizations         map[string]domain.OAuthAuthorization
	oauthTokens                 map[string]domain.OAuthAccessToken
	webhookEvents               map[string]domain.WebhookEvent
	webhookDeliveries           map[string]domain.WebhookDelivery
	webhookClaims               map[string]time.Time
	developerApplications       map[string]domain.DeveloperApplication
	developerInvitations        map[string]domain.DeveloperInvitation
	organizationEntitlements    map[string]domain.OrganizationEntitlement
	developerOrganizations      map[string]domain.DeveloperOrganization
	organizationMembers         map[string]map[string]domain.DeveloperOrganizationMember
	identitySessions            map[string]domain.IdentitySession
	merchantIdentities          map[string]domain.MerchantIdentity
	catalogListings             map[string]domain.AppCatalogListing
	merchantSessionGrants       map[string]domain.MerchantSessionGrant
	oidcLoginStates             map[string]domain.OIDCLoginState
	identityMemberships         map[string][]domain.OrganizationMembership
	oidcUsers                   map[string]string
	auditEvents                 map[string][]domain.AuditEvent
	idempotency                 map[string]string
	id                          ids.Generator
	now                         func() time.Time
}

func NewRepository(id ids.Generator, now func() time.Time) *Repository {
	return &Repository{
		appPlans:                   make(map[string][]domain.AppPlan),
		appBilling:                 make(map[string]ports.AppBillingState),
		extensionConnections:       make(map[string]domain.ExtensionConnection),
		apps:                       make(map[string]domain.App),
		versions:                   make(map[string]map[string]domain.AppVersion),
		extensions:                 make(map[string]map[string]domain.AppExtension),
		scopes:                     make(map[string]map[string]domain.AppScope),
		credentials:                make(map[string]map[string]domain.AppCredential),
		webhooks:                   make(map[string]map[string]domain.WebhookSubscription),
		webhookEventDefinitions:    developmentWebhookEventDefinitions(),
		installations:              make(map[string]map[string]domain.AppInstallation),
		developmentInstallRequests: make(map[string]map[string]domain.DevelopmentInstallRequest),
		oauthAuthorizations:        make(map[string]domain.OAuthAuthorization),
		oauthTokens:                make(map[string]domain.OAuthAccessToken),
		webhookEvents:              make(map[string]domain.WebhookEvent),
		webhookDeliveries:          make(map[string]domain.WebhookDelivery),
		webhookClaims:              make(map[string]time.Time),
		developerApplications:      make(map[string]domain.DeveloperApplication),
		developerInvitations:       make(map[string]domain.DeveloperInvitation),
		organizationEntitlements:   make(map[string]domain.OrganizationEntitlement),
		developerOrganizations:     make(map[string]domain.DeveloperOrganization),
		organizationMembers:        make(map[string]map[string]domain.DeveloperOrganizationMember),
		identitySessions:           make(map[string]domain.IdentitySession),
		merchantIdentities:         make(map[string]domain.MerchantIdentity),
		catalogListings:            make(map[string]domain.AppCatalogListing),
		merchantSessionGrants:      make(map[string]domain.MerchantSessionGrant),
		oidcLoginStates:            make(map[string]domain.OIDCLoginState),
		identityMemberships:        make(map[string][]domain.OrganizationMembership),
		oidcUsers:                  make(map[string]string),
		auditEvents:                make(map[string][]domain.AuditEvent),
		idempotency:                make(map[string]string),
		id:                         id,
		now:                        now,
	}
}

func (r *Repository) GetOrganizationEntitlement(_ context.Context, organizationID string) (domain.OrganizationEntitlement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entitlement, ok := r.organizationEntitlements[organizationID]
	if !ok {
		return domain.OrganizationEntitlement{}, domain.ErrNotFound
	}
	return entitlement, nil
}

func (r *Repository) ListApps(_ context.Context, organizationID string, filter ports.AppFilter) ([]domain.App, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]domain.App, 0)
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	for _, app := range r.apps {
		if app.OrganizationID != organizationID {
			continue
		}
		if filter.Status != "" && app.Status != filter.Status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(app.Name+" "+app.Slug), search) {
			continue
		}
		items = append(items, cloneApp(app))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return paginate(items, filter)
}

func (r *Repository) CreateApp(_ context.Context, app domain.App, meta ports.MutationMeta) (domain.App, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.App{}, fmt.Errorf("generate audit id: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := idempotencyKey(app.OrganizationID, meta.Action, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneApp(r.apps[existingID]), nil
	}
	for _, existing := range r.apps {
		if existing.OrganizationID == app.OrganizationID && existing.Slug == app.Slug && existing.Status != domain.AppStatusArchived {
			return domain.App{}, fmt.Errorf("%w: app slug already exists", domain.ErrConflict)
		}
	}
	r.apps[app.ID] = cloneApp(app)
	r.idempotency[key] = app.ID
	r.appendAuditLocked(auditID, app.OrganizationID, meta, "app", app.ID, map[string]any{"slug": app.Slug})
	return cloneApp(app), nil
}

func (r *Repository) GetApp(_ context.Context, organizationID, appID string) (domain.App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.App{}, domain.ErrNotFound
	}
	return cloneApp(app), nil
}

func (r *Repository) UpdateApp(_ context.Context, app domain.App, expectedRevision int64, meta ports.MutationMeta) (domain.App, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.App{}, fmt.Errorf("generate audit id: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.apps[app.ID]
	if !ok || current.OrganizationID != app.OrganizationID {
		return domain.App{}, domain.ErrNotFound
	}
	if current.Revision != expectedRevision {
		return domain.App{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	for _, existing := range r.apps {
		if existing.ID != app.ID && existing.OrganizationID == app.OrganizationID && existing.Slug == app.Slug && existing.Status != domain.AppStatusArchived {
			return domain.App{}, fmt.Errorf("%w: app slug already exists", domain.ErrConflict)
		}
	}
	app.Revision = current.Revision + 1
	app.UpdatedAt = r.now().UTC()
	r.apps[app.ID] = cloneApp(app)
	r.appendAuditLocked(auditID, app.OrganizationID, meta, "app", app.ID, map[string]any{"revision": app.Revision})
	return cloneApp(app), nil
}

func (r *Repository) ArchiveApp(_ context.Context, organizationID, appID string, meta ports.MutationMeta) error {
	auditID, err := r.id()
	if err != nil {
		return fmt.Errorf("generate audit id: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	key := idempotencyKey(organizationID, meta.Action+":"+appID, meta.IdempotencyKey)
	if r.idempotency[key] != "" {
		return nil
	}
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.ErrNotFound
	}
	app.Status = domain.AppStatusArchived
	app.Revision++
	app.UpdatedAt = r.now().UTC()
	r.apps[appID] = app
	r.idempotency[key] = appID
	r.appendAuditLocked(auditID, organizationID, meta, "app", appID, nil)
	return nil
}

func (r *Repository) ListVersions(_ context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.AppVersion, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return nil, ports.PageMeta{}, domain.ErrNotFound
	}
	items := make([]domain.AppVersion, 0, len(r.versions[appID]))
	for _, version := range r.versions[appID] {
		items = append(items, cloneVersion(version))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return paginate(items, filter)
}

func (r *Repository) CreateVersion(_ context.Context, organizationID string, version domain.AppVersion, meta ports.MutationMeta) (domain.AppVersion, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppVersion{}, fmt.Errorf("generate audit id: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[version.AppID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppVersion{}, domain.ErrNotFound
	}
	key := idempotencyKey(organizationID, meta.Action+":"+version.AppID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneVersion(r.versions[version.AppID][existingID]), nil
	}
	if r.versions[version.AppID] == nil {
		r.versions[version.AppID] = make(map[string]domain.AppVersion)
	}
	for _, existing := range r.versions[version.AppID] {
		if existing.Version == version.Version {
			return domain.AppVersion{}, fmt.Errorf("%w: version already exists", domain.ErrConflict)
		}
	}
	r.versions[version.AppID][version.ID] = cloneVersion(version)
	r.idempotency[key] = version.ID
	r.appendAuditLocked(auditID, organizationID, meta, "app_version", version.ID, map[string]any{"version": version.Version})
	return cloneVersion(version), nil
}

func (r *Repository) GetVersion(_ context.Context, organizationID, appID, versionID string) (domain.AppVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppVersion{}, domain.ErrNotFound
	}
	version, ok := r.versions[appID][versionID]
	if !ok {
		return domain.AppVersion{}, domain.ErrNotFound
	}
	return cloneVersion(version), nil
}

func (r *Repository) ActivateVersion(_ context.Context, organizationID, appID, versionID string, expectedStatus domain.VersionStatus, expectedActiveVersionID *string, meta ports.MutationMeta) (domain.AppVersion, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppVersion{}, fmt.Errorf("generate audit id: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != organizationID {
		return domain.AppVersion{}, domain.ErrNotFound
	}
	if expectedActiveVersionID != nil {
		if app.ActiveVersionID == nil || *app.ActiveVersionID != *expectedActiveVersionID {
			return domain.AppVersion{}, fmt.Errorf("%w: active version changed", domain.ErrConflict)
		}
	}
	key := idempotencyKey(organizationID, meta.Action+":"+versionID, meta.IdempotencyKey)
	if existingID := r.idempotency[key]; existingID != "" {
		return cloneVersion(r.versions[appID][existingID]), nil
	}
	target, ok := r.versions[appID][versionID]
	if !ok {
		return domain.AppVersion{}, domain.ErrNotFound
	}
	if target.Status != expectedStatus {
		return domain.AppVersion{}, fmt.Errorf("%w: expected version status %s, current status %s", domain.ErrConflict, expectedStatus, target.Status)
	}
	now := r.now().UTC()
	for id, version := range r.versions[appID] {
		if version.Status == domain.VersionStatusActive {
			version.Status = domain.VersionStatusReleased
			r.versions[appID][id] = version
		}
	}
	target.Status = domain.VersionStatusActive
	target.ReleasedAt = &now
	target.ReleasedBy = stringPointer(meta.ActorID)
	r.versions[appID][versionID] = target
	app.Status = domain.AppStatusActive
	app.ActiveVersionID = stringPointer(versionID)
	app.Revision++
	app.UpdatedAt = now
	r.apps[appID] = app
	for _, snapshotExtension := range target.Snapshot.Extensions {
		extension, exists := r.extensions[appID][snapshotExtension.ExtensionID]
		if !exists {
			continue
		}
		extension.Status = domain.ExtensionStatusActive
		extension.UpdatedAt = now
		r.extensions[appID][snapshotExtension.ExtensionID] = extension
	}
	r.idempotency[key] = versionID
	r.appendAuditLocked(auditID, organizationID, meta, "app_version", versionID, map[string]any{"version": target.Version})
	return cloneVersion(target), nil
}

func (r *Repository) ListAuditEvents(_ context.Context, organizationID string) ([]domain.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := r.auditEvents[organizationID]
	result := make([]domain.AuditEvent, len(items))
	copy(result, items)
	return result, nil
}

func (r *Repository) appendAuditLocked(id, organizationID string, meta ports.MutationMeta, resourceType, resourceID string, metadata map[string]any) {
	r.auditEvents[organizationID] = append(r.auditEvents[organizationID], domain.AuditEvent{
		ID:             id,
		OrganizationID: organizationID,
		ActorID:        meta.ActorID,
		Action:         meta.Action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Metadata:       metadata,
		CreatedAt:      r.now().UTC(),
	})
}

func idempotencyKey(organizationID, action, key string) string {
	return organizationID + ":" + action + ":" + key
}

func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid cursor", domain.ErrValidation)
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("%w: invalid cursor", domain.ErrValidation)
	}
	return offset, nil
}

func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func paginate[T any](items []T, filter ports.AppFilter) ([]T, ports.PageMeta, error) {
	offset, err := decodeCursor(filter.Cursor)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset > len(items) {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: cursor is out of range", domain.ErrValidation)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	meta := ports.PageMeta{HasMore: end < len(items)}
	if meta.HasMore {
		next := encodeCursor(end)
		meta.NextCursor = &next
	}
	result := make([]T, end-offset)
	copy(result, items[offset:end])
	return result, meta, nil
}

func cloneApp(app domain.App) domain.App {
	if app.Description != nil {
		description := *app.Description
		app.Description = &description
	}
	if app.AppURL != nil {
		appURL := *app.AppURL
		app.AppURL = &appURL
	}
	if app.ContactEmail != nil {
		contactEmail := *app.ContactEmail
		app.ContactEmail = &contactEmail
	}
	if app.ActiveVersionID != nil {
		activeVersionID := *app.ActiveVersionID
		app.ActiveVersionID = &activeVersionID
	}
	return app
}

func cloneVersion(version domain.AppVersion) domain.AppVersion {
	if version.ReleaseNote != nil {
		releaseNote := *version.ReleaseNote
		version.ReleaseNote = &releaseNote
	}
	if version.ReleasedBy != nil {
		releasedBy := *version.ReleasedBy
		version.ReleasedBy = &releasedBy
	}
	if version.ReleasedAt != nil {
		releasedAt := *version.ReleasedAt
		version.ReleasedAt = &releasedAt
	}
	version.Snapshot.Extensions = append([]domain.SnapshotExtension(nil), version.Snapshot.Extensions...)
	version.Snapshot.Scopes = append([]domain.SnapshotScope(nil), version.Snapshot.Scopes...)
	version.Snapshot.WebhookSubscriptions = append([]domain.SnapshotWebhook(nil), version.Snapshot.WebhookSubscriptions...)
	version.Snapshot.RedirectURLs = append([]string(nil), version.Snapshot.RedirectURLs...)
	return version
}

func stringPointer(value string) *string {
	return &value
}

var _ ports.Repository = (*Repository)(nil)
