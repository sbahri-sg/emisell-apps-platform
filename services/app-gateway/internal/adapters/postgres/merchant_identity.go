package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) UpsertMerchantIdentity(ctx context.Context, identity domain.MerchantIdentity) (domain.MerchantIdentity, error) {
	var result domain.MerchantIdentity
	var environment string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO merchant_identities (
			merchant_id, user_id, merchant_name, merchant_domain, environment, created_at, updated_at
		) VALUES ($1, $2::uuid, $3, $4, $5, $6, $7)
		ON CONFLICT (merchant_id, user_id, environment) DO UPDATE SET
			merchant_name = EXCLUDED.merchant_name,
			merchant_domain = EXCLUDED.merchant_domain,
			updated_at = EXCLUDED.updated_at
		RETURNING merchant_id::text, user_id::text, merchant_name, merchant_domain,
			environment, created_at, updated_at`,
		identity.MerchantID, identity.UserID, identity.Name, identity.Domain, identity.Environment,
		identity.CreatedAt, identity.UpdatedAt,
	).Scan(&result.MerchantID, &result.UserID, &result.Name, &result.Domain, &environment, &result.CreatedAt, &result.UpdatedAt)
	result.Environment = domain.Environment(environment)
	return result, mapError(err)
}

func (r *Repository) GetMerchantIdentity(ctx context.Context, userID, merchantID string, requestedEnvironment domain.Environment) (domain.MerchantIdentity, error) {
	var identity domain.MerchantIdentity
	var environment string
	err := r.pool.QueryRow(ctx, `
		SELECT merchant_id::text, user_id::text, merchant_name, merchant_domain,
			environment, created_at, updated_at
		FROM merchant_identities
		WHERE user_id = $1::uuid AND merchant_id = $2 AND environment = $3`, userID, merchantID, requestedEnvironment).Scan(
		&identity.MerchantID, &identity.UserID, &identity.Name, &identity.Domain,
		&environment, &identity.CreatedAt, &identity.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MerchantIdentity{}, domain.ErrNotFound
	}
	identity.Environment = domain.Environment(environment)
	return identity, mapError(err)
}

func (r *Repository) ResolveMerchantIdentity(ctx context.Context, merchantID string) (domain.MerchantIdentity, error) {
	var identity domain.MerchantIdentity
	var environment string
	err := r.pool.QueryRow(ctx, `
		SELECT merchant_id::text, user_id::text, merchant_name, merchant_domain,
			environment, created_at, updated_at
		FROM merchant_identities
		WHERE merchant_id = $1
		ORDER BY updated_at DESC, user_id DESC
		LIMIT 1`, merchantID).Scan(
		&identity.MerchantID, &identity.UserID, &identity.Name, &identity.Domain,
		&environment, &identity.CreatedAt, &identity.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MerchantIdentity{}, domain.ErrNotFound
	}
	identity.Environment = domain.Environment(environment)
	return identity, mapError(err)
}

func (r *Repository) ListMerchantInstalledApps(ctx context.Context, merchantID string, environment domain.Environment) ([]domain.MerchantInstalledApp, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.organization_id::text, i.id::text, a.id::text, a.name, a.description, a.app_url,
			v.version, i.status, i.granted_scopes, i.installed_at, i.updated_at
		FROM app_installations i
		JOIN apps a ON a.id = i.app_id
		JOIN app_versions v ON v.id = i.installed_version_id
		WHERE i.merchant_id = $1 AND i.environment = $2 AND i.status <> 'uninstalled'
		ORDER BY i.installed_at DESC, i.id DESC`, merchantID, environment)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.MerchantInstalledApp, 0)
	for rows.Next() {
		item, err := scanMerchantInstalledApp(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *Repository) GetMerchantInstalledApp(ctx context.Context, merchantID, installationID string, environment domain.Environment) (domain.MerchantInstalledApp, error) {
	return scanMerchantInstalledApp(r.pool.QueryRow(ctx, `
		SELECT a.organization_id::text, i.id::text, a.id::text, a.name, a.description, a.app_url,
			v.version, i.status, i.granted_scopes, i.installed_at, i.updated_at
		FROM app_installations i
		JOIN apps a ON a.id = i.app_id
		JOIN app_versions v ON v.id = i.installed_version_id
		WHERE i.merchant_id = $1 AND i.id = $2::uuid AND i.environment = $3`,
		merchantID, installationID, environment))
}

func scanMerchantInstalledApp(row rowScanner) (domain.MerchantInstalledApp, error) {
	var item domain.MerchantInstalledApp
	var status string
	var encodedScopes []byte
	if err := row.Scan(
		&item.OrganizationID, &item.InstallationID, &item.AppID, &item.AppName,
		&item.AppDescription, &item.AppURL, &item.Version, &status, &encodedScopes,
		&item.InstalledAt, &item.UpdatedAt,
	); err != nil {
		return domain.MerchantInstalledApp{}, mapError(err)
	}
	if err := json.Unmarshal(encodedScopes, &item.GrantedScopes); err != nil {
		return domain.MerchantInstalledApp{}, fmt.Errorf("decode merchant installation scopes: %w", err)
	}
	item.Status = domain.InstallationStatus(status)
	return item, nil
}

var _ ports.MerchantRepository = (*Repository)(nil)
