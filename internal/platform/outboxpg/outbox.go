// Package outboxpg is an infrastructure primitive instantiated by each owner.
package outboxpg

import (
	"context"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/sdk/events"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Store struct {
	Pool   *pgxpool.Pool
	Schema string
}

func (s Store) table() string {
	// Identifiers only originate in the owning module, never operator/user input.
	if s.Schema != "platform_installation" && s.Schema != "platform_capability" {
		panic("invalid outbox owner")
	}
	return s.Schema + ".events"
}
func (s Store) DeliverOne(ctx context.Context, publisher event.Publisher) (event.Delivery, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return event.Delivery{}, err
	}
	defer tx.Rollback(ctx)
	var id, tenant string
	var raw []byte
	var attempts int
	var occurred time.Time
	err = tx.QueryRow(ctx, `SELECT id,tenant_id,envelope,attempts,occurred_at FROM `+s.table()+` WHERE published_at IS NULL AND dead_at IS NULL AND next_attempt_at<=now() ORDER BY next_attempt_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &tenant, &raw, &attempts, &occurred)
	if errors.Is(err, pgx.ErrNoRows) {
		return event.Delivery{}, nil
	}
	if err != nil {
		return event.Delivery{}, err
	}
	result := event.Delivery{Found: true, Outcome: "published"}
	e, decodeErr := events.Decode(raw)
	if decodeErr != nil || e.ID != id || e.TenantID != tenant {
		err = event.ErrPermanent
	} else {
		publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = publisher.Publish(publishCtx, e)
		cancel()
	}
	attempts++
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE `+s.table()+` SET published_at=now(),attempts=$2,last_error=NULL WHERE id=$1`, id, attempts)
	} else {
		// No raw broker errors: they can include credentials or payload snippets.
		code := "publish_unavailable"
		dead := attempts >= 12 || time.Since(occurred) > 24*time.Hour
		if errors.Is(err, event.ErrPermanent) {
			code = "invalid_event"
			dead = true
		}
		result.Outcome = "retry"
		if dead {
			result.Outcome = "dead"
		}
		delay := time.Second * time.Duration(1<<min(attempts, 8))
		_, err = tx.Exec(ctx, `UPDATE `+s.table()+` SET attempts=$2,next_attempt_at=$3,last_error=$4,dead_at=CASE WHEN $5 THEN now() ELSE NULL END WHERE id=$1`, id, attempts, time.Now().Add(delay), code, dead)
	}
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
func (s Store) Counts(ctx context.Context) (event.Counts, error) {
	var c event.Counts
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE published_at IS NULL AND dead_at IS NULL),count(*) FILTER(WHERE dead_at IS NOT NULL) FROM `+s.table()).Scan(&c.Pending, &c.Dead)
	return c, err
}
func (s Store) Replay(ctx context.Context, id, actor, reason string) error {
	if !events.Token.MatchString(id) || actor == "" || len(reason) < 8 || len(reason) > 500 {
		return fault.Invalid
	}
	table := s.table()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE `+table+` SET attempts=0,next_attempt_at=now(),dead_at=NULL,last_error=NULL WHERE id=$1 AND dead_at IS NOT NULL AND published_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fault.NotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO `+s.Schema+`.outbox_replays(event_id,actor,reason) VALUES($1,$2,$3)`, id, actor, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
