package identity

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"testing"
	"time"
)

type unifiedRepo struct {
	PortalRepository
	accounts map[string]PortalPrincipal
	hash     string
	issued   string
}

func (r *unifiedRepo) FindPortal(_ context.Context, surface, email string) (PortalPrincipal, string, error) {
	p, ok := r.accounts[surface]
	if !ok || p.Email != email {
		return p, "", fault.NotFound
	}
	return p, r.hash, nil
}
func (r *unifiedRepo) ReplacePortalSession(_ context.Context, id, surface, token, old, hash string, _ time.Time) error {
	r.issued = surface
	return nil
}
func TestUnifiedLoginIdentity(t *testing.T) {
	hash, _ := HashPassword("test-password-123456")
	r := &unifiedRepo{hash: hash, accounts: map[string]PortalPrincipal{"developer": {ID: "dev", Email: "demo@example.test", Surface: "developer", Role: "developer"}}}
	s := Portals{Repo: r}
	_, _, err := s.LoginUnified(context.Background(), "demo@example.test", "test-password-123456", "")
	if err != fault.Unauthenticated || r.issued != "" {
		t.Fatal("developer password login must be retired", err)
	}
	r.issued = ""
	if _, _, err = s.LoginUnified(context.Background(), "demo@example.test", "wrong", ""); err != fault.Unauthenticated || r.issued != "" {
		t.Fatal("wrong password accepted")
	}
	r.accounts["admin"] = PortalPrincipal{ID: "admin", Email: "demo@example.test", Surface: "admin", Role: "administrator"}
	if p, _, err := s.LoginUnified(context.Background(), "demo@example.test", "test-password-123456", ""); err != nil || p.Surface != "admin" || r.issued != "admin" {
		t.Fatal("admin password login unavailable", err)
	}
}
