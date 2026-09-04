package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const developerApplicationColumns = `
	id::text, platform_organization_id::text, company_name, company_domain,
	contact_name, contact_email, requested_app_name, app_type, use_case,
	requested_scopes, status, review_notes, organization_id::text,
	submitted_by::text, reviewed_by::text, reviewed_at, created_at, updated_at, revision`

const developerInvitationColumns = `
	id::text, application_id::text, organization_id::text, email, role,
	token_hash, status, created_by::text, accepted_by::text, created_at,
	expires_at, accepted_at, revoked_at`

func scanDeveloperApplication(row rowScanner) (domain.DeveloperApplication, error) {
	var application domain.DeveloperApplication
	var appType, status string
	var scopes []byte
	if err := row.Scan(
		&application.ID, &application.PlatformOrgID, &application.CompanyName, &application.CompanyDomain,
		&application.ContactName, &application.ContactEmail, &application.RequestedAppName, &appType,
		&application.UseCase, &scopes, &status, &application.ReviewNotes, &application.OrganizationID,
		&application.SubmittedBy, &application.ReviewedBy, &application.ReviewedAt,
		&application.CreatedAt, &application.UpdatedAt, &application.Revision,
	); err != nil {
		return domain.DeveloperApplication{}, mapError(err)
	}
	if err := json.Unmarshal(scopes, &application.RequestedScopes); err != nil {
		return domain.DeveloperApplication{}, fmt.Errorf("decode requested scopes: %w", err)
	}
	application.AppType = domain.DeveloperAppType(appType)
	application.Status = domain.DeveloperApplicationStatus(status)
	return application, nil
}

func scanDeveloperInvitation(row rowScanner) (domain.DeveloperInvitation, error) {
	var invitation domain.DeveloperInvitation
	var role, status string
	if err := row.Scan(
		&invitation.ID, &invitation.ApplicationID, &invitation.OrganizationID, &invitation.Email,
		&role, &invitation.TokenHash, &status, &invitation.CreatedBy, &invitation.AcceptedBy,
		&invitation.CreatedAt, &invitation.ExpiresAt, &invitation.AcceptedAt, &invitation.RevokedAt,
	); err != nil {
		return domain.DeveloperInvitation{}, mapError(err)
	}
	invitation.Role = domain.Role(role)
	invitation.Status = domain.DeveloperInvitationStatus(status)
	return invitation, nil
}

func (r *Repository) attachCurrentInvitation(ctx context.Context, source queryRower, application *domain.DeveloperApplication) error {
	invitation, err := scanDeveloperInvitation(source.QueryRow(ctx, `
		SELECT `+developerInvitationColumns+`
		FROM developer_invitations
		WHERE application_id = $1::uuid
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, application.ID))
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if invitation.Status == domain.DeveloperInvitationStatusPending && !r.now().UTC().Before(invitation.ExpiresAt) {
		invitation.Status = domain.DeveloperInvitationStatusExpired
	}
	application.CurrentInvitation = &invitation
	return nil
}

func (r *Repository) ListDeveloperApplications(ctx context.Context, platformOrgID string, filter ports.DeveloperApplicationFilter) ([]domain.DeveloperApplication, ports.PageMeta, error) {
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
	query := `SELECT ` + developerApplicationColumns + ` FROM developer_applications WHERE platform_organization_id = $1::uuid`
	args := []any{platformOrgID}
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		query += fmt.Sprintf(` AND (company_name ILIKE $%d OR company_domain ILIKE $%d OR contact_name ILIKE $%d OR contact_email ILIKE $%d OR requested_app_name ILIKE $%d)`, len(args), len(args), len(args), len(args), len(args))
	}
	args = append(args, limit+1, offset)
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.DeveloperApplication, 0, limit+1)
	for rows.Next() {
		application, scanErr := scanDeveloperApplication(rows)
		if scanErr != nil {
			return nil, ports.PageMeta{}, scanErr
		}
		items = append(items, application)
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
	for index := range items {
		if err := r.attachCurrentInvitation(ctx, r.pool, &items[index]); err != nil {
			return nil, ports.PageMeta{}, err
		}
	}
	return items, meta, nil
}

func (r *Repository) CreateDeveloperApplication(ctx context.Context, application domain.DeveloperApplication, meta ports.MutationMeta) (domain.DeveloperApplication, error) {
	scopes, err := json.Marshal(application.RequestedScopes)
	if err != nil {
		return domain.DeveloperApplication{}, fmt.Errorf("encode requested scopes: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	created, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		INSERT INTO developer_applications (
			id, platform_organization_id, company_name, company_domain, contact_name,
			contact_email, requested_app_name, app_type, use_case, requested_scopes,
			status, submitted_by, created_at, updated_at, revision
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12::uuid, $13, $13, 1)
		RETURNING `+developerApplicationColumns,
		application.ID, application.PlatformOrgID, application.CompanyName, application.CompanyDomain,
		application.ContactName, application.ContactEmail, application.RequestedAppName, application.AppType,
		application.UseCase, scopes, application.Status, application.SubmittedBy, application.CreatedAt))
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	if err := r.appendAudit(ctx, tx, application.PlatformOrgID, meta, "developer_application", application.ID, map[string]any{"contact_email": application.ContactEmail}); err != nil {
		return domain.DeveloperApplication{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeveloperApplication{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetDeveloperApplication(ctx context.Context, platformOrgID, applicationID string) (domain.DeveloperApplication, error) {
	application, err := scanDeveloperApplication(r.pool.QueryRow(ctx, `
		SELECT `+developerApplicationColumns+`
		FROM developer_applications
		WHERE platform_organization_id = $1::uuid AND id = $2::uuid`, platformOrgID, applicationID))
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	if err := r.attachCurrentInvitation(ctx, r.pool, &application); err != nil {
		return domain.DeveloperApplication{}, err
	}
	return application, nil
}

func (r *Repository) StartDeveloperApplicationReview(ctx context.Context, platformOrgID, applicationID, actorID string, expectedRevision int64, notes *string) (domain.DeveloperApplication, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	application, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		UPDATE developer_applications
		SET status = 'under_review', review_notes = $4, reviewed_by = $3::uuid,
			reviewed_at = $5, updated_at = $5, revision = revision + 1
		WHERE platform_organization_id = $1::uuid AND id = $2::uuid
			AND revision = $6 AND status = 'submitted'
		RETURNING `+developerApplicationColumns,
		platformOrgID, applicationID, actorID, notes, r.now().UTC(), expectedRevision))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: application can no longer enter review", domain.ErrConflict)
	}
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	if err := r.appendAudit(ctx, tx, platformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_application.review_started"}, "developer_application", applicationID, nil); err != nil {
		return domain.DeveloperApplication{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeveloperApplication{}, mapError(err)
	}
	return application, nil
}

func (r *Repository) ApproveDeveloperApplication(ctx context.Context, platformOrgID string, application domain.DeveloperApplication, organizationID, organizationName, organizationSlug string, entitlement domain.OrganizationEntitlement, invitation domain.DeveloperInvitation, expectedRevision int64, meta ports.MutationMeta) (domain.DeveloperApplication, domain.DeveloperInvitation, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		SELECT `+developerApplicationColumns+` FROM developer_applications
		WHERE platform_organization_id = $1::uuid AND id = $2::uuid FOR UPDATE`, platformOrgID, application.ID))
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	if current.Revision != expectedRevision || (current.Status != domain.DeveloperApplicationStatusSubmitted && current.Status != domain.DeveloperApplicationStatusUnderReview) {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, fmt.Errorf("%w: application can no longer be approved", domain.ErrConflict)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1::uuid, $2, $3)`, organizationID, organizationName, organizationSlug); err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_entitlements (
			organization_id, sandbox_access, production_access, max_apps, max_webhooks, created_at, updated_at
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $6)`, entitlement.OrganizationID, entitlement.SandboxAccess, entitlement.ProductionAccess, entitlement.MaxApps, entitlement.MaxWebhooks, entitlement.CreatedAt); err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, mapError(err)
	}
	createdInvitation, err := scanDeveloperInvitation(tx.QueryRow(ctx, `
		INSERT INTO developer_invitations (
			id, application_id, organization_id, email, role, token_hash, status, created_by, created_at, expires_at
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8::uuid, $9, $10)
		RETURNING `+developerInvitationColumns,
		invitation.ID, invitation.ApplicationID, invitation.OrganizationID, invitation.Email, invitation.Role,
		invitation.TokenHash, invitation.Status, invitation.CreatedBy, invitation.CreatedAt, invitation.ExpiresAt))
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	now := r.now().UTC()
	updated, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		UPDATE developer_applications
		SET status = 'invited', organization_id = $3::uuid, reviewed_by = $4::uuid,
			reviewed_at = $5, updated_at = $5, revision = revision + 1
		WHERE platform_organization_id = $1::uuid AND id = $2::uuid
		RETURNING `+developerApplicationColumns, platformOrgID, application.ID, organizationID, meta.ActorID, now))
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	if err := r.appendAudit(ctx, tx, platformOrgID, meta, "developer_application", application.ID, map[string]any{"organization_id": organizationID, "invitation_id": invitation.ID}); err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, mapError(err)
	}
	updated.CurrentInvitation = &createdInvitation
	return updated, createdInvitation, nil
}

func (r *Repository) RejectDeveloperApplication(ctx context.Context, platformOrgID, applicationID, actorID string, expectedRevision int64, notes string) (domain.DeveloperApplication, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := r.now().UTC()
	application, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		UPDATE developer_applications
		SET status = 'rejected', review_notes = $4, reviewed_by = $3::uuid,
			reviewed_at = $5, updated_at = $5, revision = revision + 1
		WHERE platform_organization_id = $1::uuid AND id = $2::uuid AND revision = $6
			AND status IN ('submitted', 'under_review')
		RETURNING `+developerApplicationColumns, platformOrgID, applicationID, actorID, notes, now, expectedRevision))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: application can no longer be rejected", domain.ErrConflict)
	}
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	if err := r.appendAudit(ctx, tx, platformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_application.rejected"}, "developer_application", applicationID, nil); err != nil {
		return domain.DeveloperApplication{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeveloperApplication{}, mapError(err)
	}
	return application, nil
}

func (r *Repository) RotateDeveloperInvitation(ctx context.Context, platformOrgID, applicationID string, invitation domain.DeveloperInvitation, expectedRevision int64, meta ports.MutationMeta) (domain.DeveloperApplication, domain.DeveloperInvitation, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	application, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		SELECT `+developerApplicationColumns+` FROM developer_applications
		WHERE platform_organization_id = $1::uuid AND id = $2::uuid FOR UPDATE`, platformOrgID, applicationID))
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	if application.Revision != expectedRevision || (application.Status != domain.DeveloperApplicationStatusInvited && application.Status != domain.DeveloperApplicationStatusApproved) || application.OrganizationID == nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, fmt.Errorf("%w: invitation can no longer be rotated", domain.ErrConflict)
	}
	now := r.now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE developer_invitations SET status = 'revoked', revoked_at = $2
		WHERE application_id = $1::uuid AND status = 'pending'`, applicationID, now); err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, mapError(err)
	}
	createdInvitation, err := scanDeveloperInvitation(tx.QueryRow(ctx, `
		INSERT INTO developer_invitations (
			id, application_id, organization_id, email, role, token_hash, status, created_by, created_at, expires_at
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8::uuid, $9, $10)
		RETURNING `+developerInvitationColumns,
		invitation.ID, invitation.ApplicationID, invitation.OrganizationID, invitation.Email, invitation.Role,
		invitation.TokenHash, invitation.Status, invitation.CreatedBy, invitation.CreatedAt, invitation.ExpiresAt))
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	updated, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		UPDATE developer_applications SET status = 'invited', updated_at = $3, revision = revision + 1
		WHERE platform_organization_id = $1::uuid AND id = $2::uuid
		RETURNING `+developerApplicationColumns, platformOrgID, applicationID, now))
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	if err := r.appendAudit(ctx, tx, platformOrgID, meta, "developer_invitation", invitation.ID, map[string]any{"application_id": applicationID}); err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, mapError(err)
	}
	updated.CurrentInvitation = &createdInvitation
	return updated, createdInvitation, nil
}

func (r *Repository) RevokeDeveloperInvitation(ctx context.Context, platformOrgID, invitationID, actorID string) (domain.DeveloperInvitation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DeveloperInvitation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	invitation, err := scanDeveloperInvitation(tx.QueryRow(ctx, `
		SELECT `+developerInvitationColumns+` FROM developer_invitations i
		WHERE i.id = $1::uuid AND EXISTS (
			SELECT 1 FROM developer_applications a
			WHERE a.id = i.application_id AND a.platform_organization_id = $2::uuid
		) FOR UPDATE`, invitationID, platformOrgID))
	if err != nil {
		return domain.DeveloperInvitation{}, err
	}
	if invitation.Status != domain.DeveloperInvitationStatusPending {
		return domain.DeveloperInvitation{}, fmt.Errorf("%w: invitation is not pending", domain.ErrConflict)
	}
	now := r.now().UTC()
	updated, err := scanDeveloperInvitation(tx.QueryRow(ctx, `
		UPDATE developer_invitations SET status = 'revoked', revoked_at = $2
		WHERE id = $1::uuid RETURNING `+developerInvitationColumns, invitationID, now))
	if err != nil {
		return domain.DeveloperInvitation{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE developer_applications SET status = 'approved', updated_at = $2, revision = revision + 1
		WHERE id = $1::uuid AND status = 'invited'`, invitation.ApplicationID, now); err != nil {
		return domain.DeveloperInvitation{}, mapError(err)
	}
	if err := r.appendAudit(ctx, tx, platformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_invitation.revoked"}, "developer_invitation", invitationID, nil); err != nil {
		return domain.DeveloperInvitation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeveloperInvitation{}, mapError(err)
	}
	return updated, nil
}

func (r *Repository) AcceptDeveloperInvitation(ctx context.Context, tokenHash, actorID, actorEmail, displayName string, acceptedAt time.Time) (domain.DeveloperApplication, domain.OrganizationEntitlement, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	invitation, err := scanDeveloperInvitation(tx.QueryRow(ctx, `
		SELECT `+developerInvitationColumns+` FROM developer_invitations
		WHERE token_hash = $1 FOR UPDATE`, tokenHash))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
		}
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, err
	}
	if invitation.Status != domain.DeveloperInvitationStatusPending || !strings.EqualFold(invitation.Email, actorEmail) {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
	}
	if !acceptedAt.Before(invitation.ExpiresAt) {
		if _, updateErr := tx.Exec(ctx, `UPDATE developer_invitations SET status = 'expired' WHERE id = $1::uuid`, invitation.ID); updateErr != nil {
			return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, mapError(updateErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, mapError(commitErr)
		}
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
	}
	application, err := scanDeveloperApplication(tx.QueryRow(ctx, `
		SELECT `+developerApplicationColumns+` FROM developer_applications
		WHERE id = $1::uuid FOR UPDATE`, invitation.ApplicationID))
	if err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, err
	}
	if application.Status != domain.DeveloperApplicationStatusInvited {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
	}
	var existingUserID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE lower(email) = lower($1)`, actorEmail).Scan(&existingUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, mapError(err)
	}
	if existingUserID != "" && !strings.EqualFold(existingUserID, actorID) {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
	}
	if existingUserID == "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, email, display_name) VALUES ($1::uuid, $2, $3)`, actorID, actorEmail, displayName); err != nil {
			return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1::uuid, $2::uuid, 'owner')
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = 'owner', updated_at = $3`, invitation.OrganizationID, actorID, acceptedAt); err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE developer_invitations
		SET status = 'accepted', accepted_by = $2::uuid, accepted_at = $3
		WHERE id = $1::uuid`, invitation.ID, actorID, acceptedAt); err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, mapError(err)
	}
	application, err = scanDeveloperApplication(tx.QueryRow(ctx, `
		UPDATE developer_applications SET status = 'active', updated_at = $2, revision = revision + 1
		WHERE id = $1::uuid RETURNING `+developerApplicationColumns, application.ID, acceptedAt))
	if err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, err
	}
	var entitlement domain.OrganizationEntitlement
	if err := tx.QueryRow(ctx, `
		UPDATE organization_entitlements SET sandbox_access = true, updated_at = $2
		WHERE organization_id = $1::uuid
		RETURNING organization_id::text, sandbox_access, production_access, max_apps, max_webhooks, created_at, updated_at`, invitation.OrganizationID, acceptedAt).Scan(
		&entitlement.OrganizationID, &entitlement.SandboxAccess, &entitlement.ProductionAccess,
		&entitlement.MaxApps, &entitlement.MaxWebhooks, &entitlement.CreatedAt, &entitlement.UpdatedAt,
	); err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, mapError(err)
	}
	if err := r.appendAudit(ctx, tx, application.PlatformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_invitation.accepted"}, "developer_invitation", invitation.ID, map[string]any{"organization_id": invitation.OrganizationID}); err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, mapError(err)
	}
	return application, entitlement, nil
}

var _ ports.DeveloperProgramRepository = (*Repository)(nil)
