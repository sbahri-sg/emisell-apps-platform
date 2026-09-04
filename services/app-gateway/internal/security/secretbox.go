package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(key []byte) (*SecretBox, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM cipher: %w", err)
	}
	return &SecretBox{aead: aead}, nil
}

func (box *SecretBox) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, box.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate encryption nonce: %w", err)
	}
	return box.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (box *SecretBox) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := box.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("invalid encrypted secret")
	}
	plaintext, err := box.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return plaintext, nil
}
