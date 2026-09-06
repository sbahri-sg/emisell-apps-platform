package postgres

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/mail"
	"strings"
	"time"
)

func (p Repository) FindPortal(ctx context.Context, surface, email string) (identity.PortalPrincipal, string, error) {
	var v identity.PortalPrincipal
	var hash string
	err := p.Pool.QueryRow(ctx, `SELECT id,email,surface,role,password_hash FROM platform_identity.portal_accounts WHERE surface=$1 AND email=$2 AND enabled`, surface, email).Scan(&v.ID, &v.Email, &v.Surface, &v.Role, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return v, hash, err
}

func (p Repository) ListStaff(ctx context.Context, after string) ([]identity.StaffAccount, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,email,role,enabled FROM platform_identity.portal_accounts WHERE surface='admin' AND id>$1 ORDER BY id LIMIT 51`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []identity.StaffAccount{}
	for rows.Next() {
		var account identity.StaffAccount
		if err = rows.Scan(&account.ID, &account.Email, &account.Role, &account.Enabled); err != nil {
			return nil, err
		}
		out = append(out, account)
	}
	return out, rows.Err()
}
func (p Repository) ReplacePortalSession(ctx context.Context, id, surface, hash, old, expectedPassword string, expires time.Time) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize with credential updates so an in-flight old-password login cannot
	// recreate a session after revocation.
	var current string
	err = tx.QueryRow(ctx, `SELECT password_hash FROM platform_identity.portal_accounts WHERE id=$1 AND surface=$2 AND enabled FOR UPDATE`, id, surface).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current != expectedPassword) {
		return fault.Unauthenticated
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM platform_identity.portal_sessions WHERE surface=$1 AND (token_hash=$2 OR expires_at<=now())`, surface, old); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_sessions(token_hash,account_id,surface,expires_at) VALUES($1,$2,$3,$4)`, hash, id, surface, expires); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_audit(id,actor_id,action) VALUES($1,$2,'login')`, ids.New("aud"), id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdatePrimaryAdmin is local maintenance, not a browser API. Identity and role
// are preserved; credential replacement and session revocation are atomic.
func (p Repository) UpdatePrimaryAdmin(ctx context.Context, expected identity.PortalPrincipal, previousPassword, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 || expected.ID != "portal-local-admin" || expected.Surface != "admin" || expected.Role != "administrator" {
		return fault.Invalid
	}
	hash, err := identity.HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var actual identity.PortalPrincipal
	var current string
	err = tx.QueryRow(ctx, `SELECT id,email,surface,role,password_hash FROM platform_identity.portal_accounts WHERE id=$1 AND enabled FOR UPDATE`, expected.ID).Scan(&actual.ID, &actual.Email, &actual.Surface, &actual.Role, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return err
	}
	if actual.Surface != "admin" || actual.Role != "administrator" {
		return fault.Conflict
	}
	if actual.Email == email && identity.VerifyPassword(password, current) {
		return nil
	}
	if actual != expected || !identity.VerifyPassword(previousPassword, current) {
		return fault.Conflict
	}
	if _, err = tx.Exec(ctx, `UPDATE platform_identity.portal_accounts SET email=$2,password_hash=$3 WHERE id=$1`, actual.ID, email, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM platform_identity.portal_sessions WHERE account_id=$1`, actual.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_audit(id,actor_id,action) VALUES($1,$2,'credentials_updated_sessions_revoked')`, ids.New("aud"), actual.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p Repository) PortalSession(ctx context.Context, surface, hash string) (identity.PortalPrincipal, error) {
	var v identity.PortalPrincipal
	err := p.Pool.QueryRow(ctx, `SELECT a.id,a.email,a.surface,a.role FROM platform_identity.portal_sessions s JOIN platform_identity.portal_accounts a ON a.id=s.account_id AND a.surface=s.surface WHERE s.surface=$1 AND s.token_hash=$2 AND s.expires_at>now() AND a.enabled`, surface, hash).Scan(&v.ID, &v.Email, &v.Surface, &v.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Unauthenticated
	}
	return v, err
}
func (p Repository) DeletePortalSession(ctx context.Context, surface, hash string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var actor string
	err = tx.QueryRow(ctx, `DELETE FROM platform_identity.portal_sessions WHERE surface=$1 AND token_hash=$2 RETURNING account_id`, surface, hash).Scan(&actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_audit(id,actor_id,action) VALUES($1,$2,'logout')`, ids.New("aud"), actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SeedPortal is explicit CLI/test provisioning, never a browser registration path.
// Re-runs verify the existing account; they never reset passwords or upgrade roles.
func (p Repository) SeedPortal(ctx context.Context, v identity.PortalPrincipal, password string) error {
	if v.ID == "" || v.Email == "" || !((v.Surface == "developer" && v.Role == "developer") || (v.Surface == "admin" && (v.Role == "administrator" || v.Role == "reviewer" || v.Role == "operator"))) {
		return fault.Invalid
	}
	v.Email = strings.ToLower(strings.TrimSpace(v.Email))
	hash, err := identity.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = p.Pool.Exec(ctx, `INSERT INTO platform_identity.portal_accounts(id,email,surface,role,password_hash) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, v.ID, v.Email, v.Surface, v.Role, hash)
	if err != nil {
		return err
	}
	existing, h, err := p.FindPortal(ctx, v.Surface, v.Email)
	if err != nil {
		return err
	}
	if existing != v || !identity.VerifyPassword(password, h) {
		return fault.Conflict
	}
	return nil
}
