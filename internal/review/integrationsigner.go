package review

import (
	"crypto/ed25519"
	"emisell.app/platform/pkg/integrationmanifest"
	"errors"
)

// Dedicated trust domain and key; never reuse the catalog or public fixture key.
type IntegrationSigner struct{ Key ed25519.PrivateKey }

func (s IntegrationSigner) Sign(m integrationmanifest.Manifest) (integrationmanifest.Package, error) {
	return integrationmanifest.Sign(m, s.Key)
}
func (s IntegrationSigner) Verify(p integrationmanifest.Package) error {
	if len(s.Key) != ed25519.PrivateKeySize {
		return errors.New("integration signer unavailable")
	}
	return integrationmanifest.Verify(p, s.Key.Public().(ed25519.PublicKey))
}
func (s IntegrationSigner) PublicKey() []byte {
	if len(s.Key) != ed25519.PrivateKeySize {
		return nil
	}
	return s.Key.Public().(ed25519.PublicKey)
}
