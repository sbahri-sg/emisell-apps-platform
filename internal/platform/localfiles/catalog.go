package localfiles

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
)

const CatalogKeyPath = ".local/catalog-signing.json"

type CatalogKey struct {
	Seed []byte `json:"seed"`
}

func ReadCatalogKey() (ed25519.PrivateKey, error) {
	var c CatalogKey
	if err := Read(CatalogKeyPath, &c); err != nil {
		return nil, err
	}
	if len(c.Seed) != ed25519.SeedSize {
		return nil, errors.New("invalid catalog signing key")
	}
	return ed25519.NewKeyFromSeed(c.Seed), nil
}
func InitCatalogKey() error {
	if _, err := ReadCatalogKey(); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return err
	}
	return Write(CatalogKeyPath, CatalogKey{Seed: seed}, false)
}
