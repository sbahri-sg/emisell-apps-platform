package service

import (
	"context"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/webhookconfig"
	"strings"
	"testing"
	"time"
)

type readyResourceTest struct{}

func (readyResourceTest) ReadyResource(context.Context, domain.IntentRelease) error { return nil }

func TestResourceConsentPolicyAndSnapshot(t *testing.T) {
	r := domain.IntentRelease{InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy, AppID: "app_resource", Version: "1.0.0", ManifestDigest: strings.Repeat("a", 64), Scopes: []string{"read_products"}, Capabilities: []string{}, ResourceBinding: &domain.ResourceBinding{ReleaseID: "release_1", ClientID: "client_1", AccessScopes: accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{"read_orders"}}, Webhooks: &webhookconfig.Config{APIVersion: webhookconfig.APIVersion, Subscriptions: []webhookconfig.Subscription{{Topics: []string{"products.created"}, URI: "https://example.com/events"}}}}}
	s := Lifecycle{Resources: readyResourceTest{}}
	ctx := context.WithValue(context.Background(), managedReleaseKey{}, r)
	if s.eligible(ctx, r) != nil {
		t.Fatal("valid configuration denied")
	}
	if s.eligible(context.Background(), r) == nil || (Lifecycle{}).eligible(ctx, r) == nil {
		t.Fatal("missing verifier/source accepted")
	}
	for _, mutate := range []func(*domain.IntentRelease){
		func(v *domain.IntentRelease) { v.Scopes = []string{"read_orders", "read_products"} },
		func(v *domain.IntentRelease) { v.Capabilities = []string{"shipping/v1"} },
		func(v *domain.IntentRelease) { v.ExecutionProfile = "local-remote" },
		func(v *domain.IntentRelease) { v.ResourceBinding.ClientID = "" },
		func(v *domain.IntentRelease) { v.ResourceBinding.AccessScopes.Required = []string{"shipping.read"} },
		func(v *domain.IntentRelease) { v.InstallPolicy = domain.InstallPolicy },
	} {
		v := domain.NewInstallIntent("i", domain.IntentOwner{}, r, time.Now()).Release
		mutate(&v)
		if s.eligible(context.WithValue(ctx, managedReleaseKey{}, v), v) == nil {
			t.Fatal("invalid binding accepted")
		}
	}
	i := domain.NewInstallIntent("i", domain.IntentOwner{TenantID: "m", ServiceID: "core", ActorID: "staff"}, r, time.Now())
	r.ResourceBinding.AccessScopes.Required[0] = "read_orders"
	r.ResourceBinding.Webhooks.Subscriptions[0].Topics[0] = "orders.created"
	if i.Release.ResourceBinding.AccessScopes.Required[0] != "read_products" || i.Release.ResourceBinding.Webhooks.Subscriptions[0].Topics[0] != "products.created" {
		t.Fatal("mutable consent snapshot")
	}
}
