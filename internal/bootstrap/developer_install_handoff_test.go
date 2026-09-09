package bootstrap_test

import "testing"

func TestDeveloperInstallHandoff(t *testing.T) {
	t.Setenv("EMISELL_SELLER_ORIGIN", "https://seller.emisell.test")
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	created := pexpect(t, dev, "POST", "/api/v1/developer/apps", map[string]any{"revision": 0, "document": portalDocument()}, "handoff-create", 200)["app"].(map[string]any)
	id := created["id"].(string)
	path := "/api/v1/developer/apps/" + id + "/install-url"
	want := "https://seller.emisell.test/auth/stores?app=" + id + "&version=" + portalDocument().Version
	if got := pexpect(t, dev, "GET", path, nil, "", 200)["url"]; got != want {
		t.Fatalf("unexpected selection URL: %v", got)
	}
	doc := portalDocument()
	doc.Version = "2.0.0"
	pexpect(t, dev, "PUT", "/api/v1/developer/apps/"+id, map[string]any{"revision": 1, "document": doc}, "handoff-edit", 200)
	if got := pexpect(t, dev, "GET", path, nil, "", 200)["url"]; got != want {
		t.Fatal("working draft changed installation version")
	}
	pexpect(t, other, "GET", path, nil, "", 404)
	// A merchant cookie/origin cannot act as a developer portal session.
	pexpect(t, f, "GET", path, nil, "", 403)
	anonymous := *dev
	anonymous.cookie = nil
	status, _, _, err := anonymous.call("GET", path, nil, "", developerOrigin)
	if err != nil || status != 401 {
		t.Fatalf("anonymous handoff: %d %v", status, err)
	}
	t.Setenv("EMISELL_SELLER_ORIGIN", "https://seller.emisell.test/arbitrary")
	pexpect(t, dev, "GET", path, nil, "", 503)
}
