package bootstrap_test

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/ids"
	"encoding/json"
	"strings"
	"testing"
)

func TestAdminCreatesDeveloper(t *testing.T) {
	f := setup(t)
	admin := portalAccount(t, f, "admin", "administrator")
	reviewer := portalAccount(t, f, "admin", "reviewer")
	email := strings.ToLower(ids.New("new")) + "@example.invalid"
	password := ids.New("password")
	body := map[string]string{"organizationName": "New Developer", "email": " " + strings.ToUpper(email) + " ", "password": password}
	pexpect(t, reviewer, "POST", "/api/v1/admin/developers", body, "", 403)
	pexpect(t, f, "POST", "/api/v1/admin/developers", body, "", 401)
	result := pexpect(t, admin, "POST", "/api/v1/admin/developers", body, "", 201)
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), password) || strings.Contains(string(encoded), "password") {
		t.Fatal("credential leaked")
	}
	org := result["organization"].(map[string]any)["id"].(string)
	var id, hash, role string
	var enabled bool
	if err := f.pool.QueryRow(context.Background(), `SELECT id,password_hash,role,enabled FROM platform_identity.portal_accounts WHERE email=$1 AND surface='developer'`, email).Scan(&id, &hash, &role, &enabled); err != nil {
		t.Fatal(err)
	}
	if !identity.VerifyPassword(password, hash) || hash == password || role != "developer" || !enabled {
		t.Fatal("invalid account")
	}
	var linked, memberRole string
	if err := f.pool.QueryRow(context.Background(), `SELECT organization_id,role FROM platform_developer.memberships WHERE account_id=$1`, id).Scan(&linked, &memberRole); err != nil || linked != org || memberRole != "owner" {
		t.Fatal("missing owner membership", err)
	}
	var audits int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM platform_identity.portal_audit WHERE actor_id=$1 AND action=$2`, admin.user, "developer.created:"+id).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("missing audit", err)
	}
	pexpect(t, admin, "POST", "/api/v1/admin/developers", body, "", 409)
	body["email"] = admin.email
	pexpect(t, admin, "POST", "/api/v1/admin/developers", body, "", 409)
	body["email"] = "invalid email"
	pexpect(t, admin, "POST", "/api/v1/admin/developers", body, "", 400)
	body["email"] = "valid@example.invalid"
	body["password"] = "short"
	pexpect(t, admin, "POST", "/api/v1/admin/developers", body, "", 400)
	client := *f
	client.cookie = nil
	status, _, cookies, err := client.call("POST", "/api/v1/portal/login", map[string]string{"email": email, "password": password}, "", origin)
	if err != nil || status != 200 {
		t.Fatalf("new login: %d %v", status, err)
	}
	for _, cookie := range cookies {
		if cookie.Name == "emisell_portal_session" {
			client.cookie = cookie
		}
	}
	if client.cookie == nil {
		t.Fatal("missing unified session")
	}
	pexpect(t, &client, "POST", "/api/v1/admin/developers", body, "", 401)
	status, session, _, err := client.call("GET", "/api/v1/developer/session", nil, "", origin)
	if err != nil || status != 200 {
		t.Fatalf("developer session: %d %v", status, err)
	}
	if session["organization"].(map[string]any)["id"] != org {
		t.Fatal("wrong developer organization")
	}
}
