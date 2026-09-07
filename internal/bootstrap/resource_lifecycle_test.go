package bootstrap_test

import (
	"context"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/webhookconfig"
	"strings"
	"testing"
)

// Explicit isolated-test verifier. Production composition must supply actual
// release/client/runtime verification; this is never installed by bootstrap.
type resourceTestReadiness struct{ denied bool }

func (s *resourceTestReadiness) ReadyResource(context.Context, domain.IntentRelease) error {
	if s.denied {
		return fault.Unavailable
	}
	return nil
}

func TestResourceConsentPersistentLifecycle(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, p, _, _ := lifecycleClient(t, f)
	b := &domain.ResourceBinding{ReleaseID: "release_resource", ClientID: "client_resource", AccessScopes: accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{"read_orders"}}, Webhooks: &webhookconfig.Config{APIVersion: webhookconfig.APIVersion, Subscriptions: []webhookconfig.Subscription{{Topics: []string{"products.created"}, URI: "https://example.com/events"}}}}
	source := &reviewedSource{merchant: f.tenant, release: domain.IntentRelease{AppID: "app_resource_" + key(), Name: "Resource test", DeveloperID: "developer_test", Version: "1.0.0", InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy, ManifestDigest: strings.Repeat("a", 64), Scopes: []string{"read_products"}, Capabilities: []string{}, ResourceBinding: b}}
	intents := service.Intents{Repo: installrepo.Repository{Pool: f.pool}, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, Managed: source}
	readiness := &resourceTestReadiness{}
	lifecycle := service.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}, Intents: intents, Resources: readiness}
	intent, err := intents.Prepare(ctx, p, "staff", key(), service.PrepareIntent{AppID: source.release.AppID, Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.Consume(ctx, p, "staff", key(), intent.ID, intent.ConsentDigest); err == nil {
		t.Fatal("consent bypass")
	}
	if _, err = intents.Decide(ctx, p, "staff", key(), service.DecideIntent{ID: intent.ID, Digest: intent.ConsentDigest, Decision: "consent"}); err != nil {
		t.Fatal(err)
	}
	lifecycle.Resources = nil
	if _, err = lifecycle.Consume(ctx, p, "staff", key(), intent.ID, intent.ConsentDigest); err == nil {
		t.Fatal("default production path granted access")
	}
	lifecycle.Resources = readiness
	consumed, err := lifecycle.Consume(ctx, p, "staff", key(), intent.ID, intent.ConsentDigest)
	if err != nil {
		t.Fatal(err)
	}
	id := consumed.Access.Installation.ID
	if len(consumed.Access.GrantedScopes) != 0 {
		t.Fatal("pending installation granted scopes")
	}
	active, err := lifecycle.Execute(ctx, p, "staff", key(), id, "activate")
	if err != nil {
		t.Fatal(err)
	}
	if len(active.Access.GrantedScopes) != 1 || active.Access.GrantedScopes[0] != "read_products" {
		t.Fatal("optional scope granted")
	}
	if _, err = lifecycle.Execute(ctx, p, "staff", key(), id, "issue_token"); err == nil {
		t.Fatal("fixture token issued for resource app")
	}
	calls := 0
	check := func(a domain.Access) error {
		calls++
		if a.Release.ResourceBinding.Webhooks == nil {
			t.Fatal("binding lost")
		}
		return nil
	}
	if err = lifecycle.WithResourceAccess(ctx, p, "staff", id, []string{"read_products"}, check); err != nil || calls != 1 {
		t.Fatal("required access", err)
	}
	if err = lifecycle.WithResourceAccess(ctx, p, "staff", id, []string{"read_orders"}, check); err == nil {
		t.Fatal("optional access allowed")
	}
	if err = lifecycle.WithResourceAccess(ctx, p, "other-staff", id, []string{"read_products"}, check); err == nil {
		t.Fatal("owner bypass")
	}
	source.release.ManifestDigest = strings.Repeat("b", 64)
	if err = lifecycle.WithResourceAccess(ctx, p, "staff", id, []string{"read_products"}, check); err == nil {
		t.Fatal("changed release accepted")
	}
	source.release.ManifestDigest = strings.Repeat("a", 64)
	readiness.denied = true
	if err = lifecycle.WithResourceAccess(ctx, p, "staff", id, []string{"read_products"}, check); err == nil {
		t.Fatal("unavailable runtime accepted")
	}
	source.denied = true
	if _, err = lifecycle.Execute(ctx, p, "staff", key(), id, "uninstall"); err != nil {
		t.Fatal("uninstall required available source", err)
	}
	source.denied = false
	readiness.denied = false
	if err = lifecycle.WithResourceAccess(ctx, p, "staff", id, []string{"read_products"}, check); err == nil || calls != 1 {
		t.Fatal("revoked access accepted", err)
	}
}
