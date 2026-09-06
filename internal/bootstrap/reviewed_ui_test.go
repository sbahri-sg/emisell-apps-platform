package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/embedded"
)

// Controlled current-review source, NOT a runtime adapter or catalog bypass.
type reviewedSource struct {
	merchant string
	release  domain.IntentRelease
	denied   bool
}

func (s *reviewedSource) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if s.denied || merchant != s.merchant || app != s.release.AppID || version != s.release.Version {
		return fault.Forbidden
	}
	return fn(s.release)
}

func TestReviewedUIPersistentLifecycle(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, principal, _, _ := lifecycleClient(t, f)
	pub, keyPair, _ := ed25519.GenerateKey(rand.Reader)
	launch := embedded.Launch{AppID: "app_ui_test", ClientID: "client_ui_test", ReleaseDigest: strings.Repeat("a", 64), URL: "https://ui.example/app", ParentOrigin: "https://core.example", Mode: "embedded"}
	sig, err := embedded.SignLaunch(launch, keyPair, false)
	if err != nil {
		t.Fatal(err)
	}
	source := &reviewedSource{merchant: f.tenant, release: domain.IntentRelease{AppID: launch.AppID, Name: "UI test", DeveloperID: "test", Version: "1.0.0", InstallPolicy: domain.ReviewedUIPolicy, ExecutionProfile: domain.ReviewedUIPolicy, ManifestDigest: launch.ReleaseDigest, Scopes: []string{}, Capabilities: []string{}, UIBinding: &domain.UIBinding{ReleaseID: "release_ui", Launch: launch, Signature: sig}}}
	intents := service.Intents{Repo: installrepo.Repository{Pool: f.pool}, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, Managed: source}
	lifecycle := service.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}, Intents: intents, ReviewedUIKey: pub}
	intent, err := intents.Prepare(ctx, principal, "staff", key(), service.PrepareIntent{AppID: launch.AppID, Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.Consume(ctx, principal, "staff", key(), intent.ID, intent.ConsentDigest); err == nil {
		t.Fatal("consent bypass")
	}
	_, err = intents.Decide(ctx, principal, "staff", key(), service.DecideIntent{ID: intent.ID, Digest: intent.ConsentDigest, Decision: "consent"})
	if err != nil {
		t.Fatal(err)
	}
	receiptKey := key()
	result, err := lifecycle.Consume(ctx, principal, "staff", receiptKey, intent.ID, intent.ConsentDigest)
	if err != nil {
		t.Fatal(err)
	}
	again, err := lifecycle.Consume(ctx, principal, "staff", receiptKey, intent.ID, intent.ConsentDigest)
	if err != nil || again.Access.Installation.ID != result.Access.Installation.ID {
		t.Fatal("retry", err)
	}
	id := result.Access.Installation.ID
	calls := 0
	check := func(actor string) error {
		return lifecycle.WithReviewedUIAccess(ctx, principal, actor, id, func(b domain.UIBinding) error {
			calls++
			if b.Launch != launch {
				t.Fatal("binding changed")
			}
			return nil
		})
	}
	if check("staff") == nil {
		t.Fatal("pending access")
	}
	if _, err = lifecycle.Execute(ctx, principal, "staff", key(), id, "activate"); err != nil {
		t.Fatal(err)
	}
	if err = check("staff"); err != nil || calls != 1 {
		t.Fatal("active access", err)
	}
	if check("other-staff") == nil {
		t.Fatal("actor leak")
	}
	other := principal
	other.TenantID = "other-merchant"
	if lifecycle.WithReviewedUIAccess(ctx, other, "staff", id, func(domain.UIBinding) error { t.Fatal("merchant leak"); return nil }) == nil {
		t.Fatal("merchant accepted")
	}
	if _, err = lifecycle.Execute(ctx, principal, "staff", key(), id, "issue_token"); err == nil {
		t.Fatal("business token issued")
	}
	list, _, err := lifecycle.List(ctx, principal, "staff", "", 20)
	if err != nil || len(list) != 1 {
		t.Fatal("list", err)
	}
	source.denied = true
	if check("staff") == nil {
		t.Fatal("revoked source allowed")
	}
	if _, err = lifecycle.Execute(ctx, principal, "staff", key(), id, "uninstall"); err != nil {
		t.Fatal("uninstall requires source", err)
	}
	source.denied = false
	if check("staff") == nil {
		t.Fatal("uninstalled allowed")
	}
}
