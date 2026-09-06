// Package uirelease defines UI-only release attestations. Neither a signature
// nor testing eligibility is an installation, client proof or merchant grant.
package uirelease

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
)

const Schema = "emisell.ui-release/v1"
const Policy = "reviewed-ui/v1"

var ErrInvalid = errors.New("invalid UI release")
var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var version = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$`)
var hostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

// No business scopes/capabilities or provider credentials exist in v1. Adding
// them requires a new explicit contract, not dropping fields during decoding.
type Manifest struct {
	Schema      string `json:"schema"`
	Policy      string `json:"policy"`
	AppID       string `json:"appId"`
	DeveloperID string `json:"developerId"`
	Version     string `json:"version"`
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Mode        string `json:"mode"`
	URL         string `json:"url"`
	Pricing     string `json:"pricing"`
}

type Package struct {
	Manifest  Manifest `json:"manifest"`
	SHA256    string   `json:"sha256"`
	KeyID     string   `json:"keyId"`
	Signature []byte   `json:"signature"`
}

func (m Manifest) Validate() error {
	if m.Schema != Schema || m.Policy != Policy || !strings.HasPrefix(m.AppID, "app_") || !identifier.MatchString(m.AppID) || !identifier.MatchString(m.DeveloperID) || !version.MatchString(m.Version) ||
		strings.TrimSpace(m.Name) == "" || len(m.Name) > 100 || strings.TrimSpace(m.Summary) == "" || len(m.Summary) > 180 ||
		(m.Mode != "embedded" && m.Mode != "external") || m.Pricing != "free" || len(m.URL) > 2048 {
		return ErrInvalid
	}
	u, err := url.Parse(m.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(m.URL, "#") ||
		(u.Port() != "" && u.Port() != "443") || net.ParseIP(u.Hostname()) != nil || !hostname.MatchString(u.Hostname()) ||
		strings.HasSuffix(u.Hostname(), ".localhost") || strings.HasSuffix(u.Hostname(), ".local") {
		return ErrInvalid
	}
	return nil
}

// Decode rejects unknown fields so a business permission can never disappear
// silently when a UI-only submission is decoded. It performs no network I/O.
func Decode(raw []byte) (Manifest, error) {
	var m Manifest
	if len(raw) > 8192 {
		return m, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&m) != nil {
		return Manifest{}, ErrInvalid
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return Manifest{}, ErrInvalid
	}
	return m, m.Validate()
}
func Canonical(m Manifest) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}
func Digest(raw []byte) string           { v := sha256.Sum256(raw); return hex.EncodeToString(v[:]) }
func KeyID(key ed25519.PublicKey) string { return "ui-release-" + Digest(key) }
func Sign(m Manifest, key ed25519.PrivateKey) (Package, error) {
	raw, err := Canonical(m)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return Package{}, ErrInvalid
	}
	return Package{m, Digest(raw), KeyID(key.Public().(ed25519.PublicKey)), ed25519.Sign(key, append([]byte(Schema+"\x00"), raw...))}, nil
}
func Verify(p Package, key ed25519.PublicKey) error {
	raw, err := Canonical(p.Manifest)
	if err != nil || len(key) != ed25519.PublicKeySize || p.KeyID != KeyID(key) || p.SHA256 != Digest(raw) || !ed25519.Verify(key, append([]byte(Schema+"\x00"), raw...), p.Signature) {
		return ErrInvalid
	}
	return nil
}
