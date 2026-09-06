package recovery

import (
	"strings"
	"testing"
)

func TestRecoveryValidation(t *testing.T) {
	for _, reason := range []string{"Layanan sudah pulih", "🙂🙂🙂"} {
		if err := (Request{Reason: reason}).Validate("a_valid_key_12345"); err != nil {
			t.Fatal(err)
		}
	}
	for _, reason := range []string{"short", "   abc   ", "reason\nline", strings.Repeat("🙂", 61), "valid reason\x00", string([]byte{0xff})} {
		if err := (Request{Reason: reason}).Validate("a_valid_key_12345"); err == nil {
			t.Fatal("invalid reason accepted", reason)
		}
	}
	if (Request{Reason: "Layanan sudah pulih", ExpectedRevision: -1}).Validate("a_valid_key_12345") == nil {
		t.Fatal("negative revision accepted")
	}
	if (Request{Reason: "Layanan sudah pulih"}).Validate("short") == nil {
		t.Fatal("invalid key accepted")
	}
}
