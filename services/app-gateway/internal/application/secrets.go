package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

type SecretCipher interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

func randomToken(prefix string, bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(value), nil
}

func secretFingerprint(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return "sha256:" + hex.EncodeToString(hash[:16])
}
