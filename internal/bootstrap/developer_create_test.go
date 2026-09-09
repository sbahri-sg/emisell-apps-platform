package bootstrap_test

import "testing"

func TestManualDeveloperCreationRetired(t *testing.T) {
	f := setup(t)
	admin := portalAccount(t, f, "admin", "administrator")
	status, _, _, _ := admin.call("POST", "/api/v1/admin/developers", map[string]string{"email": "new@example.invalid", "password": "unused-password", "organizationName": "Old flow"}, "", origin)
	if status != 405 {
		t.Fatalf("manual creation still available: %d", status)
	}
	pexpect(t, admin, "GET", "/api/v1/admin/developers", nil, "", 200)
}
