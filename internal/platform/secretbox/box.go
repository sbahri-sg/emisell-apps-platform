package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
)

// Box uses random-nonce AES-256-GCM; purpose binds ciphertext to its tenant,
// installation and secret type. Key material never belongs in the database.
type Box struct{ aead cipher.AEAD }

func New(encoded string) (Box, error) {
	key, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return Box{}, errors.New("invalid encryption key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Box{}, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	return Box{aead: aead}, err
}
func (b Box) Seal(purpose string, raw []byte) []byte {
	return b.aead.Seal(nil, nil, raw, []byte(purpose))
}
func (b Box) Open(purpose string, sealed []byte) ([]byte, error) {
	value, err := b.aead.Open(nil, nil, sealed, []byte(purpose))
	if err != nil {
		return nil, errors.New("secret integrity check failed")
	}
	return value, nil
}
