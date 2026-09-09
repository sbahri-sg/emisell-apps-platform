package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionPrivateAppsConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("EMISELL_ENV", "production")
	t.Setenv("EMISELL_PRIVATE_APPS_FILE", "")
	if _, err := readProductionPrivateApps(); err == nil {
		t.Fatal("missing production config accepted")
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	config := map[string]any{"environment": "production", "releaseSeed": key.Seed(), "coreOrigin": "https://core.example.test", "keyId": "production-1", "privateKeyPem": string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}))}
	path := filepath.Join(t.TempDir(), "private-apps.json")
	t.Setenv("EMISELL_PRIVATE_APPS_FILE", path)
	write := func(v map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(v)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(config)
	runtime, err := readProductionPrivateApps()
	if err != nil || runtime == nil || !runtime.Key.Equal(key) || runtime.Products == nil {
		t.Fatal("production config rejected", err)
	}
	for field, invalid := range map[string]any{"environment": "development", "releaseSeed": []byte{1}, "coreOrigin": "http://127.0.0.1:8000", "keyId": "bad/key", "privateKeyPem": "secret-invalid", "allowHTTP": true} {
		t.Run(field, func(t *testing.T) {
			bad := map[string]any{}
			for k, v := range config {
				bad[k] = v
			}
			bad[field] = invalid
			write(bad)
			if _, err := readProductionPrivateApps(); err == nil || strings.Contains(err.Error(), "secret-invalid") {
				t.Fatal("invalid configuration accepted or exposed", err)
			}
		})
	}
	write(config)
	internal := map[string]any{}
	for k, v := range config {
		internal[k] = v
	}
	internal["coreOrigin"], internal["transport"] = "http://127.0.0.1:8000", "private-network"
	write(internal)
	if value, err := readProductionPrivateApps(); err != nil || value == nil {
		t.Fatal("explicit internal production config rejected", err)
	}
	internal["coreOrigin"] = "http://169.254.169.254"
	write(internal)
	if _, err := readProductionPrivateApps(); err == nil {
		t.Fatal("metadata server allowed")
	}
	write(config)
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readProductionPrivateApps(); err == nil {
		t.Fatal("public-readable private key accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := path + ".link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EMISELL_PRIVATE_APPS_FILE", link)
	if _, err := readProductionPrivateApps(); err == nil {
		t.Fatal("symlink accepted")
	}
	t.Setenv("EMISELL_ENV", "development")
	if _, err := readProductionPrivateApps(); err == nil {
		t.Fatal("production config loaded in development")
	}
	t.Setenv("EMISELL_PRIVATE_APPS_FILE", "")
	if value, err := readProductionPrivateApps(); value != nil || err != nil {
		t.Fatal("local startup changed", err)
	}
}
