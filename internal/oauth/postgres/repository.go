package postgres

import (
	"context"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

func (p Repository) Begin(ctx context.Context, s oauth.State) (oauth.State, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return s, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO platform_oauth.states(state_hash,tenant_id,installation_id,actor_id,session_hash,request_key,material,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, s.Hash, s.Tenant, s.Installation, s.Actor, s.Session, s.Key, s.Material, s.Expires)
	if err != nil {
		return s, err
	}
	err = tx.QueryRow(ctx, "SELECT state_hash,material,expires_at FROM platform_oauth.states WHERE tenant_id=$1 AND installation_id=$2 AND actor_id=$3 AND session_hash=$4 AND request_key=$5 AND used_at IS NULL AND expires_at>now()", s.Tenant, s.Installation, s.Actor, s.Session, s.Key).Scan(&s.Hash, &s.Material, &s.Expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, fault.Conflict
	}
	if err != nil {
		return s, err
	}
	return s, tx.Commit(ctx)
}
func (p Repository) Consume(ctx context.Context, hash, actor, session string) (oauth.State, error) {
	var s oauth.State
	err := p.Pool.QueryRow(ctx, `UPDATE platform_oauth.states SET used_at=now() WHERE state_hash=$1 AND actor_id=$2 AND session_hash=$3 AND used_at IS NULL AND expires_at>now() RETURNING tenant_id,installation_id,material`, hash, actor, session).Scan(&s.Tenant, &s.Installation, &s.Material)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, fault.Invalid
	}
	return s, err
}
func (p Repository) Load(ctx context.Context, tenant, ins string) (string, []byte, error) {
	var status string
	var raw []byte
	err := p.Pool.QueryRow(ctx, "SELECT status,secret FROM platform_oauth.connections WHERE tenant_id=$1 AND installation_id=$2", tenant, ins).Scan(&status, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "needs_connection", nil, nil
	}
	return status, raw, err
}
func (p Repository) Save(ctx context.Context, tenant, ins, actor, status string, raw []byte) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO platform_oauth.connections(tenant_id,installation_id,status,secret) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,installation_id) DO UPDATE SET status=excluded.status,secret=excluded.secret,updated_at=now()`, tenant, ins, status, raw)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_oauth.audit(tenant_id,installation_id,actor_id,action) VALUES($1,$2,$3,$4)", tenant, ins, actor, status)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p Repository) Invalidate(ctx context.Context, tenant, ins string) error {
	if err := p.Save(ctx, tenant, ins, "system-cleanup", "revoked", nil); err != nil {
		return err
	}
	_, err := p.Pool.Exec(ctx, "UPDATE platform_oauth.states SET used_at=now(),material=''::bytea WHERE tenant_id=$1 AND installation_id=$2 AND used_at IS NULL", tenant, ins)
	return err
}
