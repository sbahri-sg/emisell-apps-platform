package postgres

import (
	"context"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

// Composed with current Core identity checks without opening a nested pool lease.
func (p Repository) OwnsOrganizationTx(ctx context.Context, tx pgx.Tx, account, org string) error {
	var actual string
	err := tx.QueryRow(ctx, `SELECT organization_id FROM platform_developer.memberships WHERE account_id=$1 FOR SHARE`, account).Scan(&actual)
	if err != nil || actual != org {
		return fault.Forbidden
	}
	return nil
}

func (p Repository) ListOrganizations(ctx context.Context, after string) ([]developer.AdminOrganization, error) {
	rows, err := p.Pool.Query(ctx, `SELECT o.id,o.name,(SELECT count(*) FROM platform_developer.memberships m WHERE m.organization_id=o.id) FROM platform_developer.organizations o WHERE o.id>$1 ORDER BY o.id LIMIT 51`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []developer.AdminOrganization{}
	for rows.Next() {
		var v developer.AdminOrganization
		if err = rows.Scan(&v.ID, &v.Name, &v.MemberCount); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (p Repository) GetOrganization(ctx context.Context, id string) (developer.AdminOrganization, error) {
	var v developer.AdminOrganization
	err := p.Pool.QueryRow(ctx, `SELECT o.id,o.name,(SELECT count(*) FROM platform_developer.memberships m WHERE m.organization_id=o.id) FROM platform_developer.organizations o WHERE o.id=$1`, id).Scan(&v.ID, &v.Name, &v.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return v, err
}

func (p Repository) Organization(ctx context.Context, account string) (developer.Organization, error) {
	var v developer.Organization
	err := p.Pool.QueryRow(ctx, `SELECT o.id,o.name,m.role FROM platform_developer.memberships m JOIN platform_developer.organizations o ON o.id=m.organization_id WHERE m.account_id=$1`, account).Scan(&v.ID, &v.Name, &v.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Forbidden
	}
	return v, err
}
func (p Repository) SeedOrganization(ctx context.Context, account, id, name string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO platform_developer.organizations(id,name) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, name); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_developer.memberships(account_id,organization_id,role) VALUES($1,$2,'owner') ON CONFLICT DO NOTHING`, account, id); err != nil {
		return err
	}
	var existing string
	if err = tx.QueryRow(ctx, `SELECT organization_id FROM platform_developer.memberships WHERE account_id=$1`, account).Scan(&existing); err != nil {
		return err
	}
	if existing != id {
		return fault.Conflict
	}
	return tx.Commit(ctx)
}
