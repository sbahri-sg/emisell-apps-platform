package config

import (
	"path/filepath"
	"testing"
)

func TestProductConfigurationFailsClosed(t *testing.T) {
	t.Setenv("EMISELL_RESOURCE_ENABLED", "false")
	t.Setenv("EMISELL_RESOURCE_PRIVATE_KEY_FILE", filepath.Join(t.TempDir(), "missing.pem"))
	client, err := LoadProducts("development")
	if err != nil || client != nil {
		t.Fatal("disabled resource must not load credentials")
	}
	t.Setenv("EMISELL_RESOURCE_ENABLED", "not-a-boolean")
	if _, err := LoadProducts("development"); err == nil {
		t.Fatal("invalid feature flag accepted")
	}
	t.Setenv("EMISELL_RESOURCE_ENABLED", "true")
	if _, err := LoadProducts("development"); err == nil {
		t.Fatal("enabled resource without signing key accepted")
	}
}
