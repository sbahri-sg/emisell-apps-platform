package identity

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"strings"
	"testing"
)

func TestPlatformKeysRoleInputAndTenantBinding(t *testing.T) {
	s := PlatformKeys{}
	ctx := context.Background()
	admin := PortalPrincipal{ID: "admin", Surface: "admin", Role: "administrator"}
	for _, p := range []PortalPrincipal{{}, {ID: "x", Surface: "developer", Role: "administrator"}, {ID: "x", Surface: "admin", Role: "reviewer"}, {ID: "x", Surface: "admin", Role: "operator"}} {
		if _, e := s.List(ctx, p); e != fault.Forbidden {
			t.Fatal("list bypass")
		}
		if _, _, e := s.Create(ctx, p, "request-000000001", CreatePlatformKey{Name: "Core"}); e != fault.Forbidden {
			t.Fatal("create bypass")
		}
		if _, e := s.Revoke(ctx, p, "platformkey_x"); e != fault.Forbidden {
			t.Fatal("revoke bypass")
		}
	}
	for _, name := range []string{"", "  ", "bad\nname", strings.Repeat("x", 81)} {
		if _, _, e := s.Create(ctx, admin, "request-000000001", CreatePlatformKey{Name: name}); e != fault.Invalid {
			t.Fatal("invalid name")
		}
	}
	if _, _, e := s.Create(ctx, admin, "", CreatePlatformKey{Name: "Core"}); e != fault.Invalid {
		t.Fatal("missing request key")
	}
	if _, e := s.Revoke(ctx, admin, "legacy_key"); e != fault.Invalid {
		t.Fatal("legacy revoke path")
	}
	legacy := ServicePrincipal{ID: "svc", TenantID: "tenant_a", Scopes: []string{"payments.read"}}
	if legacy.AllowsServiceScope("payments.write") {
		t.Fatal("implicit full access")
	}
	if _, e := legacy.BindTenant("tenant_b"); e != fault.NotFound {
		t.Fatal("cross-tenant legacy")
	}
	if p, e := legacy.BindTenant(""); e != nil || p.TenantID != "tenant_a" {
		t.Fatal("legacy compatibility")
	}
	full := ServicePrincipal{ID: "platformkey_test", PlatformFull: true}
	if !full.AllowsServiceScope(ScopeInstallIntentsConsent) {
		t.Fatal("full integration access missing")
	}
	if _, e := full.BindTenant(""); e != fault.Invalid {
		t.Fatal("missing platform tenant")
	}
	if p, e := full.BindTenant("tenant_b"); e != nil || p.TenantID != "tenant_b" || full.TenantID != "" {
		t.Fatal("principal mutated")
	}
	if _, e := (ServiceAccounts{}).Issue(ctx, full, false, "admin"); e != fault.Invalid {
		t.Fatal("legacy issuance elevated")
	}
}
