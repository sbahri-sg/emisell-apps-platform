package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/catalogmanifest"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogCLIValidateVerify(t *testing.T) {
	dir := t.TempDir()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	m := catalogmanifest.Manifest{Schema: catalogmanifest.Schema, Policy: catalogmanifest.Policy, AppID: "app_cli", DeveloperID: "org_cli", Version: "1.0.0", Name: "CLI test", Summary: "Test export", Description: "Test catalog only", Capability: "payment/v1", Scopes: []string{"orders.read", "payments.read", "payments.write"}, Runtime: "remote", Pricing: "free", SourceSHA256: catalogmanifest.Digest([]byte("test"))}
	manifest := filepath.Join(dir, "manifest.json")
	packPath := filepath.Join(dir, "package.json")
	keyPath := filepath.Join(dir, "public.txt")
	raw, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(manifest, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := catalogCommand([]string{"catalog-validate", manifest}); err != nil {
		t.Fatal(err)
	}
	pack, err := catalogmanifest.Sign(m, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.MarshalIndent(pack, "", "  ")
	if err = os.WriteFile(packPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(pub)), 0600); err != nil {
		t.Fatal(err)
	}
	if err = catalogCommand([]string{"catalog-verify", packPath, keyPath}); err != nil {
		t.Fatal(err)
	}
	pack.Manifest.Name = "tampered"
	raw, _ = json.Marshal(pack)
	if err = os.WriteFile(packPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if catalogCommand([]string{"catalog-verify", packPath, keyPath}) == nil {
		t.Fatal("tampered package passed")
	}
	if catalogCommand([]string{"catalog-validate"}) == nil {
		t.Fatal("invalid args accepted")
	}
}
