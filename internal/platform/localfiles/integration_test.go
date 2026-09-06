package localfiles

import (
	"bytes"
	"os"
	"testing"
)

func TestIntegrationKeyPrivateIndependentAndNotOverwritten(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := InitCatalogKey(); err != nil {
		t.Fatal(err)
	}
	if err := InitIntegrationKey(); err != nil {
		t.Fatal(err)
	}
	first, err := ReadIntegrationKey()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ReadCatalogKey()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, catalog) {
		t.Fatal("trust domains share a key")
	}
	if err := InitIntegrationKey(); err != nil {
		t.Fatal(err)
	}
	again, err := ReadIntegrationKey()
	if err != nil || !bytes.Equal(first, again) {
		t.Fatal("key overwritten", err)
	}
	st, err := os.Stat(IntegrationKeyPath)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("private file permissions", err)
	}
	if err := os.Chmod(IntegrationKeyPath, 0644); err != nil {
		t.Fatal(err)
	}
	if err := InitIntegrationKey(); err == nil {
		t.Fatal("insecure existing key silently accepted or replaced")
	}
}
