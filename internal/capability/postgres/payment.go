package postgres

import (
	"context"
	"crypto/sha256"
	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/sdk/events"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

// savePayment runs inside the caller's transaction and installation gate.
// Snapshots may skip intermediate states but can never regress or change money.
func savePayment(ctx context.Context, tx pgx.Tx, tenant, ins, actor, correlation string, next *capability.Resource) (*capability.Resource, string, error) {
	var old capability.Resource
	var installation string
	var capabilityID string
	var revision int64
	err := tx.QueryRow(ctx, "SELECT installation_id,capability,data,status_revision FROM platform_capability.resources WHERE tenant_id=$1 AND id=$2 FOR UPDATE", tenant, next.ID).Scan(&installation, &capabilityID, &old, &revision)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}
	changed := !exists || old.Status != next.Status || revision == 0
	if exists {
		if capabilityID != "payment/v1" || installation != ins || old.Reference != next.Reference || old.AmountMinor != next.AmountMinor || old.Currency != next.Currency {
			return nil, "", fault.Conflict
		}
		if next.Revision < old.Revision {
			return &old, "stale", nil
		}
		if next.Revision == old.Revision && next.Status != old.Status {
			return nil, "", fault.Conflict
		}
		if events.PaymentRank(next.Status) < events.PaymentRank(old.Status) {
			return nil, "", fault.Conflict
		}
	}
	if changed {
		revision++
	}
	p := events.PaymentStatus{ResourceID: next.ID, InstallationID: ins, Reference: next.Reference, Status: next.Status, AmountMinor: next.AmountMinor, Currency: next.Currency, Revision: revision, Simulation: true}
	if p.Validate() != nil {
		return nil, "", fault.Invalid
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return nil, "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_capability.resources(tenant_id,installation_id,id,capability,data,status_revision,observed_at) VALUES($1,$2,$3,'payment/v1',$4,$5,now())
 ON CONFLICT(tenant_id,id) DO UPDATE SET data=excluded.data,status_revision=excluded.status_revision,observed_at=CASE WHEN platform_capability.resources.status_revision<>excluded.status_revision THEN now() ELSE platform_capability.resources.observed_at END`, tenant, ins, next.ID, raw, revision)
	if err != nil {
		return nil, "", err
	}
	if !changed {
		return next, "unchanged", nil
	}
	e := event.New(events.PaymentStatusType, tenant, actor, next.ID, correlation, p)
	raw, err = json.Marshal(e)
	if err != nil {
		return nil, "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_capability.events(id,tenant_id,envelope,occurred_at) VALUES($1,$2,$3,$4)", e.ID, tenant, raw, e.OccurredAt)
	return next, "applied", err
}

func (p Repository) ApplyCallback(ctx context.Context, tenant, ins, delivery, body, correlation string, u appapi.PaymentUpdate) (string, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	h := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(h[:])
	var prior string
	err = tx.QueryRow(ctx, "SELECT body_hash FROM platform_capability.callback_inbox WHERE tenant_id=$1 AND installation_id=$2 AND delivery_id=$3", tenant, ins, delivery).Scan(&prior)
	if err == nil {
		if prior != hash {
			return "", fault.Conflict
		}
		return "duplicate", nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	var found bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM platform_capability.resources WHERE tenant_id=$1 AND installation_id=$2 AND id=$3 AND capability='payment/v1')", tenant, ins, u.Resource.ID).Scan(&found)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fault.NotFound
	}
	_, outcome, err := savePayment(ctx, tx, tenant, ins, "app-callback", correlation, &u.Resource)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO platform_capability.callback_inbox(tenant_id,installation_id,delivery_id,resource_id,body_hash,outcome) VALUES($1,$2,$3,$4,$5,$6)", tenant, ins, delivery, u.Resource.ID, hash, outcome)
	if err != nil {
		return "", err
	}
	return outcome, tx.Commit(ctx)
}

func (p Repository) Payments(ctx context.Context, tenant, status, cursor string) (capability.PaymentPage, error) {
	page := capability.PaymentPage{Items: []capability.PaymentRecord{}}
	rows, err := p.Pool.Query(ctx, `SELECT data,installation_id,observed_at,status_revision FROM platform_capability.resources WHERE tenant_id=$1 AND capability='payment/v1' AND ($2='' OR data->>'status'=$2) AND ($3='' OR id>$3) ORDER BY id LIMIT 21`, tenant, status, cursor)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var r capability.PaymentRecord
		if err = rows.Scan(&r.Resource, &r.InstallationID, &r.ObservedAt, &r.StatusRevision); err != nil {
			return page, err
		}
		page.Items = append(page.Items, r)
	}
	if len(page.Items) > 20 {
		page.Items = page.Items[:20]
		page.NextCursor = page.Items[19].ID
	}
	return page, rows.Err()
}
func (p Repository) Payment(ctx context.Context, tenant, id string) (capability.PaymentDetail, error) {
	d := capability.PaymentDetail{History: []capability.PaymentHistory{}}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, "SELECT data,installation_id,observed_at,status_revision FROM platform_capability.resources WHERE tenant_id=$1 AND id=$2 AND capability='payment/v1'", tenant, id).Scan(&d.Resource, &d.InstallationID, &d.ObservedAt, &d.StatusRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, fault.NotFound
	}
	if err != nil {
		return d, err
	}
	rows, err := tx.Query(ctx, `SELECT id,envelope->'payload'->>'status',(envelope->'payload'->>'revision')::bigint,CASE WHEN envelope->>'actorId'='app-callback' THEN 'app_callback' ELSE 'capability_invocation' END,occurred_at FROM platform_capability.events WHERE tenant_id=$1 AND envelope->>'subject'=$2 AND envelope->>'type'='emisell.payment.status_changed.v1' ORDER BY occurred_at DESC,id DESC LIMIT 50`, tenant, id)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var h capability.PaymentHistory
		if err = rows.Scan(&h.ID, &h.Status, &h.Revision, &h.Source, &h.OccurredAt); err != nil {
			rows.Close()
			return d, err
		}
		d.History = append(d.History, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
