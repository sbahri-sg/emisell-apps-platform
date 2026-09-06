package postgres

import (
	"context"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

// WithInstallation serializes OAuth and webhook operations with uninstall.
func (p Repository) WithInstallation(ctx context.Context, tenant, id string, fn func(domain.Installation) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, tenant); err != nil {
		return err
	}
	ins, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM platform_installation.installations WHERE tenant_id=$1 AND id=$2", tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return err
	}
	if ins.Status == "active" {
		if err = activeGrant(ctx, tx, tenant, ins); err != nil {
			return err
		}
	}
	if err = fn(ins); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Cleanup struct {
	Tenant, ID, App string
	Attempts        int
	Revision        int64
}

func (p Repository) RetryCleanup(ctx context.Context, tenant, id, reason string) error {
	if len(strings.TrimSpace(reason)) < 8 || len(reason) > 240 {
		return fault.Invalid
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, tenant); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "UPDATE platform_installation.installations SET cleanup_attempts=0,cleanup_next_at=now(),cleanup_revision=cleanup_revision+1 WHERE tenant_id=$1 AND id=$2 AND status='disabling'", tenant, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fault.NotFound
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_installation.cleanup_audit(tenant_id,installation_id,reason) VALUES($1,$2,$3)", tenant, id, reason)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p Repository) PendingCleanup(ctx context.Context) ([]Cleanup, error) {
	rows, err := p.Pool.Query(ctx, "SELECT tenant_id,id,app_id,cleanup_attempts,cleanup_revision FROM platform_installation.installations WHERE status='disabling' AND cleanup_attempts<12 AND cleanup_next_at<=now() ORDER BY cleanup_next_at LIMIT 20")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Cleanup{}
	for rows.Next() {
		var c Cleanup
		if err = rows.Scan(&c.Tenant, &c.ID, &c.App, &c.Attempts, &c.Revision); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Cleanup is internal-only. No public lifecycle action can skip revocation.
func (p Repository) CompleteCleanup(ctx context.Context, c Cleanup, revoke func(context.Context, string, string) error) error {
	_, err := p.Change(ctx, c.Tenant, "system-cleanup", "cleanup_"+c.ID, "cleanup_"+c.ID, c.App, func(ins *domain.Installation) (*domain.Installation, *event.Envelope, error) {
		if ins == nil || ins.ID != c.ID || ins.Status != "disabling" {
			return nil, nil, fault.Conflict
		}
		if err := revoke(ctx, c.Tenant, c.ID); err != nil {
			return nil, nil, err
		}
		next := *ins
		next.Status = "uninstalled"
		e := event.New("emisell.app.uninstalled.v1", c.Tenant, "system-cleanup", c.ID, "cleanup_"+c.ID, map[string]any{"appId": c.App, "name": "Remote app", "status": "uninstalled", "version": ins.Version})
		return &next, &e, nil
	})
	if err == nil {
		return nil
	}
	retry := time.Now().Add(time.Duration(1<<min(c.Attempts+1, 8)) * time.Second)
	// Ignore stale failures after another worker or owner has advanced recovery.
	_, recordErr := p.Pool.Exec(ctx, `WITH changed AS (
 UPDATE platform_installation.installations SET cleanup_attempts=cleanup_attempts+1,cleanup_next_at=$3,cleanup_revision=cleanup_revision+1 WHERE tenant_id=$1 AND id=$2 AND status='disabling' AND cleanup_revision=$4 RETURNING tenant_id,id
 ) INSERT INTO platform_installation.cleanup_audit(tenant_id,installation_id,reason,actor_id,action) SELECT tenant_id,id,'revocation_failed','system-cleanup','cleanup_failed' FROM changed`, c.Tenant, c.ID, retry, c.Revision)
	if recordErr != nil {
		return recordErr
	}
	return err
}
