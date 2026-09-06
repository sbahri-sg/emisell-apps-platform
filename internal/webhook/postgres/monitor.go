package postgres

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/recovery"
	"emisell.app/platform/internal/webhook"
	"emisell.app/platform/pkg/sdk/events"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

const monitorColumns = "id,installation_id,event_id,status,attempts,revision,enqueued_at,CASE WHEN status='pending' THEN next_at END,last_attempt_at,completed_at,COALESCE(last_error,'')"

func scanRecord(row pgx.Row) (webhook.Record, error) {
	var r webhook.Record
	err := row.Scan(&r.ID, &r.InstallationID, &r.EventID, &r.Status, &r.Attempts, &r.Revision, &r.EnqueuedAt, &r.NextAt, &r.LastAttemptAt, &r.CompletedAt, &r.LastError)
	return r, err
}
func (p Repository) Page(ctx context.Context, tenant, status, cursor string) (webhook.Page, error) {
	result := webhook.Page{Items: []webhook.Record{}}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if cursor != "" {
		var exists bool
		err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM platform_webhook.deliveries WHERE tenant_id=$1 AND id=$2)", tenant, cursor).Scan(&exists)
		if err != nil {
			return result, err
		}
		if !exists {
			return result, fault.Invalid
		}
	}
	err = tx.QueryRow(ctx, "SELECT count(*) FILTER(WHERE status='pending'),count(*) FILTER(WHERE status='delivered'),count(*) FILTER(WHERE status='dead'),count(*) FILTER(WHERE status='cancelled') FROM platform_webhook.deliveries WHERE tenant_id=$1", tenant).Scan(&result.Summary.Pending, &result.Summary.Delivered, &result.Summary.Dead, &result.Summary.Cancelled)
	if err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, "SELECT "+monitorColumns+" FROM platform_webhook.deliveries WHERE tenant_id=$1 AND ($2='' OR status=$2) AND ($3='' OR (enqueued_at,id)<(SELECT enqueued_at,id FROM platform_webhook.deliveries WHERE tenant_id=$1 AND id=$3)) ORDER BY enqueued_at DESC,id DESC LIMIT 21", tenant, status, cursor)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			rows.Close()
			return result, err
		}
		result.Items = append(result.Items, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(result.Items) > 20 {
		result.Items = result.Items[:20]
		result.NextCursor = result.Items[19].ID
	}
	return result, tx.Commit(ctx)
}
func (p Repository) Detail(ctx context.Context, tenant, id string) (webhook.Detail, error) {
	var d webhook.Detail
	d.History = []webhook.Audit{}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	d.Record, err = scanRecord(tx.QueryRow(ctx, "SELECT "+monitorColumns+" FROM platform_webhook.deliveries WHERE tenant_id=$1 AND id=$2", tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return d, fault.NotFound
	}
	if err != nil {
		return d, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, "SELECT body FROM platform_webhook.deliveries WHERE tenant_id=$1 AND id=$2", tenant, id).Scan(&raw)
	if err != nil {
		return d, err
	}
	e, err := events.Decode(raw)
	if err != nil || e.TenantID != tenant || e.Subject != d.InstallationID || e.ID != d.EventID {
		return d, fault.Unavailable
	}
	d.EventType = e.Type
	d.CorrelationID = e.CorrelationID
	rows, err := tx.Query(ctx, "SELECT action,reason,actor_id,occurred_at FROM platform_webhook.audit WHERE tenant_id=$1 AND delivery_id=$2 ORDER BY occurred_at DESC,id DESC LIMIT 50", tenant, id)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var a webhook.Audit
		if err = rows.Scan(&a.Action, &a.Reason, &a.ActorID, &a.OccurredAt); err != nil {
			rows.Close()
			return d, err
		}
		d.History = append(d.History, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
func (p Repository) RequestReplay(ctx context.Context, user, tenant, id, key string, request recovery.Request) (recovery.Result, error) {
	var result recovery.Result
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	// Same actor/key serialized even when two different deliveries are submitted concurrently.
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "webhook-recovery:"+tenant+":"+user+":"+key)
	if err != nil {
		return result, err
	}
	var hash string
	err = tx.QueryRow(ctx, "SELECT request_hash,response FROM platform_webhook.recovery_requests WHERE tenant_id=$1 AND actor_id=$2 AND key=$3", tenant, user, key).Scan(&hash, &result)
	if err == nil {
		if hash != request.Hash(id) {
			return result, fault.Conflict
		}
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	var ins string
	err = tx.QueryRow(ctx, "UPDATE platform_webhook.deliveries SET status='pending',attempts=0,next_at=now(),created_at=now(),last_error=NULL,completed_at=NULL,revision=revision+1 WHERE tenant_id=$1 AND id=$2 AND status='dead' AND revision=$3 RETURNING installation_id,revision", tenant, id, request.ExpectedRevision).Scan(&ins, &result.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, fault.Conflict
	}
	if err != nil {
		return result, err
	}
	result.Status = "pending"
	_, err = tx.Exec(ctx, "INSERT INTO platform_webhook.audit(tenant_id,installation_id,delivery_id,action,reason,actor_id) VALUES($1,$2,$3,'owner_replay',$4,$5)", tenant, ins, id, request.Reason, user)
	if err != nil {
		return result, err
	}
	raw, _ := json.Marshal(result)
	_, err = tx.Exec(ctx, "INSERT INTO platform_webhook.recovery_requests(tenant_id,actor_id,key,request_hash,response) VALUES($1,$2,$3,$4,$5)", tenant, user, key, request.Hash(id), raw)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
