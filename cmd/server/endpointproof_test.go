package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalProofConfigurationFailsClosed(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := readProofVerifier(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(".local", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".local", "endpoint-proof.json"), []byte(`{"environment":"development","origin":"https://app.emisell.test","certificatePem":"invalid"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, variable := range []string{"EMISELL_ENV", "NODE_ENV"} {
		t.Run(variable, func(t *testing.T) {
			t.Setenv(variable, "production")
			if _, err := readProofVerifier(); err == nil {
				t.Fatal("production loaded local verifier")
			}
		})
	}
	if _, err := readProofVerifier(); err == nil {
		t.Fatal("invalid config silently ignored")
	}
}
