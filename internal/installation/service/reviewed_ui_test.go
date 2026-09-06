package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/pkg/embedded"
)

func TestReviewedUIConsentBinding(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	for _, mode := range []string{"embedded", "external"} {
		launch := embedded.Launch{AppID: "app_ui", ClientID: "client_ui", ReleaseDigest: strings.Repeat("a", 64), URL: "https://ui.example/app", ParentOrigin: "https://core.example", Mode: mode}
		sig, err := embedded.SignLaunch(launch, key, false)
		if err != nil {
			t.Fatal(err)
		}
		r := domain.IntentRelease{InstallPolicy: domain.ReviewedUIPolicy, ExecutionProfile: domain.ReviewedUIPolicy, AppID: launch.AppID, Version: "1.0.0", ManifestDigest: launch.ReleaseDigest, Scopes: []string{}, Capabilities: []string{}, UIBinding: &domain.UIBinding{ReleaseID: "release_ui", Launch: launch, Signature: sig}}
		s := Lifecycle{ReviewedUIKey: pub}
		ctx := context.WithValue(context.Background(), managedReleaseKey{}, r)
		if err := s.eligible(ctx, r); err != nil {
			t.Fatal(mode, err)
		}
		if s.eligible(context.Background(), r) == nil {
			t.Fatal("missing live source accepted")
		}
		if (Lifecycle{}).eligible(ctx, r) == nil {
			t.Fatal("missing key accepted")
		}
		for _, mutate := range []func(*domain.IntentRelease){
			func(v *domain.IntentRelease) { v.Scopes = []string{"read_orders"} },
			func(v *domain.IntentRelease) { v.Capabilities = []string{"shipping/v1"} },
			func(v *domain.IntentRelease) { v.UIBinding.Launch.URL = "https://evil.example/app" },
			func(v *domain.IntentRelease) { v.UIBinding.Launch.ClientID = "other" },
			func(v *domain.IntentRelease) { v.ManifestDigest = strings.Repeat("b", 64) },
			func(v *domain.IntentRelease) { v.UIBinding.Signature = "invalid" },
		} {
			v := r
			b := *r.UIBinding
			v.UIBinding = &b
			mutate(&v)
			if s.eligible(ctx, v) == nil {
				t.Fatal("consent drift allowed")
			}
			if s.eligible(context.WithValue(ctx, managedReleaseKey{}, v), v) == nil {
				t.Fatal("invalid source allowed")
			}
		}
		intent := domain.NewInstallIntent("intent", domain.IntentOwner{TenantID: "merchant", ServiceID: "core", ActorID: "staff"}, r, time.Now())
		r.UIBinding.Launch.URL = "https://changed.example/app"
		if intent.Release.UIBinding.Launch.URL != launch.URL {
			t.Fatal("mutable consent binding")
		}
	}
}
