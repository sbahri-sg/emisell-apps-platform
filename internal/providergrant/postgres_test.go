package providergrant

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"testing"
)

func TestPersistentSnapshotRevision(t *testing.T) {
	dsn := os.Getenv("PROVIDER_GRANT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("disposable database not configured")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || u.Path != "/provider_grant_test" {
		t.Fatal("disposable database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Minimal real tables exercise the query and revision migration without seeding a merchant database.
	_, err = pool.Exec(ctx, `CREATE SCHEMA platform_installation;
 CREATE TABLE platform_installation.installations(tenant_id text,id text,app_id text,intent_id text,status text);
 CREATE TABLE platform_installation.intent_consumptions(tenant_id text,installation_id text,intent_id text,release jsonb);
 CREATE TABLE platform_installation.access_grants(tenant_id text,installation_id text,state text,scopes jsonb);`)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../migrations/0022_provider_grant_revision.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO platform_installation.installations VALUES('m','ins','app','intent','active');
 INSERT INTO platform_installation.intent_consumptions VALUES('m','ins','intent','{"appId":"app","shippingProvider":{"engine":"api-kurir","providerCode":"rajaongkir"},"scopes":["shipping.read"]}');
 INSERT INTO platform_installation.access_grants(tenant_id,installation_id,state,scopes) VALUES('m','ins','active','["shipping.read","shipping.write"]');`)
	if err != nil {
		t.Fatal(err)
	}
	p := Postgres{Pool: pool}
	r := Request{MerchantID: "m", AppID: "app", InstallationID: "ins", ProviderCode: "rajaongkir"}
	v, err := p.Snapshot(ctx, r)
	if err != nil || !v.Active || len(v.Scopes) != 1 || v.Scopes[0] != "shipping.read" {
		t.Fatal("consent intersection", v, err)
	}
	_, err = pool.Exec(ctx, `UPDATE platform_installation.access_grants SET state='revoked',scopes='[]'`)
	if err != nil {
		t.Fatal(err)
	}
	next, err := p.Snapshot(ctx, r)
	if err != nil || !next.Revoked || next.Active || next.Revision <= v.Revision {
		t.Fatal("revision did not advance", next, err)
	}
	page, err := p.Targets(ctx, map[string]string{"app": "rajaongkir"}, "")
	if err != nil || len(page.Targets) != 1 || page.Targets[0].InstallationID != "ins" {
		t.Fatal("revoked target missing", page, err)
	}
	page, err = p.Targets(ctx, map[string]string{"app": "other"}, "")
	if err != nil || len(page.Targets) != 0 {
		t.Fatal("provider enrollment leak", page, err)
	}
}
