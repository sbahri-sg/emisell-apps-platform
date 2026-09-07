package providergrant

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (p Postgres) Snapshot(ctx context.Context, r Request) (Snapshot, error) {
	var v Snapshot
	// Both consented scopes and current grants must allow the operation. App and
	// engine binding come from the consumed release, not request metadata.
	err := p.Pool.QueryRow(ctx, `SELECT i.tenant_id,c.release->'shippingProvider'->>'providerCode',i.app_id,i.id,g.provider_revision,
 i.status='active' AND g.state='active', g.state='revoked' OR i.status IN ('disabling','uninstalled'),
 COALESCE((SELECT jsonb_agg(s) FROM jsonb_array_elements_text(g.scopes) s WHERE c.release->'scopes' ? s),'[]'::jsonb)
 FROM platform_installation.installations i
 JOIN platform_installation.intent_consumptions c ON c.tenant_id=i.tenant_id AND c.installation_id=i.id AND c.intent_id=i.intent_id
 JOIN platform_installation.access_grants g ON g.tenant_id=i.tenant_id AND g.installation_id=i.id
 WHERE i.tenant_id=$1 AND i.id=$2 AND i.app_id=$3 AND c.release->>'appId'=i.app_id
 AND c.release->'shippingProvider'->>'engine'='api-kurir' AND c.release->'shippingProvider'->>'providerCode'=$4`, r.MerchantID, r.InstallationID, r.AppID, r.ProviderCode).Scan(&v.MerchantID, &v.ProviderCode, &v.AppID, &v.InstallationID, &v.Revision, &v.Active, &v.Revoked, &v.Scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, Denied
	}
	return v, err
}
