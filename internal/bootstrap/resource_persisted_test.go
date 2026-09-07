package bootstrap_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appclient"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/resourcerelease"
	"emisell.app/platform/pkg/accessscope"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPersistedResourceReleaseConsentAndRevocation(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	_, principal, _, _ := lifecycleClient(t, f)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	r := domain.IntentRelease{AppID: "app_" + key(), Name: "Product pilot", DeveloperID: "org_" + key(), Version: "1.0.0", InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy, Scopes: []string{"read_products"}, Capabilities: []string{}, ResourceBinding: &domain.ResourceBinding{ReleaseID: "release_" + key(), ClientID: "client_" + key(), AccessScopes: accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}}}}
	r.ResourceBinding.AccessScopes.Optional = []string{}
	r.ManifestDigest = resourcerelease.Digest(r)
	binding := appclient.Binding{ReleaseID: r.ResourceBinding.ReleaseID, AppID: r.AppID, OrganizationID: r.DeveloperID, Version: r.Version, Name: r.Name, Digest: r.ManifestDigest, Endpoint: "https://app.example.com"}
	rawBinding, _ := json.Marshal(binding)
	_, err := f.pool.Exec(ctx, `INSERT INTO platform_oauth.app_clients(id,organization_id,release_id,binding,status,revision,challenge_id,challenge,challenge_expires_at,verified_until,last_result,secret_hash,secret_version) VALUES($1,$2,$3,$4,'verified',1,'challenge','proof',now()+interval '1 hour',now()+interval '1 hour','verified',$5,1)`, r.ResourceBinding.ClientID, r.DeveloperID, r.ResourceBinding.ReleaseID, rawBinding, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	envelope := resourcerelease.Envelope{Schema: resourcerelease.Schema, MerchantID: f.tenant, Environment: "sandbox", ExpiresAt: time.Now().UTC().Add(time.Hour), Release: r, Client: binding}
	raw, _ := json.Marshal(envelope)
	signature := ed25519.Sign(priv, raw)
	assignment := "assignment_" + key()
	_, err = f.pool.Exec(ctx, `INSERT INTO platform_app.resource_release_assignments(id,merchant_id,app_id,version,release_id,envelope,signature) VALUES($1,$2,$3,$4,$5,$6,$7)`, assignment, f.tenant, r.AppID, r.Version, r.ResourceBinding.ReleaseID, raw, signature)
	if err != nil {
		t.Fatal(err)
	}
	source := resourcerelease.Source{Pool: f.pool, PublicKey: pub, Environment: "sandbox", Clients: appclient.Service{Repo: clientrepo.Repository{Pool: f.pool}}}
	lifecycle := service.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}, Resources: source, Intents: service.Intents{Repo: installrepo.Repository{Pool: f.pool}, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, Managed: source}}
	intent, err := lifecycle.Intents.Prepare(ctx, principal, "staff", key(), service.PrepareIntent{AppID: r.AppID, Version: r.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.Consume(ctx, principal, "staff", key(), intent.ID, intent.ConsentDigest); err == nil {
		t.Fatal("consent bypass")
	}
	if _, err = lifecycle.Intents.Decide(ctx, principal, "staff", key(), service.DecideIntent{ID: intent.ID, Digest: intent.ConsentDigest, Decision: "consent"}); err != nil {
		t.Fatal(err)
	}
	consumed, err := lifecycle.Consume(ctx, principal, "staff", key(), intent.ID, intent.ConsentDigest)
	if err != nil {
		t.Fatal(err)
	}
	id := consumed.Access.Installation.ID
	if _, err = lifecycle.Execute(ctx, principal, "staff", key(), id, "activate"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	read := func(domain.Access) error { calls++; return nil }
	if err = lifecycle.WithResourceAccess(ctx, principal, "staff", id, []string{"read_products"}, read); err != nil || calls != 1 {
		t.Fatal("active read", err)
	}
	if err = lifecycle.WithResourceAccess(ctx, principal, "staff", id, []string{"read_orders"}, read); err == nil {
		t.Fatal("extra scope accepted")
	}
	wrong := source
	wrong.PublicKey = make(ed25519.PublicKey, ed25519.PublicKeySize)
	if err = wrong.WithRelease(ctx, f.tenant, r.AppID, r.Version, func(domain.IntentRelease) error { t.Fatal("untrusted signature accepted"); return nil }); err == nil {
		t.Fatal("untrusted signing key")
	}
	wrong = source
	wrong.Environment = "production"
	if err = wrong.WithRelease(ctx, f.tenant, r.AppID, r.Version, func(domain.IntentRelease) error { t.Fatal("wrong environment accepted"); return nil }); err == nil {
		t.Fatal("wrong environment")
	}
	if err = source.WithRelease(ctx, f.other, r.AppID, r.Version, func(domain.IntentRelease) error { t.Fatal("foreign merchant"); return nil }); err == nil {
		t.Fatal("foreign assignment")
	}
	_, err = f.pool.Exec(ctx, `UPDATE platform_oauth.app_clients SET status='revoked',secret_hash='',revision=revision+1 WHERE id=$1`, r.ResourceBinding.ClientID)
	if err != nil {
		t.Fatal(err)
	}
	if err = lifecycle.WithResourceAccess(ctx, principal, "staff", id, []string{"read_products"}, read); err == nil || calls != 1 {
		t.Fatal("revoked client read")
	}
	_, err = f.pool.Exec(ctx, `UPDATE platform_app.resource_release_assignments SET status='revoked' WHERE id=$1`, assignment)
	if err != nil {
		t.Fatal(err)
	}
	if err = lifecycle.WithResourceAccess(ctx, principal, "staff", id, []string{"read_products"}, read); err == nil || calls != 1 {
		t.Fatal("revoked assignment read")
	}
	if _, err = lifecycle.Execute(ctx, principal, "staff", key(), id, "uninstall"); err != nil {
		t.Fatal("uninstall after revocation", err)
	}
	if err = lifecycle.WithResourceAccess(ctx, principal, "staff", id, []string{"read_products"}, read); err == nil {
		t.Fatal("uninstalled read")
	}
	_, err = f.pool.Exec(ctx, `UPDATE platform_app.resource_release_assignments SET status='approved' WHERE id=$1`, assignment)
	if err == nil {
		t.Fatal("revoked assignment resurrected")
	}
}
