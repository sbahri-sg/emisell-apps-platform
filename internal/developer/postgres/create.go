package postgres

import (
	"context"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
)

// Provision all records atomically; never overwrite an existing login.
func (p Repository) CreateDeveloper(ctx context.Context, actor, name, email, hash string) (developer.AdminOrganization, error) {
	result := developer.AdminOrganization{ID: ids.New("org"), Name: name, MemberCount: 1}
	account := ids.New("dev")
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return developer.AdminOrganization{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "portal-email:"+email); err != nil {
		return developer.AdminOrganization{}, err
	}
	var exists bool
	// Avoid ambiguous unified logins, including an existing administrator email.
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_identity.portal_accounts WHERE lower(email)=$1)`, email).Scan(&exists); err != nil {
		return developer.AdminOrganization{}, err
	}
	if exists {
		return developer.AdminOrganization{}, fault.Conflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_accounts(id,email,password_hash,surface,role,enabled) VALUES($1,$2,$3,'developer','developer',true)`, account, email, hash); err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return developer.AdminOrganization{}, fault.Conflict
		}
		return developer.AdminOrganization{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_developer.organizations(id,name) VALUES($1,$2)`, result.ID, name); err != nil {
		return developer.AdminOrganization{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_developer.memberships(account_id,organization_id,role) VALUES($1,$2,'owner')`, account, result.ID); err != nil {
		return developer.AdminOrganization{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_audit(id,actor_id,action) VALUES($1,$2,$3)`, ids.New("audit"), actor, "developer.created:"+account); err != nil {
		return developer.AdminOrganization{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return developer.AdminOrganization{}, err
	}
	return result, nil
}
