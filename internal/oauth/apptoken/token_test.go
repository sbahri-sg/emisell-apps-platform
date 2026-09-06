package apptoken

import (
	"strings"
	"testing"
)

func TestOpaqueAppTokenHasSeparateNamespace(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		s := New()
		if !Valid(s) || seen[s] || len(Hash(s)) != 64 || strings.Contains(Hash(s), s) {
			t.Fatal("invalid opaque token")
		}
		seen[s] = true
	}
	for _, s := range []string{"", strings.Repeat("x", 43), "epk_" + strings.Repeat("x", 43), "eat_" + strings.Repeat("!", 43), New() + " "} {
		if Valid(s) {
			t.Fatal("foreign or malformed token accepted")
		}
	}
}
