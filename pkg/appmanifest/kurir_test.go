package appmanifest

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestKurirReferenceProfileIsClosedAndReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*Manifest)
		allowed bool
	}{
		{"reference", func(*Manifest) {}, true},
		{"identity", func(m *Manifest) { m.ID = "other" }, false},
		{"digest", func(m *Manifest) { m.ArtifactSHA256 = ArtifactDigest() }, false},
		{"scope expansion", func(m *Manifest) { m.Scopes = append(m.Scopes, "shipping.write") }, false},
		{"resource scope", func(m *Manifest) { m.Scopes = []string{"read_shipping"} }, false},
		{"payment", func(m *Manifest) { m.Capabilities = []string{"payment/v1"} }, false},
		{"subscription", func(m *Manifest) { m.Subscriptions = []string{"emisell.order.created.v1"} }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := KurirFixture()
			tc.change(&m)
			raw, _ := json.Marshal(m)
			sig := base64.StdEncoding.EncodeToString(ed25519.Sign(LocalFixtureKey(), raw))
			_, err := VerifyLocal(raw, sig)
			if (err == nil) != tc.allowed {
				t.Fatal("unexpected profile policy", err)
			}
		})
	}
}
