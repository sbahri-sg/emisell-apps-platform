// Package postgres persists reviewed launch bindings. It owns only launch tables.
package postgres

import (
	"context"
	"crypto/ed25519"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/embedded"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type Record struct {
	ID        string          `json:"id"`
	Launch    embedded.Launch `json:"binding"`
	Status    string          `json:"status"`
	Signature string          `json:"signature"`
	Revision  int             `json:"revision"`
}
type Repository struct{ Pool *pgxpool.Pool }

func (r Repository) ForClient(ctx context.Context, client string) (Record, error) {
	return scan(r.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_app.embedded_launches WHERE client_id=$1`, client))
}

func (r Repository) Get(ctx context.Context, id string) (Record, error) {
	return scan(r.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_app.embedded_launches WHERE id=$1`, id))
}

func scan(row pgx.Row) (Record, error) {
	var v Record
	var raw []byte
	err := row.Scan(&v.ID, &raw, &v.Status, &v.Signature, &v.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, fault.NotFound
	}
	if err != nil {
		return v, err
	}
	if json.Unmarshal(raw, &v.Launch) != nil {
		return Record{}, fault.Invalid
	}
	return v, nil
}

const columns = `id,launch,status,coalesce(signature,''),revision`

// Submit is internal: the caller must resolve ownership/current signed release
// before passing launch metadata. No arbitrary browser identity may reach here.
func (r Repository) Submit(ctx context.Context, actor string, l embedded.Launch, reason string) (Record, error) {
	if actor == "" || strings.TrimSpace(reason) == "" || len(reason) > 2000 || l.Validate(false) != nil {
		return Record{}, fault.Invalid
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx)
	raw, _ := json.Marshal(l)
	id := ids.New("launch")
	tag, err := tx.Exec(ctx, `INSERT INTO platform_app.embedded_launches(id,app_id,client_id,release_digest,launch,submitted_by) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(app_id,client_id,release_digest) DO NOTHING`, id, l.AppID, l.ClientID, l.ReleaseDigest, raw, actor)
	if err != nil {
		return Record{}, err
	}
	v, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_app.embedded_launches WHERE app_id=$1 AND client_id=$2 AND release_digest=$3 FOR UPDATE`, l.AppID, l.ClientID, l.ReleaseDigest))
	if err != nil {
		return v, err
	}
	var owner string
	if err = tx.QueryRow(ctx, `SELECT submitted_by FROM platform_app.embedded_launches WHERE id=$1`, v.ID).Scan(&owner); err != nil {
		return Record{}, err
	}
	if v.Launch != l || owner != actor {
		return Record{}, fault.Conflict
	}
	if tag.RowsAffected() == 1 {
		_, err = tx.Exec(ctx, `INSERT INTO platform_app.embedded_launch_audit(launch_id,actor_id,action,reason) VALUES($1,$2,'submitted',$3)`, v.ID, actor, reason)
		if err != nil {
			return Record{}, err
		}
	}
	return v, tx.Commit(ctx)
}

// Review only accepts authenticated admin principals. Approval and signing are
// one atomic action; release eligibility must be held by the application caller.
func (r Repository) Review(ctx context.Context, p identity.PortalPrincipal, id string, revision int, status, reason string, key ed25519.PrivateKey) (Record, error) {
	if p.ID == "" || p.Surface != "admin" || p.Role != "administrator" {
		return Record{}, fault.Forbidden
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 2000 || revision < 1 || (status != "approved" && status != "rejected" && status != "revoked") {
		return Record{}, fault.Invalid
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_app.embedded_launches WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return v, err
	}
	// An exact repeated decision is harmless; never resurrect terminal states.
	if v.Status == status && v.Revision == revision+1 {
		var same bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_app.embedded_launch_audit WHERE launch_id=$1 AND actor_id=$2 AND action=$3 AND reason=$4)`, id, p.ID, status, reason).Scan(&same); err != nil {
			return Record{}, err
		}
		if !same {
			return Record{}, fault.Conflict
		}
		return v, tx.Commit(ctx)
	}
	if v.Revision != revision || !((v.Status == "submitted" && (status == "approved" || status == "rejected")) || (v.Status == "approved" && status == "revoked")) {
		return Record{}, fault.Conflict
	}
	if status == "approved" {
		v.Signature, err = embedded.SignLaunch(v.Launch, key, false)
		if err != nil {
			return Record{}, fault.Unavailable
		}
	}
	v, err = scan(tx.QueryRow(ctx, `UPDATE platform_app.embedded_launches SET status=$2,signature=nullif($3,''),revision=revision+1 WHERE id=$1 RETURNING `+columns, id, status, v.Signature))
	if err != nil {
		return v, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_app.embedded_launch_audit(launch_id,actor_id,action,reason) VALUES($1,$2,$3,$4)`, id, p.ID, status, reason)
	if err != nil {
		return Record{}, err
	}
	return v, tx.Commit(ctx)
}

// Resolve performs a current read each time; never cache approval/revocation.
func (r Repository) Resolve(ctx context.Context, app, client, digest string, key ed25519.PublicKey) (Record, error) {
	v, err := scan(r.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_app.embedded_launches WHERE app_id=$1 AND client_id=$2 AND release_digest=$3`, app, client, digest))
	if err != nil {
		return v, err
	}
	if v.Status != "approved" || embedded.VerifyLaunch(v.Launch, v.Signature, key, false) != nil {
		return Record{}, fault.Forbidden
	}
	return v, nil
}

// WithApproved serializes a short dependent operation against review revocation.
// Callers must hold the current release and client locks first. This is not a
// merchant grant: installation/actor authorization must still run in fn.
// Do not perform remote requests in fn or retain the record after it returns.
func (r Repository) WithApproved(ctx context.Context, app, client, digest string, key ed25519.PublicKey, fn func(Record) error) error {
	if r.Pool == nil {
		return fault.Unavailable
	}
	if fn == nil || app == "" || client == "" || digest == "" {
		return fault.Invalid
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	v, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_app.embedded_launches WHERE app_id=$1 AND client_id=$2 AND release_digest=$3 FOR SHARE`, app, client, digest))
	if err != nil {
		return err
	}
	if v.Status != "approved" || v.Launch.AppID != app || v.Launch.ClientID != client || v.Launch.ReleaseDigest != digest || embedded.VerifyLaunch(v.Launch, v.Signature, key, false) != nil {
		return fault.Forbidden
	}
	if err = fn(v); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
