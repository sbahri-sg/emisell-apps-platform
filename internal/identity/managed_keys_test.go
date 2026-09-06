package identity

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"strings"
	"testing"
)

// Invalid input must fail before any repository access (nil on purpose).
func TestManagedKeysEnforceRoleAndServiceScopeBoundary(t *testing.T) {
	s := ManagedKeys{}
	p := PortalPrincipal{ID: "admin", Surface: "admin", Role: "administrator"}
	in := CreateManagedKey{Name: "Core", TenantID: "tenant_a", Scopes: []string{"payments.read"}, ValidDays: 7}
	ctx := context.Background()
	for _, principal := range []PortalPrincipal{{}, {ID: "x", Surface: "developer", Role: "administrator"}, {ID: "x", Surface: "admin", Role: "reviewer"}, {ID: "x", Surface: "admin", Role: "operator"}} {
		if _, e := s.List(ctx, principal); e != fault.Forbidden {
			t.Fatal("list role bypass")
		}
		if _, _, e := s.Create(ctx, principal, "request-000000001", in); e != fault.Forbidden {
			t.Fatal("create role bypass")
		}
		if _, e := s.Revoke(ctx, principal, "key_a"); e != fault.Forbidden {
			t.Fatal("revoke role bypass")
		}
	}
	for _, scopes := range [][]string{nil, {"read_products"}, {"write_orders"}, {"*"}, {"payments.read", "payments.read"}, {"unknown"}} {
		v := in
		v.Scopes = scopes
		if _, _, e := s.Create(ctx, p, "request-000000001", v); e != fault.Invalid {
			t.Fatal("scope accepted", scopes)
		}
	}
	for _, days := range []int{0, -1, 31} {
		v := in
		v.ValidDays = days
		if _, _, e := s.Create(ctx, p, "request-000000001", v); e != fault.Invalid {
			t.Fatal("invalid lifetime")
		}
	}
	for _, name := range []string{"", "  ", "bad\nname", strings.Repeat("x", 81)} {
		v := in
		v.Name = name
		if _, _, e := s.Create(ctx, p, "request-000000001", v); e != fault.Invalid {
			t.Fatal("invalid name")
		}
	}
	if _, _, e := s.Create(ctx, p, "", in); e != fault.Invalid {
		t.Fatal("missing idempotency key")
	}
}
