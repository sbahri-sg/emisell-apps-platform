package bootstrap_test

import (
	"context"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"errors"
	"testing"
	"time"
)

func TestPrimaryAdminCredentialUpdate(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	repo := identityrepo.Repository{Pool: f.pool}
	p := identity.PortalPrincipal{ID: "portal-local-admin", Email: "primary-test@portal.invalid", Surface: "admin", Role: "administrator"}
	// Fixed local provisioning ID is used only in the explicitly isolated test DB.
	if _, err := f.pool.Exec(ctx, `DELETE FROM platform_identity.portal_sessions WHERE account_id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM platform_identity.portal_accounts WHERE id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	oldPassword, newPassword := ids.New("old"), ids.New("new")
	if err := repo.SeedPortal(ctx, p, oldPassword); err != nil {
		t.Fatal(err)
	}
	service := identity.Portals{Repo: repo}
	_, oldToken, err := service.Login(ctx, "admin", p.Email, oldPassword, "")
	if err != nil {
		t.Fatal(err)
	}
	other := portalAccount(t, f, "admin", "reviewer")
	_, oldHash, err := repo.FindPortal(ctx, "admin", p.Email)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.UpdatePrimaryAdmin(ctx, p, "incorrect-password", "updated@portal.invalid", newPassword); err != fault.Conflict {
		t.Fatal("old credential check", err)
	}
	if err = repo.UpdatePrimaryAdmin(ctx, p, oldPassword, "updated@portal.invalid", newPassword); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, "admin", oldToken); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("stale session", err)
	}
	if _, _, err = service.Login(ctx, "admin", p.Email, oldPassword, ""); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("old login", err)
	}
	if err = repo.ReplacePortalSession(ctx, p.ID, "admin", ids.New("hash"), "", oldHash, time.Now().Add(time.Hour)); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("in-flight old login", err)
	}
	actual, token, err := service.Login(ctx, "admin", "updated@portal.invalid", newPassword, "")
	if err != nil || actual.ID != p.ID || actual.Role != p.Role {
		t.Fatal("new login", err)
	}
	if err = repo.UpdatePrimaryAdmin(ctx, p, oldPassword, "updated@portal.invalid", newPassword); err != nil {
		t.Fatal("retry", err)
	}
	if _, err = service.Authenticate(ctx, "admin", token); err != nil {
		t.Fatal("retry revoked new session", err)
	}
	pexpect(t, other, "GET", "/api/v1/admin/session", nil, "", 200)
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_identity.portal_audit WHERE actor_id=$1 AND action='credentials_updated_sessions_revoked'`, p.ID).Scan(&count); err != nil || count < 1 {
		t.Fatal("audit", err)
	}
}
