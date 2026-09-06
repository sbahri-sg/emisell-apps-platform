// Package referencecore demonstrates a client-owned inbox, not Core production logic.
package referencecore

import (
	"context"
	_ "embed"
	"emisell.app/platform/pkg/sdk/events"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"time"
)

//go:embed inbox.sql
var schema string

//go:embed payments.sql
var paymentsSchema string

func Init(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, paymentsSchema)
	return err
}

type Inbox struct{ Pool *pgxpool.Pool }

// Apply commits inbox deduplication and its reference side effect together.
// Invalid/foreign messages are quarantined without persisting their payload.
func (i Inbox) Apply(ctx context.Context, tenant string, sequence uint64, subject string, raw []byte) (string, error) {
	e, err := events.Decode(raw)
	reason := ""
	if err != nil {
		reason = "invalid_contract"
	} else if e.TenantID != tenant || subject != events.Subject(e) {
		reason = "tenant_or_subject_mismatch"
	}
	if reason != "" {
		_, err = i.Pool.Exec(ctx, `INSERT INTO reference_core.dead_letters(tenant_id,stream_sequence,reason) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, tenant, int64(sequence), reason)
		return "quarantined", err
	}
	tx, err := i.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO reference_core.inbox(tenant_id,event_id,envelope) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, tenant, e.ID, raw)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		var same bool
		if err = tx.QueryRow(ctx, `SELECT envelope=$3::jsonb FROM reference_core.inbox WHERE tenant_id=$1 AND event_id=$2`, tenant, e.ID, raw).Scan(&same); err != nil {
			return "", err
		}
		if same {
			return "duplicate", nil
		}
		if _, err = tx.Exec(ctx, `INSERT INTO reference_core.dead_letters(tenant_id,stream_sequence,reason) VALUES($1,$2,'event_id_collision') ON CONFLICT DO NOTHING`, tenant, int64(sequence)); err != nil {
			return "", err
		}
		return "quarantined", tx.Commit(ctx)
	}
	if e.Type == events.PaymentStatusType {
		reason, err := applyPayment(ctx, tx, e)
		if err != nil {
			return "", err
		}
		if reason != "" {
			if _, err = tx.Exec(ctx, "DELETE FROM reference_core.inbox WHERE tenant_id=$1 AND event_id=$2", tenant, e.ID); err != nil {
				return "", err
			}
			if _, err = tx.Exec(ctx, "INSERT INTO reference_core.dead_letters(tenant_id,stream_sequence,reason) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", tenant, int64(sequence), reason); err != nil {
				return "", err
			}
			return "quarantined", tx.Commit(ctx)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reference_core.receipts(tenant_id,event_type,count) VALUES($1,$2,1) ON CONFLICT(tenant_id,event_type) DO UPDATE SET count=reference_core.receipts.count+1`, tenant, e.Type); err != nil {
		return "", err
	}
	return "applied", tx.Commit(ctx)
}

// ConsumeOne ACKs only after durable commit. DB outages are not ACKed; the
// broker re-delivers with bounded backoff. No global ordering is assumed.
func (i Inbox) ConsumeOne(ctx context.Context, consumer jetstream.Consumer, tenant string) (string, error) {
	msg, err := consumer.Next(jetstream.FetchMaxWait(time.Second))
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, nats.ErrTimeout) {
		return "idle", nil
	}
	if err != nil {
		return "", err
	}
	md, err := msg.Metadata()
	if err != nil {
		return "", err
	}
	outcome, err := i.Apply(ctx, tenant, md.Sequence.Stream, msg.Subject(), msg.Data())
	if err != nil {
		return "", err
	}
	if outcome == "quarantined" {
		return outcome, msg.Term()
	}
	ackCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return outcome, msg.DoubleAck(ackCtx)
}
