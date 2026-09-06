package catalogmanifest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/accessscope"
	"encoding/json"
	"fmt"
	"testing"
)

func sample() Manifest {
	return Manifest{Schema: Schema, Policy: Policy, AppID: "app_1", DeveloperID: "dev_1", Version: "1.0.0", Name: "Shipping", Summary: "Rates", Description: "Shipping rates", Capability: "shipping/v1", Scopes: []string{"orders.read", "shipping.read", "shipping.write"}, Runtime: "remote", Pricing: "free", SourceSHA256: Digest([]byte("source"))}
}

func TestResourceScopesVersioningAndSignature(t *testing.T) {
	m := sample()
	raw, err := Canonical(m)
	golden := fmt.Sprintf(`{"schema":"emisell.catalog/v1","policy":"catalog-metadata/v1","appId":"app_1","developerId":"dev_1","version":"1.0.0","name":"Shipping","summary":"Rates","description":"Shipping rates","capability":"shipping/v1","scopes":["orders.read","shipping.read","shipping.write"],"runtime":"remote","pricing":"free","installable":false,"sourceSha256":"%s"}`, Digest([]byte("source")))
	if err != nil || string(raw) != golden {
		t.Fatal("v1 canonical bytes changed", err)
	}
	m.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"write_products"}, Optional: []string{"read_customers"}}
	if m.Validate() == nil {
		t.Fatal("v1 silently gained fields")
	}
	m.Schema, m.Policy = SchemaAccessScopes, PolicyAccessScopes
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	p, err := Sign(m, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(p)
	decoded, err := DecodePackage(raw)
	if err != nil || Verify(decoded, pub) != nil {
		t.Fatal("v2 roundtrip", err)
	}
	decoded.Manifest.AccessScopes.Optional = []string{"read_orders"}
	if Verify(decoded, pub) == nil {
		t.Fatal("scope tamper accepted")
	}
	for _, mutate := range []func(*Manifest){
		func(m *Manifest) { m.AccessScopes = nil },
		func(m *Manifest) { m.Installable = true },
		func(m *Manifest) { m.Policy = Policy },
	} {
		bad := m
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatal("v2 bypass")
		}
	}
}

func TestBrowserExportEscapesAndDuplicateFields(t *testing.T) {
	m := sample()
	m.Name = "Shipping & Orders <QA>"
	raw, _ := Canonical(m)
	browser := bytes.ReplaceAll(bytes.ReplaceAll(bytes.ReplaceAll(raw, []byte(`\u0026`), []byte("&")), []byte(`\u003c`), []byte("<")), []byte(`\u003e`), []byte(">"))
	if _, err := Decode(browser); err != nil {
		t.Fatal("browser export rejected", err)
	}
	dup := bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"bad","schema":`), 1)
	if _, err := Decode(dup); err == nil {
		t.Fatal("duplicate manifest field accepted")
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	p, _ := Sign(m, key)
	raw, _ = json.Marshal(p)
	dup = bytes.Replace(raw, []byte(`"name":`), []byte(`"name":"bad","name":`), 1)
	if _, err := DecodePackage(dup); err == nil {
		t.Fatal("nested duplicate package field accepted")
	}
}
func TestCatalogSignatureAndPolicy(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	m := sample()
	p, err := Sign(m, key)
	if err != nil {
		t.Fatal(err)
	}
	if err = Verify(p, pub); err != nil {
		t.Fatal(err)
	}
	raw, _ := Canonical(m)
	if _, err = Decode(raw); err != nil {
		t.Fatal(err)
	}
	pretty, _ := json.MarshalIndent(m, "", "  ")
	if _, err = Decode(pretty); err != nil {
		t.Fatal(err)
	}
	wrong, _, _ := ed25519.GenerateKey(rand.Reader)
	if Verify(p, wrong) == nil {
		t.Fatal("untrusted signer accepted")
	}
	p.Manifest.DeveloperID = "different"
	if Verify(p, pub) == nil {
		t.Fatal("publisher tamper")
	}
	p, _ = Sign(m, key)
	p.SHA256 = Digest([]byte("wrong"))
	if Verify(p, pub) == nil {
		t.Fatal("checksum tamper")
	}
	for _, mutation := range []func(*Manifest){func(v *Manifest) { v.Installable = true }, func(v *Manifest) { v.Schema = "emisell.app/v1" }, func(v *Manifest) { v.Pricing = "paid" }, func(v *Manifest) { v.Runtime = "node" }, func(v *Manifest) { v.Scopes = append(v.Scopes, "admin.write") }, func(v *Manifest) { v.Version = "01.0.0" }, func(v *Manifest) { v.Name = "bad\x00" }} {
		bad := m
		mutation(&bad)
		if bad.Validate() == nil {
			t.Fatal("policy bypass")
		}
	}
	if _, err = Decode(append(raw, []byte(`{}`)...)); err == nil {
		t.Fatal("trailing data")
	}
	if _, err = Decode([]byte(`{"schema":"emisell.catalog/v1","endpoint":"https://secret.invalid"}`)); err == nil {
		t.Fatal("unknown endpoint field")
	}
}
