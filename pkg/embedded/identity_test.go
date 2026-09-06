package embedded_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/embedded"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTokenTrustAndLifetime(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	id := embedded.Identity{MerchantID: "m", ActorID: "u", AppID: "a", InstallationID: "i"}
	now := time.Unix(1788690000, 0)
	a, err := embedded.Issue(key, "k", "issuer", "client", id, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := embedded.Issue(key, "k", "issuer", "client", id, now)
	if err != nil || a == b {
		t.Fatal("tokens must be unique", err)
	}
	keys := map[string]ed25519.PublicKey{"k": pub}
	parts := strings.Split(a, ".")
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		t.Fatal("claims")
	}
	for _, mutation := range []func(map[string]any){
		func(c map[string]any) { c["exp"] = now.Add(time.Hour).Unix() },
		func(c map[string]any) { c["scope"] = "write_orders" },
		func(c map[string]any) { c["aud"] = "other-client" },
	} {
		var c map[string]any
		json.Unmarshal(raw, &c)
		mutation(c)
		data, _ := json.Marshal(c)
		input := parts[0] + "." + base64.RawURLEncoding.EncodeToString(data)
		bad := input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(input)))
		if _, err = embedded.Verify(bad, keys, "issuer", "client", id, now); err == nil {
			t.Fatal("invalid signed contract accepted")
		}
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err = embedded.Verify(a, map[string]ed25519.PublicKey{"k": other}, "issuer", "client", id, now); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err = embedded.Issue(nil, "k", "issuer", "client", id, now); err == nil {
		t.Fatal("invalid signer")
	}
}
