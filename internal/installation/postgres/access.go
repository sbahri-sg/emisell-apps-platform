package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/oauth/apptoken"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"github.com/jackc/pgx/v5"
)

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func activeGrant(ctx context.Context, q queryer, tenant string, ins domain.Installation) error {
	if ins.IntentID == "" {
		return nil
	} // Existing installations are not promoted.
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_installation.access_grants g
 JOIN platform_installation.intent_consumptions c USING(tenant_id,installation_id)
 WHERE g.tenant_id=$1 AND g.installation_id=$2 AND c.intent_id=$3 AND g.state='active' AND g.scopes=$4::jsonb)`, tenant, ins.ID, ins.IntentID, ins.Scopes).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return fault.Forbidden
	}
	return nil
}

func access(ctx context.Context, q queryer, o domain.IntentOwner, id string) (domain.Access, error) {
	var a domain.Access
	var appID, version, intentID string
	var scopes, capabilities []string
	a.Owner = o
	err := q.QueryRow(ctx, `SELECT c.intent_id,c.release,c.consent_digest,c.consumed_at,
 COALESCE(i.status,'uninstalled'),g.state,g.scopes,
 COALESCE(i.app_id,''),COALESCE(i.version,''),COALESCE(i.intent_id,''),COALESCE(i.scopes,'[]'),COALESCE(i.capabilities,'[]')
 FROM platform_installation.intent_consumptions c
 JOIN platform_installation.access_grants g USING(tenant_id,installation_id)
 LEFT JOIN platform_installation.installations i ON i.tenant_id=c.tenant_id AND i.id=c.installation_id
 WHERE c.tenant_id=$1 AND c.service_id=$2 AND c.actor_id=$3 AND c.installation_id=$4`, o.TenantID, o.ServiceID, o.ActorID, id).Scan(&a.IntentID, &a.Release, &a.ConsentDigest, &a.ConsumedAt, &a.Installation.Status, &a.GrantState, &a.GrantedScopes, &appID, &version, &intentID, &scopes, &capabilities)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, fault.NotFound
	}
	if err != nil {
		return a, err
	}
	if a.Installation.Status == "pending" || a.Installation.Status == "active" {
		if appID != a.Release.AppID || version != a.Release.Version || intentID != a.IntentID || !slices.Equal(scopes, a.Release.Scopes) || !slices.Equal(capabilities, a.Release.RoutedCapabilities()) {
			return a, fault.Forbidden
		}
	}
	a.Installation.ID, a.Installation.AppID, a.Installation.Version = id, a.Release.AppID, a.Release.Version
	a.Installation.IntentID = a.IntentID
	a.Installation.InstalledAt = a.ConsumedAt
	a.Installation.ExecutionProfile = a.Release.ExecutionProfile
	if a.Installation.Status == "pending" || a.Installation.Status == "active" {
		a.Installation.Scopes, a.Installation.Capabilities = slices.Clone(a.Release.Scopes), a.Release.RoutedCapabilities()
	} else {
		a.Installation.Scopes, a.Installation.Capabilities = []string{}, []string{}
	}
	return a, err
}

func (p Repository) GetAccess(ctx context.Context, o domain.IntentOwner, id string) (domain.Access, error) {
	return access(ctx, p.Pool, o, id)
}

func (p Repository) WithCoreAccess(ctx context.Context, o domain.IntentOwner, id string, fn func(domain.Access) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, o.TenantID); err != nil {
		return err
	}
	a, err := access(ctx, tx, o, id)
	if err != nil {
		return err
	}
	if err = fn(a); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func receipt(ctx context.Context, tx pgx.Tx, o domain.IntentOwner, key, hash string) (domain.AccessResult, bool, error) {
	var oldHash, id string
	var tokenID *string
	err := tx.QueryRow(ctx, `SELECT request_hash,installation_id,token_id FROM platform_installation.access_requests WHERE tenant_id=$1 AND service_id=$2 AND actor_id=$3 AND request_key=$4`, o.TenantID, o.ServiceID, o.ActorID, key).Scan(&oldHash, &id, &tokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AccessResult{}, false, nil
	}
	if err != nil {
		return domain.AccessResult{}, true, err
	}
	if oldHash != hash {
		return domain.AccessResult{}, true, fault.Conflict
	}
	a, err := access(ctx, tx, o, id)
	v := domain.AccessResult{Access: a, Replayed: true}
	if err == nil && tokenID != nil {
		v.Token.ID = *tokenID
		err = tx.QueryRow(ctx, `SELECT expires_at,(revoked_at IS NOT NULL OR expires_at<=clock_timestamp()) FROM platform_installation.app_tokens WHERE id=$1 AND tenant_id=$2 AND installation_id=$3`, *tokenID, o.TenantID, id).Scan(&v.Token.ExpiresAt, &v.Token.Revoked)
	}
	return v, true, err
}

func saveReceipt(ctx context.Context, tx pgx.Tx, o domain.IntentOwner, key, hash, id, token string) error {
	_, err := tx.Exec(ctx, `INSERT INTO platform_installation.access_requests(tenant_id,service_id,actor_id,request_key,request_hash,installation_id,token_id) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''))`, o.TenantID, o.ServiceID, o.ActorID, key, hash, id, token)
	return err
}

func auditAccess(ctx context.Context, tx pgx.Tx, a domain.Access, action string, now time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO platform_installation.access_audit(id,tenant_id,service_id,actor_id,installation_id,intent_id,action,consent_digest,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, ids.New("audit"), a.Owner.TenantID, a.Owner.ServiceID, a.Owner.ActorID, a.Installation.ID, a.IntentID, action, a.ConsentDigest, now)
	return err
}

func accessEvent(ctx context.Context, tx pgx.Tx, a domain.Access, kind, key string) error {
	e := event.New("emisell.app."+kind+".v1", a.Owner.TenantID, a.Owner.ActorID, a.Installation.ID, key, map[string]any{"appId": a.Release.AppID, "name": a.Release.Name, "version": a.Release.Version, "status": a.Installation.Status, "scopes": a.Installation.Scopes})
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_installation.events(id,tenant_id,envelope,occurred_at) VALUES($1,$2,$3,$4)`, e.ID, e.TenantID, raw, e.OccurredAt)
	return err
}

func (p Repository) ConsumeIntent(ctx context.Context, o domain.IntentOwner, key, hash, id string, validate func(domain.InstallIntent, time.Time) error) (domain.AccessResult, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return domain.AccessResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, o.TenantID); err != nil {
		return domain.AccessResult{}, err
	}
	if v, found, err := receipt(ctx, tx, o, key, hash); found || err != nil {
		return v, err
	}
	v, err := scanIntent(tx.QueryRow(ctx, "SELECT "+intentColumns+" FROM platform_installation.install_intents WHERE id=$1 AND tenant_id=$2 AND service_id=$3 AND actor_id=$4", id, o.TenantID, o.ServiceID, o.ActorID))
	if err != nil {
		return domain.AccessResult{}, err
	}
	var consumed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_installation.intent_consumptions WHERE intent_id=$1)`, id).Scan(&consumed); err != nil {
		return domain.AccessResult{}, err
	}
	if consumed {
		return domain.AccessResult{}, fault.Conflict
	}
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return domain.AccessResult{}, err
	}
	if err = validate(v, now); err != nil {
		return domain.AccessResult{}, err
	}
	// Expiry is measured again after signature/policy work, before writes.
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return domain.AccessResult{}, err
	}
	if !now.Before(v.ExpiresAt) {
		return domain.AccessResult{}, fault.Conflict
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM platform_installation.installations WHERE tenant_id=$1 AND app_id=$2`, o.TenantID, v.Release.AppID).Scan(&status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.AccessResult{}, err
	}
	if err == nil && status != "uninstalled" {
		return domain.AccessResult{}, fault.Conflict
	}
	ins := domain.Installation{ID: ids.New("ins"), IntentID: id, AppID: v.Release.AppID, Version: v.Release.Version, Status: "pending", Scopes: slices.Clone(v.Release.Scopes), Capabilities: v.Release.RoutedCapabilities(), InstalledAt: now, ExecutionProfile: v.Release.ExecutionProfile}
	a := domain.Access{Owner: o, IntentID: id, Release: v.Release, ConsentDigest: v.ConsentDigest, Installation: ins, GrantState: "pending", GrantedScopes: []string{}, ConsumedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO platform_installation.intent_consumptions(intent_id,tenant_id,service_id,actor_id,installation_id,release,consent_digest,consumed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, o.TenantID, o.ServiceID, o.ActorID, ins.ID, v.Release, v.ConsentDigest, now)
	if err != nil {
		return domain.AccessResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_installation.installations(tenant_id,app_id,id,version,status,scopes,capabilities,installed_at,intent_id)
 VALUES($1,$2,$3,$4,'pending',$5,$6,$7,$8) ON CONFLICT(tenant_id,app_id) DO UPDATE SET id=excluded.id,version=excluded.version,status=excluded.status,scopes=excluded.scopes,capabilities=excluded.capabilities,installed_at=excluded.installed_at,intent_id=excluded.intent_id,updated_at=now(),cleanup_attempts=0,cleanup_next_at=now(),cleanup_revision=platform_installation.installations.cleanup_revision+1`, o.TenantID, ins.AppID, ins.ID, ins.Version, ins.Scopes, ins.Capabilities, now, id)
	if err != nil {
		return domain.AccessResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_installation.access_grants(tenant_id,installation_id,state,scopes) VALUES($1,$2,'pending','[]')`, o.TenantID, ins.ID)
	if err != nil {
		return domain.AccessResult{}, err
	}
	if err = auditAccess(ctx, tx, a, "consumed", now); err != nil {
		return domain.AccessResult{}, err
	}
	if err = accessEvent(ctx, tx, a, "installed", key); err != nil {
		return domain.AccessResult{}, err
	}
	if err = saveReceipt(ctx, tx, o, key, hash, ins.ID, ""); err != nil {
		return domain.AccessResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.AccessResult{}, err
	}
	return domain.AccessResult{Access: a}, nil
}

func (p Repository) ChangeAccess(ctx context.Context, o domain.IntentOwner, key, hash, id, action, tokenID, tokenHash string, change func(domain.Access, time.Time) (domain.Access, error)) (domain.AccessResult, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return domain.AccessResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, o.TenantID); err != nil {
		return domain.AccessResult{}, err
	}
	if v, found, err := receipt(ctx, tx, o, key, hash); found || err != nil {
		return v, err
	}
	before, err := access(ctx, tx, o, id)
	if err != nil {
		return domain.AccessResult{}, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return domain.AccessResult{}, err
	}
	a, err := change(before, now)
	if err != nil {
		return domain.AccessResult{}, err
	}
	// Begin token TTL after any remote handshake, never before it.
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return domain.AccessResult{}, err
	}
	v := domain.AccessResult{Access: a}
	if a.Installation.Status != before.Installation.Status {
		if a.Installation.Status == "active" {
			if a.Release.ExecutionProfile == domain.ManagedShippingProfile {
				var busy bool
				err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_installation.installations i JOIN platform_installation.intent_consumptions c ON c.tenant_id=i.tenant_id AND c.installation_id=i.id WHERE i.tenant_id=$1 AND i.id!=$2 AND i.status='active' AND c.release->>'executionProfile'=$3 AND c.release->'shippingProvider'=$4::jsonb)`, o.TenantID, id, domain.ManagedShippingProfile, a.Release.ShippingProvider).Scan(&busy)
				if err != nil {
					return v, err
				}
				if busy {
					return v, fault.Conflict
				}
			}
			for _, cap := range a.Installation.Capabilities {
				var busy bool
				err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_installation.installations WHERE tenant_id=$1 AND id!=$2 AND status='active' AND capabilities ? $3)`, o.TenantID, id, cap).Scan(&busy)
				if err != nil {
					return v, err
				}
				if busy {
					return v, fault.Conflict
				}
			}
			_, err = tx.Exec(ctx, `UPDATE platform_installation.access_grants SET state='active',scopes=$3 WHERE tenant_id=$1 AND installation_id=$2 AND state='pending'`, o.TenantID, id, a.GrantedScopes)
			if err != nil {
				return v, err
			}
		}
		tag, e := tx.Exec(ctx, `UPDATE platform_installation.installations SET status=$3,scopes=$4,capabilities=$5,updated_at=now(),cleanup_attempts=0,cleanup_next_at=now(),cleanup_revision=cleanup_revision+1 WHERE tenant_id=$1 AND id=$2`, o.TenantID, id, a.Installation.Status, a.Installation.Scopes, a.Installation.Capabilities)
		if e != nil {
			return v, e
		}
		if tag.RowsAffected() != 1 {
			return v, fault.Conflict
		}
		kind, audit := "activated", "activated"
		if action == "uninstall" {
			kind, audit = "uninstalled", "uninstalled"
			if a.Installation.Status == "disabling" {
				kind = "disabling"
			}
		}
		if err = auditAccess(ctx, tx, a, audit, now); err != nil {
			return v, err
		}
		if err = accessEvent(ctx, tx, a, kind, key); err != nil {
			return v, err
		}
	}
	if action == "issue_token" {
		_, err = tx.Exec(ctx, `UPDATE platform_installation.app_tokens SET revoked_at=$3 WHERE tenant_id=$1 AND installation_id=$2 AND revoked_at IS NULL`, o.TenantID, id, now)
		if err != nil {
			return v, err
		}
		v.Token = domain.AppToken{ID: tokenID, ExpiresAt: now.Add(apptoken.TTL)}
		_, err = tx.Exec(ctx, `INSERT INTO platform_installation.app_tokens(id,tenant_id,installation_id,token_hash,audience,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, tokenID, o.TenantID, id, tokenHash, apptoken.Audience, now, v.Token.ExpiresAt)
		if err != nil {
			return v, err
		}
		if err = auditAccess(ctx, tx, a, "token_issued", now); err != nil {
			return v, err
		}
	}
	if err = saveReceipt(ctx, tx, o, key, hash, id, tokenID); err != nil {
		return v, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.AccessResult{}, err
	}
	return v, nil
}

func (p Repository) WithAppAccess(ctx context.Context, hash, audience, tenant, app, id string, check func(domain.Access) error) (domain.Access, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return domain.Access{}, err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, tenant); err != nil {
		return domain.Access{}, err
	}
	o := domain.IntentOwner{TenantID: tenant}
	err = tx.QueryRow(ctx, `SELECT c.service_id,c.actor_id FROM platform_installation.app_tokens t
 JOIN platform_installation.intent_consumptions c USING(tenant_id,installation_id)
 JOIN platform_installation.installations i ON i.tenant_id=t.tenant_id AND i.id=t.installation_id
 JOIN platform_installation.access_grants g ON g.tenant_id=t.tenant_id AND g.installation_id=t.installation_id
 WHERE t.token_hash=$1 AND t.audience=$2 AND t.tenant_id=$3 AND t.installation_id=$4 AND i.app_id=$5
 AND t.revoked_at IS NULL AND t.expires_at>clock_timestamp() AND i.status='active' AND g.state='active' AND g.scopes=i.scopes`, hash, audience, tenant, id, app).Scan(&o.ServiceID, &o.ActorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Access{}, fault.Unauthenticated
	}
	if err != nil {
		return domain.Access{}, err
	}
	a, err := access(ctx, tx, o, id)
	if err != nil {
		return domain.Access{}, err
	}
	if !slices.Equal(a.GrantedScopes, a.Release.Scopes) {
		return domain.Access{}, fault.Forbidden
	}
	if err = check(a); err != nil {
		return domain.Access{}, err
	}
	// Recheck expiry after registry work under the same lifecycle lock.
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT expires_at>clock_timestamp() AND revoked_at IS NULL FROM platform_installation.app_tokens WHERE token_hash=$1`, hash).Scan(&valid); err != nil {
		return domain.Access{}, err
	}
	if !valid {
		return domain.Access{}, fault.Unauthenticated
	}
	return a, tx.Commit(ctx)
}
