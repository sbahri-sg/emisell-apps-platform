package localfiles

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
)

const ManagedShippingKeyPath = ".local/managed-shipping-signing.json"

func ReadManagedShippingKey() (ed25519.PrivateKey, error) {
	var c struct {
		Seed []byte `json:"seed"`
	}
	if err := Read(ManagedShippingKeyPath, &c); err != nil {
		return nil, err
	}
	if len(c.Seed) != ed25519.SeedSize {
		return nil, errors.New("invalid managed shipping signing key")
	}
	return ed25519.NewKeyFromSeed(c.Seed), nil
}
func InitManagedShippingKey() error {
	if _, err := ReadManagedShippingKey(); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return err
	}
	return Write(ManagedShippingKeyPath, struct {
		Seed []byte `json:"seed"`
	}{seed}, false)
}
