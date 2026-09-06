package appmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
)

type Manifest struct {
	Schema           string                   `json:"schema"`
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Version          string                   `json:"version"`
	DeveloperID      string                   `json:"developerId"`
	Runtime          string                   `json:"runtime"`
	ExecutionProfile string                   `json:"executionProfile"`
	Compatibility    string                   `json:"compatibility"`
	ArtifactSHA256   string                   `json:"artifactSha256"`
	Category         string                   `json:"category"`
	Summary          string                   `json:"summary"`
	Description      string                   `json:"description"`
	Icon             string                   `json:"icon"`
	Color            string                   `json:"color"`
	Scopes           []string                 `json:"scopes"`
	Capabilities     []string                 `json:"capabilities"`
	Subscriptions    []string                 `json:"subscriptions,omitempty"`
	ShippingProvider *ShippingProviderBinding `json:"shippingProvider,omitempty"`
}

// This fixture trust root is PUBLIC test material. Never use it for publishers
// or remote artifacts. The local server refuses non-loopback operation.
func LocalFixtureKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("emisell-local-fixtures-v1-NOT-A-PRODUCTION-KEY"))
	return ed25519.NewKeyFromSeed(seed[:])
}
func ArtifactDigest() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte("emisell-local-simulator/v1")))
}
func RemoteArtifactDigest() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte("emisell-local-remote/v1")))
}

func VerifyLocal(raw []byte, signature string) (Manifest, error) {
	var m Manifest
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || !ed25519.Verify(LocalFixtureKey().Public().(ed25519.PublicKey), raw, sig) {
		return m, fmt.Errorf("invalid release signature")
	}
	if err = json.Unmarshal(raw, &m); err != nil {
		return m, err
	}
	if m.Schema != "emisell.app/v1" || m.Version != "1.0.0" || m.DeveloperID != "emisell-local" || m.Runtime != "Remote app" || m.Compatibility != "platform/v1" {
		return m, fmt.Errorf("unsupported or incompatible local release")
	}
	if m.ShippingProvider != nil && m.ExecutionProfile != ShippingProviderFixtureProfile {
		return m, fmt.Errorf("provider binding is not supported by this profile")
	}
	switch m.ExecutionProfile {
	case ShippingProviderFixtureProfile:
		if !ValidShippingProviderFixture(m) {
			return m, fmt.Errorf("invalid provider lifecycle fixture")
		}
		return m, nil
	case KurirFixtureProfile:
		if m.ID != KurirFixtureID || m.ArtifactSHA256 != KurirFixtureDigest() || len(m.Subscriptions) != 0 || !slices.Equal(m.Capabilities, []string{"shipping/v1"}) || !slices.Equal(m.Scopes, []string{"shipping.read"}) {
			return m, fmt.Errorf("invalid kurir reference profile")
		}
		return m, nil
	case "local-simulator":
		if m.ArtifactSHA256 != ArtifactDigest() || len(m.Subscriptions) != 0 {
			return m, fmt.Errorf("invalid simulator profile")
		}
	case "local-remote":
		if m.ID != "remote-pay" || m.ArtifactSHA256 != RemoteArtifactDigest() || !slices.Equal(m.Capabilities, []string{"payment/v1"}) || !slices.Equal(m.Subscriptions, []string{"emisell.capability.invoked.v1"}) {
			return m, fmt.Errorf("invalid remote fixture profile")
		}
	default:
		return m, fmt.Errorf("unsupported execution profile")
	}
	if len(m.Capabilities) != 1 || !slices.Contains([]string{"payment/v1", "shipping/v1"}, m.Capabilities[0]) {
		return m, fmt.Errorf("invalid capability")
	}
	expected := []string{"orders.read", "payments.read", "payments.write"}
	if m.Capabilities[0] == "shipping/v1" {
		expected = []string{"orders.read", "shipping.read", "shipping.write"}
	}
	if !slices.Equal(m.Scopes, expected) {
		return m, fmt.Errorf("unsupported scopes")
	}
	return m, nil
}
