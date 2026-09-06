package appmanifest

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func verifyProviderManifest(m Manifest) error {
	raw, _ := json.Marshal(m)
	_, err := VerifyLocal(raw, base64.StdEncoding.EncodeToString(ed25519.Sign(LocalFixtureKey(), raw)))
	return err
}

func TestShippingProviderBindingIsClosedSignedAndIndependent(t *testing.T) {
	ids, digests := map[string]bool{}, map[string]bool{}
	for _, p := range []string{"emisell", "rajaongkir", "kiriminaja", "mengantar"} {
		m, err := ShippingProviderFixture(p)
		if err != nil || verifyProviderManifest(m) != nil || ids[m.ID] || digests[m.ArtifactSHA256] {
			t.Fatal("provider fixtures must have valid distinct identities/digests", p, err)
		}
		ids[m.ID], digests[m.ArtifactSHA256] = true, true
		for name, change := range map[string]func(*Manifest){
			"engine":       func(m *Manifest) { m.ShippingProvider.Engine = "other" },
			"provider":     func(m *Manifest) { m.ShippingProvider.ProviderCode = "unknown" },
			"missing":      func(m *Manifest) { m.ShippingProvider = nil },
			"app":          func(m *Manifest) { m.ID = "other-provider-reference" },
			"legacy":       func(m *Manifest) { m.ExecutionProfile = "local-simulator"; m.ArtifactSHA256 = ArtifactDigest() },
			"scopes":       func(m *Manifest) { m.Scopes = []string{"shipping.read", "shipping.write"} },
			"resource":     func(m *Manifest) { m.Scopes = []string{"read_shipping"} },
			"capability":   func(m *Manifest) { m.Capabilities = []string{"payment/v1"} },
			"digest":       func(m *Manifest) { m.ArtifactSHA256 = KurirFixtureDigest() },
			"subscription": func(m *Manifest) { m.Subscriptions = []string{"order.created"} },
		} {
			t.Run(p+"/"+name, func(t *testing.T) {
				v, _ := ShippingProviderFixture(p)
				change(&v)
				if verifyProviderManifest(v) == nil {
					t.Fatal("accepted mutated provider release")
				}
			})
		}
	}
	for _, p := range []string{"", "API-Kurir", "../rajaongkir", "rajaongkir?key=secret", "RajaOngkir", "biteship", "emisell-kurir"} {
		if _, err := ShippingProviderFixture(p); err == nil {
			t.Fatal("accepted unsupported provider", p)
		}
	}
	legacy := KurirFixture()
	raw, _ := json.Marshal(legacy)
	if strings.Contains(string(raw), "shippingProvider") || verifyProviderManifest(legacy) != nil {
		t.Fatal("historical aggregator fixture changed")
	}
	m, _ := ShippingProviderFixture("rajaongkir")
	raw, _ = json.Marshal(m)
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(LocalFixtureKey(), raw))
	if _, err := VerifyLocal([]byte(strings.ReplaceAll(string(raw), "rajaongkir", "kiriminaja")), signature); err == nil {
		t.Fatal("provider substitution did not invalidate signature")
	}
}
