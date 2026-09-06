package embedded_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	service "emisell.app/platform/internal/oauth/embedded"
	"emisell.app/platform/pkg/embedded"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIdentityIsBoundFreshAndRevocable(t *testing.T) {
	pub, private, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Unix(1788690000, 0)
	id := embedded.Identity{MerchantID: "merchant-a", ActorID: "staff-a", AppID: "app-a", InstallationID: "installation-a"}
	denied := errors.New("revoked")
	var gateErr error
	checks := 0
	s := service.Service{PrivateKey: private, Keys: map[string]ed25519.PublicKey{"key-a": pub}, KeyID: "key-a", Issuer: "https://platform.example", Now: func() time.Time { return now }, Authorize: func(_ context.Context, got embedded.Identity, audience string) error {
		checks++
		if got != id || audience != "client-a" {
			return denied
		}
		return gateErr
	}}
	ctx := context.Background()
	token, err := s.Issue(ctx, id, "client-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, token, "client-a", id); err != nil {
		t.Fatal(err)
	}
	gateErr = denied
	if _, err = s.Authenticate(ctx, token, "client-a", id); !errors.Is(err, denied) {
		t.Fatal("revocation bypass", err)
	}
	if _, err = s.Issue(ctx, id, "client-a"); !errors.Is(err, denied) {
		t.Fatal("issued after revoke", err)
	}
	gateErr = service.ErrUnavailable
	if _, err = s.Authenticate(ctx, token, "client-a", id); !errors.Is(err, service.ErrUnavailable) {
		t.Fatal("outage bypass", err)
	}
	gateErr = nil
	if checks != 5 {
		t.Fatal("grant cache", checks)
	}
	for _, aud := range []string{"client-b", ""} {
		if _, err = s.Authenticate(ctx, token, aud, id); err == nil {
			t.Fatal("audience bypass")
		}
	}
	for _, other := range []embedded.Identity{{MerchantID: "merchant-b", ActorID: id.ActorID, AppID: id.AppID, InstallationID: id.InstallationID}, {MerchantID: id.MerchantID, ActorID: "staff-b", AppID: id.AppID, InstallationID: id.InstallationID}, {MerchantID: id.MerchantID, ActorID: id.ActorID, AppID: "app-b", InstallationID: id.InstallationID}, {MerchantID: id.MerchantID, ActorID: id.ActorID, AppID: id.AppID, InstallationID: "replacement"}} {
		if _, err = s.Authenticate(ctx, token, "client-a", other); err == nil {
			t.Fatal("identity bypass")
		}
	}
	for _, bad := range []string{"", "eat_old", strings.Repeat("a", 4097), token + "x", "eyJhbGciOiJub25lIn0.e30."} {
		if _, err = s.Authenticate(ctx, bad, "client-a", id); err == nil {
			t.Fatal("invalid token accepted")
		}
	}
	now = now.Add(time.Minute)
	if _, err = s.Authenticate(ctx, token, "client-a", id); err == nil {
		t.Fatal("expired token")
	}
}

func TestMissingGateAndWrongTrustFailClosed(t *testing.T) {
	pub, private, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	id := embedded.Identity{MerchantID: "m", ActorID: "u", AppID: "a", InstallationID: "i"}
	token, err := embedded.Issue(private, "k", "issuer", "client", id, now)
	if err != nil {
		t.Fatal(err)
	}
	s := service.Service{Keys: map[string]ed25519.PublicKey{"k": pub}, Issuer: "issuer"}
	if _, err = s.Authenticate(context.Background(), token, "client", id); !errors.Is(err, service.ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err = s.Issue(context.Background(), id, "client"); !errors.Is(err, service.ErrUnavailable) {
		t.Fatal(err)
	}
	for _, issuer := range []string{"other", ""} {
		if _, err = embedded.Verify(token, s.Keys, issuer, "client", id, now); err == nil {
			t.Fatal("issuer bypass")
		}
	}
	if _, err = embedded.Verify(token, nil, "issuer", "client", id, now); err == nil {
		t.Fatal("unknown key")
	}
	if _, err = embedded.Verify(token, s.Keys, "issuer", "client", id, now.Add(-time.Second)); err == nil {
		t.Fatal("future token")
	}
}
