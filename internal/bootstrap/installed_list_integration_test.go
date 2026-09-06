package bootstrap_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	v1 "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
)

func TestCurrentMerchantInstalledList(t *testing.T) {
	f := setup(t)
	c, _, _, _ := lifecycleClient(t, f)
	ctx := context.Background()
	list := func(merchant, actor, after string, size int32) *v1.ListInstallationsResponse {
		t.Helper()
		r, err := c.Installations.ListInstallations(ctx, connect.NewRequest(&v1.ListInstallationsRequest{MerchantId: merchant, CoreActorId: actor, AfterId: after, PageSize: size}))
		if err != nil {
			t.Fatal(err)
		}
		if r.Msg.MerchantId != merchant {
			t.Fatal("merchant mismatch")
		}
		return r.Msg
	}
	if len(list(f.tenant, "another-authorized-manager", "", 0).Installations) != 0 {
		t.Fatal("must start empty")
	}
	pay := consume(t, c, f.tenant, "emisell-pay")
	ship := consume(t, c, f.tenant, "parcel")
	consume(t, c, f.other, "emisell-pay")
	if _, err := c.Installations.Activate(ctx, activateRequest(f.tenant, pay.InstallationId)); err != nil {
		t.Fatal(err)
	}
	var before, after int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_installation.access_audit").Scan(&before); err != nil {
		t.Fatal(err)
	}
	page := list(f.tenant, "another-authorized-manager", "", 1)
	if len(page.Installations) != 1 || page.NextAfterId != page.Installations[0].InstallationId {
		t.Fatal("bounded first page required")
	}
	page2 := list(f.tenant, "another-authorized-manager", page.NextAfterId, 1)
	if len(page2.Installations) != 1 || page2.NextAfterId != "" || page2.Installations[0].InstallationId <= page.NextAfterId {
		t.Fatal("invalid next page")
	}
	all := list(f.tenant, "another-authorized-manager", "", 20)
	if len(all.Installations) != 2 {
		t.Fatal("must list merchant, not installing actor")
	}
	for _, app := range all.Installations {
		if app.InstallationId == pay.InstallationId && (app.Status != "active" || app.GrantState != "active" || app.AppName != "Emisell Pay") {
			t.Fatal("active metadata mismatch")
		}
		if app.InstallationId == ship.InstallationId && (app.Status != "pending" || app.GrantState != "pending") {
			t.Fatal("pending must not appear active")
		}
	}
	other := list(f.other, "another-authorized-manager", "", 20)
	if len(other.Installations) != 1 || other.Installations[0].InstallationId == pay.InstallationId {
		t.Fatal("cross-merchant leak")
	}
	// Another trusted Core key can read merchant summaries, not own old actions.
	c2, _, _, _ := lifecycleClient(t, f)
	r2, err := c2.Installations.ListInstallations(ctx, connect.NewRequest(&v1.ListInstallationsRequest{MerchantId: f.tenant, CoreActorId: "manager"}))
	if err != nil || len(r2.Msg.Installations) != 2 {
		t.Fatal("list must survive Core key replacement", err)
	}
	if _, err := c2.Installations.Activate(ctx, activateRequest(f.tenant, pay.InstallationId)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("list must not transfer action ownership", err)
	}
	for _, request := range []*v1.ListInstallationsRequest{
		{MerchantId: f.tenant}, {CoreActorId: "manager"},
		{MerchantId: f.tenant, CoreActorId: "manager", PageSize: -1},
		{MerchantId: f.tenant, CoreActorId: "manager", PageSize: 21},
		{MerchantId: f.tenant, CoreActorId: "manager", AfterId: "../foreign"},
		{MerchantId: "unknown-merchant", CoreActorId: "manager"},
	} {
		if _, err := c.Installations.ListInstallations(ctx, connect.NewRequest(request)); err == nil {
			t.Fatal("invalid list request accepted")
		}
	}
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_installation.access_audit").Scan(&after); err != nil || before != after {
		t.Fatal("list changed lifecycle state", err)
	}
	if _, err := c.Installations.Uninstall(ctx, uninstallRequest(f.tenant, pay.InstallationId)); err != nil {
		t.Fatal(err)
	}
	if len(list(f.tenant, "manager", "", 20).Installations) != 1 {
		t.Fatal("uninstalled history included")
	}
	replacement := consume(t, c, f.tenant, "emisell-pay")
	latest := list(f.tenant, "manager", "", 20)
	if replacement.InstallationId == pay.InstallationId || len(latest.Installations) != 2 {
		t.Fatal("replacement list mismatch")
	}
	for _, item := range latest.Installations {
		if item.InstallationId == pay.InstallationId {
			t.Fatal("retired receipt leaked into current list")
		}
	}
}
