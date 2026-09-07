package bootstrap

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/uirelease"
	"emisell.app/platform/pkg/uiresource"
	"testing"
	"time"
)

type assignmentGate struct{ revoked bool }

func (g *assignmentGate) WithApproved(_ context.Context, merchant, app, version string, fn func(string, string, string) error) error {
	if g.revoked || merchant != "merchant_test" || app != "app_demo" || version != "0.1.0" {
		return fault.Forbidden
	}
	return fn("org_demo", "release_demo", "client_demo")
}

type readyClientRepo struct {
	appclient.Repository
	c appclient.Client
}

func (r *readyClientRepo) WithClient(_ context.Context, _ string, fn func(appclient.Client) error) error {
	return fn(r.c)
}
func TestUIResourceSourceConsentSnapshot(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	m := uiresource.Manifest{Schema: uiresource.Schema, Policy: uiresource.Policy, RequiredScopes: []string{"read_products"}, UI: uirelease.Manifest{Schema: uirelease.Schema, Policy: uirelease.Policy, AppID: "app_demo", DeveloperID: "org_demo", Version: "0.1.0", Name: "Products", Summary: "Read only", Mode: "embedded", URL: "https://app.example.com/", Pricing: "free"}}
	p, _ := uiresource.Sign(m, key)
	repo := &resourceBindingRepo{release: service.UIResourceRelease{ID: "release_demo", Manifest: m, Package: &p, Status: "signed"}}
	until := time.Now().Add(time.Hour)
	clients := &readyClientRepo{c: appclient.Client{ID: "client_demo", Status: "verified", VerifiedUntil: &until, SecretHash: "hash", Binding: appclient.Binding{ReleaseID: "release_demo", OrganizationID: "org_demo", AppID: "app_demo", Version: "0.1.0", Name: "Products", Digest: p.SHA256, Endpoint: m.UI.URL}}}
	gate := &assignmentGate{}
	source := UIResourceInstallSource{Assignments: gate, Releases: service.UIResourceReleases{Repo: repo, Key: key}, Clients: appclient.Service{Repo: clients}}
	var snapshot domain.IntentRelease
	read := func(merchant string) error {
		return source.WithRelease(context.Background(), merchant, "app_demo", "0.1.0", func(r domain.IntentRelease) error { snapshot = r; return nil })
	}
	if err := read("merchant_test"); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Scopes) != 1 || snapshot.Scopes[0] != "read_products" || snapshot.ManifestDigest != p.SHA256 || snapshot.UIBinding != nil {
		t.Fatal("wrong consent snapshot")
	}
	intent := domain.NewInstallIntent("intent_test", domain.IntentOwner{TenantID: "merchant_test", ServiceID: "core", ActorID: "seller"}, snapshot, time.Now())
	if intent.State != "pending" {
		t.Fatal("implicit consent")
	}
	if _, err := intent.Decide("wrong", "consent", time.Now()); err == nil {
		t.Fatal("wrong digest")
	}
	if _, err := intent.Decide(intent.ConsentDigest, "consent", time.Now()); err != nil {
		t.Fatal(err)
	}
	if read("other") == nil {
		t.Fatal("cross merchant")
	}
	gate.revoked = true
	if read("merchant_test") == nil {
		t.Fatal("revoked assignment")
	}
	gate.revoked = false
	clients.c.Status = "revoked"
	if read("merchant_test") == nil {
		t.Fatal("revoked client")
	}
	clients.c.Status = "verified"
	repo.release.Status = "suspended"
	if read("merchant_test") == nil {
		t.Fatal("suspended release")
	}
}
