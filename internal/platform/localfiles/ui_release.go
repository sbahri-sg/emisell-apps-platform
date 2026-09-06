package localfiles

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
)

const UIReleaseKeyPath = ".local/ui-release-signing.json"

func ReadUIReleaseKey() (ed25519.PrivateKey, error) {
	var c struct {
		Seed []byte `json:"seed"`
	}
	if err := Read(UIReleaseKeyPath, &c); err != nil {
		return nil, err
	}
	if len(c.Seed) != ed25519.SeedSize {
		return nil, errors.New("invalid UI release key")
	}
	return ed25519.NewKeyFromSeed(c.Seed), nil
}
func InitUIReleaseKey() error {
	if _, err := ReadUIReleaseKey(); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return err
	}
	return Write(UIReleaseKeyPath, struct {
		Seed []byte `json:"seed"`
	}{seed}, false)
}
