package postgres

import (
	"context"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Repository struct{ Pool *pgxpool.Pool }

const columns = `id,binding,status,revision,challenge_id,challenge,challenge_expires_at,verified_until,last_attempt_at,last_result,secret_hash,secret_version,created_at,updated_at`

func scan(row pgx.Row) (appclient.Client, error) {
	var v appclient.Client
	var raw []byte
	err := row.Scan(&v.ID, &raw, &v.Status, &v.Revision, &v.ChallengeID, &v.Challenge, &v.ChallengeExpiresAt, &v.VerifiedUntil, &v.LastAttemptAt, &v.LastResult, &v.SecretHash, &v.SecretVersion, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &v.Binding)
	}
	return v, err
}
func (p Repository) Get(ctx context.Context, org, id string) (appclient.Client, error) {
	return scan(p.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.app_clients WHERE id=$1 AND ($2='' OR organization_id=$2)`, id, org))
}

// Lookup only; the caller must subsequently hold WithBoundReady before use.
func (p Repository) ForRelease(ctx context.Context, org, release string) (appclient.Client, error) {
	return scan(p.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.app_clients WHERE organization_id=$1 AND release_id=$2`, org, release))
}
func (p Repository) List(ctx context.Context, org string) ([]appclient.Client, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+columns+` FROM platform_oauth.app_clients WHERE ($1='' OR organization_id=$1) ORDER BY created_at DESC,id LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []appclient.Client{}
	for rows.Next() {
		v, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Repository) History(ctx context.Context, id string) ([]appclient.Audit, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,actor_id,action,reason,occurred_at FROM platform_oauth.app_client_audit WHERE client_id=$1 ORDER BY occurred_at,id LIMIT 200`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []appclient.Audit{}
	for rows.Next() {
		var v appclient.Audit
		if e := rows.Scan(&v.ID, &v.ActorID, &v.Action, &v.Reason, &v.OccurredAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func save(ctx context.Context, tx pgx.Tx, v appclient.Client) (appclient.Client, error) {
	return scan(tx.QueryRow(ctx, `UPDATE platform_oauth.app_clients SET status=$2,revision=$3,challenge_id=$4,challenge=$5,challenge_expires_at=$6,verified_until=$7,last_attempt_at=$8,last_result=$9,secret_hash=$10,secret_version=$11,updated_at=now() WHERE id=$1 RETURNING `+columns, v.ID, v.Status, v.Revision, v.ChallengeID, v.Challenge, v.ChallengeExpiresAt, v.VerifiedUntil, v.LastAttemptAt, v.LastResult, v.SecretHash, v.SecretVersion))
}
func audit(ctx context.Context, tx pgx.Tx, id, actor, action, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO platform_oauth.app_client_audit(id,client_id,actor_id,action,reason) VALUES($1,$2,$3,$4,$5)`, ids.New("aud"), id, actor, action, reason)
	return err
}
func (p Repository) Mutate(ctx context.Context, m appclient.Mutation) (appclient.Client, bool, error) {
	var out appclient.Client
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return out, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "app-client:"+m.ActorID+":"+m.Key); err != nil {
		return out, false, err
	}
	var oldHash, id string
	err = tx.QueryRow(ctx, `SELECT request_hash,client_id FROM platform_oauth.app_client_requests WHERE actor_id=$1 AND request_key=$2`, m.ActorID, m.Key).Scan(&oldHash, &id)
	if err == nil {
		if oldHash != m.Hash {
			return out, false, fault.Conflict
		}
		out, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.app_clients WHERE id=$1 AND ($2='' OR organization_id=$2) FOR SHARE`, id, m.OrganizationID))
		return out, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, false, err
	}
	if m.Create != nil {
		v := m.Create
		raw, _ := json.Marshal(v.Binding)
		out, err = scan(tx.QueryRow(ctx, `INSERT INTO platform_oauth.app_clients(id,organization_id,release_id,binding,status,revision,challenge_id,challenge,challenge_expires_at,last_result) VALUES($1,$2,$3,$4,'pending',1,$5,$6,$7,'not_checked') RETURNING `+columns, v.ID, m.OrganizationID, v.Binding.ReleaseID, raw, v.ChallengeID, v.Challenge, v.ChallengeExpiresAt))
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			err = fault.Conflict
		}
	} else {
		out, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.app_clients WHERE id=$1 AND ($2='' OR organization_id=$2) FOR UPDATE`, m.ID, m.OrganizationID))
		if err != nil {
			return out, false, err
		}
		out, err = m.Apply(out)
		if err != nil {
			return out, false, err
		}
		out, err = save(ctx, tx, out)
	}
	if err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_oauth.app_client_requests(actor_id,request_key,request_hash,client_id) VALUES($1,$2,$3,$4)`, m.ActorID, m.Key, m.Hash, out.ID); err != nil {
		return out, false, err
	}
	if err = audit(ctx, tx, out.ID, m.ActorID, m.Action, m.Reason); err != nil {
		return out, false, err
	}
	return out, true, tx.Commit(ctx)
}
func (p Repository) WithClient(ctx context.Context, id string, fn func(appclient.Client) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	v, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.app_clients WHERE id=$1 FOR SHARE`, id))
	if err != nil {
		return err
	}
	return fn(v)
}
func (p Repository) FinishProbe(ctx context.Context, id string, revision int, ok bool) (appclient.Client, error) {
	var v appclient.Client
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	v, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.app_clients WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return v, err
	}
	if v.Revision != revision || v.Status != "pending" || v.LastResult != "checking" {
		return v, fault.Conflict
	}
	v.LastResult = "failed"
	if ok && time.Now().Before(v.ChallengeExpiresAt) {
		v.Status = "verified"
		until := time.Now().Add(appclient.ProofTTL)
		v.VerifiedUntil = &until
		v.LastResult = "verified"
	}
	v.Revision++
	v, err = save(ctx, tx, v)
	if err != nil {
		return v, err
	}
	if err = audit(ctx, tx, id, "system-endpoint-proof", v.LastResult, "Hasil verifikasi endpoint; tidak menerbitkan token/grant."); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
