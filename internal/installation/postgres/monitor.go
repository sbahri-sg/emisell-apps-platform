package postgres

import (
	"context"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/recovery"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (p Repository) ConnectionRecords(ctx context.Context, tenant string) ([]service.ConnectionRecord, error) {
	out := []service.ConnectionRecord{}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,app_id,status,cleanup_attempts,CASE WHEN status='disabling' AND cleanup_attempts<12 THEN cleanup_next_at END,cleanup_revision FROM platform_installation.installations WHERE tenant_id=$1 AND status!='uninstalled' ORDER BY installed_at,id`, tenant)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r service.ConnectionRecord
		r.History = []service.CleanupAudit{}
		if err = rows.Scan(&r.InstallationID, &r.AppID, &r.Status, &r.CleanupAttempts, &r.CleanupNextAt, &r.Revision); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		rows, err = tx.Query(ctx, "SELECT action,reason,actor_id,occurred_at FROM platform_installation.cleanup_audit WHERE tenant_id=$1 AND installation_id=$2 ORDER BY occurred_at DESC,id DESC LIMIT 50", tenant, out[i].InstallationID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var a service.CleanupAudit
			if err = rows.Scan(&a.Action, &a.Reason, &a.ActorID, &a.OccurredAt); err != nil {
				rows.Close()
				return nil, err
			}
			out[i].History = append(out[i].History, a)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, tx.Commit(ctx)
}
func (p Repository) RequestCleanup(ctx context.Context, user, tenant, id, key string, request recovery.Request) (recovery.Result, error) {
	var result recovery.Result
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, tenant); err != nil {
		return result, err
	}
	var hash string
	err = tx.QueryRow(ctx, "SELECT request_hash,response FROM platform_installation.recovery_requests WHERE tenant_id=$1 AND actor_id=$2 AND key=$3", tenant, user, key).Scan(&hash, &result)
	if err == nil {
		if hash != request.Hash(id) {
			return result, fault.Conflict
		}
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM platform_installation.installations WHERE tenant_id=$1 AND id=$2)", tenant, id).Scan(&exists)
	if err != nil {
		return result, err
	}
	if !exists {
		return result, fault.NotFound
	}
	err = tx.QueryRow(ctx, "UPDATE platform_installation.installations SET cleanup_attempts=0,cleanup_next_at=now(),cleanup_revision=cleanup_revision+1 WHERE tenant_id=$1 AND id=$2 AND status='disabling' AND cleanup_attempts>=12 AND cleanup_revision=$3 RETURNING cleanup_revision", tenant, id, request.ExpectedRevision).Scan(&result.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, fault.Conflict
	}
	if err != nil {
		return result, err
	}
	result.Status = "disabling"
	_, err = tx.Exec(ctx, "INSERT INTO platform_installation.cleanup_audit(tenant_id,installation_id,reason,actor_id,action) VALUES($1,$2,$3,$4,'owner_retry')", tenant, id, request.Reason, user)
	if err != nil {
		return result, err
	}
	raw, _ := json.Marshal(result)
	_, err = tx.Exec(ctx, "INSERT INTO platform_installation.recovery_requests(tenant_id,actor_id,key,request_hash,response) VALUES($1,$2,$3,$4,$5)", tenant, user, key, request.Hash(id), raw)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
