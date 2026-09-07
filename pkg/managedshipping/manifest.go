// Package managedshipping defines operator-reviewed provider releases. Signing
// attests the immutable binding, never merchant consent or engine readiness.
package managedshipping

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
)

const Schema = "emisell.managed-shipping-release/v1"
const Policy = "managed-kurir-provider/v1"

// ProviderAppPolicy does not change the legacy built-in pilot contract.
const ProviderAppPolicy = "managed-kurir-provider/v2"

type Binding struct {
	Engine       string `json:"engine"`
	ProviderCode string `json:"providerCode"`
}
type Manifest struct {
	Schema        string   `json:"schema"`
	Policy        string   `json:"policy"`
	AppID         string   `json:"appId"`
	DeveloperID   string   `json:"developerId"`
	Version       string   `json:"version"`
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	DraftRevision int      `json:"draftRevision"`
	SourceSHA256  string   `json:"sourceSha256"`
	Capability    string   `json:"capability"`
	Scopes        []string `json:"scopes"`
	Binding       Binding  `json:"binding"`
	Pricing       string   `json:"pricing"`
}
type Package struct {
	Manifest  Manifest `json:"manifest"`
	SHA256    string   `json:"sha256"`
	KeyID     string   `json:"keyId"`
	Signature []byte   `json:"signature"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var version = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (m Manifest) Validate() error {
	// Emisell built-in is the only reviewed engine binding for this first pilot.
	// Adding another provider requires its own readiness/credential policy.
	if m.Schema != Schema || (m.Policy != Policy && m.Policy != ProviderAppPolicy) || !identifier.MatchString(m.AppID) ||
		!identifier.MatchString(m.DeveloperID) || !version.MatchString(m.Version) || m.DraftRevision < 1 ||
		!digest.MatchString(m.SourceSHA256) || strings.TrimSpace(m.Name) == "" || len(m.Name) > 100 ||
		strings.TrimSpace(m.Summary) == "" || len(m.Summary) > 180 || strings.TrimSpace(m.Description) == "" || len(m.Description) > 5000 ||
		m.Capability != "shipping/v1" || m.Pricing != "free" {
		return errors.New("invalid managed shipping release")
	}
	if m.Policy == Policy {
		if !slices.Equal(m.Scopes, []string{"shipping.read"}) || m.Binding != (Binding{Engine: "api-kurir", ProviderCode: "emisell"}) {
			return errors.New("invalid legacy shipping binding")
		}
	} else if m.Binding.Engine != "api-kurir" || !regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`).MatchString(m.Binding.ProviderCode) ||
		(!slices.Equal(m.Scopes, []string{"shipping.read"}) && !slices.Equal(m.Scopes, []string{"shipping.read", "shipping.write"})) {
		return errors.New("invalid provider app binding or scopes")
	}
	return nil
}
func Canonical(m Manifest) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}
func Digest(raw []byte) string              { v := sha256.Sum256(raw); return hex.EncodeToString(v[:]) }
func KeyID(public ed25519.PublicKey) string { return "managed-shipping-" + Digest(public) }
func Sign(m Manifest, key ed25519.PrivateKey) (Package, error) {
	raw, err := Canonical(m)
	if err != nil {
		return Package{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return Package{}, errors.New("managed shipping signer unavailable")
	}
	// Detach mutable slices so caller edits cannot alter the returned attestation.
	m.Scopes = slices.Clone(m.Scopes)
	return Package{m, Digest(raw), KeyID(key.Public().(ed25519.PublicKey)), ed25519.Sign(key, append([]byte(m.Policy+"\n"), raw...))}, nil
}
func Verify(p Package, key ed25519.PublicKey) error {
	raw, err := Canonical(p.Manifest)
	if err != nil {
		return err
	}
	if len(key) != ed25519.PublicKeySize || p.KeyID != KeyID(key) || p.SHA256 != Digest(raw) ||
		!ed25519.Verify(key, append([]byte(p.Manifest.Policy+"\n"), raw...), p.Signature) {
		return errors.New("invalid managed shipping signature")
	}
	return nil
}
