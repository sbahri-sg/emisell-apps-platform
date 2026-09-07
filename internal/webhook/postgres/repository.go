package postgres

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/webhook"
	"emisell.app/platform/pkg/sdk/events"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type Repository struct{ Pool *pgxpool.Pool }

func (p Repository) Enqueue(ctx context.Context, e events.Envelope) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "INSERT INTO platform_webhook.inbox(event_id) VALUES($1) ON CONFLICT DO NOTHING", e.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_webhook.deliveries(id,tenant_id,installation_id,event_id,body) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING", ids.New("delivery"), e.TenantID, e.Subject, e.ID, raw)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p Repository) Candidate(ctx context.Context) (webhook.Delivery, error) {
	var d webhook.Delivery
	err := p.Pool.QueryRow(ctx, "SELECT tenant_id,installation_id,id FROM platform_webhook.deliveries WHERE status='pending' AND next_at<=now() ORDER BY next_at,id LIMIT 1").Scan(&d.Tenant, &d.Installation, &d.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return d, err
}
func (p Repository) WithDelivery(ctx context.Context, id string, fn func(webhook.Delivery) (webhook.Outcome, error)) (string, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	d := webhook.Delivery{ID: id}
	err = tx.QueryRow(ctx, "SELECT tenant_id,installation_id,event_id,body,attempts,created_at FROM platform_webhook.deliveries WHERE id=$1 AND status='pending' AND next_at<=now() FOR UPDATE SKIP LOCKED", id).Scan(&d.Tenant, &d.Installation, &d.EventID, &d.Body, &d.Attempts, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "idle", nil
	}
	if err != nil {
		return "", err
	}
	next, err := fn(d)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "UPDATE platform_webhook.deliveries SET status=$2,attempts=$3,next_at=$4,last_error=$5,revision=revision+1,last_attempt_at=CASE WHEN $3>attempts THEN now() ELSE last_attempt_at END,completed_at=CASE WHEN $2='delivered' THEN now() ELSE completed_at END WHERE id=$1", id, next.Status, next.Attempts, next.NextAt, next.Reason)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_webhook.audit(tenant_id,installation_id,delivery_id,action,reason) VALUES($1,$2,$3,$4,$5)", d.Tenant, d.Installation, id, next.Status, next.Reason)
	if err != nil {
		return "", err
	}
	return next.Status, tx.Commit(ctx)
}
func (p Repository) Cancel(ctx context.Context, d webhook.Delivery) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE platform_webhook.deliveries SET status='cancelled',last_error='installation_removed',revision=revision+1 WHERE id=$1 AND tenant_id=$2 AND installation_id=$3 AND status='pending'", d.ID, d.Tenant, d.Installation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, "INSERT INTO platform_webhook.audit(tenant_id,installation_id,delivery_id,action,reason) VALUES($1,$2,$3,'cancelled','installation_removed')", d.Tenant, d.Installation, d.ID)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (p Repository) List(ctx context.Context, tenant string) ([]webhook.Delivery, error) {
	rows, err := p.Pool.Query(ctx, "SELECT id,tenant_id,installation_id,event_id,status,attempts,next_at FROM platform_webhook.deliveries WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT 50", tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []webhook.Delivery{}
	for rows.Next() {
		var d webhook.Delivery
		if err = rows.Scan(&d.ID, &d.Tenant, &d.Installation, &d.EventID, &d.Status, &d.Attempts, &d.NextAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (p Repository) Replay(ctx context.Context, tenant, id, reason string) error {
	if !events.Token.MatchString(tenant) || !events.Token.MatchString(id) || len(strings.TrimSpace(reason)) < 8 || len(reason) > 240 {
		return fault.Invalid
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ins string
	err = tx.QueryRow(ctx, "UPDATE platform_webhook.deliveries SET status='pending',attempts=0,next_at=now(),created_at=now(),last_error=NULL,completed_at=NULL,revision=revision+1 WHERE tenant_id=$1 AND id=$2 AND status='dead' RETURNING installation_id", tenant, id).Scan(&ins)
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_webhook.audit(tenant_id,installation_id,delivery_id,action,reason,actor_id) VALUES($1,$2,$3,'operator_replay',$4,'local-operator')", tenant, ins, id, reason)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p Repository) Counts(ctx context.Context) (webhook.Counts, error) {
	var c webhook.Counts
	err := p.Pool.QueryRow(ctx, "SELECT count(*) FILTER(WHERE status='pending'),count(*) FILTER(WHERE status='dead') FROM platform_webhook.deliveries").Scan(&c.Pending, &c.Dead)
	return c, err
}
