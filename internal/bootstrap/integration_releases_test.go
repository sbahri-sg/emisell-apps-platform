package bootstrap_test

import (
	"connectrpc.com/connect"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/integrationmanifest"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func integrationInput(t *testing.T, dev, admin *fixture, capability string, scopes *accessscope.Declaration) service.IntegrationInput {
	t.Helper()
	doc := portalDocument()
	doc.Endpoint = "https://app.example.com/emisell/v1"
	doc.AccessScopes = scopes
	doc.Capability = capability
	if capability == "shipping/v1" {
		doc.Scopes = []string{"orders.read", "shipping.read", "shipping.write"}
	}
	d := pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: doc}, key(), 200)["app"].(map[string]any)
	sub := pexpect(t, dev, "POST", "/api/v1/developer/apps/"+d["id"].(string)+"/submissions", map[string]int{"revision": 1}, key(), 200)["submission"].(map[string]any)["id"].(string)
	b := service.IntegrationInput{SubmissionID: sub, Config: integrationmanifest.Config{Protocol: integrationmanifest.Protocol, Endpoint: doc.Endpoint, CallbackURL: "https://app.example.com/oauth/callback", HealthURL: "https://app.example.com/health"}}
	pexpect(t, dev, "POST", "/api/v1/developer/integration-releases/validate", b, "", 409)
	pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+sub+"/decision", review.Decision{Status: "approved", Feedback: "Metadata only; separate configuration review required"}, key(), 200)
	return b
}
func TestIntegrationReleasePipeline(t *testing.T) {
	f := setup(t)
	public, signingKey, _ := ed25519.GenerateKey(rand.Reader)
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.HandlerWithReleases(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, review.IntegrationSigner{Key: signingKey}))
	t.Cleanup(f.server.Close)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	reviewer := portalAccount(t, f, "admin", "reviewer")
	operator := portalAccount(t, f, "admin", "operator")
	for _, capability := range []string{"shipping/v1"} {
		t.Run(capability, func(t *testing.T) {
			b := integrationInput(t, dev, admin, capability, nil)
			path := "/api/v1/developer/integration-releases"
			pexpect(t, other, "POST", path+"/validate", b, "", 404)
			bad := b
			bad.Config.Endpoint = "https://app.example.com/changed"
			if pexpect(t, dev, "POST", path+"/validate", bad, "", 200)["validation"].(map[string]any)["valid"] != false {
				t.Fatal("approved endpoint drift")
			}
			pexpect(t, dev, "POST", path, bad, key(), 400)
			pexpect(t, dev, "POST", path, b, "", 400)
			k := key()
			created := pexpect(t, dev, "POST", path, b, k, 200)
			if !reflect.DeepEqual(created, pexpect(t, dev, "POST", path, b, k, 200)) {
				t.Fatal("submit replay")
			}
			pexpect(t, dev, "POST", path, b, key(), 409)
			bad = b
			bad.Config.HealthURL += "/other"
			pexpect(t, dev, "POST", path, bad, k, 409)
			id := created["release"].(map[string]any)["id"].(string)
			adminPath := "/api/v1/admin/integration-releases/" + id
			pexpect(t, other, "GET", path+"/"+id, nil, "", 404)
			for _, foreign := range pexpect(t, other, "GET", path, nil, "", 200)["releases"].([]any) {
				if foreign.(map[string]any)["id"] == id {
					t.Fatal("foreign release in list")
				}
			}
			pexpect(t, operator, "GET", adminPath, nil, "", 200)
			pexpect(t, f, "GET", adminPath, nil, "", 401)
			status, _, _, err := admin.call("GET", adminPath, nil, "", developerOrigin)
			if err != nil || status != 403 {
				t.Fatal("origin boundary", status, err)
			}
			action := service.IntegrationAction{Status: "approved", Revision: 1, Reason: "Separate configuration review completed"}
			pexpect(t, operator, "POST", adminPath+"/status", action, key(), 403)
			denied, _, _, _ := dev.call("POST", path+"/"+id+"/status", action, key(), developerOrigin)
			if denied != 404 {
				t.Fatal("developer review route exposed", denied)
			}
			pexpect(t, admin, "POST", adminPath+"/status", service.IntegrationAction{Status: "signed", Revision: 1, Reason: "Must review first"}, key(), 409)
			var wg sync.WaitGroup
			codes := make(chan int, 2)
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					code, _, _, _ := reviewer.call("POST", adminPath+"/status", action, key(), origin)
					codes <- code
				}()
			}
			wg.Wait()
			close(codes)
			counts := map[int]int{}
			for c := range codes {
				counts[c]++
			}
			if counts[200] != 1 || counts[409] != 1 {
				t.Fatal("concurrent decision", counts)
			}
			signAction := service.IntegrationAction{Status: "signed", Revision: 2, Reason: "Sign configuration, not hosted app code"}
			pexpect(t, reviewer, "POST", adminPath+"/status", signAction, key(), 403)
			missing := httptest.NewServer(bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil))))
			defer missing.Close()
			offline := *admin
			offline.server = missing
			pexpect(t, &offline, "POST", adminPath+"/status", signAction, key(), 503)
			unchanged := pexpect(t, admin, "GET", adminPath, nil, "", 200)
			if len(unchanged["history"].([]any)) != 2 || unchanged["release"].(map[string]any)["revision"] != float64(2) {
				t.Fatal("failed signing mutated state")
			}
			signKey := key()
			signed := pexpect(t, admin, "POST", adminPath+"/status", signAction, signKey, 200)
			var v service.IntegrationRelease
			raw, _ := json.Marshal(signed["release"])
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			if v.Package == nil || integrationmanifest.Verify(*v.Package, public) != nil {
				t.Fatal("persisted package invalid")
			}
			if signed["validation"].(map[string]any)["installable"] != false {
				t.Fatal("configuration enabled install")
			}
			unverified := pexpect(t, &offline, "GET", adminPath, nil, "", 200)["validation"].(map[string]any)
			if unverified["valid"] != false || unverified["installable"] != false {
				t.Fatal("missing key must fail verification")
			}
			if _, err := f.pool.Exec(context.Background(), `UPDATE platform_app.integration_releases SET manifest='{}' WHERE id=$1`, id); err == nil {
				t.Fatal("mutable snapshot")
			}
			if _, err := f.pool.Exec(context.Background(), `UPDATE platform_app.integration_releases SET package='{}' WHERE id=$1`, id); err == nil {
				t.Fatal("mutable signature")
			}
			core, _, _ := serviceClient(t, f, f.tenant, intentScopes())
			_, err = core.InstallIntents.Prepare(context.Background(), connect.NewRequest(&intent.PrepareRequest{CoreActorId: "integration-review-test", IdempotencyKey: key(), AppId: v.Manifest.Metadata.AppID, Version: v.Manifest.Metadata.Version}))
			rpcCode(t, err, connect.CodeNotFound)
			pexpect(t, admin, "GET", "/api/v1/store/apps/"+id, nil, "", 404)
			// Suspension works without key, remains non-installable, and old retries read current state.
			pexpect(t, &offline, "POST", adminPath+"/status", service.IntegrationAction{Status: "suspended", Revision: 3, Reason: "Suspend during signer outage"}, key(), 200)
			replayed := pexpect(t, admin, "POST", adminPath+"/status", signAction, signKey, 200)
			if replayed["release"].(map[string]any)["status"] != "suspended" {
				t.Fatal("retry resurrected signed state")
			}
			current := pexpect(t, dev, "POST", path, b, k, 200)
			if current["release"].(map[string]any)["status"] != "suspended" {
				t.Fatal("submit replay stale")
			}
			history := pexpect(t, admin, "GET", adminPath, nil, "", 200)["history"].([]any)
			if len(history) != 4 {
				t.Fatal("duplicate audit", len(history))
			}
		})
	}
}
func TestIntegrationResourceScopeGate(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	for _, required := range []bool{true, false} {
		d := &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{}, Optional: []string{}}
		if required {
			d.Required = []string{"write_products"}
		} else {
			d.Optional = []string{"read_products"}
		}
		b := integrationInput(t, dev, admin, "shipping/v1", d)
		report := pexpect(t, dev, "POST", "/api/v1/developer/integration-releases/validate", b, "", 200)["validation"].(map[string]any)
		if report["valid"] != !required || report["installable"] != false {
			t.Fatal("scope gate", report)
		}
		want := 200
		if required {
			want = 400
		}
		created := pexpect(t, dev, "POST", "/api/v1/developer/integration-releases", b, key(), want)
		if !required {
			id := created["release"].(map[string]any)["id"].(string)
			path := "/api/v1/admin/integration-releases/" + id + "/status"
			pexpect(t, admin, "POST", path, service.IntegrationAction{Status: "rejected", Revision: 1, Reason: "Needs endpoint ownership evidence"}, key(), 200)
			pexpect(t, admin, "POST", path, service.IntegrationAction{Status: "approved", Revision: 2, Reason: "Cannot resurrect rejected version"}, key(), 409)
		}
	}
}
