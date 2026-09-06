package uirelease

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
)

func fixture() Manifest {
	return Manifest{Schema, Policy, "app_demo", "org_demo", "1.0.0", "Demo UI", "Aplikasi UI tanpa izin bisnis", "embedded", "https://app.example.com/dashboard", "free"}
}

func TestSignedModesAndTampering(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	for _, mode := range []string{"embedded", "external"} {
		m := fixture()
		m.Mode = mode
		p, err := Sign(m, key)
		if err != nil || Verify(p, pub) != nil {
			t.Fatal(err)
		}
		for _, edit := range []func(*Package){
			func(p *Package) { p.Manifest.URL = "https://other.example.com/app" },
			func(p *Package) { p.Manifest.DeveloperID = "org_other" },
			func(p *Package) { p.Manifest.Version = "2.0.0" },
			func(p *Package) {
				p.Manifest.Mode = "external"
				if mode == "external" {
					p.Manifest.Mode = "embedded"
				}
			},
			func(p *Package) { p.KeyID = "other" },
			func(p *Package) { p.SHA256 = strings.Repeat("0", 64) },
		} {
			copy := p
			edit(&copy)
			if Verify(copy, pub) == nil {
				t.Fatal("tampering accepted")
			}
		}
		if Verify(p, nil) == nil {
			t.Fatal("missing trust key accepted")
		}
	}
}

func TestRejectUnsafeOrBusinessRelease(t *testing.T) {
	for _, target := range []string{"http://app.example.com/ui", "https://127.0.0.1/ui", "https://localhost/ui", "https://test.local/ui", "https://app.example.com:8000/ui", "https://user:secret@app.example.com/ui", "https://app.example.com/ui?token=x", "https://app.example.com/ui?", "https://app.example.com/ui#"} {
		m := fixture()
		m.URL = target
		if m.Validate() == nil {
			t.Fatal("unsafe URL", target)
		}
	}
	raw, _ := json.Marshal(fixture())
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"scopes":["orders.read"]`, `"capability":"shipping/v1"`, `"apiKey":"secret"`, `"accessScopes":{}`} {
		bad := append(append([]byte{}, raw[:len(raw)-1]...), []byte(","+field+"}")...)
		if _, err := Decode(bad); err == nil {
			t.Fatal("unknown field discarded", field)
		}
	}
	if _, err := Decode(append(raw, []byte(" {}")...)); err == nil {
		t.Fatal("trailing document accepted")
	}
	m := fixture()
	m.Mode = ""
	if m.Validate() == nil {
		t.Fatal("implicit mode accepted")
	}
	m = fixture()
	m.Pricing = "paid"
	if m.Validate() == nil {
		t.Fatal("paid accepted")
	}
	if _, err := Sign(fixture(), nil); err == nil {
		t.Fatal("missing signer accepted")
	}
}
