package appmanifest

import (
	"crypto/sha256"
	"fmt"
	"slices"
)

// ShippingProviderBinding belongs to the signed release, never a browser input.
// API-Kurir is the shared engine; provider apps have independent installations.
type ShippingProviderBinding struct {
	Engine       string `json:"engine"`
	ProviderCode string `json:"providerCode"`
}

const ShippingProviderFixtureProfile = "local-kurir-provider"

// ShippingProviderFixture is a closed lifecycle conformance fixture. It does not
// enable checkout execution, provision credentials, or publish a catalog app.
func ShippingProviderFixture(provider string) (Manifest, error) {
	name := map[string]string{"emisell": "Emisell Kurir", "rajaongkir": "RajaOngkir", "kiriminaja": "KiriminAja", "mengantar": "Mengantar"}[provider]
	if name == "" {
		return Manifest{}, fmt.Errorf("unsupported provider fixture")
	}
	return Manifest{
		Schema: "emisell.app/v1", ID: provider + "-provider-reference", Name: name + " · Lifecycle reference",
		Version: "1.0.0", DeveloperID: "emisell-local", Runtime: "Remote app", Compatibility: "platform/v1",
		ExecutionProfile: ShippingProviderFixtureProfile,
		ArtifactSHA256:   fmt.Sprintf("%x", sha256.Sum256([]byte("emisell-provider-lifecycle/v1:api-kurir:"+provider))),
		Category:         "Pengiriman", Icon: "shipping", Color: "blue",
		Summary:     "Referensi lifecycle aplikasi provider melalui engine API-Kurir.",
		Description: "Pengujian lokal saja. Tidak mengubah credential, provider checkout, atau pengiriman nyata.",
		Scopes:      []string{"shipping.read"}, Capabilities: []string{"shipping/v1"},
		ShippingProvider: &ShippingProviderBinding{Engine: "api-kurir", ProviderCode: provider},
	}, nil
}

func ValidShippingProviderFixture(m Manifest) bool {
	if m.ShippingProvider == nil || m.ShippingProvider.Engine != "api-kurir" {
		return false
	}
	expected, err := ShippingProviderFixture(m.ShippingProvider.ProviderCode)
	return err == nil && m.ID == expected.ID && m.Version == expected.Version &&
		m.ExecutionProfile == expected.ExecutionProfile && m.ArtifactSHA256 == expected.ArtifactSHA256 &&
		slices.Equal(m.Scopes, expected.Scopes) && slices.Equal(m.Capabilities, expected.Capabilities) && len(m.Subscriptions) == 0
}
