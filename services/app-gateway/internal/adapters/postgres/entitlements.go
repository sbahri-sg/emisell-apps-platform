package postgres

import (
	"context"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

func (r *Repository) GetOrganizationEntitlement(ctx context.Context, organizationID string) (domain.OrganizationEntitlement, error) {
	var entitlement domain.OrganizationEntitlement
	err := r.pool.QueryRow(ctx, `
		SELECT organization_id::text, sandbox_access, production_access,
		       max_apps, max_webhooks, created_at, updated_at
		FROM organization_entitlements
		WHERE organization_id = $1
	`, organizationID).Scan(
		&entitlement.OrganizationID,
		&entitlement.SandboxAccess,
		&entitlement.ProductionAccess,
		&entitlement.MaxApps,
		&entitlement.MaxWebhooks,
		&entitlement.CreatedAt,
		&entitlement.UpdatedAt,
	)
	if err != nil {
		return domain.OrganizationEntitlement{}, mapError(err)
	}
	return entitlement, nil
}
