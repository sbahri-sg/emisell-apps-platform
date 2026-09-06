package review

import (
	"crypto/ed25519"
	"emisell.app/platform/pkg/managedshipping"
)

type ManagedShippingSigner struct{ Key ed25519.PrivateKey }

func (s ManagedShippingSigner) Sign(m managedshipping.Manifest) (managedshipping.Package, error) {
	return managedshipping.Sign(m, s.Key)
}
func (s ManagedShippingSigner) Verify(p managedshipping.Package) error {
	return managedshipping.Verify(p, s.PublicKey())
}
func (s ManagedShippingSigner) PublicKey() []byte {
	if len(s.Key) != ed25519.PrivateKeySize {
		return nil
	}
	return s.Key.Public().(ed25519.PublicKey)
}
