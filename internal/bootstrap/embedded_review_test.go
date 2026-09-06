package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/oauth/appclient"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/oauth/embedded"
	launchrepo "emisell.app/platform/internal/oauth/embedded/postgres"
	"testing"
)

func TestEmbeddedReviewAuthenticatedPortal(t *testing.T) {
	_, keyMaterial, _ := ed25519.GenerateKey(rand.Reader)
	f, dev, other, admin := clientFixture(t, &proofVerifier{}, bootstrap.EmbeddedReviewConfig{Key: keyMaterial, ParentOrigin: "https://core.example"})
	release := signedClientRelease(t, dev, admin)
	v := clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients", appclient.Input{ReleaseID: release}, key(), 200))
	base := "/api/v1/developer/embedded-launches"
	b := embedded.LaunchInput{ClientID: v.Client.ID, URL: "https://app.example.com/embedded", Reason: "Review embedded launch"}
	lookup := "/api/v1/developer/app-clients/" + v.Client.ID + "/launch"
	if pexpect(t, dev, "GET", lookup, nil, "", 200)["launch"] != nil {
		t.Fatal("unexpected initial launch")
	}
	pexpect(t, other, "GET", lookup, nil, "", 404)
	pexpect(t, dev, "POST", base, b, key(), 409)
	actions := "/api/v1/developer/app-clients/" + v.Client.ID + "/actions"
	v = clientView(t, pexpect(t, dev, "POST", actions, appclient.Action{Action: "verify", Revision: 1, Reason: "proof"}, key(), 200))
	pexpect(t, dev, "POST", actions, appclient.Action{Action: "rotate_secret", Revision: v.Client.Revision, Reason: "credential"}, key(), 200)
	pexpect(t, other, "POST", base, b, key(), 404)
	bad := b
	bad.URL = "https://foreign.example/iframe"
	pexpect(t, dev, "POST", base, bad, key(), 400)
	bad = b
	bad.Mode = "untrusted-mode"
	pexpect(t, dev, "POST", base, bad, key(), 400)
	b.Mode = "external"
	created := pexpect(t, dev, "POST", base, b, key(), 200)
	if created["launchable"] != false {
		t.Fatal("review opened merchant launch")
	}
	id := created["launch"].(map[string]any)["id"].(string)
	if pexpect(t, dev, "GET", lookup, nil, "", 200)["launch"].(map[string]any)["id"] != id {
		t.Fatal("client lookup mismatch")
	}
	if created["launch"].(map[string]any)["binding"].(map[string]any)["mode"] != "external" {
		t.Fatal("mode not persisted")
	}
	bad = b
	bad.Mode = "embedded"
	pexpect(t, dev, "POST", base, bad, key(), 409)
	pexpect(t, other, "GET", base+"/"+id, nil, "", 404)
	path := "/api/v1/admin/embedded-launches/" + id + "/status"
	action := map[string]any{"revision": 1, "status": "approved", "reason": "Launch reviewed"}
	pexpect(t, admin, "POST", path, action, key(), 200)
	pexpect(t, admin, "POST", path, action, key(), 200)
	// Exercise the persistent OAuth gate independently of distribution. This
	// does not authorize installation of this shipping release as a UI app.
	gate := embedded.CurrentBinding{
		Clients:  appclient.Service{Repo: clientrepo.Repository{Pool: f.pool}},
		Launches: launchrepo.Repository{Pool: f.pool},
		Key:      keyMaterial.Public().(ed25519.PublicKey), ParentOrigin: "https://core.example",
	}
	called := false
	check := func(binding embedded.Binding) error {
		called = true
		if binding.Launch.URL != b.URL || binding.Launch.Mode != "external" {
			t.Fatal("wrong persisted launch")
		}
		return nil
	}
	if err := gate.WithBinding(context.Background(), v.Client.ID, v.Client.Binding, check); err != nil || !called {
		t.Fatal("persistent gate", err)
	}
	gate.ParentOrigin = "https://other.example"
	called = false
	if err := gate.WithBinding(context.Background(), v.Client.ID, v.Client.Binding, check); err == nil || called {
		t.Fatal("parent mismatch accepted")
	}
	gate.ParentOrigin = "https://core.example"
	pexpect(t, admin, "POST", path, map[string]any{"revision": 2, "status": "revoked", "reason": "Disable launch"}, key(), 200)
	if err := gate.WithBinding(context.Background(), v.Client.ID, v.Client.Binding, check); err == nil || called {
		t.Fatal("revoked review accepted")
	}
	pexpect(t, admin, "POST", path, action, key(), 409)
}
