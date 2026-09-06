package localfiles

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
)

const IntegrationKeyPath = ".local/integration-signing.json"

func ReadIntegrationKey() (ed25519.PrivateKey, error) {
	var c struct {
		Seed []byte `json:"seed"`
	}
	if err := Read(IntegrationKeyPath, &c); err != nil {
		return nil, err
	}
	if len(c.Seed) != ed25519.SeedSize {
		return nil, errors.New("invalid integration signing key")
	}
	return ed25519.NewKeyFromSeed(c.Seed), nil
}
func InitIntegrationKey() error {
	if _, err := ReadIntegrationKey(); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return err
	}
	return Write(IntegrationKeyPath, struct {
		Seed []byte `json:"seed"`
	}{seed}, false)
}
