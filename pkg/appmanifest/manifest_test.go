package appmanifest

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestLocalReleaseVerification(t *testing.T) {
	m := Manifest{Schema: "emisell.app/v1", ID: "example", Version: "1.0.0", DeveloperID: "emisell-local", Runtime: "Remote app", ExecutionProfile: "local-simulator", Compatibility: "platform/v1", ArtifactSHA256: ArtifactDigest(), Capabilities: []string{"payment/v1"}, Scopes: []string{"orders.read", "payments.read", "payments.write"}}
	sign := func(m Manifest) ([]byte, string) {
		raw, _ := json.Marshal(m)
		return raw, base64.StdEncoding.EncodeToString(ed25519.Sign(LocalFixtureKey(), raw))
	}
	raw, sig := sign(m)
	if _, err := VerifyLocal(raw, sig); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyLocal(append(raw, ' '), sig); err == nil {
		t.Fatal("tampered manifest accepted")
	}
	m.ExecutionProfile = "arbitrary-node"
	raw, sig = sign(m)
	if _, err := VerifyLocal(raw, sig); err == nil {
		t.Fatal("unsupported runtime accepted")
	}
	m.ExecutionProfile = "local-simulator"
	m.Scopes = append(m.Scopes, "admin.all")
	raw, sig = sign(m)
	if _, err := VerifyLocal(raw, sig); err == nil {
		t.Fatal("overprivileged manifest accepted")
	}
}

func TestRemoteProfilePinsCapabilityAndSubscription(t *testing.T) {
	m := Manifest{Schema: "emisell.app/v1", ID: "remote-pay", Version: "1.0.0", DeveloperID: "emisell-local", Runtime: "Remote app", ExecutionProfile: "local-remote", Compatibility: "platform/v1", ArtifactSHA256: RemoteArtifactDigest(), Capabilities: []string{"payment/v1"}, Scopes: []string{"orders.read", "payments.read", "payments.write"}, Subscriptions: []string{"emisell.capability.invoked.v1"}}
	verify := func(m Manifest) error {
		raw, _ := json.Marshal(m)
		_, err := VerifyLocal(raw, base64.StdEncoding.EncodeToString(ed25519.Sign(LocalFixtureKey(), raw)))
		return err
	}
	if err := verify(m); err != nil {
		t.Fatal(err)
	}
	m.Subscriptions = []string{"emisell.customer.export.v1"}
	if verify(m) == nil {
		t.Fatal("unsigned expansion of subscription accepted")
	}
	m.Subscriptions = []string{"emisell.capability.invoked.v1"}
	m.ArtifactSHA256 = ArtifactDigest()
	if verify(m) == nil {
		t.Fatal("wrong remote artifact identity accepted")
	}
}
