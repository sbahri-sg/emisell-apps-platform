package postgres

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

type Repository struct{ Pool *pgxpool.Pool }

func (p Repository) FindUser(ctx context.Context, email string) (identity.User, string, error) {
	var u identity.User
	var hash string
	err := p.Pool.QueryRow(ctx, "SELECT id,email,password_hash FROM platform_identity.users WHERE email=$1", email).Scan(&u.ID, &u.Email, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return u, hash, err
}
func (p Repository) SaveSession(ctx context.Context, hash, user string, expiry time.Time) error {
	_, err := p.Pool.Exec(ctx, "INSERT INTO platform_identity.sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)", hash, user, expiry)
	return err
}
func (p Repository) SessionUser(ctx context.Context, hash string) (identity.User, error) {
	var u identity.User
	err := p.Pool.QueryRow(ctx, "SELECT u.id,u.email FROM platform_identity.sessions s JOIN platform_identity.users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now()", hash).Scan(&u.ID, &u.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Unauthenticated
	}
	return u, err
}
func (p Repository) DeleteSession(ctx context.Context, hash string) error {
	_, err := p.Pool.Exec(ctx, "DELETE FROM platform_identity.sessions WHERE token_hash=$1", hash)
	return err
}
func (p Repository) Workspaces(ctx context.Context, user string) ([]identity.Workspace, error) {
	rows, err := p.Pool.Query(ctx, "SELECT w.id,w.name FROM platform_identity.workspaces w JOIN platform_identity.memberships m ON m.tenant_id=w.id WHERE m.user_id=$1 ORDER BY w.id", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []identity.Workspace{}
	for rows.Next() {
		var w identity.Workspace
		if err = rows.Scan(&w.ID, &w.Name); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (p Repository) HasMembership(ctx context.Context, user, tenant string) (bool, error) {
	var ok bool
	err := p.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM platform_identity.memberships WHERE user_id=$1 AND tenant_id=$2 AND role='owner')", user, tenant).Scan(&ok)
	return ok, err
}

func (p Repository) SeedUser(ctx context.Context, id, email, password string, workspaces []identity.Workspace) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if existing, hash, err := p.FindUser(ctx, email); err == nil {
		if existing.ID != id || !identity.VerifyPassword(password, hash) {
			return fault.Conflict
		}
	} else if !errors.Is(err, fault.NotFound) {
		return err
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
	if _, err = tx.Exec(ctx, "INSERT INTO platform_identity.users(id,email,password_hash) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", id, email, hash); err != nil {
		return err
	}
	for _, w := range workspaces {
		if _, err = tx.Exec(ctx, "INSERT INTO platform_identity.workspaces(id,name) VALUES($1,$2) ON CONFLICT DO NOTHING", w.ID, w.Name); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO platform_identity.memberships(user_id,tenant_id,role) VALUES($1,$2,'owner') ON CONFLICT DO NOTHING", id, w.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
