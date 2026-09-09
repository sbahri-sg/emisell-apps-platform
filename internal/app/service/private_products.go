package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/accessscope"
	"encoding/hex"
	"encoding/json"
	"slices"
)

const PrivateProducts = "private-products/v1"

// Explicit headless policy: no executable URL, business capabilities or webhooks.
// Signing authenticates the configuration, not a public marketplace review.
type PrivateProductVersion struct {
	ID             string      `json:"id"`
	AppID          string      `json:"appId"`
	OrganizationID string      `json:"organizationId"`
	OwnerAccountID string      `json:"ownerAccountId"`
	ClientID       string      `json:"clientId"`
	Document       AppDocument `json:"document"`
}

func (d AppDocument) privateProductsValid() bool {
	return d.Capability == PrivateProducts && len(d.Scopes) == 0 && d.Endpoint == "" && d.Webhooks == nil &&
		d.AccessScopes != nil && d.AccessScopes.Profile == accessscope.Profile &&
		slices.Equal(d.AccessScopes.Required, []string{"read_products"}) && len(d.AccessScopes.Optional) == 0
}

func (v PrivateProductVersion) payload() ([]byte, error) {
	if v.ID == "" || v.AppID == "" || v.OrganizationID == "" || v.OwnerAccountID == "" || v.ClientID == "" ||
		v.Document.Validate(false) != nil || !v.Document.privateProductsValid() {
		return nil, fault.Invalid
	}
	raw, err := json.Marshal(v)
	return append([]byte("emisell.private-products/v1\n"), raw...), err
}
func (v PrivateProductVersion) Sign(key ed25519.PrivateKey) ([]byte, error) {
	raw, err := v.payload()
	if err != nil {
		return nil, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, fault.Unavailable
	}
	return ed25519.Sign(key, raw), nil
}
func (v PrivateProductVersion) Verify(key ed25519.PublicKey, signature []byte) (string, error) {
	raw, err := v.payload()
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, raw, signature) {
		return "", fault.Forbidden
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
