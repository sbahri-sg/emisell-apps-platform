package postgres

import (
	"context"
	"fmt"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

const developerOrganizationColumns = `
	o.id::text, o.name, o.slug, o.status,
	e.organization_id::text, e.sandbox_access, e.production_access,
	e.max_apps, e.max_webhooks, e.created_at, e.updated_at,
	(SELECT count(*) FROM apps a WHERE a.organization_id = o.id AND a.status <> 'archived'),
	(SELECT count(*) FROM organization_memberships m WHERE m.organization_id = o.id),
	o.created_at, o.updated_at`

func scanDeveloperOrganization(row rowScanner) (domain.DeveloperOrganization, error) {
	var organization domain.DeveloperOrganization
	var status string
	if err := row.Scan(
		&organization.ID, &organization.Name, &organization.Slug, &status,
		&organization.Entitlement.OrganizationID, &organization.Entitlement.SandboxAccess,
		&organization.Entitlement.ProductionAccess, &organization.Entitlement.MaxApps,
		&organization.Entitlement.MaxWebhooks, &organization.Entitlement.CreatedAt,
		&organization.Entitlement.UpdatedAt, &organization.AppCount, &organization.MembershipCount,
		&organization.CreatedAt, &organization.UpdatedAt,
	); err != nil {
		return domain.DeveloperOrganization{}, mapError(err)
	}
	organization.Status = domain.OrganizationStatus(status)
	return organization, nil
}

func (r *Repository) ListDeveloperOrganizations(ctx context.Context, filter ports.DeveloperOrganizationFilter) ([]domain.DeveloperOrganization, ports.PageMeta, error) {
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
	query := `SELECT ` + developerOrganizationColumns + `
		FROM organizations o
		JOIN organization_entitlements e ON e.organization_id = o.id
		WHERE true`
	args := make([]any, 0, 4)
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		query += fmt.Sprintf(" AND o.status = $%d", len(args))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		query += fmt.Sprintf(" AND (o.name ILIKE $%d OR o.slug ILIKE $%d)", len(args), len(args))
	}
	args = append(args, limit+1, offset)
	query += fmt.Sprintf(" ORDER BY o.created_at DESC, o.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.DeveloperOrganization, 0, limit+1)
	for rows.Next() {
		organization, scanErr := scanDeveloperOrganization(rows)
		if scanErr != nil {
			return nil, ports.PageMeta{}, scanErr
		}
		items = append(items, organization)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	meta := ports.PageMeta{HasMore: len(items) > limit}
	if meta.HasMore {
		items = items[:limit]
		next := encodeCursor(offset + limit)
		meta.NextCursor = &next
	}
	return items, meta, nil
}

func (r *Repository) GetDeveloperOrganization(ctx context.Context, organizationID string) (domain.DeveloperOrganizationDetail, error) {
	organization, err := scanDeveloperOrganization(r.pool.QueryRow(ctx, `
		SELECT `+developerOrganizationColumns+`
		FROM organizations o
		JOIN organization_entitlements e ON e.organization_id = o.id
		WHERE o.id = $1::uuid`, organizationID))
	if err != nil {
		return domain.DeveloperOrganizationDetail{}, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id::text, u.email, u.display_name, m.role, m.created_at
		FROM organization_memberships m
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1::uuid
		ORDER BY m.created_at, u.id`, organizationID)
	if err != nil {
		return domain.DeveloperOrganizationDetail{}, mapError(err)
	}
	defer rows.Close()
	members := make([]domain.DeveloperOrganizationMember, 0, organization.MembershipCount)
	for rows.Next() {
		var member domain.DeveloperOrganizationMember
		var role string
		if err := rows.Scan(&member.UserID, &member.Email, &member.DisplayName, &role, &member.CreatedAt); err != nil {
			return domain.DeveloperOrganizationDetail{}, mapError(err)
		}
		member.Role = domain.Role(role)
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return domain.DeveloperOrganizationDetail{}, mapError(err)
	}
	return domain.DeveloperOrganizationDetail{DeveloperOrganization: organization, Memberships: members}, nil
}

var _ ports.DeveloperOrganizationRepository = (*Repository)(nil)
