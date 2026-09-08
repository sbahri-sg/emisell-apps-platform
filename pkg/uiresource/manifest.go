// Package uiresource defines an explicit UI-with-resource release contract.
// It does not authorize an installation or replace seller consent.
package uiresource

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"

	"emisell.app/platform/pkg/uirelease"
)

const Schema = "emisell.ui-resource-release/v1"
const Policy = "reviewed-ui-resource/v1"
const ReadProducts = "read_products"
const ReadOrders = "read_orders"
const ReadShipping = "read_shipping"
const ReadCatalogs = "read_catalogs"
const ReadCollections = "read_collections"
const ReadInventory = "read_inventory"
const ReadLocations = "read_locations"

// Signed releases use a non-empty canonical subset, never optional/implicit grants.
func ValidScopes(scopes []string) bool {
	if len(scopes) < 1 || len(scopes) > 7 {
		return false
	}
	for i, scope := range scopes {
		if scope != ReadProducts && scope != ReadOrders && scope != ReadShipping && scope != ReadCatalogs && scope != ReadCollections && scope != ReadInventory && scope != ReadLocations {
			return false
		}
		if i > 0 && scopes[i-1] >= scope {
			return false
		}
	}
	return true
}

var ErrInvalid = errors.New("invalid UI resource release")

// Composition deliberately leaves the existing UI-only contract unchanged.
// The outer signature binds UI metadata and permissions together.
type Manifest struct {
	Schema         string             `json:"schema"`
	Policy         string             `json:"policy"`
	UI             uirelease.Manifest `json:"ui"`
	RequiredScopes []string           `json:"requiredScopes"`
}

type Package struct {
	Manifest  Manifest `json:"manifest"`
	SHA256    string   `json:"sha256"`
	KeyID     string   `json:"keyId"`
	Signature []byte   `json:"signature"`
}

func (m Manifest) Validate() error {
	if m.Schema != Schema || m.Policy != Policy || m.UI.Validate() != nil ||
		!ValidScopes(m.RequiredScopes) {
		return ErrInvalid
	}
	return nil
}

// Decode rejects unknown and duplicate JSON keys, including nested metadata.
func Decode(raw []byte) (Manifest, error) {
	if len(raw) > 16384 || !uniqueJSON(raw) {
		return Manifest{}, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var m Manifest
	if d.Decode(&m) != nil {
		return Manifest{}, ErrInvalid
	}
	if d.Decode(new(any)) != io.EOF || m.Validate() != nil {
		return Manifest{}, ErrInvalid
	}
	return m, nil
}

func uniqueJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return ErrInvalid
				}
				seen[s] = true
				if err = value(); err != nil {
					return err
				}
			}
			token, err = d.Token()
			if err != nil || token != json.Delim('}') {
				return ErrInvalid
			}
		case json.Delim('['):
			for d.More() {
				if err = value(); err != nil {
					return err
				}
			}
			token, err = d.Token()
			if err != nil || token != json.Delim(']') {
				return ErrInvalid
			}
		}
		return nil
	}
	if value() != nil {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}

func Canonical(m Manifest) ([]byte, error) {
	if m.Validate() != nil {
		return nil, ErrInvalid
	}
	return json.Marshal(m)
}
func KeyID(key ed25519.PublicKey) string { return "ui-resource-" + uirelease.Digest(key) }
func Sign(m Manifest, key ed25519.PrivateKey) (Package, error) {
	raw, err := Canonical(m)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return Package{}, ErrInvalid
	}
	// Own the scope slice so caller mutation cannot change the returned package.
	m.RequiredScopes = append([]string(nil), m.RequiredScopes...)
	return Package{m, uirelease.Digest(raw), KeyID(key.Public().(ed25519.PublicKey)), ed25519.Sign(key, append([]byte(Schema+"\x00"), raw...))}, nil
}
func Verify(p Package, key ed25519.PublicKey) error {
	raw, err := Canonical(p.Manifest)
	if err != nil || len(key) != ed25519.PublicKeySize || p.KeyID != KeyID(key) || p.SHA256 != uirelease.Digest(raw) || !ed25519.Verify(key, append([]byte(Schema+"\x00"), raw...), p.Signature) {
		return ErrInvalid
	}
	return nil
}
