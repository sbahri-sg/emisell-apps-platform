package embedded_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/embedded"
	"encoding/json"
	"strings"
	"testing"
)

func TestLaunchSignatureAndOriginPolicy(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	base := embedded.Launch{AppID: "app", ClientID: "client", ReleaseDigest: strings.Repeat("a", 64), URL: "https://app.example/app", ParentOrigin: "https://core.example"}
	sig, err := embedded.SignLaunch(base, key, false)
	if err != nil {
		t.Fatal(err)
	}
	if embedded.VerifyLaunch(base, sig, pub, false) != nil {
		t.Fatal("valid binding")
	}
	for _, url := range []string{"javascript:alert(1)", "https://user:pass@app.example/", "https://app.example/?token=x", "https://app.example/#token", "https://core.example/app", "http://app.example/"} {
		bad := base
		bad.URL = url
		if bad.Validate(false) == nil {
			t.Fatal("unsafe URL", url)
		}
	}
	for _, mutate := range []func(*embedded.Launch){func(l *embedded.Launch) { l.AppID = "other" }, func(l *embedded.Launch) { l.ClientID = "other" }, func(l *embedded.Launch) { l.ReleaseDigest = strings.Repeat("b", 64) }, func(l *embedded.Launch) { l.URL = "https://app.example/new" }, func(l *embedded.Launch) { l.ParentOrigin = "https://other.example" }} {
		bad := base
		mutate(&bad)
		if embedded.VerifyLaunch(bad, sig, pub, false) == nil {
			t.Fatal("unsigned binding mutation")
		}
	}
	local := base
	local.URL = "http://127.0.0.1:49001/app"
	local.ParentOrigin = "http://127.0.0.1:49002"
	if local.Validate(false) == nil || local.Validate(true) != nil {
		t.Fatal("local mode separation")
	}
}

func TestLaunchModesPreserveHistoricalSignature(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	l := embedded.Launch{AppID: "app", ClientID: "client", ReleaseDigest: strings.Repeat("a", 64), URL: "https://app.example/ui", ParentOrigin: "https://core.example"}
	raw, _ := json.Marshal(l)
	if strings.Contains(string(raw), `"mode"`) || l.DisplayMode() != "embedded" {
		t.Fatal("legacy canonical bytes changed")
	}
	sig, _ := embedded.SignLaunch(l, key, false)
	l.Mode = "external"
	if embedded.VerifyLaunch(l, sig, pub, false) == nil {
		t.Fatal("unsigned mode change accepted")
	}
	if l.Validate(false) != nil {
		t.Fatal("external mode rejected")
	}
	l.Mode = "popup"
	if l.Validate(false) == nil {
		t.Fatal("unknown mode accepted")
	}
}
