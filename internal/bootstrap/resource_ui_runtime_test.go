package bootstrap_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appclient"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/oauth/embedded"
	launchrepo "emisell.app/platform/internal/oauth/embedded/postgres"
	"emisell.app/platform/internal/resourceclient"
	"emisell.app/platform/internal/transport/connectapi"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func TestResourceUIConsentLaunchReadAndRevocation(t *testing.T) {
	for _, tc := range []struct{ scope, path, body string }{
		{"read_products", "/v1/products", `{"data":[],"meta":{"nextCursor":null}}`},
		{"read_orders", "/v1/orders", `{"data":[],"meta":{"nextCursor":null}}`},
		{"read_shipping", "/v1/settings/shipping", `{"data":null,"meta":{"nextCursor":null}}`},
		{"read_catalogs", "/v1/catalogs", `{"data":[],"meta":{"nextCursor":null}}`},
		{"read_collections", "/v1/collections", `{"data":[],"meta":{"nextCursor":null}}`},
		{"read_inventory", "/v1/products", `{"data":[],"meta":{"nextCursor":null}}`},
		{"read_locations", "/v1/settings/location", `{"data":[],"meta":{"nextCursor":null}}`},
	} {
		t.Run(tc.scope, func(t *testing.T) { testResourceConsent(t, tc.scope, tc.path, tc.body) })
	}
}
func testResourceConsent(t *testing.T, scope, path, response string) {
	pub, signer, _ := ed25519.GenerateKey(rand.Reader)
	f, dev, _, admin := clientFixture(t, &proofVerifier{}, bootstrap.EmbeddedReviewConfig{Key: signer, ParentOrigin: "https://core.example", UIReleaseKey: signer, ResourceReleaseKey: signer})
	base := f.server.Config.Handler
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.AddUIResourceReleaseRoutes(base, f.pool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), signer))
	t.Cleanup(f.server.Close)
	dev.server, admin.server = f.server, f.server
	release := pexpect(t, dev, "POST", "/api/v1/developer/ui-resource-releases", service.UIResourceReleaseInput{Version: "1.0.0", Name: "Resource read", Summary: "Read only", Mode: "embedded", URL: "https://app.example.com/", Reason: "test", RequiredScopes: []string{scope}}, key(), 200)["release"].(map[string]any)
	id := release["id"].(string)
	app := release["manifest"].(map[string]any)["ui"].(map[string]any)["appId"].(string)
	for i, status := range []string{"approved", "signed"} {
		pexpect(t, admin, "POST", "/api/v1/admin/ui-resource-releases/"+id+"/status", service.CatalogAction{Revision: i + 1, Status: status, Reason: "test"}, key(), 200)
	}
	client := clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients", appclient.Input{ReleaseID: id}, key(), 200))
	client = clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients/"+client.Client.ID+"/actions", appclient.Action{Revision: 1, Action: "verify", Reason: "test"}, key(), 200))
	issued := pexpect(t, dev, "POST", "/api/v1/developer/app-clients/"+client.Client.ID+"/actions", appclient.Action{Revision: client.Client.Revision, Action: "rotate_secret", Reason: "test"}, key(), 200)
	client = clientView(t, issued)
	clientSecret := issued["secret"].(string)
	launch := pexpect(t, dev, "POST", "/api/v1/developer/embedded-launches", embedded.LaunchInput{ClientID: client.Client.ID, URL: "https://app.example.com/", Mode: "embedded", Reason: "test"}, key(), 200)["launch"].(map[string]any)
	pexpect(t, admin, "POST", "/api/v1/admin/embedded-launches/"+launch["id"].(string)+"/status", map[string]any{"revision": 1, "status": "approved", "reason": "test"}, key(), 200)
	a := pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", service.AssignmentInput{ReleaseKind: "ui_resource", ReleaseID: id, MerchantID: f.tenant, Reason: "test"}, key(), 200)["assignment"].(map[string]any)
	aid := a["id"].(string)
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+aid+"/status", service.AssignmentAction{Revision: 1, Status: "approved", Reason: "test"}, key(), 200)
	var reads atomic.Int32
	var searchRead atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if r.Header.Get("X-Emisell-Merchant-Id") != f.tenant || r.URL.Query().Get("limit") != "5" {
			t.Error("wrong merchant or bound")
		}
		if r.URL.Query().Get("cursor") == "page.signature" {
			searchRead.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Emisell-App-Access", "resource-v1")
		if r.URL.Path != path {
			t.Error("wrong existing endpoint", r.URL.Path)
		}
		if scope == "read_inventory" && r.URL.Query().Get("view") != "inventory" {
			t.Error("inventory projection missing")
		}
		io.WriteString(w, response)
	}))
	t.Cleanup(upstream.Close)
	private, _ := rsa.GenerateKey(rand.Reader, 2048)
	products, err := resourceclient.NewProducts(resourceclient.Options{Origin: upstream.URL, Environment: "sandbox", KeyID: "test", AllowHTTP: true, PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(private)})})
	if err != nil {
		t.Fatal(err)
	}
	source := bootstrap.ReviewedUIInstallSource{Releases: apprepo.Postgres{Pool: f.caps}, Clients: clientrepo.Repository{Pool: f.pool}, ReleaseKey: pub, ResourceKey: signer, Products: products, Current: embedded.CurrentBinding{Clients: appclient.Service{Repo: clientrepo.Repository{Pool: f.pool}}, Launches: launchrepo.Repository{Pool: f.pool}, Key: pub, ParentOrigin: "https://core.example"}}
	_, principal, coreKey, _ := lifecycleClient(t, f)
	intents := installservice.Intents{Repo: installrepo.Repository{Pool: f.pool}, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}}
	composed := connectapi.Server{Accounts: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Intents: intents, Lifecycle: installservice.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}}, Testing: service.Testing{Repo: apprepo.Postgres{Pool: f.pool}}}
	if err = bootstrap.EnableReviewedUI(&composed, source, signer); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	intents = composed.Intents
	life := composed.Lifecycle
	rows, _, err := composed.Testing.ForMerchant(ctx, f.tenant, "", 20)
	if err != nil || len(rows) != 1 || !rows[0].Readiness.Installable {
		t.Fatal("not installable", err, rows)
	}
	intent, err := intents.Prepare(ctx, principal, "staff", key(), installservice.PrepareIntent{AppID: app, Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = life.Consume(ctx, principal, "staff", key(), intent.ID, intent.ConsentDigest); err == nil {
		t.Fatal("missing consent allowed")
	}
	if _, err = intents.Decide(ctx, principal, "staff", key(), installservice.DecideIntent{ID: intent.ID, Digest: intent.ConsentDigest, Decision: "consent"}); err != nil {
		t.Fatal(err)
	}
	consumed, err := life.Consume(ctx, principal, "staff", key(), intent.ID, intent.ConsentDigest)
	if err != nil {
		t.Fatal(err)
	}
	install := consumed.Access.Installation.ID
	read := func(actor, appID, clientID string) error {
		query := url.Values{"limit": {"5"}}
		if scope == "read_inventory" {
			query.Set("view", "inventory")
		}
		_, e := products.ReadExistingForApp(ctx, life, principal, actor, install, appID, clientID, path, query, "resource-test-request-12345")
		return e
	}
	if read("staff", app, client.Client.ID) == nil || reads.Load() != 0 {
		t.Fatal("pending installation accessed products")
	}
	if _, err = life.Execute(ctx, principal, "staff", key(), install, "activate"); err != nil {
		t.Fatal(err)
	}
	installed, _, err := life.List(ctx, principal, "staff", "", 20)
	if err != nil || len(installed) != 1 || installed[0].ID != install || installed[0].ExecutionProfile != domain.ResourceAppPolicy || installed[0].Status != "active" {
		t.Fatal("resource installation missing from seller list", err, installed)
	}
	current, err := life.Get(ctx, principal, "staff", install)
	if err != nil || current.ReviewedUILaunch == nil {
		t.Fatal("missing launch", err)
	}
	if err = read("staff", app, client.Client.ID); err != nil || reads.Load() != 1 {
		t.Fatal("read failed", err)
	}
	if read("other", app, client.Client.ID) == nil || read("staff", "app_other", client.Client.ID) == nil || read("staff", app, "other_client") == nil || reads.Load() != 1 {
		t.Fatal("identity bypass")
	}
	if scope == "read_inventory" {
		if _, err := products.ReadExistingForApp(ctx, life, principal, "staff", install, app, client.Client.ID, path, url.Values{"limit": {"5"}}, "resource-test-request-12345"); err == nil || reads.Load() != 1 {
			t.Fatal("inventory installation gained product access without selector")
		}
	}
	if life.WithResourceAccess(ctx, principal, "staff", install, []string{"write_orders"}, func(domain.Access) error { return nil }) == nil {
		t.Fatal("unconsented scope")
	}
	// Exercise the actual private HTTP boundary, not only the lifecycle adapter.
	rpc := httptest.NewServer(composed.Handler())
	t.Cleanup(rpc.Close)
	request := func(patch map[string]any, mutate func(*http.Request), want int) {
		t.Helper()
		body := map[string]any{"merchantId": f.tenant, "coreActorId": "staff", "installationId": install, "appId": app, "clientId": client.Client.ID, "clientSecret": clientSecret, "limit": 5, "path": path}
		if scope == "read_inventory" {
			body["view"] = "inventory"
		}
		for k, v := range patch {
			body[k] = v
		}
		raw, _ := json.Marshal(body)
		r, _ := http.NewRequest("POST", rpc.URL+"/internal/resources/products", bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+coreKey)
		r.Header.Set("Content-Type", "application/json")
		if mutate != nil {
			mutate(r)
		}
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("private product HTTP status %d, want %d", res.StatusCode, want)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("product response cacheable")
		}
	}
	request(nil, nil, 200)
	if scope == "read_inventory" {
		request(map[string]any{"view": ""}, nil, 403)
		request(map[string]any{"view": "unknown"}, nil, 400)
	}
	request(map[string]any{"cursor": "page.signature"}, nil, 200)
	if !searchRead.Load() {
		t.Fatal("search and cursor not forwarded to product API")
	}
	for _, bad := range []map[string]any{{"clientSecret": "wrong"}, {"clientId": "other"}, {"appId": "other"}, {"merchantId": "other"}, {"coreActorId": "other"}, {"installationId": "ins_missing"}} {
		request(bad, nil, 403)
	}
	request(map[string]any{"scopes": []string{"read_orders"}}, nil, 400)
	request(map[string]any{"limit": 21}, nil, 400)
	request(map[string]any{"q": " Blue"}, nil, 400)
	request(nil, func(r *http.Request) { r.Header.Set("Origin", "https://core.example") }, 403)
	request(nil, func(r *http.Request) { r.Header.Set("Cookie", "session=not-authority") }, 403)
	request(nil, func(r *http.Request) { r.Header.Del("Authorization") }, 403)
	if reads.Load() != 3 {
		t.Fatal("denied HTTP requests reached products")
	}
	if _, err = composed.Testing.StopForMerchant(ctx, f.tenant, principal.ID, "staff", aid, key()); err != nil {
		t.Fatal("seller stop", err)
	}
	request(nil, nil, 403)
	if read("staff", app, client.Client.ID) == nil || reads.Load() != 3 {
		t.Fatal("revoked assignment accessed products")
	}
	if _, err = life.Execute(ctx, principal, "staff", key(), install, "uninstall"); err != nil {
		t.Fatal("uninstall after revoke", err)
	}
	request(nil, nil, 403)
	installed, _, err = life.List(ctx, principal, "staff", "", 20)
	if err != nil || len(installed) != 0 || reads.Load() != 3 {
		t.Fatal("uninstalled resource access remained", err)
	}
}
