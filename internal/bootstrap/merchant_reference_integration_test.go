package bootstrap_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/transport/connectapi"
	integration "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1"
	integrationconnect "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1/integrationv1connect"
)

func TestCoreMerchantReferenceRegistration(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, principal, secret, _ := lifecycleClient(t, f)
	srv := httptest.NewUnstartedServer(nil)
	h, err := connectapi.ExposeCore(http.NotFoundHandler(), bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil))), "https://"+srv.Listener.Addr().String(), "true")
	if err != nil {
		t.Fatal(err)
	}
	srv.Config.Handler = h
	srv.StartTLS()
	defer srv.Close()
	client := integrationconnect.NewConnectionServiceClient(srv.Client(), srv.URL+connectapi.CoreHTTPPrefix, connect.WithProtoJSON())
	merchant := ids.New("merchant")
	ensure := func(id, key string) (*integration.EnsureMerchantResponse, error) {
		r := connect.NewRequest(&integration.EnsureMerchantRequest{MerchantId: id, CoreActorId: "verified-owner"})
		r.Header().Set("Authorization", "Bearer "+key)
		response, err := client.EnsureMerchant(ctx, r)
		if err != nil {
			return nil, err
		}
		return response.Msg, nil
	}
	var created atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := ensure(merchant, secret)
			if e != nil {
				t.Errorf("ensure: %v", e)
				return
			}
			if !r.Registered || r.MerchantId != merchant {
				t.Error("bad receipt")
			}
			if r.Created {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatal("non-idempotent creation", created.Load())
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_identity.merchant_reference_audit WHERE merchant_id=$1 AND service_id=$2 AND core_actor_id='verified-owner'`, merchant, principal.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("registration audit", count, err)
	}
	for _, query := range []string{
		`SELECT count(*) FROM platform_identity.memberships WHERE tenant_id=$1`,
		`SELECT count(*) FROM platform_installation.installations WHERE tenant_id=$1`,
		`SELECT count(*) FROM platform_installation.access_grants WHERE tenant_id=$1`,
		`SELECT count(*) FROM platform_installation.app_tokens WHERE tenant_id=$1`,
	} {
		if err = f.pool.QueryRow(ctx, query, merchant).Scan(&count); err != nil || count != 0 {
			t.Fatal("registration granted app/member authority", count, err)
		}
	}
	if r, e := ensure(f.tenant, secret); e != nil || r.Created {
		t.Fatal("existing store modified", e)
	}
	var name string
	if err = f.pool.QueryRow(ctx, `SELECT name FROM platform_identity.workspaces WHERE id=$1`, f.tenant).Scan(&name); err != nil || name != "Test Store" {
		t.Fatal("existing name overwritten", err)
	}
	if _, e := ensure(ids.New("merchant"), "invalid"); connect.CodeOf(e) != connect.CodeUnauthenticated {
		t.Fatal("invalid key accepted", e)
	}
	if _, e := ensure("../other", secret); connect.CodeOf(e) != connect.CodeInvalidArgument {
		t.Fatal("invalid ID accepted", e)
	}
	// Repository rechecks even a principal authenticated before key revocation.
	if _, err = f.pool.Exec(ctx, `UPDATE platform_identity.core_platform_keys SET revoked_at=now() WHERE id=$1`, principal.ID); err != nil {
		t.Fatal(err)
	}
	accounts := identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}
	denied := ids.New("merchant")
	if _, e := accounts.EnsureMerchant(ctx, principal, denied, "verified-owner"); !errors.Is(e, fault.Unauthenticated) {
		t.Fatal("stale principal accepted", e)
	}
	if _, e := ensure(denied, secret); connect.CodeOf(e) != connect.CodeUnauthenticated {
		t.Fatal("revoked key accepted", e)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_identity.workspaces WHERE id=$1`, denied).Scan(&count); err != nil || count != 0 {
		t.Fatal("denied merchant created", err)
	}
}
