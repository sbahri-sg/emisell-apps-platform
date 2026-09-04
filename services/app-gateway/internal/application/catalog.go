package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type CatalogService struct {
	catalog ports.CatalogRepository
	apps    ports.Repository
	now     func() time.Time
}

type UpdateCatalogListingCommand struct {
	OrganizationID          string
	AppID                   string
	ActorID                 string
	Category                domain.CatalogCategory
	Status                  domain.CatalogListingStatus
	Featured                bool
	Revision                int64
	ExpectedAppRevision     *int64
	ExpectedActiveVersionID *string
}

func NewCatalogService(catalog ports.CatalogRepository, apps ports.Repository, now func() time.Time) *CatalogService {
	return &CatalogService{catalog: catalog, apps: apps, now: now}
}

func (s *CatalogService) List(ctx context.Context, filter ports.CatalogFilter) ([]domain.CatalogApp, ports.PageMeta, error) {
	if err := validateCatalogFilter(&filter, false); err != nil {
		return nil, ports.PageMeta{}, err
	}
	items, meta, err := s.catalog.ListCatalogApps(ctx, filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	available := items[:0]
	for _, item := range items {
		if unavailable := firstUnavailableCatalogScope(item.RequiredScopes, item.OptionalScopes); unavailable == "" && catalogLaunchURLAllowed(&item.LaunchURL) {
			available = append(available, item)
		}
	}
	return available, meta, nil
}

func (s *CatalogService) Get(ctx context.Context, appID string) (domain.CatalogApp, error) {
	if !uuidValue.MatchString(strings.TrimSpace(appID)) {
		return domain.CatalogApp{}, fmt.Errorf("%w: appId must be a UUID", domain.ErrValidation)
	}
	item, err := s.catalog.GetCatalogApp(ctx, strings.ToLower(strings.TrimSpace(appID)))
	if err != nil {
		return domain.CatalogApp{}, err
	}
	if firstUnavailableCatalogScope(item.RequiredScopes, item.OptionalScopes) != "" || !catalogLaunchURLAllowed(&item.LaunchURL) {
		return domain.CatalogApp{}, domain.ErrNotFound
	}
	return item, nil
}

func (s *CatalogService) ListCandidates(ctx context.Context, filter ports.CatalogFilter) ([]domain.CatalogCandidate, ports.PageMeta, error) {
	if err := validateCatalogFilter(&filter, true); err != nil {
		return nil, ports.PageMeta{}, err
	}
	items, meta, err := s.catalog.ListCatalogCandidates(ctx, filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	for index := range items {
		items[index].App.ReleaseStatus = domain.AppReleaseStatusDevelopment
		if items[index].Listing != nil && items[index].Listing.Status == domain.CatalogListingStatusPublished {
			items[index].App.ReleaseStatus = domain.AppReleaseStatusReleased
		}
		if !catalogLaunchURLAllowed(items[index].App.AppURL) {
			reason := "An HTTPS launch URL is required for App Store publication."
			items[index].Eligible = false
			items[index].BlockingReason = &reason
			continue
		}
		if items[index].ActiveVersion == nil {
			continue
		}
		if scope := firstUnavailableVersionScope(items[index].ActiveVersion.Snapshot.Scopes); scope != "" {
			reason := fmt.Sprintf("Scope %s is planned and has no callable Provider API endpoint yet.", scope)
			items[index].Eligible = false
			items[index].BlockingReason = &reason
		}
	}
	return items, meta, nil
}

func (s *CatalogService) GetListing(ctx context.Context, organizationID, appID string) (domain.AppCatalogListing, error) {
	if !uuidValue.MatchString(strings.TrimSpace(organizationID)) || !uuidValue.MatchString(strings.TrimSpace(appID)) {
		return domain.AppCatalogListing{}, fmt.Errorf("%w: organizationId and appId must be UUIDs", domain.ErrValidation)
	}
	return s.catalog.GetCatalogListing(ctx, strings.ToLower(organizationID), strings.ToLower(appID))
}

func (s *CatalogService) UpdateListing(ctx context.Context, command UpdateCatalogListingCommand) (domain.AppCatalogListing, error) {
	command.OrganizationID = strings.ToLower(strings.TrimSpace(command.OrganizationID))
	command.AppID = strings.ToLower(strings.TrimSpace(command.AppID))
	command.ActorID = strings.ToLower(strings.TrimSpace(command.ActorID))
	if !uuidValue.MatchString(command.OrganizationID) || !uuidValue.MatchString(command.AppID) || !uuidValue.MatchString(command.ActorID) {
		return domain.AppCatalogListing{}, fmt.Errorf("%w: organizationId, appId, and actorId must be UUIDs", domain.ErrValidation)
	}
	if !validCatalogCategory(command.Category) {
		return domain.AppCatalogListing{}, fmt.Errorf("%w: invalid catalog category", domain.ErrValidation)
	}
	if !validCatalogListingStatus(command.Status) {
		return domain.AppCatalogListing{}, fmt.Errorf("%w: invalid catalog listing status", domain.ErrValidation)
	}
	if command.Revision < 0 {
		return domain.AppCatalogListing{}, fmt.Errorf("%w: revision must not be negative", domain.ErrValidation)
	}
	if (command.ExpectedAppRevision == nil) != (command.ExpectedActiveVersionID == nil) {
		return domain.AppCatalogListing{}, fmt.Errorf("%w: expectedAppRevision and expectedActiveVersionId must be provided together", domain.ErrValidation)
	}
	if command.ExpectedAppRevision != nil && (*command.ExpectedAppRevision < 1 || !uuidValue.MatchString(*command.ExpectedActiveVersionID)) {
		return domain.AppCatalogListing{}, fmt.Errorf("%w: invalid reviewed app revision or version", domain.ErrValidation)
	}

	now := s.now().UTC()
	listing := domain.AppCatalogListing{
		AppID: command.AppID, OrganizationID: command.OrganizationID, Category: command.Category,
		Status: command.Status, Featured: command.Featured, UpdatedAt: now,
	}
	if current, err := s.catalog.GetCatalogListing(ctx, command.OrganizationID, command.AppID); err == nil {
		listing.PublishedBy = current.PublishedBy
		listing.PublishedAt = current.PublishedAt
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.AppCatalogListing{}, err
	}
	if command.Status == domain.CatalogListingStatusPublished {
		app, err := s.apps.GetApp(ctx, command.OrganizationID, command.AppID)
		if err != nil {
			return domain.AppCatalogListing{}, err
		}
		if command.ExpectedAppRevision != nil && (app.Revision != *command.ExpectedAppRevision || !sameOptionalID(app.ActiveVersionID, command.ExpectedActiveVersionID)) {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: app changed since inspection; refresh and review again", domain.ErrConflict)
		}
		if app.Status != domain.AppStatusActive || app.ActiveVersionID == nil || !catalogLaunchURLAllowed(app.AppURL) {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: app must be active, have an HTTPS launch URL, and have an active version before publication", domain.ErrValidation)
		}
		version, err := s.apps.GetVersion(ctx, command.OrganizationID, command.AppID, *app.ActiveVersionID)
		if err != nil {
			return domain.AppCatalogListing{}, err
		}
		if version.Status != domain.VersionStatusActive {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: active app version is not eligible for publication", domain.ErrValidation)
		}
		if scope := firstUnavailableVersionScope(version.Snapshot.Scopes); scope != "" {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: scope %s is planned and cannot be published until its Provider API endpoint is available", domain.ErrValidation, scope)
		}
		listing.PublishedBy = &command.ActorID
		listing.PublishedAt = &now
		// Even legacy callers without UI preconditions must not publish a
		// different version between validation and the repository transaction.
		command.ExpectedAppRevision = &app.Revision
		command.ExpectedActiveVersionID = app.ActiveVersionID
	}
	action := "app_catalog_listing.updated"
	if command.Status == domain.CatalogListingStatusPublished {
		action = "app_catalog_listing.published"
	} else if command.Status == domain.CatalogListingStatusHidden {
		action = "app_catalog_listing.hidden"
	}
	return s.catalog.UpsertCatalogListing(ctx, listing, command.Revision, ports.MutationMeta{
		ActorID: command.ActorID, Action: action,
		ExpectedAppRevision: command.ExpectedAppRevision, ExpectedActiveVersionID: command.ExpectedActiveVersionID,
	})
}

// The local HTTP exception must never make an app eligible for public distribution.
func catalogLaunchURLAllowed(raw *string) bool {
	return raw != nil && strings.TrimSpace(*raw) != "" && validateAppURL(raw, false) == nil
}

func firstUnavailableCatalogScope(groups ...[]domain.SnapshotScope) string {
	for _, scopes := range groups {
		if scope := firstUnavailableVersionScope(scopes); scope != "" {
			return scope
		}
	}
	return ""
}

func firstUnavailableVersionScope(scopes []domain.SnapshotScope) string {
	for _, scope := range scopes {
		definition, registered := LookupOfficialScope(scope.Scope)
		if !registered || definition.Availability != domain.ScopeAvailabilityAvailable {
			return scope.Scope
		}
	}
	return ""
}

func validateCatalogFilter(filter *ports.CatalogFilter, allowStatus bool) error {
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 100 {
		return fmt.Errorf("%w: search must not exceed 100 characters", domain.ErrValidation)
	}
	if filter.Category != "" && !validCatalogCategory(filter.Category) {
		return fmt.Errorf("%w: invalid catalog category", domain.ErrValidation)
	}
	if filter.Status != "" && (!allowStatus || !validCatalogListingStatus(filter.Status)) {
		return fmt.Errorf("%w: invalid catalog listing status", domain.ErrValidation)
	}
	return nil
}

func validCatalogCategory(value domain.CatalogCategory) bool {
	switch value {
	case domain.CatalogCategoryPayment, domain.CatalogCategoryShipping, domain.CatalogCategoryERP,
		domain.CatalogCategoryMarketing, domain.CatalogCategoryOperations, domain.CatalogCategoryCustom:
		return true
	default:
		return false
	}
}

func validCatalogListingStatus(value domain.CatalogListingStatus) bool {
	return value == domain.CatalogListingStatusDraft || value == domain.CatalogListingStatusPublished || value == domain.CatalogListingStatusHidden
}
