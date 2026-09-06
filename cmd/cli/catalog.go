package main

import (
	"crypto/ed25519"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/pkg/catalogmanifest"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func boundedFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 32<<10+1))
	if len(raw) > 32<<10 {
		return nil, errors.New("catalog input too large")
	}
	return raw, err
}
func catalogCommand(args []string) error {
	if args[0] == "init-catalog" && len(args) == 1 {
		if err := localfiles.InitCatalogKey(); err != nil {
			return err
		}
		fmt.Println("Private catalog signing key ready; existing key preserved. Never use for executable releases.")
		return nil
	}
	if args[0] == "catalog-validate" && len(args) == 2 {
		raw, err := boundedFile(args[1])
		if err != nil {
			return err
		}
		_, err = catalogmanifest.Decode(raw)
		if err != nil {
			return err
		}
		fmt.Println("Catalog metadata valid. Unsigned; not executable or installable.")
		return nil
	}
	if args[0] == "catalog-verify" && len(args) == 3 {
		raw, err := boundedFile(args[1])
		if err != nil {
			return err
		}
		pack, err := catalogmanifest.DecodePackage(raw)
		if err != nil {
			return err
		}
		encoded, err := boundedFile(args[2])
		if err != nil {
			return err
		}
		public, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
		if err != nil {
			return errors.New("trusted public key must be base64")
		}
		if err = catalogmanifest.Verify(pack, ed25519.PublicKey(public)); err != nil {
			return err
		}
		fmt.Println("Catalog signature and checksum verified against the supplied trusted key. Not an app-code security certification.")
		return nil
	}
	return errors.New("usage: cli init-catalog | catalog-validate <manifest.json> | catalog-verify <package.json> <trusted-key.txt>")
}
