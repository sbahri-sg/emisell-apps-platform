package referencecore

import (
	"context"
	"emisell.app/platform/pkg/sdk/events"
	"errors"
	"github.com/jackc/pgx/v5"
)

func applyPayment(ctx context.Context, tx pgx.Tx, e events.Envelope) (string, error) {
	p, err := events.DecodePayment(e)
	if err != nil {
		return "invalid_payment", nil
	}
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "reference-core-payment:"+e.TenantID+":"+p.ResourceID)
	if err != nil {
		return "", err
	}
	var old events.PaymentStatus
	err = tx.QueryRow(ctx, "SELECT installation_id,reference,status,amount_minor,currency,revision FROM reference_core.payments WHERE tenant_id=$1 AND resource_id=$2", e.TenantID, p.ResourceID).Scan(&old.InstallationID, &old.Reference, &old.Status, &old.AmountMinor, &old.Currency, &old.Revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if err == nil {
		if old.InstallationID != p.InstallationID || old.Reference != p.Reference || old.AmountMinor != p.AmountMinor || old.Currency != p.Currency {
			return "payment_identity_conflict", nil
		}
		if p.Revision < old.Revision {
			return "", nil
		}
		if p.Revision == old.Revision {
			if old.Status != p.Status {
				return "payment_revision_conflict", nil
			}
			return "", nil
		}
		if events.PaymentRank(p.Status) < events.PaymentRank(old.Status) {
			return "payment_status_regression", nil
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO reference_core.payments(tenant_id,resource_id,installation_id,reference,status,amount_minor,currency,revision,event_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(tenant_id,resource_id) DO UPDATE SET status=excluded.status,revision=excluded.revision,event_id=excluded.event_id,updated_at=now()`, e.TenantID, p.ResourceID, p.InstallationID, p.Reference, p.Status, p.AmountMinor, p.Currency, p.Revision, e.ID)
	return "", err
}
