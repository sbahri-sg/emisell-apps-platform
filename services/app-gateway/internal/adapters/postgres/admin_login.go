package postgres

import (
	"context"
	"errors"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) FindAdminAccount(ctx context.Context, email string) (domain.AdminAccount, error) {
	var a domain.AdminAccount
	err := r.pool.QueryRow(ctx, `SELECT user_id::text, platform_organization_id::text, email, display_name, password_hash, disabled_at FROM admin_password_accounts WHERE email = $1`, email).Scan(&a.UserID, &a.OrganizationID, &a.Email, &a.DisplayName, &a.PasswordHash, &a.DisabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, domain.ErrNotFound
	}
	return a, mapError(err)
}

func (r *Repository) ConsumeAdminLoginAttempt(ctx context.Context, hash string, limit int, now time.Time) (bool, error) {
	if _, err := r.pool.Exec(ctx, `DELETE FROM admin_login_attempts WHERE expires_at < $1`, now); err != nil {
		return false, mapError(err)
	}
	var attempts int
	err := r.pool.QueryRow(ctx, `INSERT INTO admin_login_attempts (bucket_hash, attempts, expires_at) VALUES ($1, 1, $2)
		ON CONFLICT (bucket_hash) DO UPDATE SET
		attempts = CASE WHEN admin_login_attempts.expires_at <= $3 THEN 1 ELSE LEAST(admin_login_attempts.attempts + 1, 100) END,
		expires_at = CASE WHEN admin_login_attempts.expires_at <= $3 THEN $2 ELSE admin_login_attempts.expires_at END
		RETURNING attempts`, hash, now.Add(15*time.Minute), now).Scan(&attempts)
	return attempts <= limit, mapError(err)
}

func (r *Repository) CreateAdminSession(ctx context.Context, s domain.IdentitySession, passwordHash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var enabled bool
	err = tx.QueryRow(ctx, `SELECT a.disabled_at IS NULL AND a.password_hash = $2 AND o.status = 'active' FROM admin_password_accounts a JOIN organizations o ON o.id = a.platform_organization_id WHERE a.user_id = $1::uuid FOR UPDATE OF a`, s.UserID, passwordHash).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !enabled) {
		return domain.ErrUnauthorized
	}
	if err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM admin_identity_sessions WHERE user_id = $1::uuid AND (expires_at <= $2 OR revoked_at IS NOT NULL)`, s.UserID, s.CreatedAt); err != nil {
		return mapError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO admin_identity_sessions (id, user_id, token_hash, csrf_token_hash, created_at, last_seen_at, expires_at, idle_expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)`, s.ID, s.UserID, s.TokenHash, s.CSRFTokenHash, s.CreatedAt, s.LastSeenAt, s.ExpiresAt, s.IdleExpiresAt)
	if err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func (r *Repository) GetAdminSession(ctx context.Context, hash string, now time.Time, idle time.Duration) (domain.IdentitySession, error) {
	var s domain.IdentitySession
	err := r.pool.QueryRow(ctx, `UPDATE admin_identity_sessions s SET last_seen_at = $2, idle_expires_at = LEAST(s.expires_at, $3)
		FROM admin_password_accounts a JOIN organizations o ON o.id = a.platform_organization_id WHERE s.token_hash = $1 AND s.user_id = a.user_id AND o.status = 'active'
		AND a.disabled_at IS NULL AND s.revoked_at IS NULL AND s.expires_at > $2 AND s.idle_expires_at > $2
		RETURNING s.id::text, s.user_id::text, a.platform_organization_id::text, a.email, a.display_name, s.token_hash, s.csrf_token_hash, s.created_at, s.last_seen_at, s.expires_at, s.idle_expires_at`, hash, now, now.Add(idle)).Scan(
		&s.ID, &s.UserID, &s.ActiveOrgID, &s.Email, &s.DisplayName, &s.TokenHash, &s.CSRFTokenHash, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt, &s.IdleExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, domain.ErrUnauthorized
	}
	s.PlatformOperator = err == nil
	return s, mapError(err)
}

func (r *Repository) RevokeAdminSession(ctx context.Context, hash string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin_identity_sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE token_hash = $1`, hash, now)
	return mapError(err)
}

// Provisioning is intentionally CLI-only. Existing developer identities are
// never silently promoted or attached by matching an email address.
func (r *Repository) CreateAdminAccount(ctx context.Context, a domain.AdminAccount) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = $1)`, a.Email).Scan(&exists); err != nil {
		return mapError(err)
	}
	if exists {
		return domain.ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1::uuid, $2, $3)`, a.UserID, a.Email, a.DisplayName); err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO admin_password_accounts (user_id, email, display_name, password_hash, platform_organization_id) VALUES ($1::uuid, $2, $3, $4, $5::uuid)`, a.UserID, a.Email, a.DisplayName, a.PasswordHash, a.OrganizationID); err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func (r *Repository) UpdateAdminPassword(ctx context.Context, email, hash string, disable bool, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM admin_password_accounts WHERE email = $1 FOR UPDATE`, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return mapError(err)
	}
	if disable {
		_, err = tx.Exec(ctx, `UPDATE admin_password_accounts SET disabled_at = $2, updated_at = $2 WHERE user_id = $1::uuid`, userID, now)
	} else {
		_, err = tx.Exec(ctx, `UPDATE admin_password_accounts SET password_hash = $2, updated_at = $3 WHERE user_id = $1::uuid`, userID, hash, now)
	}
	if err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE admin_identity_sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE user_id = $1::uuid`, userID, now); err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

var _ ports.AdminLoginRepository = (*Repository)(nil)
