package embedded_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	service "emisell.app/platform/internal/oauth/embedded"
	contract "emisell.app/platform/pkg/embedded"
)

func TestExternalLaunchDoesNotIssueIdentity(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	id := contract.Identity{MerchantID: "merchant", ActorID: "staff", AppID: "app", InstallationID: "installation"}
	metadata := contract.Launch{AppID: "app", ClientID: "client", ReleaseDigest: strings.Repeat("a", 64), URL: "https://app.example/dashboard", ParentOrigin: "https://core.example", Mode: "external"}
	sig, err := contract.SignLaunch(metadata, key, false)
	if err != nil {
		t.Fatal(err)
	}
	allowed := true
	checks := 0
	l := service.Launcher{LaunchKey: pub, Resolve: func(context.Context, contract.Identity) (service.Binding, error) {
		return service.Binding{Launch: metadata, Signature: sig}, nil
	}, Identity: service.Service{Authorize: func(_ context.Context, got contract.Identity, audience string) error {
		checks++
		if !allowed || got != id || audience != "client" {
			return service.ErrDenied
		}
		return nil
	}}}
	// No identity private key: external links must not require or mint a token.
	out, err := l.External(context.Background(), id, metadata.ParentOrigin)
	if err != nil || out != metadata || checks != 1 {
		t.Fatal("external launch failed", err)
	}
	if _, err = l.Open(context.Background(), id, metadata.ParentOrigin); err == nil {
		t.Fatal("external mode embedded")
	}
	if _, err = l.External(context.Background(), id, "https://foreign.example"); err == nil {
		t.Fatal("foreign parent accepted")
	}
	allowed = false
	if _, err = l.External(context.Background(), id, metadata.ParentOrigin); err == nil {
		t.Fatal("revoked access accepted")
	}
	l.Identity.Authorize = nil
	if _, err = l.External(context.Background(), id, metadata.ParentOrigin); err == nil {
		t.Fatal("missing authorization accepted")
	}
}
