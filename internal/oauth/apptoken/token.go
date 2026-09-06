// Package apptoken owns opaque token material, not installation persistence.
package apptoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
)

const Audience = "emisell.app-platform.local/installation-access"
const TTL = 15 * time.Minute

func New() string {
	var raw [32]byte
	rand.Read(raw[:])
	return "eat_" + base64.RawURLEncoding.EncodeToString(raw[:])
}

func Valid(s string) bool {
	if len(s) != 47 || !strings.HasPrefix(s, "eat_") {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[4:])
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == s[4:]
}

func Hash(s string) string {
	v := sha256.Sum256([]byte(s))
	return hex.EncodeToString(v[:])
}
