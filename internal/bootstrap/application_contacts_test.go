package bootstrap_test

import (
	"strings"
	"testing"
)

func TestApplicationContacts(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	created := pexpect(t, dev, "POST", "/api/v1/developer/apps", map[string]any{"revision": 0, "document": portalDocument()}, "contact-create-1", 200)
	app := created["app"].(map[string]any)["id"].(string)
	path := "/api/v1/developer/apps/" + app + "/contact"
	initial := pexpect(t, dev, "GET", path, nil, "", 200)["contact"].(map[string]any)
	if initial["revision"] != float64(0) || initial["email"] != "" {
		t.Fatal("unexpected initial contact")
	}
	body := map[string]any{"email": "api@example.com", "revision": 0}
	pexpect(t, other, "GET", path, nil, "", 404)
	pexpect(t, other, "PUT", path, body, "", 404)
	pexpect(t, dev, "PUT", path, map[string]any{"email": "Name <api@example.com>", "revision": 0}, "", 400)
	pexpect(t, dev, "PUT", path, body, "", 200)
	pexpect(t, dev, "PUT", path, body, "", 409)
	saved := pexpect(t, dev, "GET", path, nil, "", 200)["contact"].(map[string]any)
	if saved["email"] != "api@example.com" || saved["revision"] != float64(1) {
		t.Fatal("contact not persisted")
	}
	status, _, _, err := dev.call("PUT", path, map[string]any{"email": "new@example.com", "revision": 1}, "", "https://foreign.invalid")
	if err != nil || status != 403 {
		t.Fatal("missing origin boundary")
	}
	admin := portalAccount(t, f, "admin", "administrator")
	status, _, _, _ = admin.call("PUT", strings.Replace(path, "/developer/", "/admin/", 1), body, "", origin)
	if status != 404 {
		t.Fatal("admin cannot edit developer contact")
	}
	unchanged := pexpect(t, dev, "GET", "/api/v1/developer/apps/"+app, nil, "", 200)["app"].(map[string]any)
	if unchanged["revision"] != float64(1) {
		t.Fatal("contact changed release draft")
	}
}
