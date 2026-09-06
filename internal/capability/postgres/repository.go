package postgres

import (
	"context"
	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/platform/fault"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

func (p Repository) Execute(ctx context.Context, user, tenant, cap, installation, key, hash, correlation string, req capability.Request, run func(*capability.Resource) (*capability.Resource, []capability.Rate, error)) (capability.Response, error) {
	var result capability.Response
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	// The installation gate already serializes this tenant through completion.
	var oldHash string
	err = tx.QueryRow(ctx, "SELECT request_hash,response FROM platform_capability.idempotency WHERE tenant_id=$1 AND actor_id=$2 AND key=$3", tenant, user, key).Scan(&oldHash, &result)
	if err == nil {
		if oldHash != hash {
			return result, fault.Conflict
		}
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	var current *capability.Resource
	if req.ResourceID != "" {
		var value capability.Resource
		err = tx.QueryRow(ctx, "SELECT data FROM platform_capability.resources WHERE tenant_id=$1 AND installation_id=$2 AND id=$3 AND capability=$4 FOR UPDATE", tenant, installation, req.ResourceID, cap).Scan(&value)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				err = fault.NotFound
			}
			return result, err
		}
		current = &value
	}
	next, rates, err := run(current)
	if err != nil {
		return result, err
	}
	result = capability.Response{Capability: cap, Operation: req.Operation, InstallationID: installation, Simulation: true, Resource: next, Rates: rates}
	if next != nil && cap == "payment/v1" {
		next, _, err = savePayment(ctx, tx, tenant, installation, user, correlation, next)
		if err != nil {
			return result, err
		}
		result.Resource = next
	} else if next != nil {
		raw, err := json.Marshal(next)
		if err != nil {
			return result, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO platform_capability.resources(tenant_id,installation_id,id,capability,data) VALUES($1,$2,$3,$4,$5)
   ON CONFLICT(tenant_id,id) DO UPDATE SET data=excluded.data WHERE platform_capability.resources.installation_id=excluded.installation_id`, tenant, installation, next.ID, cap, raw)
		if err != nil {
			return result, err
		}
	}
	envelope := event.New("emisell.capability.invoked.v1", tenant, user, installation, correlation, map[string]string{"capability": cap, "operation": req.Operation})
	raw, err := json.Marshal(envelope)
	if err != nil {
		return result, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_capability.events(id,tenant_id,envelope,occurred_at) VALUES($1,$2,$3,$4)", envelope.ID, tenant, raw, envelope.OccurredAt)
	if err != nil {
		return result, err
	}
	raw, err = json.Marshal(result)
	if err != nil {
		return result, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_capability.idempotency(tenant_id,actor_id,key,request_hash,response) VALUES($1,$2,$3,$4,$5)", tenant, user, key, hash, raw)
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
