package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"github.com/jackc/pgx/v5"
)

const intentColumns = "snapshot,state,decided_at"

func scanIntent(row pgx.Row) (domain.InstallIntent, error) {
	var v domain.InstallIntent
	var state string
	var decided *time.Time
	err := row.Scan(&v, &state, &decided)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, fault.NotFound
	}
	v.State, v.DecidedAt = state, decided
	return v, err
}

func (p Repository) GetIntent(ctx context.Context, owner domain.IntentOwner, id string) (domain.InstallIntent, error) {
	var v domain.InstallIntent
	var state string
	var decided *time.Time
	var now time.Time
	err := p.Pool.QueryRow(ctx, "SELECT "+intentColumns+",clock_timestamp() FROM platform_installation.install_intents WHERE id=$1 AND tenant_id=$2 AND service_id=$3 AND actor_id=$4", id, owner.TenantID, owner.ServiceID, owner.ActorID).Scan(&v, &state, &decided, &now)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, fault.NotFound
	}
	if err != nil {
		return v, err
	}
	v.State, v.DecidedAt = state, decided
	return v.Effective(now), nil
}

func (p Repository) ChangeIntent(ctx context.Context, owner domain.IntentOwner, key, hash, id string, change func(*domain.InstallIntent, time.Time) (domain.InstallIntent, error)) (domain.InstallIntent, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	defer tx.Rollback(ctx)
	// Same installation lifecycle lock namespace, ready for a future atomic consume.
	if err = lock(ctx, tx, owner.TenantID); err != nil {
		return domain.InstallIntent{}, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return domain.InstallIntent{}, err
	}
	var oldHash, oldID string
	err = tx.QueryRow(ctx, "SELECT request_hash,intent_id FROM platform_installation.intent_requests WHERE tenant_id=$1 AND service_id=$2 AND actor_id=$3 AND request_key=$4", owner.TenantID, owner.ServiceID, owner.ActorID, key).Scan(&oldHash, &oldID)
	if err == nil {
		if hash != oldHash {
			return domain.InstallIntent{}, fault.Conflict
		}
		v, err := scanIntent(tx.QueryRow(ctx, "SELECT "+intentColumns+" FROM platform_installation.install_intents WHERE id=$1 AND tenant_id=$2 AND service_id=$3 AND actor_id=$4", oldID, owner.TenantID, owner.ServiceID, owner.ActorID))
		return v.Effective(now), err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.InstallIntent{}, err
	}
	var current *domain.InstallIntent
	if id != "" {
		v, err := scanIntent(tx.QueryRow(ctx, "SELECT "+intentColumns+" FROM platform_installation.install_intents WHERE id=$1 AND tenant_id=$2 AND service_id=$3 AND actor_id=$4 FOR UPDATE", id, owner.TenantID, owner.ServiceID, owner.ActorID))
		if err != nil {
			return domain.InstallIntent{}, err
		}
		current = &v
	}
	next, err := change(current, now)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	// Recheck after registry work, so an intent expiring during validation fails closed.
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return domain.InstallIntent{}, err
	}
	if !now.Before(next.ExpiresAt) {
		return domain.InstallIntent{}, fault.Conflict
	}
	if current != nil {
		next.DecidedAt = &now
	}
	if current == nil {
		raw, err := json.Marshal(next)
		if err != nil {
			return domain.InstallIntent{}, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO platform_installation.install_intents(id,tenant_id,service_id,actor_id,snapshot,state,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", next.ID, owner.TenantID, owner.ServiceID, owner.ActorID, raw, next.State, next.CreatedAt, next.ExpiresAt)
		if err != nil {
			return domain.InstallIntent{}, err
		}
	} else {
		_, err = tx.Exec(ctx, "UPDATE platform_installation.install_intents SET state=$5,decided_at=$6 WHERE id=$1 AND tenant_id=$2 AND service_id=$3 AND actor_id=$4", next.ID, owner.TenantID, owner.ServiceID, owner.ActorID, next.State, next.DecidedAt)
		if err != nil {
			return domain.InstallIntent{}, err
		}
	}
	action := next.State
	if current == nil {
		action = "prepared"
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_installation.intent_audit(id,intent_id,tenant_id,service_id,actor_id,action,consent_digest,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", ids.New("audit"), next.ID, owner.TenantID, owner.ServiceID, owner.ActorID, action, next.ConsentDigest, now)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_installation.intent_requests(tenant_id,service_id,actor_id,request_key,request_hash,intent_id) VALUES($1,$2,$3,$4,$5,$6)", owner.TenantID, owner.ServiceID, owner.ActorID, key, hash, next.ID)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.InstallIntent{}, err
	}
	return next, nil
}
