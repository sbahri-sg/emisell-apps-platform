package bootstrap_test

import (
	"context"
	"reflect"
	"testing"
)

func TestInitialActiveVersion(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	body := map[string]any{"revision": 0, "document": portalDocument()}
	created := pexpect(t, dev, "POST", "/api/v1/developer/apps", body, "active-create-1", 200)["app"].(map[string]any)
	id := created["id"].(string)
	active, ok := created["activeVersion"].(map[string]any)
	if !ok || active["revision"] != float64(1) || active["activatedAt"] == "" || !reflect.DeepEqual(active["document"], created["document"]) {
		t.Fatal("initial configuration not active")
	}
	replay := pexpect(t, dev, "POST", "/api/v1/developer/apps", body, "active-create-1", 200)["app"].(map[string]any)
	if !reflect.DeepEqual(created, replay) {
		t.Fatal("replay changed version")
	}
	doc := portalDocument()
	doc.Version = "2.0.0"
	edited := pexpect(t, dev, "PUT", "/api/v1/developer/apps/"+id, map[string]any{"revision": 1, "document": doc}, "active-update-1", 200)["app"].(map[string]any)
	if !reflect.DeepEqual(active, edited["activeVersion"]) || edited["document"].(map[string]any)["version"] != "2.0.0" {
		t.Fatal("working draft replaced active configuration")
	}
	got := pexpect(t, dev, "GET", "/api/v1/developer/apps/"+id, nil, "", 200)["app"].(map[string]any)
	if !reflect.DeepEqual(active, got["activeVersion"]) {
		t.Fatal("active snapshot not persisted")
	}
	pexpect(t, other, "GET", "/api/v1/developer/apps/"+id, nil, "", 404)
	pexpect(t, dev, "PUT", "/api/v1/developer/apps/"+id, map[string]any{"revision": 1, "document": doc}, "active-stale-1", 409)
	var count int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM platform_app.draft_audit WHERE app_id=$1 AND action='initial_version_activated'", id).Scan(&count); err != nil || count != 1 {
		t.Fatal("activation audit not exactly once", err)
	}
	if _, err := f.pool.Exec(context.Background(), "UPDATE platform_app.drafts SET active_version=NULL WHERE id=$1", id); err == nil {
		t.Fatal("active snapshot allowed mutation")
	}
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM platform_review.submissions WHERE app_id=$1", id).Scan(&count); err != nil || count != 0 {
		t.Fatal("initial activation created a review", err)
	}
}
