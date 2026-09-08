package uiresource

import (
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/uirelease"
	"encoding/json"
	"strings"
	"testing"
)

func fixture() Manifest {
	return Manifest{Schema: Schema, Policy: Policy, UI: uirelease.Manifest{Schema: uirelease.Schema, Policy: uirelease.Policy, AppID: "app_demo", DeveloperID: "org_demo", Version: "0.2.0", Name: "Product demo", Summary: "Read-only products", Mode: "embedded", URL: "https://app.example.com/", Pricing: "free"}, RequiredScopes: []string{ReadProducts}}
}
func TestSignedPermissions(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	m := fixture()
	p, err := Sign(m, key)
	if err != nil || Verify(p, pub) != nil {
		t.Fatal("valid release", err)
	}
	m.RequiredScopes[0] = "read_orders"
	if Verify(p, pub) != nil {
		t.Fatal("caller mutation changed package")
	}
	for _, modify := range []func(*Package){
		func(p *Package) { p.Manifest.UI.URL = "https://other.example.com/" },
		func(p *Package) { p.Manifest.UI.AppID = "app_other" },
		func(p *Package) { p.Manifest.UI.DeveloperID = "org_other" },
		func(p *Package) { p.Manifest.UI.Version = "0.3.0" },
		func(p *Package) { p.Manifest.RequiredScopes = []string{"read_orders"} },
		func(p *Package) { p.KeyID = "ui-release-wrong" },
	} {
		changed := p
		modify(&changed)
		if Verify(changed, pub) == nil {
			t.Fatal("tamper accepted")
		}
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if Verify(p, other) == nil {
		t.Fatal("wrong signer accepted")
	}
	legacy, _ := uirelease.Sign(p.Manifest.UI, key)
	p.Signature = legacy.Signature
	if Verify(p, pub) == nil {
		t.Fatal("UI-only signature accepted")
	}
}
func TestStrictPermissionInput(t *testing.T) {
	m := fixture()
	raw, _ := json.Marshal(m)
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
	for _, scope := range [][]string{nil, {}, {ReadProducts, ReadProducts}, {"products.read"}, {"write_orders"}, {"write_products"}, {ReadProducts, "read_orders"}} {
		changed := m
		changed.RequiredScopes = scope
		if changed.Validate() == nil {
			t.Fatal("unsupported permissions", scope)
		}
	}
	for _, raw := range []string{
		string(raw) + "{}", strings.Replace(string(raw), "{", `{"schema":"wrong",`, 1),
		strings.Replace(string(raw), `"ui":{`, `"ui":{"appId":"app_other",`, 1),
		strings.Replace(string(raw), "{", `{"optionalScopes":["read_orders"],`, 1),
	} {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	if _, err := uirelease.Decode(raw); err == nil {
		t.Fatal("resource contract accepted as UI-only")
	}
}

func TestReviewedReadScopeSubsets(t *testing.T) {
	for _, scopes := range [][]string{{ReadInventory}, {ReadLocations}, {ReadCatalogs}, {ReadCollections}, {ReadCatalogs, ReadCollections}, {ReadOrders}, {ReadShipping}, {ReadOrders, ReadShipping}, {ReadCatalogs, ReadCollections, ReadInventory, ReadLocations, ReadOrders, ReadProducts, ReadShipping}} {
		m := fixture()
		m.RequiredScopes = scopes
		if m.Validate() != nil {
			t.Fatal("supported canonical subset rejected", scopes)
		}
	}
}
