package bootstrap_test

import (
	"connectrpc.com/connect"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/accessscope"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
)

func TestCatalogPublicationBoundary(t *testing.T) {
	testCatalogPublicationBoundary(t, false)
}
func TestCatalogResourceScopesPublicationBoundary(t *testing.T) {
	testCatalogPublicationBoundary(t, true)
}
func testCatalogPublicationBoundary(t *testing.T, resourceScopes bool) {
	f := setup(t)
	_, signingKey, _ := ed25519.GenerateKey(rand.Reader)
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.HandlerWithCatalog(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), review.CatalogSigner{Key: signingKey}))
	t.Cleanup(f.server.Close)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	reviewer := portalAccount(t, f, "admin", "reviewer")
	doc := portalDocument()
	if resourceScopes {
		doc.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"write_products"}, Optional: []string{"read_customers"}}
		bad := doc
		bad.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"write_products"}, Optional: []string{"read_products"}}
		pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: bad}, key(), 400)
		bad.AccessScopes.Required = []string{"apps.install_intents.consent"}
		pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: bad}, key(), 400)
	}
	doc.Name = "Catalog " + key()
	draft := pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: doc}, key(), 200)["app"].(map[string]any)
	id := draft["id"].(string)
	path := "/api/v1/developer/apps/" + id
	tooling := pexpect(t, dev, "GET", path+"/tooling", nil, "", 200)
	if tooling["valid"] != true {
		t.Fatal("tooling", tooling)
	}
	if resourceScopes {
		m := tooling["manifest"].(map[string]any)
		if m["schema"] != "emisell.catalog/v2" || m["accessScopes"].(map[string]any)["profile"] != accessscope.Profile {
			t.Fatal("scope declaration lost in tooling")
		}
	}
	if _, exists := tooling["manifest"].(map[string]any)["endpoint"]; exists {
		t.Fatal("endpoint in public metadata")
	}
	pexpect(t, other, "GET", path+"/tooling", nil, "", 404)
	sub := pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 1}, key(), 200)["submission"].(map[string]any)["id"].(string)
	signPath := "/api/v1/admin/submissions/" + sub + "/catalog"
	pexpect(t, admin, "POST", signPath, map[string]any{}, key(), 409)
	pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+sub+"/decision", review.Decision{Status: "approved", Feedback: "Metadata only"}, key(), 200)
	pexpect(t, reviewer, "POST", signPath, map[string]any{}, key(), 403)
	k := key()
	signed := pexpect(t, admin, "POST", signPath, map[string]any{}, k, 200)
	if !reflect.DeepEqual(signed, pexpect(t, admin, "POST", signPath, map[string]any{}, k, 200)) {
		t.Fatal("sign replay")
	}
	release := signed["release"].(map[string]any)
	releaseID := release["id"].(string)
	if release["status"] != "signed" {
		t.Fatal("auto publish")
	}
	catalogPath := "/api/v1/admin/catalog/" + releaseID
	pexpect(t, other, "GET", "/api/v1/developer/catalog/"+releaseID, nil, "", 404)
	pexpect(t, dev, "GET", "/api/v1/developer/catalog/"+releaseID, nil, "", 200)
	public := *f
	public.cookie = nil
	publicPath := "/api/v1/store/apps/" + releaseID
	pexpect(t, &public, "GET", publicPath, nil, "", 404)
	pexpect(t, reviewer, "POST", catalogPath+"/status", service.CatalogAction{Status: "published", Revision: 1, Reason: "Approve listing"}, key(), 403)
	publishKey := key()
	action := service.CatalogAction{Status: "published", Revision: 1, Reason: "Publish free metadata catalog"}
	published := pexpect(t, admin, "POST", catalogPath+"/status", action, publishKey, 200)
	if !reflect.DeepEqual(published, pexpect(t, admin, "POST", catalogPath+"/status", action, publishKey, 200)) {
		t.Fatal("publish replay")
	}
	data := pexpect(t, &public, "GET", publicPath, nil, "", 200)["app"].(map[string]any)
	for _, secret := range []string{"endpoint", "feedback", "submissionId", "actorId", "package", "trustedPublicKey", "tenantId"} {
		if _, exists := data[secret]; exists {
			t.Fatal("private data", secret)
		}
	}
	if data["installable"] != false || data["pricing"] != "free" {
		t.Fatal("catalog accidentally installable/paid")
	}
	if resourceScopes {
		requested := data["accessScopes"].(map[string]any)
		if requested["profile"] != accessscope.Profile || requested["required"].([]any)[0] != "write_products" || requested["optional"].([]any)[0] != "read_customers" {
			t.Fatal("public declaration mismatch", requested)
		}
		if _, ok := data["grants"]; ok {
			t.Fatal("metadata must not claim active grants")
		}
	} else if _, ok := data["accessScopes"]; ok {
		t.Fatal("v1 silently migrated")
	}
	// A published catalog app still cannot prepare executable consent or install.
	core, _, _ := serviceClient(t, f, f.tenant, intentScopes())
	_, intentErr := core.InstallIntents.Prepare(context.Background(), connect.NewRequest(&intent.PrepareRequest{CoreActorId: "catalog-test-staff", IdempotencyKey: key(), AppId: id, Version: doc.Version}))
	rpcCode(t, intentErr, connect.CodeNotFound)
	searchPath := "/api/v1/store/apps?search=" + url.QueryEscape(doc.Name) + "&capability=shipping%2Fv1"
	listing := pexpect(t, &public, "GET", searchPath, nil, "", 200)
	if listing["total"] != float64(1) || len(listing["apps"].([]any)) != 1 {
		t.Fatal("search", listing)
	}
	pexpect(t, &public, "GET", "/api/v1/store/apps?page=0", nil, "", 400)
	pexpect(t, &public, "GET", "/api/v1/store/apps?capability=admin", nil, "", 400)
	// Catalog does not enter the executable registry, tenant installation or Core.
	f.expect(t, "POST", f.installPath(f.tenant), map[string]any{"type": "install", "appId": id, "version": "1.0.0", "grants": payScopes}, key(), 404)
	if _, err := f.pool.Exec(context.Background(), `UPDATE platform_app.catalog_releases SET package='{}' WHERE id=$1`, releaseID); err == nil {
		t.Fatal("mutable signed package")
	}
	// One public version per app; a new approved version cannot replace it silently.
	nextDoc := doc
	nextDoc.Version = "1.1.0"
	if resourceScopes {
		nextDoc.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}
	}
	pexpect(t, dev, "PUT", path, service.SaveDraft{Revision: 1, Document: nextDoc}, key(), 200)
	if resourceScopes {
		old := pexpect(t, admin, "GET", "/api/v1/admin/submissions/"+sub, nil, "", 200)["submission"].(map[string]any)["snapshot"].(map[string]any)["accessScopes"].(map[string]any)
		if old["required"].([]any)[0] != "write_products" {
			t.Fatal("draft edit changed immutable scope snapshot")
		}
	}
	nextSub := pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 2}, key(), 200)["submission"].(map[string]any)["id"].(string)
	pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+nextSub+"/decision", review.Decision{Status: "approved", Feedback: "Next catalog version"}, key(), 200)
	nextRelease := pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+nextSub+"/catalog", map[string]any{}, key(), 200)["release"].(map[string]any)["id"].(string)
	pexpect(t, admin, "POST", "/api/v1/admin/catalog/"+nextRelease+"/status", service.CatalogAction{Status: "published", Revision: 1, Reason: "Must suspend previous first"}, key(), 409)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _, _, err := admin.call("POST", catalogPath+"/status", service.CatalogAction{Status: "suspended", Revision: 2, Reason: "QA suspend"}, key(), origin)
			if err != nil {
				codes <- 0
			} else {
				codes <- status
			}
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("concurrent transition", counts)
	}
	pexpect(t, &public, "GET", publicPath, nil, "", 404)
	listing = pexpect(t, &public, "GET", searchPath, nil, "", 200)
	if listing["total"] != float64(0) {
		t.Fatal("suspended still listed")
	}
	history := pexpect(t, admin, "GET", catalogPath, nil, "", 200)["history"].([]any)
	if len(history) != 3 {
		t.Fatal("audit duplicates", len(history))
	}
	if !reflect.DeepEqual(published, pexpect(t, admin, "POST", catalogPath+"/status", action, publishKey, 200)) {
		t.Fatal("retry after suspend must return original response")
	}
	pexpect(t, &public, "GET", publicPath, nil, "", 404)
	// A missing key blocks publication but never prevents emergency suspension.
	missing := httptest.NewServer(bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer missing.Close()
	offline := *admin
	offline.server = missing
	pexpect(t, &offline, "POST", catalogPath+"/status", service.CatalogAction{Status: "published", Revision: 3, Reason: "Key missing"}, key(), 503)
}
