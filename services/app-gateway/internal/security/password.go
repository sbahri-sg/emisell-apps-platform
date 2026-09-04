package security

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"
)

// PBKDF2-HMAC-SHA256, 600,000 iterations. Versioned format, random 128-bit
// salt, no reversible password storage, no password truncation.
const passwordPrefix = "pbkdf2-sha256$600000$"

func HashPassword(password string) (string, error) {
	return hashPassword(password, 15)
}

// HashDevelopmentPassword exists only for explicitly local development
// fixtures. Callers must independently reject it outside APP_ENV=development.
func HashDevelopmentPassword(password string) (string, error) {
	return hashPassword(password, 12)
}

func hashPassword(password string, minimumRunes int) (string, error) {
	if utf8.RuneCountInString(password) < minimumRunes || len(password) > 256 || !utf8.ValidString(password) {
		return "", fmt.Errorf("password must have at least %d characters and at most 256 bytes", minimumRunes)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	if err != nil {
		return "", err
	}
	return passwordPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func CheckPassword(encoded, password string) bool {
	if !strings.HasPrefix(encoded, passwordPrefix) || len(password) > 256 {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(encoded, passwordPrefix), "$")
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil || len(want) != 32 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
