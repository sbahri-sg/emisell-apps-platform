package bootstrap

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/pkg/uirelease"
	"emisell.app/platform/pkg/uiresource"
	"testing"
)

type resourceBindingRepo struct {
	service.UIResourceReleaseRepository
	release service.UIResourceRelease
}

func (r *resourceBindingRepo) UIResourceGet(context.Context, string, string) (service.UIResourceRelease, error) {
	return r.release, nil
}
func (r *resourceBindingRepo) UIResourceWith(_ context.Context, _, _ string, fn func(service.UIResourceRelease) error) error {
	return fn(r.release)
}

func TestResourceClientBindsPermissionDigest(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	m := uiresource.Manifest{Schema: uiresource.Schema, Policy: uiresource.Policy, RequiredScopes: []string{uiresource.ReadProducts}, UI: uirelease.Manifest{Schema: uirelease.Schema, Policy: uirelease.Policy, AppID: "app_demo", DeveloperID: "org_demo", Version: "0.1.0", Name: "Demo", Summary: "Read only", Mode: "embedded", URL: "https://app.example.com/", Pricing: "free"}}
	pkg, err := uiresource.Sign(m, key)
	if err != nil {
		t.Fatal(err)
	}
	repo := &resourceBindingRepo{release: service.UIResourceRelease{ID: "release_demo", Manifest: m, Package: &pkg, Status: "signed"}}
	source := clientReleases{Resources: service.UIResourceReleases{Repo: repo, Key: key}}
	called := false
	err = source.WithBinding(context.Background(), "org_demo", "release_demo", func(b appclient.Binding) error {
		called = true
		inner, _ := uirelease.Canonical(m.UI)
		if b.Digest != pkg.SHA256 || b.Digest == uirelease.Digest(inner) || b.AppID != m.UI.AppID {
			t.Fatal("incorrect binding")
		}
		return nil
	})
	if err != nil || !called {
		t.Fatal(err)
	}
	repo.release.Status = "suspended"
	if source.WithBinding(context.Background(), "org_demo", "release_demo", func(appclient.Binding) error { t.Fatal("suspended source reached client"); return nil }) == nil {
		t.Fatal("suspended accepted")
	}
	if source.WithBinding(context.Background(), "org_demo", "release_demo", nil) == nil {
		t.Fatal("nil callback accepted")
	}
}
