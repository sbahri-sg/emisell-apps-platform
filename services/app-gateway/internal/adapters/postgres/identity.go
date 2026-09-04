package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListOrganizationMemberships(ctx context.Context, userID string) ([]domain.OrganizationMembership, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id::text, o.name, o.slug, o.status, m.role
		FROM organization_memberships m
		JOIN organizations o ON o.id = m.organization_id
		WHERE m.user_id = $1::uuid
		ORDER BY lower(o.name), o.id`, userID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]domain.OrganizationMembership, 0)
	for rows.Next() {
		var membership domain.OrganizationMembership
		var role string
		if err := rows.Scan(&membership.OrganizationID, &membership.Name, &membership.Slug, &membership.Status, &role); err != nil {
			return nil, mapError(err)
		}
		membership.Role = domain.Role(role)
		result = append(result, membership)
	}
	return result, mapError(rows.Err())
}

func (r *Repository) CreateIdentitySession(ctx context.Context, session domain.IdentitySession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO identity_sessions (
			id, token_hash, csrf_token_hash, user_id, active_organization_id, merchant_id, merchant_environment, platform_operator,
			created_at, last_seen_at, expires_at, idle_expires_at
		) VALUES ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6, $7, $8, $9, $10, $11, $12)`,
		session.ID, session.TokenHash, session.CSRFTokenHash, session.UserID, session.ActiveOrgID, session.MerchantID, session.MerchantEnvironment, session.PlatformOperator,
		session.CreatedAt, session.LastSeenAt, session.ExpiresAt, session.IdleExpiresAt)
	return mapError(err)
}

func (r *Repository) GetIdentitySessionByTokenHash(ctx context.Context, tokenHash string, now time.Time, idleTTL time.Duration) (domain.IdentitySession, *domain.OrganizationMembership, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.IdentitySession{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var session domain.IdentitySession
	var organizationID, organizationName, organizationSlug, organizationStatus, role *string
	err = tx.QueryRow(ctx, `
		SELECT s.id::text, s.token_hash, s.csrf_token_hash, s.user_id::text, s.active_organization_id::text,
			s.merchant_id::text, s.merchant_environment,
			u.email, u.display_name, s.platform_operator, s.created_at, s.last_seen_at,
			s.expires_at, s.idle_expires_at, s.revoked_at,
			o.id::text, o.name, o.slug, o.status, m.role
		FROM identity_sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN organization_memberships m
			ON m.user_id = s.user_id AND m.organization_id = s.active_organization_id
		LEFT JOIN organizations o ON o.id = m.organization_id AND o.status = 'active'
		WHERE s.token_hash = $1
		FOR UPDATE OF s`, tokenHash).Scan(
		&session.ID, &session.TokenHash, &session.CSRFTokenHash, &session.UserID, &session.ActiveOrgID, &session.MerchantID, &session.MerchantEnvironment,
		&session.Email, &session.DisplayName, &session.PlatformOperator, &session.CreatedAt,
		&session.LastSeenAt, &session.ExpiresAt, &session.IdleExpiresAt, &session.RevokedAt,
		&organizationID, &organizationName, &organizationSlug, &organizationStatus, &role,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IdentitySession{}, nil, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.IdentitySession{}, nil, mapError(err)
	}
	if session.RevokedAt != nil || !now.Before(session.ExpiresAt) || !now.Before(session.IdleExpiresAt) {
		return domain.IdentitySession{}, nil, domain.ErrUnauthorized
	}
	var membership *domain.OrganizationMembership
	if organizationID != nil && role != nil {
		membership = &domain.OrganizationMembership{
			OrganizationID: *organizationID, Name: *organizationName, Slug: *organizationSlug,
			Status: *organizationStatus, Role: domain.Role(*role),
		}
	} else if session.ActiveOrgID != nil {
		if _, err := tx.Exec(ctx, `UPDATE identity_sessions SET active_organization_id = NULL WHERE id = $1::uuid`, session.ID); err != nil {
			return domain.IdentitySession{}, nil, mapError(err)
		}
		session.ActiveOrgID = nil
	}
	newIdleExpiry := now.Add(idleTTL)
	if newIdleExpiry.After(session.ExpiresAt) {
		newIdleExpiry = session.ExpiresAt
	}
	if _, err := tx.Exec(ctx, `
		UPDATE identity_sessions SET last_seen_at = $2, idle_expires_at = $3
		WHERE id = $1::uuid`, session.ID, now, newIdleExpiry); err != nil {
		return domain.IdentitySession{}, nil, mapError(err)
	}
	session.LastSeenAt = now
	session.IdleExpiresAt = newIdleExpiry
	if err := tx.Commit(ctx); err != nil {
		return domain.IdentitySession{}, nil, mapError(err)
	}
	return session, membership, nil
}

func (r *Repository) SwitchIdentitySessionOrganization(ctx context.Context, sessionID, userID, organizationID string, now time.Time) (domain.IdentitySession, domain.OrganizationMembership, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var membership domain.OrganizationMembership
	var role string
	err = tx.QueryRow(ctx, `
		SELECT o.id::text, o.name, o.slug, o.status, m.role
		FROM organization_memberships m
		JOIN organizations o ON o.id = m.organization_id
		WHERE m.user_id = $1::uuid AND m.organization_id = $2::uuid AND o.status = 'active'`, userID, organizationID).Scan(
		&membership.OrganizationID, &membership.Name, &membership.Slug, &membership.Status, &role,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, mapError(err)
	}
	membership.Role = domain.Role(role)
	var session domain.IdentitySession
	err = tx.QueryRow(ctx, `
		UPDATE identity_sessions
		SET active_organization_id = $3::uuid, last_seen_at = $4
		WHERE id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL
			AND expires_at > $4 AND idle_expires_at > $4
		RETURNING id::text, token_hash, csrf_token_hash, user_id::text, active_organization_id::text,
			merchant_id::text, merchant_environment, platform_operator, created_at, last_seen_at, expires_at, idle_expires_at, revoked_at`,
		sessionID, userID, organizationID, now).Scan(
		&session.ID, &session.TokenHash, &session.CSRFTokenHash, &session.UserID, &session.ActiveOrgID, &session.MerchantID, &session.MerchantEnvironment,
		&session.PlatformOperator, &session.CreatedAt, &session.LastSeenAt,
		&session.ExpiresAt, &session.IdleExpiresAt, &session.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, mapError(err)
	}
	return session, membership, nil
}

func (r *Repository) RevokeIdentitySession(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE identity_sessions SET revoked_at = COALESCE(revoked_at, $2)
		WHERE token_hash = $1`, tokenHash, now)
	return mapError(err)
}

func (r *Repository) CreateOIDCLoginState(ctx context.Context, state domain.OIDCLoginState) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO oidc_login_states (
			id, state_hash, nonce_hash, code_verifier_ciphertext, return_to, created_at, expires_at
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)`,
		state.ID, state.StateHash, state.NonceHash, state.CodeVerifierCiphertext,
		state.ReturnTo, state.CreatedAt, state.ExpiresAt)
	return mapError(err)
}

func (r *Repository) ConsumeOIDCLoginState(ctx context.Context, stateHash string, now time.Time) (domain.OIDCLoginState, error) {
	var state domain.OIDCLoginState
	err := r.pool.QueryRow(ctx, `
		UPDATE oidc_login_states SET consumed_at = $2
		WHERE state_hash = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING id::text, state_hash, nonce_hash, code_verifier_ciphertext,
			return_to, created_at, expires_at, consumed_at`, stateHash, now).Scan(
		&state.ID, &state.StateHash, &state.NonceHash, &state.CodeVerifierCiphertext,
		&state.ReturnTo, &state.CreatedAt, &state.ExpiresAt, &state.ConsumedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OIDCLoginState{}, domain.ErrInvalidGrant
	}
	return state, mapError(err)
}

func (r *Repository) ProvisionOIDCUser(ctx context.Context, provider, subject, email, displayName, userID string, now time.Time) (string, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var resolvedUserID string
	err = tx.QueryRow(ctx, `
		SELECT user_id::text FROM user_identities
		WHERE provider = $1 AND subject = $2 FOR UPDATE`, provider, subject).Scan(&resolvedUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", mapError(err)
	}
	if resolvedUserID == "" {
		err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE lower(email) = lower($1) FOR UPDATE`, email).Scan(&resolvedUserID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", mapError(err)
		}
		if resolvedUserID == "" {
			resolvedUserID = userID
			if _, err := tx.Exec(ctx, `
				INSERT INTO users (id, email, display_name) VALUES ($1::uuid, $2, $3)`, resolvedUserID, strings.ToLower(email), displayName); err != nil {
				return "", mapError(err)
			}
		}
		identityID, err := r.id()
		if err != nil {
			return "", fmt.Errorf("generate identity id: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_identities (id, provider, subject, user_id, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4::uuid, $5, $5)`, identityID, provider, subject, resolvedUserID, now); err != nil {
			return "", mapError(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET email = $2, display_name = $3, updated_at = $4
		WHERE id = $1::uuid`, resolvedUserID, strings.ToLower(email), displayName, now); err != nil {
		return "", mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", mapError(err)
	}
	return resolvedUserID, nil
}

var _ ports.IdentityRepository = (*Repository)(nil)
