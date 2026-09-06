package postgres

import (
	"context"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
	"slices"
)

func (p Repository) ManagedTarget(ctx context.Context, merchant, provider string) (domain.Access, error) {
	var o domain.IntentOwner
	var id string
	o.TenantID = merchant
	rows, err := p.Pool.Query(ctx, `SELECT c.service_id,c.actor_id,i.id FROM platform_installation.installations i
 JOIN platform_installation.intent_consumptions c ON c.tenant_id=i.tenant_id AND c.installation_id=i.id AND c.intent_id=i.intent_id
 JOIN platform_installation.access_grants g ON g.tenant_id=i.tenant_id AND g.installation_id=i.id
 WHERE i.tenant_id=$1 AND i.status='active' AND g.state='active' AND c.release->>'executionProfile'=$2
 AND c.release->'shippingProvider'->>'providerCode'=$3 LIMIT 2`, merchant, domain.ManagedShippingProfile, provider)
	if err != nil {
		return domain.Access{}, err
	}
	n := 0
	for rows.Next() {
		n++
		if err = rows.Scan(&o.ServiceID, &o.ActorID, &id); err != nil {
			rows.Close()
			return domain.Access{}, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return domain.Access{}, err
	}
	if n != 1 {
		return domain.Access{}, fault.Forbidden
	}
	return access(ctx, p.Pool, o, id)
}

// Authorization is linearized with local uninstall. No grant response is cached.
// A bounded engine operation already authorized may finish during revocation.
func (p Repository) WithManagedAccess(ctx context.Context, merchant, id string, release domain.IntentRelease) (domain.Access, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return domain.Access{}, err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, merchant); err != nil {
		return domain.Access{}, err
	}
	o := domain.IntentOwner{TenantID: merchant}
	err = tx.QueryRow(ctx, `SELECT c.service_id,c.actor_id FROM platform_installation.intent_consumptions c JOIN platform_installation.installations i ON i.tenant_id=c.tenant_id AND i.id=c.installation_id AND i.intent_id=c.intent_id WHERE c.tenant_id=$1 AND c.installation_id=$2 AND i.status='active'`, merchant, id).Scan(&o.ServiceID, &o.ActorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Access{}, fault.Forbidden
	}
	if err != nil {
		return domain.Access{}, err
	}
	a, err := access(ctx, tx, o, id)
	if err != nil {
		return a, err
	}
	if a.GrantState != "active" || !slices.Equal(a.GrantedScopes, []string{"shipping.read"}) || service.RequestHash(a.Release) != service.RequestHash(release) {
		return a, fault.Forbidden
	}
	return a, tx.Commit(ctx)
}
