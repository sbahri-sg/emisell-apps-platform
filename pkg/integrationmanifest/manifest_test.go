package integrationmanifest

import (
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/catalogmanifest"
	"strings"
	"testing"
)

func validManifest() Manifest {
	return Manifest{Schema: Schema, Policy: Policy, SubmissionID: "sub_test", Metadata: catalogmanifest.Manifest{Schema: catalogmanifest.Schema, Policy: catalogmanifest.Policy, AppID: "app_test", DeveloperID: "org_test", Version: "1.0.0", Name: "Test", Summary: "Summary", Description: "Description", Capability: "payment/v1", Scopes: []string{"orders.read", "payments.read", "payments.write"}, Runtime: "remote", Pricing: "free", SourceSHA256: strings.Repeat("a", 64)}, Config: Config{Protocol: Protocol, Endpoint: "https://app.example.com/api", CallbackURL: "https://app.example.com/oauth/callback", HealthURL: "https://app.example.com/health"}}
}
func TestIntegrationManifestPolicyAndSignature(t *testing.T) {
	m := validManifest()
	public, key, _ := ed25519.GenerateKey(rand.Reader)
	p, err := Sign(m, key)
	if err != nil || Verify(p, public) != nil {
		t.Fatal("signature", err)
	}
	r := Inspect(m)
	if !r.Valid || r.Installable || len(r.Blockers) < 3 {
		t.Fatal(r)
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if Verify(p, other) == nil {
		t.Fatal("untrusted key accepted")
	}
	changed := p
	changed.Manifest.Config.HealthURL += "/changed"
	if Verify(changed, public) == nil {
		t.Fatal("tampered configuration")
	}
	changed = p
	changed.SHA256 = strings.Repeat("b", 64)
	if Verify(changed, public) == nil {
		t.Fatal("tampered digest")
	}
	raw, _ := Canonical(m)
	changed = p
	changed.Signature = ed25519.Sign(key, raw)
	if Verify(changed, public) == nil {
		t.Fatal("missing trust-domain prefix")
	}
	changed = p
	changed.KeyID = "catalog-" + Digest(public)
	if Verify(changed, public) == nil {
		t.Fatal("catalog key domain")
	}
	if _, err := Sign(m, nil); err == nil {
		t.Fatal("missing key")
	}
	m.Metadata.Capability = "shipping/v1"
	m.Metadata.Scopes = []string{"orders.read", "shipping.read", "shipping.write"}
	if !Inspect(m).Valid {
		t.Fatal("shipping reference")
	}
}
func TestIntegrationStaticEndpointPolicy(t *testing.T) {
	for _, raw := range []string{"http://app.example.com/api", "https://localhost/api", "https://app.local/api", "https://127.0.0.1/api", "https://[::1]/api", "https://10.0.0.1/api", "https://2130706433/api", "https://app.example.com:8080/api", "https://u:p@app.example.com/api", "https://app.example.com/api?token=secret", "https://app.example.com/api?", "https://app.example.com/api#", "https://app.example.com./api", "https://app.invalid/api", "https://-app.example.com/api", "https://app.example.com:bad/api"} {
		t.Run(raw, func(t *testing.T) {
			m := validManifest()
			m.Config.Endpoint = raw
			if Inspect(m).Valid {
				t.Fatal("unsafe/unsupported endpoint")
			}
		})
	}
	m := validManifest()
	m.Config.CallbackURL = "https://other.example.com/callback"
	if Inspect(m).Valid {
		t.Fatal("cross-origin callback")
	}
	m = validManifest()
	m.Config.Protocol = "arbitrary-node"
	if Inspect(m).Valid {
		t.Fatal("unsupported protocol")
	}
	m = validManifest()
	m.Metadata.Runtime = "node"
	if Inspect(m).Valid {
		t.Fatal("unsupported runtime")
	}
	m = validManifest()
	m.Metadata.Installable = true
	if Inspect(m).Valid {
		t.Fatal("installable metadata")
	}
}
func TestIntegrationScopeReadiness(t *testing.T) {
	m := validManifest()
	m.Metadata.Schema = catalogmanifest.SchemaAccessScopes
	m.Metadata.Policy = catalogmanifest.PolicyAccessScopes
	m.Metadata.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"write_products"}, Optional: []string{}}
	if Inspect(m).Valid {
		t.Fatal("Plan required scope accepted")
	}
	m.Metadata.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{}, Optional: []string{"read_products"}}
	r := Inspect(m)
	if !r.Valid || r.Installable || len(r.Blockers) != 4 {
		t.Fatal("optional Plan must not grant", r)
	}
}
