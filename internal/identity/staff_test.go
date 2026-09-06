package identity

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"strings"
	"testing"
)

type staffRepo struct {
	PortalRepository
	calls int
}

func (r *staffRepo) ListStaff(context.Context, string) ([]StaffAccount, error) {
	r.calls++
	return []StaffAccount{{ID: "admin", Email: "admin@example.test", Role: "administrator", Enabled: true}}, nil
}
func TestStaffDirectoryAuthorization(t *testing.T) {
	r := &staffRepo{}
	s := Portals{Repo: r}
	for _, p := range []PortalPrincipal{{}, {ID: "d", Surface: "developer", Role: "administrator"}, {ID: "r", Surface: "admin", Role: "reviewer"}, {ID: "o", Surface: "admin", Role: "operator"}} {
		if _, err := s.ListStaff(context.Background(), p, ""); err != fault.Forbidden {
			t.Fatalf("expected forbidden: %v", err)
		}
	}
	if r.calls != 0 {
		t.Fatal("unauthorized repository call")
	}
	admin := PortalPrincipal{ID: "a", Surface: "admin", Role: "administrator"}
	if _, err := s.ListStaff(context.Background(), admin, strings.Repeat("x", 201)); err != fault.Invalid {
		t.Fatal("invalid cursor accepted")
	}
	rows, err := s.ListStaff(context.Background(), admin, "")
	if err != nil || len(rows) != 1 || r.calls != 1 {
		t.Fatalf("authorized directory failed: %v", err)
	}
}
