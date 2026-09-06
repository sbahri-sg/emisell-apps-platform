package identity

import "testing"

func TestPasswordHash(t *testing.T) {
	const password = "unique-local-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(password, hash) || VerifyPassword("incorrect", hash) || VerifyPassword(password, "malformed") {
		t.Fatal("password verification invariant failed")
	}
	second, err := HashPassword(password)
	if err != nil || second == hash {
		t.Fatal("salt must be random")
	}
	if _, err = HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
}
