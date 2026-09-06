package sdk

import (
	"strings"
	"testing"
)

func TestLocalClientNeverForwardsCredentialsExternally(t *testing.T) {
	for _, address := range []string{"https://example.com", "http://example.com", "http://127.0.0.1.evil.invalid", "http://user:password@localhost:8088", "http://localhost:8088/path", "http://localhost:8088?redirect=external", "http://localhost:8088#fragment"} {
		if _, err := NewLocalClient(address, strings.Repeat("a", 43)); err == nil {
			t.Errorf("unsafe endpoint accepted: %s", address)
		}
	}
	if _, err := NewLocalClient("http://127.0.0.1:8088", "short"); err == nil {
		t.Fatal("invalid credential accepted")
	}
	if _, err := NewLocalClient("http://127.0.0.1:8088", strings.Repeat("a", 43)); err != nil {
		t.Fatal(err)
	}
	if c, err := NewLocalClient("http://127.0.0.1:8088", "epk_"+strings.Repeat("a", 43)); err != nil || c.Connection == nil {
		t.Fatal("platform key rejected")
	}
	if _, err := NewLocalClient("http://127.0.0.1:8088", strings.Repeat("a", 47)); err == nil {
		t.Fatal("unknown token type")
	}
}
