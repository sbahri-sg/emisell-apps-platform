package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListCatalogApps(ctx context.Context, filter ports.CatalogFilter) ([]domain.CatalogApp, ports.PageMeta, error) {
	offset, limit, err := catalogPagination(filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	hasFeatured := filter.Featured != nil
	featured := false
	if filter.Featured != nil {
		featured = *filter.Featured
	}
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM app_catalog_listings l
		JOIN apps a ON a.id = l.app_id
		JOIN organizations o ON o.id = a.organization_id
		JOIN app_versions v ON v.id = a.active_version_id AND v.app_id = a.id
		WHERE l.status = 'published' AND a.status = 'active' AND o.status = 'active'
		  AND v.status = 'active' AND a.app_url IS NOT NULL AND a.app_url <> ''
		  AND ($1::text = '' OR l.category = $1)
		  AND (NOT $2::boolean OR l.featured = $3)
		  AND ($4::text = '' OR a.name ILIKE '%' || $4 || '%' OR a.slug ILIKE '%' || $4 || '%'
		       OR COALESCE(a.description, '') ILIKE '%' || $4 || '%' OR o.name ILIKE '%' || $4 || '%')`,
		filter.Category, hasFeatured, featured, filter.Search).Scan(&total); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	if offset > total {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: cursor is out of range", domain.ErrValidation)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT a.id::text, a.slug, a.name, a.description, o.name, l.category,
		       l.featured, a.app_url, v.id::text, v.version, v.snapshot,
		       l.published_at, l.updated_at
		FROM app_catalog_listings l
		JOIN apps a ON a.id = l.app_id
		JOIN organizations o ON o.id = a.organization_id
		JOIN app_versions v ON v.id = a.active_version_id AND v.app_id = a.id
		WHERE l.status = 'published' AND a.status = 'active' AND o.status = 'active'
		  AND v.status = 'active' AND a.app_url IS NOT NULL AND a.app_url <> ''
		  AND ($1::text = '' OR l.category = $1)
		  AND (NOT $2::boolean OR l.featured = $3)
		  AND ($4::text = '' OR a.name ILIKE '%' || $4 || '%' OR a.slug ILIKE '%' || $4 || '%'
		       OR COALESCE(a.description, '') ILIKE '%' || $4 || '%' OR o.name ILIKE '%' || $4 || '%')
		ORDER BY l.featured DESC, l.published_at DESC, a.id DESC
		LIMIT $5 OFFSET $6`, filter.Category, hasFeatured, featured, filter.Search, limit, offset)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.CatalogApp, 0, limit)
	for rows.Next() {
		item, err := scanCatalogApp(rows)
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	return items, pageMeta(offset, limit, total), nil
}

func (r *Repository) GetCatalogApp(ctx context.Context, appID string) (domain.CatalogApp, error) {
	return scanCatalogApp(r.pool.QueryRow(ctx, `
		SELECT a.id::text, a.slug, a.name, a.description, o.name, l.category,
		       l.featured, a.app_url, v.id::text, v.version, v.snapshot,
		       l.published_at, l.updated_at
		FROM app_catalog_listings l
		JOIN apps a ON a.id = l.app_id
		JOIN organizations o ON o.id = a.organization_id
		JOIN app_versions v ON v.id = a.active_version_id AND v.app_id = a.id
		WHERE a.id = $1::uuid AND l.status = 'published' AND a.status = 'active'
		  AND o.status = 'active' AND v.status = 'active'
		  AND a.app_url IS NOT NULL AND a.app_url <> ''`, appID))
}

func scanCatalogApp(row rowScanner) (domain.CatalogApp, error) {
	var item domain.CatalogApp
	var category string
	var snapshotBytes []byte
	if err := row.Scan(
		&item.AppID, &item.Slug, &item.Name, &item.Description, &item.DeveloperName,
		&category, &item.Featured, &item.LaunchURL, &item.ActiveVersionID,
		&item.Version, &snapshotBytes, &item.PublishedAt, &item.ListingUpdatedAt,
	); err != nil {
		return domain.CatalogApp{}, mapError(err)
	}
	var snapshot domain.VersionSnapshot
	if err := json.Unmarshal(snapshotBytes, &snapshot); err != nil {
		return domain.CatalogApp{}, fmt.Errorf("decode catalog version snapshot: %w", err)
	}
	item.Category = domain.CatalogCategory(category)
	item.ExtensionTypes, item.RequiredScopes, item.OptionalScopes = catalogSnapshotSummary(snapshot)
	return item, nil
}

func (r *Repository) ListCatalogCandidates(ctx context.Context, filter ports.CatalogFilter) ([]domain.CatalogCandidate, ports.PageMeta, error) {
	offset, limit, err := catalogPagination(filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	hasFeatured := filter.Featured != nil
	featured := false
	if filter.Featured != nil {
		featured = *filter.Featured
	}
	where := `
		FROM apps a
		JOIN organizations o ON o.id = a.organization_id
		LEFT JOIN app_versions v ON v.id = a.active_version_id AND v.app_id = a.id
		LEFT JOIN app_catalog_listings l ON l.app_id = a.id
		WHERE a.status <> 'archived'
		  AND ($1::text = '' OR COALESCE(l.status, 'draft') = $1)
		  AND ($2::text = '' OR l.category = $2)
		  AND (NOT $3::boolean OR l.featured = $4)
		  AND ($5::text = '' OR a.name ILIKE '%' || $5 || '%' OR a.slug ILIKE '%' || $5 || '%'
		       OR o.name ILIKE '%' || $5 || '%')`
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+where,
		filter.Status, filter.Category, hasFeatured, featured, filter.Search).Scan(&total); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	if offset > total {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: cursor is out of range", domain.ErrValidation)
	}
	rows, err := r.pool.Query(ctx, `SELECT
		a.id::text, a.organization_id::text, a.name, a.slug, a.description,
		a.distribution, a.status, a.app_url, a.contact_email, a.active_version_id::text,
		a.created_by::text, a.created_at, a.updated_at, a.revision, o.name,
		v.id::text, v.version, v.status, v.release_note, v.snapshot, v.created_by::text,
		v.released_by::text, v.created_at, v.released_at,
		l.category, l.status, l.featured, l.published_by::text, l.published_at,
		l.updated_at, l.revision `+where+`
		ORDER BY a.updated_at DESC, a.id DESC LIMIT $6 OFFSET $7`,
		filter.Status, filter.Category, hasFeatured, featured, filter.Search, limit, offset)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.CatalogCandidate, 0, limit)
	for rows.Next() {
		item, err := scanCatalogCandidate(rows)
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	return items, pageMeta(offset, limit, total), nil
}

func scanCatalogCandidate(row rowScanner) (domain.CatalogCandidate, error) {
	var candidate domain.CatalogCandidate
	var distribution, appStatus string
	var activeVersionID *string
	var versionID, versionValue, versionStatus, releaseNote, versionCreatedBy, releasedBy *string
	var snapshotBytes []byte
	var versionCreatedAt, releasedAt *time.Time
	var category, listingStatus, publishedBy *string
	var featured *bool
	var publishedAt, listingUpdatedAt *time.Time
	var listingRevision *int64
	if err := row.Scan(
		&candidate.App.ID, &candidate.App.OrganizationID, &candidate.App.Name, &candidate.App.Slug,
		&candidate.App.Description, &distribution, &appStatus, &candidate.App.AppURL,
		&candidate.App.ContactEmail, &activeVersionID, &candidate.App.CreatedBy,
		&candidate.App.CreatedAt, &candidate.App.UpdatedAt, &candidate.App.Revision,
		&candidate.DeveloperName, &versionID, &versionValue, &versionStatus, &releaseNote,
		&snapshotBytes, &versionCreatedBy, &releasedBy, &versionCreatedAt, &releasedAt,
		&category, &listingStatus, &featured, &publishedBy, &publishedAt, &listingUpdatedAt, &listingRevision,
	); err != nil {
		return domain.CatalogCandidate{}, mapError(err)
	}
	candidate.App.Distribution = domain.Distribution(distribution)
	candidate.App.Status = domain.AppStatus(appStatus)
	candidate.App.ActiveVersionID = activeVersionID
	if versionID != nil && versionValue != nil && versionStatus != nil && versionCreatedBy != nil && versionCreatedAt != nil {
		var snapshot domain.VersionSnapshot
		if err := json.Unmarshal(snapshotBytes, &snapshot); err != nil {
			return domain.CatalogCandidate{}, fmt.Errorf("decode candidate version snapshot: %w", err)
		}
		candidate.ActiveVersion = &domain.AppVersion{
			ID: *versionID, AppID: candidate.App.ID, Version: *versionValue,
			Status: domain.VersionStatus(*versionStatus), ReleaseNote: releaseNote,
			Snapshot: snapshot, CreatedBy: *versionCreatedBy, ReleasedBy: releasedBy,
			CreatedAt: *versionCreatedAt, ReleasedAt: releasedAt,
		}
	}
	if category != nil && listingStatus != nil && featured != nil && listingUpdatedAt != nil && listingRevision != nil {
		candidate.Listing = &domain.AppCatalogListing{
			AppID: candidate.App.ID, OrganizationID: candidate.App.OrganizationID,
			Category: domain.CatalogCategory(*category), Status: domain.CatalogListingStatus(*listingStatus),
			Featured: *featured, PublishedBy: publishedBy, PublishedAt: publishedAt,
			UpdatedAt: *listingUpdatedAt, Revision: *listingRevision,
		}
	}
	candidate.Eligible = candidate.App.Status == domain.AppStatusActive && candidate.App.AppURL != nil && strings.TrimSpace(*candidate.App.AppURL) != "" && candidate.ActiveVersion != nil && candidate.ActiveVersion.Status == domain.VersionStatusActive
	if !candidate.Eligible {
		reason := catalogCandidateBlockingReason(candidate)
		candidate.BlockingReason = &reason
	}
	return candidate, nil
}

func (r *Repository) GetCatalogListing(ctx context.Context, organizationID, appID string) (domain.AppCatalogListing, error) {
	return scanCatalogListing(r.pool.QueryRow(ctx, `
		SELECT l.app_id::text, a.organization_id::text, l.category, l.status,
		       l.featured, l.published_by::text, l.published_at, l.updated_at, l.revision
		FROM app_catalog_listings l JOIN apps a ON a.id = l.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid`, organizationID, appID))
}

func (r *Repository) UpsertCatalogListing(ctx context.Context, listing domain.AppCatalogListing, expectedRevision int64, meta ports.MutationMeta) (domain.AppCatalogListing, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.AppCatalogListing{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var lockedAppID, appStatus, organizationStatus string
	var appRevision int64
	var appURL, activeVersionID *string
	if err := tx.QueryRow(ctx, `
		SELECT a.id::text, a.status, a.app_url, a.active_version_id::text, o.status, a.revision
		FROM apps a JOIN organizations o ON o.id = a.organization_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid
		FOR UPDATE OF a, o`, listing.OrganizationID, listing.AppID).Scan(
		&lockedAppID, &appStatus, &appURL, &activeVersionID, &organizationStatus, &appRevision,
	); errors.Is(err, pgx.ErrNoRows) {
		return domain.AppCatalogListing{}, domain.ErrNotFound
	} else if err != nil {
		return domain.AppCatalogListing{}, mapError(err)
	}
	if listing.Status == domain.CatalogListingStatusPublished {
		if meta.ExpectedAppRevision != nil && (appRevision != *meta.ExpectedAppRevision || activeVersionID == nil || meta.ExpectedActiveVersionID == nil || *activeVersionID != *meta.ExpectedActiveVersionID) {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: app changed during publication; inspect the latest version", domain.ErrConflict)
		}
		if appStatus != string(domain.AppStatusActive) || organizationStatus != string(domain.OrganizationStatusActive) || appURL == nil || strings.TrimSpace(*appURL) == "" || activeVersionID == nil {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: app is no longer eligible for catalog publication", domain.ErrConflict)
		}
		var versionStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM app_versions WHERE id = $1::uuid AND app_id = $2::uuid`, *activeVersionID, listing.AppID).Scan(&versionStatus); err != nil {
			return domain.AppCatalogListing{}, mapError(err)
		}
		if versionStatus != string(domain.VersionStatusActive) {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: active version is no longer eligible for catalog publication", domain.ErrConflict)
		}
	}
	var currentRevision int64
	err = tx.QueryRow(ctx, `SELECT revision FROM app_catalog_listings WHERE app_id = $1::uuid FOR UPDATE`, listing.AppID).Scan(&currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		if expectedRevision != 0 {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: expected revision must be 0 for a new listing", domain.ErrConflict)
		}
		listing.Revision = 1
		created, err := scanCatalogListing(tx.QueryRow(ctx, `
			INSERT INTO app_catalog_listings (
				app_id, category, status, featured, published_by, published_at, updated_at, revision
			) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $6, $7, $8)
			RETURNING app_id::text, $9::text, category, status, featured,
				published_by::text, published_at, updated_at, revision`,
			listing.AppID, listing.Category, listing.Status, listing.Featured,
			listing.PublishedBy, listing.PublishedAt, listing.UpdatedAt, listing.Revision,
			listing.OrganizationID))
		if err != nil {
			return domain.AppCatalogListing{}, err
		}
		listing = created
	} else if err != nil {
		return domain.AppCatalogListing{}, mapError(err)
	} else {
		if currentRevision != expectedRevision {
			return domain.AppCatalogListing{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, currentRevision)
		}
		listing.Revision = currentRevision + 1
		updated, err := scanCatalogListing(tx.QueryRow(ctx, `
			UPDATE app_catalog_listings SET category = $2, status = $3, featured = $4,
				published_by = $5::uuid, published_at = $6, updated_at = $7, revision = $8
			WHERE app_id = $1::uuid
			RETURNING app_id::text, $9::text, category, status, featured,
				published_by::text, published_at, updated_at, revision`,
			listing.AppID, listing.Category, listing.Status, listing.Featured,
			listing.PublishedBy, listing.PublishedAt, listing.UpdatedAt, listing.Revision,
			listing.OrganizationID))
		if err != nil {
			return domain.AppCatalogListing{}, err
		}
		listing = updated
	}
	if err := r.appendAudit(ctx, tx, listing.OrganizationID, meta, "app_catalog_listing", listing.AppID, map[string]any{
		"status": listing.Status, "category": listing.Category, "featured": listing.Featured, "revision": listing.Revision,
	}); err != nil {
		return domain.AppCatalogListing{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppCatalogListing{}, mapError(err)
	}
	return listing, nil
}

func scanCatalogListing(row rowScanner) (domain.AppCatalogListing, error) {
	var listing domain.AppCatalogListing
	var category, status string
	if err := row.Scan(
		&listing.AppID, &listing.OrganizationID, &category, &status, &listing.Featured,
		&listing.PublishedBy, &listing.PublishedAt, &listing.UpdatedAt, &listing.Revision,
	); err != nil {
		return domain.AppCatalogListing{}, mapError(err)
	}
	listing.Category = domain.CatalogCategory(category)
	listing.Status = domain.CatalogListingStatus(status)
	return listing, nil
}

func catalogSnapshotSummary(snapshot domain.VersionSnapshot) ([]domain.ExtensionType, []domain.SnapshotScope, []domain.SnapshotScope) {
	extensionSet := make(map[domain.ExtensionType]struct{})
	for _, extension := range snapshot.Extensions {
		extensionSet[domain.ExtensionType(extension.Type)] = struct{}{}
	}
	extensionTypes := make([]domain.ExtensionType, 0, len(extensionSet))
	for extensionType := range extensionSet {
		extensionTypes = append(extensionTypes, extensionType)
	}
	required := make([]domain.SnapshotScope, 0)
	optional := make([]domain.SnapshotScope, 0)
	for _, scope := range snapshot.Scopes {
		if domain.ScopeAccess(scope.Access) == domain.ScopeAccessRequired {
			required = append(required, scope)
		} else {
			optional = append(optional, scope)
		}
	}
	sort.Slice(extensionTypes, func(i, j int) bool { return extensionTypes[i] < extensionTypes[j] })
	sort.Slice(required, func(i, j int) bool { return required[i].Scope < required[j].Scope })
	sort.Slice(optional, func(i, j int) bool { return optional[i].Scope < optional[j].Scope })
	return extensionTypes, required, optional
}

func catalogCandidateBlockingReason(candidate domain.CatalogCandidate) string {
	switch {
	case candidate.App.Status != domain.AppStatusActive:
		return "App must be active."
	case candidate.ActiveVersion == nil || candidate.ActiveVersion.Status != domain.VersionStatusActive:
		return "App must have an active immutable version."
	case candidate.App.AppURL == nil || strings.TrimSpace(*candidate.App.AppURL) == "":
		return "App must define an HTTPS installation launch URL."
	default:
		return "App is not eligible for catalog publication."
	}
}

func catalogPagination(filter ports.CatalogFilter) (int, int, error) {
	return pagination(ports.AppFilter{Cursor: filter.Cursor, Limit: filter.Limit})
}

var _ ports.CatalogRepository = (*Repository)(nil)

func (r *Repository) CreateMerchantSessionGrant(ctx context.Context, grant domain.MerchantSessionGrant) error {
	permissions, err := json.Marshal(grant.Permissions)
	if err != nil {
		return fmt.Errorf("encode merchant session grant permissions: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO merchant_session_grants (
			id, code_hash, source_jti, subject, actor_email, actor_display_name,
			merchant_id, merchant_name, merchant_domain, environment, permissions,
			return_to, created_at, expires_at
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		grant.ID, grant.CodeHash, grant.SourceJTI, grant.Subject, grant.Email, grant.DisplayName,
		grant.MerchantID, grant.MerchantName, grant.Domain, grant.Environment, permissions,
		grant.ReturnTo, grant.CreatedAt, grant.ExpiresAt)
	return mapError(err)
}

func (r *Repository) ConsumeMerchantSessionGrant(ctx context.Context, codeHash string, now time.Time) (domain.MerchantSessionGrant, error) {
	var grant domain.MerchantSessionGrant
	var environment string
	var permissions []byte
	err := r.pool.QueryRow(ctx, `
		UPDATE merchant_session_grants SET consumed_at = $2
		WHERE code_hash = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING id::text, code_hash, source_jti, subject, actor_email,
			actor_display_name, merchant_id::text, merchant_name, merchant_domain,
			environment, permissions, return_to, created_at, expires_at, consumed_at`,
		codeHash, now).Scan(
		&grant.ID, &grant.CodeHash, &grant.SourceJTI, &grant.Subject, &grant.Email,
		&grant.DisplayName, &grant.MerchantID, &grant.MerchantName, &grant.Domain,
		&environment, &permissions, &grant.ReturnTo, &grant.CreatedAt, &grant.ExpiresAt, &grant.ConsumedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MerchantSessionGrant{}, domain.ErrInvalidGrant
	}
	if err != nil {
		return domain.MerchantSessionGrant{}, mapError(err)
	}
	if err := json.Unmarshal(permissions, &grant.Permissions); err != nil {
		return domain.MerchantSessionGrant{}, fmt.Errorf("decode merchant session grant permissions: %w", err)
	}
	grant.Environment = domain.Environment(environment)
	return grant, nil
}

var _ ports.MerchantSessionGrantRepository = (*Repository)(nil)
