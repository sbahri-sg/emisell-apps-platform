package appidentity

import (
	"bytes"
	"emisell.app/platform/internal/platform/secretbox"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestCredentialEncryptionAndSerialization(t *testing.T) {
	box, err := secretbox.New(base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	repo := Repository{Box: &box}
	c := Credential{ClientID: "eai_test", OrganizationID: "org", AppID: "app", Version: 1}
	if err = repo.material(&c); err != nil {
		t.Fatal(err)
	}
	raw, err := box.Open(purpose(c), c.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 40 || digest(string(raw)) != c.Hash || bytes.Contains(c.Ciphertext, raw) {
		t.Fatal("bad secret material")
	}
	other := c
	other.AppID = "other"
	if _, err = box.Open(purpose(other), c.Ciphertext); err == nil {
		t.Fatal("ciphertext not bound to app")
	}
	other = c
	other.Version++
	if _, err = box.Open(purpose(other), c.Ciphertext); err == nil {
		t.Fatal("ciphertext not bound to version")
	}
	encoded, _ := json.Marshal(c)
	if bytes.Contains(encoded, raw) || bytes.Contains(encoded, []byte(c.Hash)) || bytes.Contains(encoded, []byte("ciphertext")) {
		t.Fatal("private material serialized")
	}
	oldHash := c.Hash
	c.Version++
	if err = repo.material(&c); err != nil || oldHash == c.Hash {
		t.Fatal("material not rotated")
	}
}
