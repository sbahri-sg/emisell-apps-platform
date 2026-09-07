package bootstrap_test

import (
	"context"
	"testing"
)

func TestOverviewUsesDatabaseAndIsAdminOnly(t *testing.T) {
	f := setup(t)
	admin := portalAccount(t, f, "admin", "administrator")
	dev := portalAccount(t, f, "developer", "developer")
	pexpect(t, f, "GET", "/api/v1/admin/overview", nil, "", 401)
	pexpect(t, dev, "GET", "/api/v1/admin/overview", nil, "", 403)
	data := pexpect(t, admin, "GET", "/api/v1/admin/overview", nil, "", 200)
	var expected int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM platform_identity.portal_accounts WHERE surface='developer' AND enabled").Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if data["developers"] != float64(expected) {
		t.Fatalf("developer count: %v want %d", data["developers"], expected)
	}
	history, ok := data["history"].([]any)
	if !ok || len(history) != 30 {
		t.Fatalf("history: %v", data["history"])
	}
	for _, point := range history {
		p := point.(map[string]any)
		if len(p["date"].(string)) != 10 || p["count"].(float64) < 0 {
			t.Fatal(p)
		}
	}
	if data["checkedAt"] == nil {
		t.Fatal("missing snapshot timestamp")
	}
}
