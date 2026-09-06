package bootstrap_test

import (
	"connectrpc.com/connect"
	"context"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/sdk"
	pay "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
)

func serviceClient(t *testing.T, f *fixture, tenant string, scopes []string) (*sdk.Client, identity.ServicePrincipal, string) {
	t.Helper()
	p := identity.ServicePrincipal{ID: ids.New("svc"), TenantID: tenant, Scopes: scopes, ExpiresAt: time.Now().Add(time.Hour)}
	token, err := (identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}).Issue(context.Background(), p, false, "test-operator")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	c, err := sdk.NewLocalClient(server.URL, token)
	if err != nil {
		t.Fatal(err)
	}
	return c, p, token
}
func rpcCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if err == nil || connect.CodeOf(err) != want {
		t.Fatalf("got %v, want %s", err, want)
	}
}
func TestInternalRPCProviderNeutralLifecycle(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, _, _ := serviceClient(t, f, f.tenant, []string{"payments.read", "payments.write", "shipping.read", "shipping.write"})
	for _, a := range []struct {
		id     string
		scopes []string
	}{{"emisell-pay", payScopes}, {"parcel", shippingScopes}} {
		for _, kind := range []string{"install", "activate"} {
			f.expect(t, "POST", f.installPath(f.tenant), action(kind, a.id, a.scopes), key(), 200)
		}
	}
	req := connect.NewRequest(&pay.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "rpc-order", AmountMinor: 12000, Currency: "IDR"})
	first, err := c.Payment.Create(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := c.Payment.Create(ctx, req)
	if err != nil || again.Msg.Payment.Id != first.Msg.Payment.Id {
		t.Fatal("RPC retry duplicated payment", err)
	}
	req.Msg.AmountMinor++
	_, err = c.Payment.Create(ctx, req)
	rpcCode(t, err, connect.CodeAlreadyExists)
	id := first.Msg.Payment.Id
	if _, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: id})); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Payment.Capture(ctx, connect.NewRequest(&pay.CaptureRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: id})); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Payment.Refund(ctx, connect.NewRequest(&pay.RefundRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: id})); err != nil {
		t.Fatal(err)
	}
	rates, err := c.Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{TenantId: f.tenant, IdempotencyKey: key(), WeightGrams: 1000, DestinationZone: "ID-JKT"}))
	if err != nil || len(rates.Msg.Rates) != 1 {
		t.Fatal(err)
	}
	shipment, err := c.Shipping.Create(ctx, connect.NewRequest(&ship.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "rpc-order", WeightGrams: 1000, DestinationZone: "ID-JKT"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Shipping.Track(ctx, connect.NewRequest(&ship.TrackRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: shipment.Msg.Shipment.Id})); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "POST", f.installPath(f.tenant), action("uninstall", "emisell-pay", nil), key(), 200)
	_, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: id}))
	rpcCode(t, err, connect.CodeNotFound)
	for _, kind := range []string{"install", "activate"} {
		f.expect(t, "POST", f.installPath(f.tenant), action(kind, "emisell-pay-alt", payScopes), key(), 200)
	}
	// Exact same Core client and contract, only installation routing changed.
	req.Msg.IdempotencyKey = key()
	second, err := c.Payment.Create(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Msg.Simulation || first.Msg.InstallationId == second.Msg.InstallationId {
		t.Fatal("provider routing did not change")
	}
	_, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: id}))
	rpcCode(t, err, connect.CodeNotFound)
}
func TestInternalRPCServiceIsolationExpiryRevocation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, p, _ := serviceClient(t, f, f.tenant, []string{"payments.read"})
	_, err := c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "blocked", AmountMinor: 1, Currency: "IDR"}))
	rpcCode(t, err, connect.CodePermissionDenied)
	_, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.other, IdempotencyKey: key(), ResourceId: "foreign"}))
	rpcCode(t, err, connect.CodeNotFound)
	_, err = f.pool.Exec(ctx, "UPDATE platform_identity.service_accounts SET expires_at=now()-interval '1 second' WHERE id=$1", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: "foreign"}))
	rpcCode(t, err, connect.CodeUnauthenticated)
	accounts := identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}
	token, err := accounts.Issue(ctx, p, true, "test-operator")
	if err != nil {
		t.Fatal(err)
	}
	// Rotation cannot restore access to the old client token.
	_, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: "foreign"}))
	rpcCode(t, err, connect.CodeUnauthenticated)
	if _, err = accounts.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err = accounts.Revoke(ctx, p.ID, "test-operator"); err != nil {
		t.Fatal(err)
	}
	if _, err = accounts.Authenticate(ctx, token); err == nil {
		t.Fatal("revoked token accepted")
	}
	server := httptest.NewServer(bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()
	// Browser session token is not a service credential.
	browserClient, err := sdk.NewLocalClient(server.URL, f.cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	_, err = browserClient.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: "x"}))
	rpcCode(t, err, connect.CodeUnauthenticated)
}
