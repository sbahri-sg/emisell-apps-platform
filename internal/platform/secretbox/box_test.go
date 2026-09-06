package secretbox

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestSecretEncryptionBindsPurposeAndUsesRandomNonce(t *testing.T) {
	b, err := New(base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("private-token")
	sealed := b.Seal("tenant-a:ins-a:token", raw)
	if bytes.Equal(sealed, b.Seal("tenant-a:ins-a:token", raw)) || bytes.Contains(sealed, raw) {
		t.Fatal("insecure ciphertext")
	}
	value, err := b.Open("tenant-a:ins-a:token", sealed)
	if err != nil || !bytes.Equal(value, raw) {
		t.Fatal(err)
	}
	if _, err = b.Open("tenant-b:ins-a:token", sealed); err == nil {
		t.Fatal("cross-tenant substitution accepted")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err = b.Open("tenant-a:ins-a:token", sealed); err == nil {
		t.Fatal("tampering accepted")
	}
	if _, err = New("not-a-key"); err == nil {
		t.Fatal("invalid encryption key")
	}
}
