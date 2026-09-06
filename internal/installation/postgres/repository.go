package postgres

import (
	"context"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

const columns = "id,app_id,version,status,scopes,capabilities,installed_at,COALESCE(intent_id,'')"

func scan(row pgx.Row) (domain.Installation, error) {
	var i domain.Installation
	err := row.Scan(&i.ID, &i.AppID, &i.Version, &i.Status, &i.Scopes, &i.Capabilities, &i.InstalledAt, &i.IntentID)
	return i, err
}
func lock(ctx context.Context, tx pgx.Tx, tenant string) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "installation:"+tenant)
	return err
}
func (p Repository) List(ctx context.Context, tenant string) ([]domain.Installation, error) {
	rows, err := p.Pool.Query(ctx, "SELECT "+columns+" FROM platform_installation.installations WHERE tenant_id=$1 AND status!='uninstalled' ORDER BY installed_at,id", tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Installation{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Repository) Events(ctx context.Context, tenant string) ([]event.Envelope, error) {
	rows, err := p.Pool.Query(ctx, "SELECT envelope FROM platform_installation.events WHERE tenant_id=$1 ORDER BY occurred_at DESC,id DESC LIMIT 100", tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []event.Envelope{}
	for rows.Next() {
		var e event.Envelope
		if err = rows.Scan(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (p Repository) Change(ctx context.Context, tenant, user, key, hash, app string, change func(*domain.Installation) (*domain.Installation, *event.Envelope, error)) (*domain.Installation, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, tenant); err != nil {
		return nil, err
	}
	var oldHash string
	var cached domain.Installation
	err = tx.QueryRow(ctx, "SELECT request_hash,response FROM platform_installation.idempotency WHERE tenant_id=$1 AND actor_id=$2 AND key=$3", tenant, user, key).Scan(&oldHash, &cached)
	if err == nil {
		if oldHash != hash {
			return nil, fault.Conflict
		}
		return &cached, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	current, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM platform_installation.installations WHERE tenant_id=$1 AND app_id=$2 FOR UPDATE", tenant, app))
	var ptr *domain.Installation
	if err == nil {
		ptr = &current
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	next, envelope, err := change(ptr)
	if err != nil {
		return nil, err
	}
	if envelope != nil {
		if next.Status == "active" {
			for _, cap := range next.Capabilities {
				var count int
				err = tx.QueryRow(ctx, "SELECT count(*) FROM platform_installation.installations WHERE tenant_id=$1 AND app_id!=$2 AND status='active' AND capabilities ? $3", tenant, app, cap).Scan(&count)
				if err != nil {
					return nil, err
				}
				if count > 0 {
					return nil, fault.Conflict
				}
			}
		}
		scopes, _ := json.Marshal(next.Scopes)
		caps, _ := json.Marshal(next.Capabilities)
		_, err = tx.Exec(ctx, `INSERT INTO platform_installation.installations(tenant_id,app_id,id,version,status,scopes,capabilities,installed_at)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(tenant_id,app_id) DO UPDATE SET
   id=excluded.id,version=excluded.version,status=excluded.status,scopes=excluded.scopes,capabilities=excluded.capabilities,installed_at=excluded.installed_at,updated_at=now(),cleanup_attempts=0,cleanup_next_at=now(),cleanup_revision=platform_installation.installations.cleanup_revision+1`, tenant, app, next.ID, next.Version, next.Status, scopes, caps, next.InstalledAt)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(envelope)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO platform_installation.events(id,tenant_id,envelope,occurred_at) VALUES($1,$2,$3,$4)", envelope.ID, tenant, raw, envelope.OccurredAt)
		if err != nil {
			return nil, err
		}
	}
	response, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_installation.idempotency(tenant_id,actor_id,key,request_hash,response) VALUES($1,$2,$3,$4,$5)", tenant, user, key, hash, response)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return next, nil
}
func (p Repository) WithActive(ctx context.Context, tenant, capability string, fn func(domain.Installation) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, tenant); err != nil {
		return err
	}
	ins, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM platform_installation.installations WHERE tenant_id=$1 AND status='active' AND capabilities ? $2", tenant, capability))
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return err
	}
	if err = activeGrant(ctx, tx, tenant, ins); err != nil {
		return err
	}
	if err = fn(ins); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
