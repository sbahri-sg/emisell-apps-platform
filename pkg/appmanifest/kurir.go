package appmanifest

import (
	"crypto/sha256"
	"fmt"
)

const KurirFixtureID = "api-kurir-reference"
const KurirFixtureProfile = "local-kurir-reference"

func KurirFixtureDigest() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte("emisell-kurir-rates-reference/v1")))
}

// KurirFixture is opt-in test material, never a developer release or production app.
// It grants rates access only; booking, tracking and resource scopes stay closed.
func KurirFixture() Manifest {
	return Manifest{
		Schema: "emisell.app/v1", ID: KurirFixtureID, Name: "API-Kurir · Local reference",
		Version: "1.0.0", DeveloperID: "emisell-local", Runtime: "Remote app",
		ExecutionProfile: KurirFixtureProfile, Compatibility: "platform/v1",
		ArtifactSHA256: KurirFixtureDigest(), Category: "Pengiriman", Icon: "shipping", Color: "blue",
		Summary:     "Referensi lokal cek ongkir melalui API-Kurir tiruan.",
		Description: "Tanpa transaksi atau pengiriman asli. Tidak mengubah credential atau pilihan provider merchant.",
		Scopes:      []string{"shipping.read"}, Capabilities: []string{"shipping/v1"},
	}
}
