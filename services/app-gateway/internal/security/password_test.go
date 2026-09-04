package security

import (
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	password := "test-only long password phrase"
	a, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || strings.Contains(a, password) {
		t.Fatal("hash must be salted and irreversible")
	}
	if !CheckPassword(a, password) || CheckPassword(a, "incorrect password") {
		t.Fatal("password verification failed")
	}
	for _, value := range []string{"", "too short", strings.Repeat("x", 257), strings.Repeat("é", 129)} {
		if _, err := HashPassword(value); err == nil {
			t.Fatal("invalid password length accepted")
		}
	}
	if _, err := HashDevelopmentPassword("aplikasi2026"); err != nil {
		t.Fatal("development-only 12 character password rejected")
	}
	if _, err := HashDevelopmentPassword("short"); err == nil {
		t.Fatal("development override accepted fewer than 12 characters")
	}
	for _, value := range []string{"garbage", "pbkdf2-sha256$999999999$AA$BB", "pbkdf2-sha256$600000$AA$BB"} {
		if CheckPassword(value, password) {
			t.Fatal("malformed hash accepted")
		}
	}
}
