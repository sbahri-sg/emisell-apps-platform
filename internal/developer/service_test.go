package developer

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"testing"
)

type directoryFake struct{ calls int }

func (f *directoryFake) Organization(context.Context, string) (Organization, error) {
	return Organization{}, nil
}
func (f *directoryFake) ListOrganizations(context.Context, string) ([]AdminOrganization, error) {
	f.calls++
	return []AdminOrganization{{ID: "org", Name: "Demo", MemberCount: 1}}, nil
}
func (f *directoryFake) GetOrganization(context.Context, string) (AdminOrganization, error) {
	f.calls++
	return AdminOrganization{ID: "org"}, nil
}
func TestDirectoryAuthorization(t *testing.T) {
	f := &directoryFake{}
	s := Service{Repo: f}
	for _, p := range []identity.PortalPrincipal{{}, {ID: "x", Surface: "developer", Role: "developer"}, {ID: "x", Surface: "admin", Role: "reviewer"}, {ID: "x", Surface: "admin", Role: "operator"}} {
		if _, err := s.ListOrganizations(context.Background(), p, ""); err != fault.Forbidden {
			t.Fatal("list must deny", p)
		}
		if _, err := s.GetOrganization(context.Background(), p, "org"); err != fault.Forbidden {
			t.Fatal("detail must deny", p)
		}
	}
	if f.calls != 0 {
		t.Fatal("unauthorized repository access")
	}
	p := identity.PortalPrincipal{ID: "admin", Surface: "admin", Role: "administrator"}
	if _, err := s.ListOrganizations(context.Background(), p, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrganization(context.Background(), p, "org"); err != nil {
		t.Fatal(err)
	}
	if f.calls != 2 {
		t.Fatal("missing authorized calls")
	}
}
