package postgres_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/oauth/embedded/postgres"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/migrations"
	"emisell.app/platform/pkg/embedded"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPersistentLaunchReviewRevocation(t *testing.T) {
	dsn := os.Getenv("EMISELL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated test database required")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || u.Path != "/emisell_local_test" {
		t.Fatal("isolated database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal("migration replay", err)
	}
	r := postgres.Repository{Pool: pool}
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	l := embedded.Launch{AppID: ids.New("app"), ClientID: ids.New("client"), ReleaseDigest: strings.Repeat("a", 64), URL: "https://app.example/embedded", ParentOrigin: "https://core.example"}
	v, err := r.Submit(ctx, "developer-test", l, "test launch")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Resolve(ctx, l.AppID, l.ClientID, l.ReleaseDigest, pub); !errors.Is(err, fault.Forbidden) {
		t.Fatal("unreviewed launch", err)
	}
	replay, err := r.Submit(ctx, "developer-test", l, "test launch")
	if err != nil || replay.ID != v.ID {
		t.Fatal("submission replay", err)
	}
	if _, err = r.Submit(ctx, "other-developer", l, "other"); !errors.Is(err, fault.Conflict) {
		t.Fatal("ownership replay", err)
	}
	p := identity.PortalPrincipal{ID: "admin-test", Surface: "admin", Role: "administrator"}
	if _, err = r.Review(ctx, identity.PortalPrincipal{ID: "dev", Surface: "developer", Role: "administrator"}, v.ID, 1, "approved", "review", key); !errors.Is(err, fault.Forbidden) {
		t.Fatal("role escalation")
	}
	approved, err := r.Review(ctx, p, v.ID, 1, "approved", "review", key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Review(ctx, p, v.ID, 1, "approved", "review", key); err != nil {
		t.Fatal("approval retry", err)
	}
	if _, err = r.Resolve(ctx, l.AppID, l.ClientID, l.ReleaseDigest, pub); err != nil {
		t.Fatal(err)
	}
	called := false
	if err = r.WithApproved(ctx, l.AppID, l.ClientID, l.ReleaseDigest, nil, func(postgres.Record) error {
		called = true
		return nil
	}); !errors.Is(err, fault.Forbidden) || called {
		t.Fatal("invalid key reached callback", err)
	}
	sentinel := errors.New("dependent operation failed")
	if err = r.WithApproved(ctx, l.AppID, l.ClientID, l.ReleaseDigest, pub, func(got postgres.Record) error {
		if got.ID != approved.ID || got.Signature != approved.Signature {
			t.Fatal("incorrect approved binding")
		}
		// A competing writer must not obtain the review row while the
		// dependent operation holds it. NOWAIT avoids timing-based assertions.
		tx, e := pool.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		_, e = tx.Exec(ctx, `SELECT id FROM platform_app.embedded_launches WHERE id=$1 FOR UPDATE NOWAIT`, v.ID)
		var pgErr *pgconn.PgError
		if !errors.As(e, &pgErr) || pgErr.Code != "55P03" {
			t.Fatal("expected locked review row", e)
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatal("callback failure not propagated", err)
	}
	// Failure must release the shared lock, not leave revocation blocked.
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err = r.WithApproved(lockCtx, l.AppID, l.ClientID, l.ReleaseDigest, pub, func(postgres.Record) error { return nil }); err != nil {
		t.Fatal("lock cleanup", err)
	}
	if _, err = r.Resolve(ctx, l.AppID, l.ClientID, strings.Repeat("b", 64), pub); !errors.Is(err, fault.NotFound) {
		t.Fatal("wrong release")
	}
	if _, err = pool.Exec(ctx, `UPDATE platform_app.embedded_launches SET launch='{}',revision=revision+1 WHERE id=$1`, v.ID); err == nil {
		t.Fatal("mutable launch")
	}
	if _, err = r.Review(ctx, p, v.ID, approved.Revision, "revoked", "revoke", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Resolve(ctx, l.AppID, l.ClientID, l.ReleaseDigest, pub); !errors.Is(err, fault.Forbidden) {
		t.Fatal("revocation bypass", err)
	}
	called = false
	if err = r.WithApproved(ctx, l.AppID, l.ClientID, l.ReleaseDigest, pub, func(postgres.Record) error {
		called = true
		return nil
	}); !errors.Is(err, fault.Forbidden) || called {
		t.Fatal("revoked launch reached dependent operation", err)
	}
	if _, err = r.Review(ctx, p, v.ID, 1, "approved", "review", key); !errors.Is(err, fault.Conflict) {
		t.Fatal("old approval resurrected launch", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM platform_app.embedded_launch_audit WHERE launch_id=$1`, v.ID).Scan(&count); err != nil || count != 3 {
		t.Fatal("audit duplicate", count, err)
	}
}
