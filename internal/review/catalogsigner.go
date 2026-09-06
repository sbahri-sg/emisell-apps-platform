package review

import (
	"crypto/ed25519"
	"emisell.app/platform/pkg/catalogmanifest"
)

// CatalogSigner attests only versioned catalog-metadata policies. Never reuse the
// public local-fixture key or accept this signature for executable releases.
type CatalogSigner struct{ Key ed25519.PrivateKey }

func (s CatalogSigner) Sign(m catalogmanifest.Manifest) (catalogmanifest.Package, error) {
	return catalogmanifest.Sign(m, s.Key)
}
func (s CatalogSigner) Verify(p catalogmanifest.Package) error {
	if len(s.Key) != ed25519.PrivateKeySize {
		return catalogmanifest.Verify(p, nil)
	}
	return catalogmanifest.Verify(p, s.Key.Public().(ed25519.PublicKey))
}
func (s CatalogSigner) PublicKey() []byte {
	if len(s.Key) != ed25519.PrivateKeySize {
		return nil
	}
	return s.Key.Public().(ed25519.PublicKey)
}
