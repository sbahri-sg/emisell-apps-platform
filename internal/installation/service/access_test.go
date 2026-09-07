package service

import (
	"context"
	"testing"

	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/pkg/appmanifest"
)

type accessRegistry struct{ manifest appmanifest.Manifest }

func TestProviderAppEligibilityUsesSignedSourceAndSeparateRouting(t *testing.T) {
	r := domain.IntentRelease{InstallPolicy: domain.ProviderAppPolicy, AppID: "app_raja", Version: "1.0.0", ExecutionProfile: domain.ProviderAppProfile, Scopes: []string{"shipping.read", "shipping.write"}, Capabilities: []string{"shipping/v1"}, ShippingProvider: &appmanifest.ShippingProviderBinding{Engine: "api-kurir", ProviderCode: "rajaongkir"}, ManagedSource: &domain.ManagedSource{ReleaseID: "r", AssignmentID: "a", MerchantID: "m", Environment: "staging"}}
	s := Lifecycle{}
	ctx := context.WithValue(context.Background(), managedReleaseKey{}, r)
	if err := s.eligible(ctx, r); err != nil {
		t.Fatal(err)
	}
	if len(r.RoutedCapabilities()) != 0 {
		t.Fatal("provider installation changed checkout route")
	}
	r.Scopes = []string{"orders.write"}
	ctx = context.WithValue(context.Background(), managedReleaseKey{}, r)
	if err := s.eligible(ctx, r); err == nil {
		t.Fatal("foreign scope allowed")
	}
	r.Scopes = []string{"shipping.read"}
	r.ManagedSource = nil
	ctx = context.WithValue(context.Background(), managedReleaseKey{}, r)
	if err := s.eligible(ctx, r); err == nil {
		t.Fatal("missing provenance allowed")
	}
}

func (r accessRegistry) Get(context.Context, string) (appmanifest.Manifest, error) {
	return r.manifest, nil
}
func (r accessRegistry) List(context.Context) ([]appmanifest.Manifest, error) {
	return []appmanifest.Manifest{r.manifest}, nil
}

func TestFixtureGrantPolicyDeniesPlannedAndForeignScopes(t *testing.T) {
	ctx := context.Background()
	base := appmanifest.Manifest{ID: "fixture", Schema: "emisell.app/v1", Version: "1.0.0", ExecutionProfile: "local-simulator", Scopes: []string{"orders.read", "payments.read", "payments.write"}, Capabilities: []string{"payment/v1"}}
	for _, tc := range []struct {
		name            string
		scopes, caps    []string
		schema, profile string
		allowed         bool
	}{
		{"payment", base.Scopes, base.Capabilities, base.Schema, base.ExecutionProfile, true},
		{"shipping", []string{"orders.read", "shipping.read", "shipping.write"}, []string{"shipping/v1"}, base.Schema, base.ExecutionProfile, true},
		{"planned", []string{"read_products"}, base.Capabilities, base.Schema, base.ExecutionProfile, false},
		{"mixed", []string{"orders.read", "payments.read", "payments.write", "read_products"}, base.Capabilities, base.Schema, base.ExecutionProfile, false},
		{"foreign", []string{"*"}, base.Capabilities, base.Schema, base.ExecutionProfile, false},
		{"capability", base.Scopes, []string{"products/v1"}, base.Schema, base.ExecutionProfile, false},
		{"catalog", base.Scopes, base.Capabilities, "emisell.catalog/v2", base.ExecutionProfile, false},
		{"runtime", base.Scopes, base.Capabilities, base.Schema, "arbitrary-node", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			m.Scopes, m.Capabilities, m.Schema, m.ExecutionProfile = tc.scopes, tc.caps, tc.schema, tc.profile
			s := Lifecycle{Intents: Intents{Apps: accessRegistry{m}}}
			r, err := s.Intents.release(ctx, m.ID, m.Version)
			if err == nil {
				err = s.eligible(ctx, r)
			}
			if (err == nil) != tc.allowed {
				t.Fatal("eligibility mismatch", err)
			}
			if err == nil {
				r.InstallPolicy = ""
				if s.eligible(ctx, r) == nil {
					t.Fatal("historical consent accepted")
				}
				r.InstallPolicy = domain.InstallPolicy
				r.ManifestDigest = "changed"
				if s.eligible(ctx, r) == nil {
					t.Fatal("changed digest accepted")
				}
			}
		})
	}
}

func TestProviderConsentCannotBeReboundOrShareMutableRegistryState(t *testing.T) {
	m, _ := appmanifest.ShippingProviderFixture("rajaongkir")
	s := Lifecycle{Intents: Intents{Apps: accessRegistry{m}}}
	r, err := s.Intents.release(context.Background(), m.ID, m.Version)
	if err != nil || s.eligible(context.Background(), r) != nil {
		t.Fatal("valid provider fixture rejected", err)
	}
	if r.ShippingProvider == m.ShippingProvider {
		t.Fatal("snapshot retains mutable registry binding")
	}
	r.ShippingProvider.ProviderCode = "kiriminaja"
	if s.eligible(context.Background(), r) == nil {
		t.Fatal("consent accepted a different provider")
	}
	if m.ShippingProvider.ProviderCode != "rajaongkir" {
		t.Fatal("snapshot mutation changed registry")
	}
}
