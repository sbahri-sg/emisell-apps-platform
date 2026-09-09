package repository

import (
	"context"
	"crypto/ed25519"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

// Called once inside the app-creation transaction after credential creation.
func (p Postgres) CreatePrivateProductTx(ctx context.Context, tx pgx.Tx, org, app, actor, client string, key ed25519.PrivateKey) error {
	d, err := scanDraft(tx.QueryRow(ctx, `SELECT `+draftColumns+` FROM platform_app.drafts WHERE id=$1 AND organization_id=$2`, app, org))
	if err != nil {
		return err
	}
	if d.Document.Capability != service.PrivateProducts {
		return nil
	}
	v := service.PrivateProductVersion{ID: ids.New("privateversion"), AppID: app, OrganizationID: org, OwnerAccountID: actor, ClientID: client, Document: d.Document}
	signature, err := v.Sign(key)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_app.private_product_versions(id,app_id,organization_id,owner_account_id,version,document,signature) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, app, org, actor, v.Document.Version, raw, signature)
	return err
}
func (p Postgres) HasPrivateProducts(ctx context.Context, app string) (bool, error) {
	var found bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_app.private_product_versions WHERE app_id=$1)`, app).Scan(&found)
	return found, err
}
func (p Postgres) WithPrivateProducts(ctx context.Context, app, version string, key ed25519.PublicKey, fn func(service.PrivateProductVersion, string) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var raw, signature []byte
	err = tx.QueryRow(ctx, `SELECT document,signature FROM platform_app.private_product_versions WHERE app_id=$1 AND version=$2 FOR SHARE`, app, version).Scan(&raw, &signature)
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return err
	}
	var v service.PrivateProductVersion
	if json.Unmarshal(raw, &v) != nil || v.AppID != app || v.Document.Version != version {
		return fault.Forbidden
	}
	digest, err := v.Verify(key, signature)
	if err != nil {
		return err
	}
	return fn(v, digest)
}
