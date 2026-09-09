package localfiles

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"
)

func TestApplicationCredentialKeyIsPersistentAndProductionExplicit(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("EMISELL_ENV", "development")
	t.Setenv("EMISELL_APP_CREDENTIAL_KEY", "")
	if err := InitApplicationCredentialKey(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(applicationKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = InitApplicationCredentialKey(); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(applicationKeyFile)
	if !bytes.Equal(first, second) {
		t.Fatal("key overwritten")
	}
	st, _ := os.Stat(applicationKeyFile)
	if st.Mode().Perm() != 0600 {
		t.Fatal("key permissions")
	}
	t.Setenv("EMISELL_ENV", "production")
	if _, err = ReadApplicationCredentialBox(); err == nil {
		t.Fatal("production used local key")
	}
	t.Setenv("EMISELL_APP_CREDENTIAL_KEY", base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	if _, err = ReadApplicationCredentialBox(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EMISELL_APP_CREDENTIAL_KEY", "bad")
	if _, err = ReadApplicationCredentialBox(); err == nil {
		t.Fatal("invalid env silently fell back")
	}
}
