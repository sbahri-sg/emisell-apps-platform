package bootstrap_test

import (
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/webhook/subscriptions"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/webhookconfig"
	"testing"
)

func TestWebhookSubscriptionAuthoring(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	doc := portalDocument()
	doc.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}
	app := pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: doc}, key(), 200)["app"].(map[string]any)
	path := "/api/v1/developer/apps/" + app["id"].(string) + "/webhook-subscriptions"
	b := subscriptions.Input{DraftRevision: 1, Version: doc.Version, Topic: "products.created", Endpoint: "https://example.com/events"}
	k := key()
	first := pexpect(t, dev, "POST", path, b, k, 200)["subscription"].(map[string]any)
	if first["status"] != "pending" || first["deliveryEnabled"] != false {
		t.Fatal(first)
	}
	replay := pexpect(t, dev, "POST", path, b, k, 200)["subscription"].(map[string]any)
	if replay["id"] != first["id"] {
		t.Fatal("replay duplicated request")
	}
	changed := b
	changed.Endpoint = "https://example.com/other"
	pexpect(t, dev, "POST", path, changed, k, 409)
	pexpect(t, dev, "POST", path, b, key(), 409)
	changed = b
	changed.Topic = "orders.created"
	pexpect(t, dev, "POST", path, changed, key(), 403)
	changed = b
	changed.Topic = "products.deleted"
	pexpect(t, dev, "POST", path, changed, key(), 400)
	changed = b
	changed.Endpoint = "https://127.0.0.1/events"
	pexpect(t, dev, "POST", path, changed, key(), 400)
	changed = b
	changed.DraftRevision = 2
	pexpect(t, dev, "POST", path, changed, key(), 409)
	pexpect(t, other, "GET", path, nil, "", 404)
	pexpect(t, other, "POST", path, b, key(), 404)
	rows := pexpect(t, dev, "GET", path, nil, "", 200)["subscriptions"].([]any)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	revoke := path + "/" + first["id"].(string) + "/revoke"
	pexpect(t, other, "POST", revoke, struct{}{}, "", 404)
	for range 2 {
		result := pexpect(t, dev, "POST", revoke, struct{}{}, "", 200)["subscription"].(map[string]any)
		if result["status"] != "revoked" || result["deliveryEnabled"] != false {
			t.Fatal(result)
		}
	}
	replay = pexpect(t, dev, "POST", path, b, k, 200)["subscription"].(map[string]any)
	if replay["status"] != "revoked" {
		t.Fatal("replay reactivated request")
	}
	var auditCount int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM platform_webhook.subscription_audit WHERE subscription_id=$1`, first["id"]).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("audit %d %v", auditCount, err)
	}
	pexpect(t, dev, "GET", "/api/v1/developer/webhook-topics", nil, "", 200)
}

func TestAppSpecificWebhookReviewSnapshot(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	doc := portalDocument()
	doc.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}
	doc.Webhooks = &webhookconfig.Config{APIVersion: webhookconfig.APIVersion, Subscriptions: []webhookconfig.Subscription{{Topics: []string{"products.created"}, URI: "https://example.com/events"}}}
	app := pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: doc}, key(), 200)["app"].(map[string]any)
	path := "/api/v1/developer/apps/" + app["id"].(string)
	sub := pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 1}, key(), 200)["submission"].(map[string]any)
	doc.Webhooks.Subscriptions[0].URI = "https://other.com/events"
	pexpect(t, dev, "PUT", path, service.SaveDraft{Revision: 1, Document: doc}, key(), 200)
	frozen := pexpect(t, dev, "GET", "/api/v1/developer/submissions/"+sub["id"].(string), nil, "", 200)["submission"].(map[string]any)
	hook := frozen["snapshot"].(map[string]any)["webhooks"].(map[string]any)["subscriptions"].([]any)[0].(map[string]any)
	if hook["uri"] != "https://example.com/events" {
		t.Fatal("review snapshot changed", hook)
	}
	var requests int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM platform_webhook.subscription_requests WHERE app_id=$1`, app["id"]).Scan(&requests); err != nil || requests != 0 {
		t.Fatal("app configuration created a separate approval request", err)
	}
}
