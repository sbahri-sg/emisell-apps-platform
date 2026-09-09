package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"emisell.app/platform/internal/resourceclient"
)

type privateAppsRuntime struct {
	Key      ed25519.PrivateKey
	Products *resourceclient.Products
}

// One operator-owned production secret, never read from .local demo files.
// There is no automatic generation or fallback when a configured key is lost.
func readProductionPrivateApps() (*privateAppsRuntime, error) {
	path := os.Getenv("EMISELL_PRIVATE_APPS_FILE")
	if os.Getenv("EMISELL_ENV") != "production" {
		if path != "" {
			return nil, errors.New("production private apps configuration requires EMISELL_ENV=production")
		}
		return nil, nil
	}
	invalid := errors.New("production private apps require a private EMISELL_PRIVATE_APPS_FILE with release signing seed and trusted Core resource credentials")
	if !filepath.IsAbs(path) {
		return nil, invalid
	}
	stat, err := os.Lstat(path)
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() > 16384 {
		return nil, invalid
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, invalid
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(stat, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm()&0077 != 0 || opened.Size() > 16384 {
		return nil, invalid
	}
	var config struct {
		Environment   string `json:"environment"`
		Transport     string `json:"transport"`
		ReleaseSeed   []byte `json:"releaseSeed"`
		CoreOrigin    string `json:"coreOrigin"`
		KeyID         string `json:"keyId"`
		PrivateKeyPEM string `json:"privateKeyPem"`
	}
	raw, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(raw) > 16384 {
		return nil, invalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&config) != nil || d.Decode(new(any)) != io.EOF || config.Environment != "production" || len(config.ReleaseSeed) != ed25519.SeedSize || (config.Transport != "" && config.Transport != "https" && config.Transport != "private-network") {
		return nil, invalid
	}
	products, err := resourceclient.NewProducts(resourceclient.Options{Origin: config.CoreOrigin, KeyID: config.KeyID, Environment: "production", PrivateKeyPEM: []byte(config.PrivateKeyPEM), PrivateNetworkHTTP: config.Transport == "private-network"})
	if err != nil {
		return nil, invalid
	}
	return &privateAppsRuntime{Key: ed25519.NewKeyFromSeed(config.ReleaseSeed), Products: products}, nil
}
