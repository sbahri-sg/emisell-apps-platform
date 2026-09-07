package managedshipping

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

func manifest() Manifest {
	return Manifest{Schema: Schema, Policy: Policy, AppID: "app_demo", DeveloperID: "org_demo", Version: "1.0.0", Name: "Emisell Kurir", Summary: "Managed provider", Description: "Read-only shipping test", DraftRevision: 1, SourceSHA256: strings.Repeat("a", 64), Capability: "shipping/v1", Scopes: []string{"shipping.read"}, Binding: Binding{"api-kurir", "emisell"}, Pricing: "free"}
}
func TestProviderAppV2IsSeparateAndSigned(t *testing.T) {
	public, key, _ := ed25519.GenerateKey(rand.Reader)
	for _, provider := range []string{"rajaongkir", "other_provider"} {
		m := manifest()
		m.Policy = ProviderAppPolicy
		m.Binding.ProviderCode = provider
		m.Scopes = []string{"shipping.read", "shipping.write"}
		p, err := Sign(m, key)
		if err != nil || Verify(p, public) != nil {
			t.Fatal(err)
		}
		p.Manifest.Policy = Policy
		if Verify(p, public) == nil {
			t.Fatal("policy downgrade accepted")
		}
		m.Scopes = append(m.Scopes, "orders.write")
		if m.Validate() == nil {
			t.Fatal("foreign scope accepted")
		}
	}
}
func TestSignVerifyManagedProvider(t *testing.T) {
	public, key, _ := ed25519.GenerateKey(rand.Reader)
	m := manifest()
	p, err := Sign(m, key)
	if err != nil || Verify(p, public) != nil {
		t.Fatal(err)
	}
	m.Scopes[0] = "shipping.write"
	if Verify(p, public) != nil {
		t.Fatal("caller mutated signed package")
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if Verify(p, other) == nil {
		t.Fatal("foreign trust root")
	}
	p.Manifest.AppID = "app_other"
	if Verify(p, public) == nil {
		t.Fatal("app identity not signed")
	}
}
func TestManagedManifestClosedPolicy(t *testing.T) {
	for name, change := range map[string]func(*Manifest){
		"engine":         func(m *Manifest) { m.Binding.Engine = "https://arbitrary.invalid" },
		"provider":       func(m *Manifest) { m.Binding.ProviderCode = "rajaongkir" },
		"payment":        func(m *Manifest) { m.Capability = "payment/v1" },
		"extra_scope":    func(m *Manifest) { m.Scopes = append(m.Scopes, "shipping.write") },
		"resource_scope": func(m *Manifest) { m.Scopes = []string{"read_shipping"} },
		"paid":           func(m *Manifest) { m.Pricing = "paid" },
		"fixture_policy": func(m *Manifest) { m.Policy = "local-kurir-provider" },
		"source":         func(m *Manifest) { m.SourceSHA256 = "" },
		"owner":          func(m *Manifest) { m.DeveloperID = "" },
		"revision":       func(m *Manifest) { m.DraftRevision = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			m := manifest()
			change(&m)
			if m.Validate() == nil {
				t.Fatal("invalid release accepted")
			}
		})
	}
}
