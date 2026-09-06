// Package catalogmanifest defines signed, non-executable App Store metadata.
// This is NOT emisell.app/v1 and can never be loaded by an extension runtime.
package catalogmanifest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"emisell.app/platform/pkg/accessscope"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

const Schema = "emisell.catalog/v1"
const Policy = "catalog-metadata/v1"
const SchemaAccessScopes = "emisell.catalog/v2"
const PolicyAccessScopes = "catalog-metadata/v2"

type Manifest struct {
	Schema       string                   `json:"schema"`
	Policy       string                   `json:"policy"`
	AppID        string                   `json:"appId"`
	DeveloperID  string                   `json:"developerId"`
	Version      string                   `json:"version"`
	Name         string                   `json:"name"`
	Summary      string                   `json:"summary"`
	Description  string                   `json:"description"`
	Capability   string                   `json:"capability"`
	Scopes       []string                 `json:"scopes"`
	Runtime      string                   `json:"runtime"`
	Pricing      string                   `json:"pricing"`
	Installable  bool                     `json:"installable"`
	SourceSHA256 string                   `json:"sourceSha256"`
	AccessScopes *accessscope.Declaration `json:"accessScopes,omitempty"`
}
type Package struct {
	Manifest  Manifest `json:"manifest"`
	SHA256    string   `json:"sha256"`
	KeyID     string   `json:"keyId"`
	Signature []byte   `json:"signature"`
}

var semver = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$`)
var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func textValid(s string, max int) bool {
	if strings.TrimSpace(s) == "" || len(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}

// Validate is a strict metadata policy check, not an app-code/security scan.
func (m Manifest) Validate() error {
	invalid := errors.New("invalid catalog manifest")
	if m.AccessScopes == nil {
		if m.Schema != Schema || m.Policy != Policy {
			return invalid
		}
	} else {
		if m.Schema != SchemaAccessScopes || m.Policy != PolicyAccessScopes || m.AccessScopes.Validate() != nil || !slices.IsSorted(m.AccessScopes.Required) || !slices.IsSorted(m.AccessScopes.Optional) {
			return invalid
		}
	}
	if !identifier.MatchString(m.AppID) || !identifier.MatchString(m.DeveloperID) || !semver.MatchString(m.Version) || !textValid(m.Name, 100) || !textValid(m.Summary, 180) || !textValid(m.Description, 5000) || m.Runtime != "remote" || m.Pricing != "free" || m.Installable {
		return invalid
	}
	digest, err := hex.DecodeString(m.SourceSHA256)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != m.SourceSHA256 {
		return invalid
	}
	scopes := []string{"orders.read", "payments.read", "payments.write"}
	if m.Capability == "shipping/v1" {
		scopes = []string{"orders.read", "shipping.read", "shipping.write"}
	} else if m.Capability != "payment/v1" {
		return invalid
	}
	if !slices.Equal(m.Scopes, scopes) {
		return invalid
	}
	return nil
}
func Canonical(m Manifest) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}
func Digest(raw []byte) string              { return hex.EncodeToString(sum(raw)) }
func sum(raw []byte) []byte                 { h := sha256.Sum256(raw); return h[:] }
func KeyID(public ed25519.PublicKey) string { return "catalog-" + Digest(public) }
func Sign(m Manifest, key ed25519.PrivateKey) (Package, error) {
	raw, err := Canonical(m)
	if err != nil {
		return Package{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return Package{}, errors.New("catalog signer unavailable")
	}
	return Package{Manifest: m, SHA256: Digest(raw), KeyID: KeyID(key.Public().(ed25519.PublicKey)), Signature: ed25519.Sign(key, raw)}, nil
}
func Verify(p Package, public ed25519.PublicKey) error {
	raw, err := Canonical(p.Manifest)
	if err != nil {
		return err
	}
	if len(public) != ed25519.PublicKeySize || p.KeyID != KeyID(public) || p.SHA256 != Digest(raw) || !ed25519.Verify(public, raw, p.Signature) {
		return errors.New("catalog integrity check failed")
	}
	return nil
}

// Decode accepts equivalent JSON formatting/escaping from browser exports,
// rejecting unknown/duplicate fields and trailing data before canonicalization.
func Decode(raw []byte) (Manifest, error) {
	var m Manifest
	if len(raw) > 16<<10 {
		return m, errors.New("catalog manifest too large")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return m, errors.New("trailing catalog data")
	}
	if err := uniqueJSON(raw); err != nil {
		return m, err
	}
	return m, m.Validate()
}

func DecodePackage(raw []byte) (Package, error) {
	var p Package
	if len(raw) > 32<<10 {
		return p, errors.New("catalog package too large")
	}
	if err := uniqueJSON(raw); err != nil {
		return p, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, err
	}
	return p, p.Manifest.Validate()
}
func uniqueJSON(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var value func(int) error
	value = func(depth int) error {
		if depth > 8 {
			return errors.New("catalog JSON too deeply nested")
		}
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					name, err := dec.Token()
					if err != nil {
						return err
					}
					key, ok := name.(string)
					if !ok || seen[key] {
						return errors.New("duplicate catalog field")
					}
					seen[key] = true
					if err = value(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for dec.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("invalid catalog JSON")
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing catalog data")
	}
	return nil
}
