package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListCatalogApps(_ context.Context, filter ports.CatalogFilter) ([]domain.CatalogApp, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.CatalogApp, 0)
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	for appID, listing := range r.catalogListings {
		if listing.Status != domain.CatalogListingStatusPublished || (filter.Category != "" && listing.Category != filter.Category) || (filter.Featured != nil && listing.Featured != *filter.Featured) {
			continue
		}
		app, version, developerName, eligible := r.catalogSourceLocked(appID)
		if !eligible || (search != "" && !strings.Contains(strings.ToLower(app.Name+" "+app.Slug+" "+developerName+" "+pointerValue(app.Description)), search)) {
			continue
		}
		items = append(items, catalogApp(app, version, developerName, listing))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Featured != items[j].Featured {
			return items[i].Featured
		}
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
	return paginate(items, ports.AppFilter{Cursor: filter.Cursor, Limit: filter.Limit})
}

func (r *Repository) GetCatalogApp(_ context.Context, appID string) (domain.CatalogApp, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	listing, exists := r.catalogListings[appID]
	if !exists || listing.Status != domain.CatalogListingStatusPublished {
		return domain.CatalogApp{}, domain.ErrNotFound
	}
	app, version, developerName, eligible := r.catalogSourceLocked(appID)
	if !eligible {
		return domain.CatalogApp{}, domain.ErrNotFound
	}
	return catalogApp(app, version, developerName, listing), nil
}

func (r *Repository) ListCatalogCandidates(_ context.Context, filter ports.CatalogFilter) ([]domain.CatalogCandidate, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.CatalogCandidate, 0)
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	for appID, app := range r.apps {
		if app.Status == domain.AppStatusArchived || (search != "" && !strings.Contains(strings.ToLower(app.Name+" "+app.Slug), search)) {
			continue
		}
		listing, hasListing := r.catalogListings[appID]
		if filter.Status != "" && (!hasListing || listing.Status != filter.Status) {
			continue
		}
		if filter.Category != "" && (!hasListing || listing.Category != filter.Category) {
			continue
		}
		if filter.Featured != nil && (!hasListing || listing.Featured != *filter.Featured) {
			continue
		}
		_, version, developerName, eligible := r.catalogSourceLocked(appID)
		var activeVersion *domain.AppVersion
		if app.ActiveVersionID != nil && version.ID != "" {
			copy := cloneVersion(version)
			activeVersion = &copy
		}
		var listingCopy *domain.AppCatalogListing
		if hasListing {
			copy := cloneCatalogListing(listing)
			listingCopy = &copy
		}
		var reason *string
		if !eligible {
			value := catalogBlockingReason(app, version)
			reason = &value
		}
		items = append(items, domain.CatalogCandidate{
			OrganizationID: app.OrganizationID, DeveloperName: developerName, App: cloneApp(app),
			ActiveVersion: activeVersion, Listing: listingCopy, Eligible: eligible, BlockingReason: reason,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].App.UpdatedAt.After(items[j].App.UpdatedAt) })
	return paginate(items, ports.AppFilter{Cursor: filter.Cursor, Limit: filter.Limit})
}

func (r *Repository) GetCatalogListing(_ context.Context, organizationID, appID string) (domain.AppCatalogListing, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, appExists := r.apps[appID]
	listing, exists := r.catalogListings[appID]
	if !appExists || app.OrganizationID != organizationID || !exists {
		return domain.AppCatalogListing{}, domain.ErrNotFound
	}
	return cloneCatalogListing(listing), nil
}

func (r *Repository) UpsertCatalogListing(_ context.Context, listing domain.AppCatalogListing, expectedRevision int64, meta ports.MutationMeta) (domain.AppCatalogListing, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppCatalogListing{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, exists := r.apps[listing.AppID]
	if !exists || app.OrganizationID != listing.OrganizationID {
		return domain.AppCatalogListing{}, domain.ErrNotFound
	}
	if listing.Status == domain.CatalogListingStatusPublished {
		if meta.ExpectedAppRevision != nil && (app.Revision != *meta.ExpectedAppRevision || app.ActiveVersionID == nil || meta.ExpectedActiveVersionID == nil || *app.ActiveVersionID != *meta.ExpectedActiveVersionID) {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: app changed during publication; inspect the latest version", domain.ErrConflict)
		}
		_, version, _, eligible := r.catalogSourceLocked(listing.AppID)
		if !eligible || version.Status != domain.VersionStatusActive {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: app is no longer eligible for catalog publication", domain.ErrConflict)
		}
	}
	current, exists := r.catalogListings[listing.AppID]
	if exists {
		if current.Revision != expectedRevision {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
		}
		listing.Revision = current.Revision + 1
	} else {
		if expectedRevision != 0 {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: expected revision must be 0 for a new listing", domain.ErrConflict)
		}
		listing.Revision = 1
	}
	r.catalogListings[listing.AppID] = cloneCatalogListing(listing)
	r.appendAuditLocked(auditID, listing.OrganizationID, meta, "app_catalog_listing", listing.AppID, map[string]any{
		"status": listing.Status, "category": listing.Category, "revision": listing.Revision,
	})
	return cloneCatalogListing(listing), nil
}

func (r *Repository) catalogSourceLocked(appID string) (domain.App, domain.AppVersion, string, bool) {
	app, exists := r.apps[appID]
	if !exists || app.ActiveVersionID == nil {
		return app, domain.AppVersion{}, "", false
	}
	version, exists := r.versions[appID][*app.ActiveVersionID]
	developerName := app.OrganizationID
	organizationActive := true
	if organization, ok := r.developerOrganizations[app.OrganizationID]; ok {
		developerName = organization.Name
		organizationActive = organization.Status == domain.OrganizationStatusActive
	}
	return app, version, developerName, exists && organizationActive && app.Status == domain.AppStatusActive && version.Status == domain.VersionStatusActive && app.AppURL != nil && strings.TrimSpace(*app.AppURL) != ""
}

func catalogApp(app domain.App, version domain.AppVersion, developerName string, listing domain.AppCatalogListing) domain.CatalogApp {
	extensionSet := make(map[domain.ExtensionType]struct{})
	for _, extension := range version.Snapshot.Extensions {
		extensionSet[domain.ExtensionType(extension.Type)] = struct{}{}
	}
	extensionTypes := make([]domain.ExtensionType, 0, len(extensionSet))
	for extensionType := range extensionSet {
		extensionTypes = append(extensionTypes, extensionType)
	}
	sort.Slice(extensionTypes, func(i, j int) bool { return extensionTypes[i] < extensionTypes[j] })
	required := make([]domain.SnapshotScope, 0)
	optional := make([]domain.SnapshotScope, 0)
	for _, scope := range version.Snapshot.Scopes {
		if domain.ScopeAccess(scope.Access) == domain.ScopeAccessRequired {
			required = append(required, scope)
		} else {
			optional = append(optional, scope)
		}
	}
	sort.Slice(required, func(i, j int) bool { return required[i].Scope < required[j].Scope })
	sort.Slice(optional, func(i, j int) bool { return optional[i].Scope < optional[j].Scope })
	return domain.CatalogApp{
		AppID: app.ID, Slug: app.Slug, Name: app.Name, Description: cloneString(app.Description), DeveloperName: developerName,
		Category: listing.Category, Featured: listing.Featured, LaunchURL: *app.AppURL,
		ActiveVersionID: version.ID, Version: version.Version, ExtensionTypes: extensionTypes,
		RequiredScopes: required, OptionalScopes: optional, PublishedAt: *listing.PublishedAt, ListingUpdatedAt: listing.UpdatedAt,
	}
}

func catalogBlockingReason(app domain.App, version domain.AppVersion) string {
	switch {
	case app.Status != domain.AppStatusActive:
		return "App must be active."
	case app.ActiveVersionID == nil || version.ID == "" || version.Status != domain.VersionStatusActive:
		return "App must have an active immutable version."
	case app.AppURL == nil || strings.TrimSpace(*app.AppURL) == "":
		return "App must define an HTTPS installation launch URL."
	default:
		return "App is not eligible for catalog publication."
	}
}

func cloneCatalogListing(listing domain.AppCatalogListing) domain.AppCatalogListing {
	listing.PublishedBy = cloneString(listing.PublishedBy)
	if listing.PublishedAt != nil {
		value := *listing.PublishedAt
		listing.PublishedAt = &value
	}
	return listing
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ ports.CatalogRepository = (*Repository)(nil)

func (r *Repository) CreateMerchantSessionGrant(_ context.Context, grant domain.MerchantSessionGrant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, current := range r.merchantSessionGrants {
		if current.CodeHash == grant.CodeHash || current.SourceJTI == grant.SourceJTI {
			return domain.ErrConflict
		}
	}
	r.merchantSessionGrants[grant.ID] = cloneMerchantSessionGrant(grant)
	return nil
}

func (r *Repository) ConsumeMerchantSessionGrant(_ context.Context, codeHash string, now time.Time) (domain.MerchantSessionGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, grant := range r.merchantSessionGrants {
		if grant.CodeHash != codeHash || grant.ConsumedAt != nil || !grant.ExpiresAt.After(now) {
			continue
		}
		consumed := now
		grant.ConsumedAt = &consumed
		r.merchantSessionGrants[id] = grant
		return cloneMerchantSessionGrant(grant), nil
	}
	return domain.MerchantSessionGrant{}, domain.ErrInvalidGrant
}

func cloneMerchantSessionGrant(grant domain.MerchantSessionGrant) domain.MerchantSessionGrant {
	grant.Domain = cloneString(grant.Domain)
	grant.Permissions = append([]string(nil), grant.Permissions...)
	if grant.ConsumedAt != nil {
		value := *grant.ConsumedAt
		grant.ConsumedAt = &value
	}
	return grant
}

var _ ports.MerchantSessionGrantRepository = (*Repository)(nil)
