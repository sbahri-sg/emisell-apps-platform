// Package appidentity owns stable app-level credentials, not installation grants.
// Release-bound appclient evidence remains separate during compatibility migration.
package appidentity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/platform/secretbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Credential struct {
	ClientID        string    `json:"clientId"`
	OrganizationID  string    `json:"-"`
	AppID           string    `json:"appId"`
	Version         int       `json:"version"`
	CreatedAt       time.Time `json:"createdAt"`
	SecretCreatedAt time.Time `json:"secretCreatedAt"`
	Hash            string    `json:"-"`
	Ciphertext      []byte    `json:"-"`
}
type Repository struct {
	Pool *pgxpool.Pool
	Box  *secretbox.Box
}

const columns = `client_id,organization_id,app_id,version,created_at,secret_created_at,secret_hash,secret_ciphertext`

// Explicit composition port; does not read the owning application's tables.
func (p Repository) ClientIDTx(ctx context.Context, tx pgx.Tx, org, app string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT client_id FROM platform_oauth.application_credentials WHERE organization_id=$1 AND app_id=$2 FOR SHARE`, org, app).Scan(&id)
	return id, err
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

func scan(row pgx.Row) (c Credential, err error) {
	err = row.Scan(&c.ClientID, &c.OrganizationID, &c.AppID, &c.Version, &c.CreatedAt, &c.SecretCreatedAt, &c.Hash, &c.Ciphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func purpose(c Credential) string {
	return fmt.Sprintf("application-credential:%s:%s:%s:%d", c.OrganizationID, c.AppID, c.ClientID, c.Version)
}
func (p Repository) material(c *Credential) error {
	if p.Box == nil {
		return fault.Unavailable
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fault.Unavailable
	}
	secret := "ecs_" + base64.RawURLEncoding.EncodeToString(b)
	c.Hash = digest(secret)
	c.Ciphertext = p.Box.Seal(purpose(*c), []byte(secret))
	return nil
}
func audit(ctx context.Context, tx pgx.Tx, c Credential, actor, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO platform_oauth.application_credential_audit(id,client_id,actor_id,action,version) VALUES($1,$2,$3,$4,$5)`, ids.New("aud"), c.ClientID, actor, action, c.Version)
	return err
}

// EnsureTx is composed inside the owning draft transaction; it never reads app tables.
func (p Repository) EnsureTx(ctx context.Context, tx pgx.Tx, org, app, actor string) error {
	if p.Box == nil {
		return fault.Unavailable
	}
	if org == "" || app == "" || actor == "" {
		return fault.Invalid
	}
	c := Credential{ClientID: ids.New("eai"), OrganizationID: org, AppID: app, Version: 1}
	if err := p.material(&c); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `INSERT INTO platform_oauth.application_credentials(client_id,organization_id,app_id,version,secret_hash,secret_ciphertext) VALUES($1,$2,$3,1,$4,$5) ON CONFLICT(app_id) DO NOTHING`, c.ClientID, org, app, c.Hash, c.Ciphertext)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 1 {
		return audit(ctx, tx, c, actor, "created")
	}
	var owner string
	if err = tx.QueryRow(ctx, `SELECT organization_id FROM platform_oauth.application_credentials WHERE app_id=$1`, app).Scan(&owner); err != nil {
		return err
	}
	if owner != org {
		return fault.Conflict
	}
	return nil
}
func (p Repository) Get(ctx context.Context, org, app string) (Credential, error) {
	if p.Pool == nil || p.Box == nil {
		return Credential{}, fault.Unavailable
	}
	return scan(p.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.application_credentials WHERE organization_id=$1 AND app_id=$2`, org, app))
}

// Reveal is explicit, owner-scoped and audited. Normal GET never returns secret material.
func (p Repository) Reveal(ctx context.Context, org, app, actor string, version int) (Credential, string, error) {
	if p.Pool == nil || p.Box == nil {
		return Credential{}, "", fault.Unavailable
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Credential{}, "", err
	}
	defer tx.Rollback(ctx)
	c, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.application_credentials WHERE organization_id=$1 AND app_id=$2 FOR SHARE`, org, app))
	if err != nil {
		return c, "", err
	}
	if version != c.Version {
		return c, "", fault.Conflict
	}
	raw, err := p.Box.Open(purpose(c), c.Ciphertext)
	if err != nil {
		return c, "", fault.Unavailable
	}
	if err = audit(ctx, tx, c, actor, "revealed"); err != nil {
		return c, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return c, "", err
	}
	return c, string(raw), nil
}
func (p Repository) Rotate(ctx context.Context, org, app, actor, key string, version int) (Credential, error) {
	if p.Pool == nil || p.Box == nil {
		return Credential{}, fault.Unavailable
	}
	if !keyPattern.MatchString(key) || version < 1 {
		return Credential{}, fault.Invalid
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return Credential{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "app-identity:"+org+":"+actor+":"+key); err != nil {
		return Credential{}, err
	}
	c, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.application_credentials WHERE organization_id=$1 AND app_id=$2 FOR UPDATE`, org, app))
	if err != nil {
		return c, err
	}
	var oldApp string
	var expected, result int
	err = tx.QueryRow(ctx, `SELECT app_id,expected_version,resulting_version FROM platform_oauth.application_credential_requests WHERE organization_id=$1 AND actor_id=$2 AND request_key=$3`, org, actor, key).Scan(&oldApp, &expected, &result)
	if err == nil {
		if oldApp != app || expected != version || result != c.Version {
			return c, fault.Conflict
		}
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return c, err
	}
	if c.Version != version {
		return c, fault.Conflict
	}
	c.Version++
	if err = p.material(&c); err != nil {
		return c, err
	}
	c, err = scan(tx.QueryRow(ctx, `UPDATE platform_oauth.application_credentials SET version=$3,secret_hash=$4,secret_ciphertext=$5,secret_created_at=now() WHERE organization_id=$1 AND app_id=$2 RETURNING `+columns, org, app, c.Version, c.Hash, c.Ciphertext))
	if err != nil {
		return c, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_oauth.application_credential_requests VALUES($1,$2,$3,$4,$5,$6)`, org, actor, key, app, version, c.Version); err != nil {
		return c, err
	}
	if err = audit(ctx, tx, c, actor, "rotated"); err != nil {
		return c, err
	}
	return c, tx.Commit(ctx)
}

// Authenticate identifies an app only. It does not select a release or grant store access.
func (p Repository) Authenticate(ctx context.Context, id, secret string) (Credential, error) {
	if p.Pool == nil || p.Box == nil {
		return Credential{}, fault.Unavailable
	}
	if len(id) > 100 || len(secret) > 200 || secret == "" {
		return Credential{}, fault.Unauthenticated
	}
	c, err := scan(p.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_oauth.application_credentials WHERE client_id=$1`, id))
	if err != nil || subtle.ConstantTimeCompare([]byte(c.Hash), []byte(digest(secret))) != 1 {
		return Credential{}, fault.Unauthenticated
	}
	return c, nil
}
