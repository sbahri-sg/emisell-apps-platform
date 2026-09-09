package bootstrap_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	installservice "emisell.app/platform/internal/installation/service"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/resourceclient"
	"emisell.app/platform/pkg/accessscope"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestPrivateProductCreateConsentAndIsolation(t *testing.T) {
	t.Setenv("EMISELL_APP_CREDENTIAL_KEY", base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{53}, 32)))
	_, signer, _ := ed25519.GenerateKey(rand.Reader)
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE platform_identity.developer_core_links SET core_subject=$2 WHERE account_id=$1`, dev.user, "private-owner-"+dev.user); err != nil {
		t.Fatal(err)
	}
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.HandlerWithPrivateProducts(f.pool, f.caps, f.pool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil, signer))
	t.Cleanup(f.server.Close)
	dev.server, other.server = f.server, f.server
	doc := appservice.AppDocument{Name: "Product Reader", Version: "1.0.0", Capability: appservice.PrivateProducts, Scopes: []string{}, AccessScopes: &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}}
	body := map[string]any{"revision": 0, "document": doc}
	created := pexpect(t, dev, "POST", "/api/v1/developer/apps", body, "private-create-1", 200)["app"].(map[string]any)
	app := created["id"].(string)
	if created["activeVersion"] == nil {
		t.Fatal("missing active configuration")
	}
	replay := pexpect(t, dev, "POST", "/api/v1/developer/apps", body, "private-create-1", 200)["app"].(map[string]any)
	if !reflect.DeepEqual(created, replay) {
		t.Fatal("duplicate creation")
	}
	pexpect(t, other, "GET", "/api/v1/developer/apps/"+app, nil, "", 404)
	credentials := pexpect(t, dev, "GET", "/api/v1/developer/apps/"+app+"/credentials", nil, "", 200)["credential"].(map[string]any)
	_, principal, coreKey, _ := lifecycleClient(t, f)
	var reads atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if r.URL.Path != "/v1/products" || r.Header.Get("X-Emisell-Merchant-ID") != f.tenant {
			t.Error("wrong existing endpoint/merchant")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Emisell-App-Access", "resource-v1")
		io.WriteString(w, `{"data":[],"meta":{"nextCursor":null}}`)
	}))
	t.Cleanup(upstream.Close)
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	products, err := resourceclient.NewProducts(resourceclient.Options{Origin: upstream.URL, Environment: "sandbox", KeyID: "test", AllowHTTP: true, PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)})})
	if err != nil {
		t.Fatal(err)
	}
	source := bootstrap.PrivateProductInstallSource{UI: bootstrap.ReviewedUIInstallSource{Releases: apprepo.Postgres{Pool: f.caps}, Clients: clientrepo.Repository{Pool: f.pool}, ResourceKey: signer, Products: products}}
	intents := installservice.Intents{Repo: installrepo.Repository{Pool: f.pool}, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, Managed: source}
	life := installservice.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}, Intents: intents, Resources: source}
	actor := "private-owner-" + dev.user
	input := installservice.PrepareIntent{AppID: app, Version: "1.0.0"}
	if _, err := intents.Prepare(ctx, principal, "different-owner", key(), input); err == nil {
		t.Fatal("foreign owner prepared private app")
	}
	prepareKey := key()
	v, err := intents.Prepare(ctx, principal, actor, prepareKey, input)
	if err != nil {
		t.Fatal(err)
	}
	if v.Release.Name != doc.Name || !reflect.DeepEqual(v.Release.Scopes, []string{"read_products"}) || len(v.Release.Capabilities) != 0 || v.Release.UIBinding != nil || v.Release.ResourceBinding.ClientID != credentials["clientId"] {
		t.Fatal("incorrect consent")
	}
	replayIntent, err := intents.Prepare(ctx, principal, actor, prepareKey, input)
	if err != nil || replayIntent.ID != v.ID {
		t.Fatal("duplicate intent", err)
	}
	if _, err = life.Consume(ctx, principal, actor, key(), v.ID, v.ConsentDigest); err == nil {
		t.Fatal("install without consent")
	}
	if _, err = intents.Decide(ctx, principal, actor, key(), installservice.DecideIntent{ID: v.ID, Digest: v.ConsentDigest, Decision: "consent"}); err != nil {
		t.Fatal(err)
	}
	access, err := life.Consume(ctx, principal, actor, key(), v.ID, v.ConsentDigest)
	if err != nil {
		t.Fatal(err)
	}
	installation := access.Access.Installation.ID
	read := func(p identity.ServicePrincipal, who, scope string) error {
		return life.WithResourceAccess(ctx, p, who, installation, []string{scope}, func(domain.Access) error { return nil })
	}
	if read(principal, actor, "read_products") == nil {
		t.Fatal("pending read allowed")
	}
	active, err := life.Execute(ctx, principal, actor, key(), installation, "activate")
	if err != nil {
		t.Fatal(err)
	}
	if active.Access.GrantState != "active" || !reflect.DeepEqual(active.Access.GrantedScopes, []string{"read_products"}) {
		t.Fatal("wrong grant")
	}
	if read(principal, actor, "read_products") != nil {
		t.Fatal("product read denied")
	}
	secret := pexpect(t, dev, "POST", "/api/v1/developer/apps/"+app+"/credentials/reveal", map[string]int{"version": 1}, "", 200)["secret"].(string)
	transport, err := bootstrap.InternalHandlerWithPrivateProducts(f.pool, f.caps, f.pool, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, signer, products)
	if err != nil {
		t.Fatal(err)
	}
	rpc := httptest.NewServer(transport)
	t.Cleanup(rpc.Close)
	// The production composition must not fall back to historical fixtures.
	for _, id := range []string{"parcel", "emisell-pay", "embedded-local-demo", "app_AAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		raw, _ := json.Marshal(map[string]string{"merchantId": f.tenant, "coreActorId": actor, "appId": id, "version": "1.0.0", "idempotencyKey": key()})
		req, _ := http.NewRequest("POST", rpc.URL+"/emisell.installation.v1.InstallIntentService/Prepare", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+coreKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 && res.StatusCode != 404 {
			t.Fatalf("fixture/unknown app %s status %d", id, res.StatusCode)
		}
	}
	requestRead := func(clientSecret, path string, want int) {
		raw, _ := json.Marshal(map[string]any{"merchantId": f.tenant, "coreActorId": actor, "appId": app, "clientId": credentials["clientId"], "clientSecret": clientSecret, "installationId": installation, "limit": 5, "path": path})
		req, _ := http.NewRequest("POST", rpc.URL+"/internal/resources/products", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+coreKey)
		req.Header.Set("Content-Type", "application/json")
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("resource status %d want %d", response.StatusCode, want)
		}
	}
	requestRead("invalid-secret", "/v1/products", 403)
	requestRead(secret, "/v1/orders", 403)
	if reads.Load() != 0 {
		t.Fatal("denied request reached Core")
	}
	requestRead(secret, "/v1/products", 200)
	if reads.Load() != 1 {
		t.Fatal("existing Core product read not called")
	}
	pexpect(t, dev, "POST", "/api/v1/developer/apps/"+app+"/credentials/rotate", map[string]int{"version": 1}, "private-rotate-1", 200)
	requestRead(secret, "/v1/products", 403)
	if reads.Load() != 1 {
		t.Fatal("old credential reached Core")
	}
	for _, scope := range []string{"write_products", "read_orders", "read_customers", "read_shipping", "read_users", "write_metaobjects"} {
		if read(principal, actor, scope) == nil {
			t.Fatal("scope escalation", scope)
		}
	}
	cross := principal
	cross.TenantID = f.other
	if read(cross, actor, "read_products") == nil || read(principal, "different-owner", "read_products") == nil {
		t.Fatal("cross-context read")
	}
	if _, err = life.Execute(ctx, principal, actor, key(), installation, "issue_token"); err == nil {
		t.Fatal("legacy token issued")
	}
	if _, err = f.pool.Exec(ctx, `UPDATE platform_app.private_product_versions SET version='2.0.0' WHERE app_id=$1`, app); err == nil {
		t.Fatal("mutable release")
	}
	// The account's current identity, not cached stores or an earlier grant, is authoritative.
	if _, err = f.pool.Exec(ctx, `UPDATE platform_identity.portal_accounts SET enabled=false WHERE id=$1`, dev.user); err != nil {
		t.Fatal(err)
	}
	if read(principal, actor, "read_products") == nil {
		t.Fatal("disabled owner read allowed")
	}
	if _, err = life.Execute(ctx, principal, actor, key(), installation, "uninstall"); err != nil {
		t.Fatal("cannot revoke while source disabled", err)
	}
	if read(principal, actor, "read_products") == nil {
		t.Fatal("revoked grant allowed")
	}
	var reviews, assignments int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_review.submissions WHERE app_id=$1`, app).Scan(&reviews); err != nil || reviews != 0 {
		t.Fatal("private creation used public review", err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_app.test_assignments WHERE organization_id=$1`, created["organizationId"]).Scan(&assignments); err != nil || assignments != 0 {
		t.Fatal("private creation assigned stores", err)
	}
}
