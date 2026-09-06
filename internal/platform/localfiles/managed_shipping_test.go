package localfiles

import (
	"bytes"
	"os"
	"testing"
)

func TestManagedShippingKeyPrivateIndependentAndNotOverwritten(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := InitCatalogKey(); err != nil {
		t.Fatal(err)
	}
	if err := InitManagedShippingKey(); err != nil {
		t.Fatal(err)
	}
	first, err := ReadManagedShippingKey()
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
	if err := InitManagedShippingKey(); err != nil {
		t.Fatal(err)
	}
	again, err := ReadManagedShippingKey()
	if err != nil || !bytes.Equal(first, again) {
		t.Fatal("key overwritten", err)
	}
	st, err := os.Stat(ManagedShippingKeyPath)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("private file permissions", err)
	}
	if err := os.Chmod(ManagedShippingKeyPath, 0644); err != nil {
		t.Fatal(err)
	}
	if err := InitManagedShippingKey(); err == nil {
		t.Fatal("insecure existing key silently accepted or replaced")
	}
}
