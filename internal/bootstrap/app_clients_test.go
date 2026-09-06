package bootstrap_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/review"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type proofVerifier struct {
	calls   atomic.Int32
	fail    atomic.Bool
	entered chan struct{}
	proceed chan struct{}
}

func (v *proofVerifier) Verify(ctx context.Context, url string, p appclient.Proof) error {
	v.calls.Add(1)
	if !strings.HasPrefix(url, "https://app.example.com/.well-known/emisell-app-verification/proof_") || p.Schema != appclient.ProofSchema || len(p.Challenge) != 43 {
		return errors.New("bad proof")
	}
	if v.entered != nil {
		close(v.entered)
		select {
		case <-v.proceed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if v.fail.Load() {
		return errors.New("remote body must not leak")
	}
	return nil
}
func clientFixture(t *testing.T, v *proofVerifier, embeddedConfig ...bootstrap.EmbeddedReviewConfig) (*fixture, *fixture, *fixture, *fixture) {
	f := setup(t)
	clientPool, err := pgxpool.NewWithConfig(context.Background(), f.pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(clientPool.Close)
	_, signer, _ := ed25519.GenerateKey(rand.Reader)
	f.server.Close()
	if len(embeddedConfig) > 0 {
		f.server = httptest.NewServer(bootstrap.HandlerWithEmbeddedReviews(f.pool, f.caps, clientPool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), review.IntegrationSigner{Key: signer}, v, embeddedConfig[0]))
	} else {
		f.server = httptest.NewServer(bootstrap.HandlerWithAppClients(f.pool, f.caps, clientPool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, review.IntegrationSigner{Key: signer}, v))
	}
		t.Cleanup(f.server.Close)
	return f, portalAccount(t, f, "developer", "developer"), portalAccount(t, f, "developer", "developer"), portalAccount(t, f, "admin", "administrator")
}
func signedClientRelease(t *testing.T, dev, admin *fixture) string {
	b := integrationInput(t, dev, admin, "shipping/v1", nil)
	id := pexpect(t, dev, "POST", "/api/v1/developer/integration-releases", b, key(), 200)["release"].(map[string]any)["id"].(string)
	path := "/api/v1/admin/integration-releases/" + id + "/status"
	pexpect(t, admin, "POST", path, service.IntegrationAction{Status: "approved", Revision: 1, Reason: "Config reviewed"}, key(), 200)
	pexpect(t, admin, "POST", path, service.IntegrationAction{Status: "signed", Revision: 2, Reason: "Config signed"}, key(), 200)
	return id
}
func clientView(t *testing.T, data map[string]any) appclient.View {
	t.Helper()
	raw, _ := json.Marshal(data["view"])
	var v appclient.View
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func clientCheck(t *testing.T, f *fixture, id, secret string, want int) {
	t.Helper()
	req, _ := http.NewRequest("POST", f.server.URL+"/api/v1/app/client-check", bytes.NewBufferString("{}"))
	req.SetBasicAuth(id, secret)
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != want {
		t.Fatal("client auth", r.StatusCode, want)
	}
	if want == 200 {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b["oauthEnabled"] != false || b["installable"] != false || b["resourceGatewayAllowed"] != false {
			t.Fatal("client check elevated access")
		}
	}
}
func TestAppClientRegistrationProofSecretIsolation(t *testing.T) {
	verifier := &proofVerifier{}
	f, dev, other, admin := clientFixture(t, verifier)
	release := signedClientRelease(t, dev, admin)
	path := "/api/v1/developer/app-clients"
	body := appclient.Input{ReleaseID: release}
	createKey := key()
	pexpect(t, other, "POST", path, body, key(), 404)
	v := clientView(t, pexpect(t, dev, "POST", path, body, createKey, 200))
	id := v.Client.ID
	if v.Ready || v.Installable || v.OAuthEnabled || verifier.calls.Load() != 0 {
		t.Fatal("auto activated/probed")
	}
	pexpect(t, dev, "POST", path, body, key(), 409)
	if clientView(t, pexpect(t, dev, "POST", path, body, createKey, 200)).Client.ID != id {
		t.Fatal("create replay")
	}
	pexpect(t, other, "GET", path+"/"+id, nil, "", 404)
	pexpect(t, other, "POST", path+"/"+id+"/actions", appclient.Action{Action: "revoke", Revision: 1, Reason: "Foreign"}, key(), 404)
	api := path + "/" + id + "/actions"
	pexpect(t, admin, "POST", "/api/v1/admin/app-clients/"+id+"/actions", appclient.Action{Action: "rotate_secret", Revision: 1, Reason: "No admin secret"}, key(), 403)
	pexpect(t, dev, "POST", api, appclient.Action{Action: "rotate_secret", Revision: 1, Reason: "Cannot issue before proof"}, key(), 409)
	verifyKey := key()
	verifyAction := appclient.Action{Action: "verify", Revision: 1, Reason: "I control the reviewed endpoint"}
	v = clientView(t, pexpect(t, dev, "POST", api, verifyAction, verifyKey, 200))
	if v.Client.Status != "verified" || v.Ready {
		t.Fatal("proof state", v.Client.Status)
	}
	pexpect(t, dev, "POST", api, verifyAction, verifyKey, 200)
	if verifier.calls.Load() != 1 {
		t.Fatal("proof replay sent network again")
	}
	rotateKey := key()
	rotate := appclient.Action{Action: "rotate_secret", Revision: v.Client.Revision, Reason: "Issue backend credential"}
	issued := pexpect(t, dev, "POST", api, rotate, rotateKey, 200)
	secret := issued["secret"].(string)
	if len(secret) != 48 || issued["secretAvailable"] != true {
		t.Fatal("secret not issued")
	}
	if replay := pexpect(t, dev, "POST", api, rotate, rotateKey, 200); replay["secret"] != "" || replay["secretAvailable"] != false {
		t.Fatal("secret leaked on retry")
	}
	read := pexpect(t, dev, "GET", path+"/"+id, nil, "", 200)
	encoded, _ := json.Marshal(read)
	if bytes.Contains(encoded, []byte(secret)) || bytes.Contains(encoded, []byte("secretHash")) {
		t.Fatal("secret/hash in read DTO")
	}
	clientCheck(t, f, id, secret, 200)
	clientCheck(t, f, id, "epk_"+strings.Repeat("x", 43), 401)
	v = clientView(t, read)
	rotate.Revision = v.Client.Revision
	rotated := pexpect(t, dev, "POST", api, rotate, key(), 200)
	second := rotated["secret"].(string)
	clientCheck(t, f, id, secret, 401)
	clientCheck(t, f, id, second, 200)
	// Neither owner login cookies nor browser-origin requests can check credentials.
	status, _, _, _ := dev.call("POST", "/api/v1/app/client-check", map[string]any{}, "", developerOrigin)
	if status != 403 {
		t.Fatal("browser auth allowed", status)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE platform_oauth.app_clients SET binding='{}',revision=revision+1 WHERE id=$1`, id); err == nil {
		t.Fatal("mutable binding")
	}
	// Release suspension blocks authentication without needing cross-schema writes.
	pexpect(t, admin, "POST", "/api/v1/admin/integration-releases/"+release+"/status", service.IntegrationAction{Status: "suspended", Revision: 3, Reason: "Security pause"}, key(), 200)
	clientCheck(t, f, id, second, 401)
	v = clientView(t, pexpect(t, dev, "GET", path+"/"+id, nil, "", 200))
	if v.Ready {
		t.Fatal("suspended release readiness")
	}
	pexpect(t, admin, "POST", "/api/v1/admin/app-clients/"+id+"/actions", appclient.Action{Action: "revoke", Revision: v.Client.Revision, Reason: "Revoke without release availability"}, key(), 200)
	clientCheck(t, f, id, second, 401)
}
func TestAppClientProbeRevokeRace(t *testing.T) {
	verifier := &proofVerifier{entered: make(chan struct{}), proceed: make(chan struct{})}
	f, dev, _, admin := clientFixture(t, verifier)
	release := signedClientRelease(t, dev, admin)
	v := clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients", appclient.Input{ReleaseID: release}, key(), 200))
	path := "/api/v1/developer/app-clients/" + v.Client.ID
	done := make(chan int, 1)
	go func() {
		status, _, _, _ := dev.call("POST", path+"/actions", appclient.Action{Action: "verify", Revision: 1, Reason: "Verify pending"}, key(), developerOrigin)
		done <- status
	}()
	<-verifier.entered
	v = clientView(t, pexpect(t, dev, "GET", path, nil, "", 200))
	if v.Client.LastResult != "checking" {
		t.Fatal("missing pending evidence")
	}
	pexpect(t, dev, "POST", path+"/actions", appclient.Action{Action: "revoke", Revision: v.Client.Revision, Reason: "Cancel while probe in flight"}, key(), 200)
	close(verifier.proceed)
	if got := <-done; got != 409 {
		t.Fatal("stale probe committed", got)
	}
	v = clientView(t, pexpect(t, dev, "GET", path, nil, "", 200))
	if v.Client.Status != "revoked" || v.Ready {
		t.Fatal("revocation resurrected")
	}
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM platform_oauth.app_client_audit WHERE client_id=$1 AND action='verified'`, v.Client.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("stale verification audit")
	}
}
func TestAppClientProofExpiryFailureRenewal(t *testing.T) {
	verifier := &proofVerifier{}
	verifier.fail.Store(true)
	f, dev, _, admin := clientFixture(t, verifier)
	release := signedClientRelease(t, dev, admin)
	v := clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients", appclient.Input{ReleaseID: release}, key(), 200))
	path := "/api/v1/developer/app-clients/" + v.Client.ID
	actionPath := path + "/actions"
	v = clientView(t, pexpect(t, dev, "POST", actionPath, appclient.Action{Action: "verify", Revision: 1, Reason: "Verify fixture failure"}, key(), 200))
	if v.Client.LastResult != "failed" || v.Client.Status != "pending" {
		t.Fatal("failed proof elevated")
	}
	pexpect(t, dev, "POST", actionPath, appclient.Action{Action: "verify", Revision: v.Client.Revision, Reason: "Cooldown"}, key(), 409)
	_, err := f.pool.Exec(context.Background(), `UPDATE platform_oauth.app_clients SET last_attempt_at=now()-interval '2 minutes',challenge_expires_at=now()-interval '1 second',revision=revision+1 WHERE id=$1`, v.Client.ID)
	if err != nil {
		t.Fatal(err)
	}
	v = clientView(t, pexpect(t, dev, "GET", path, nil, "", 200))
	pexpect(t, dev, "POST", actionPath, appclient.Action{Action: "verify", Revision: v.Client.Revision, Reason: "Expired"}, key(), 409)
	old := v.Client.Challenge
	v = clientView(t, pexpect(t, dev, "POST", actionPath, appclient.Action{Action: "challenge", Revision: v.Client.Revision, Reason: "Renew proof"}, key(), 200))
	if old == v.Client.Challenge {
		t.Fatal("nonce reused")
	}
	verifier.fail.Store(false)
	v = clientView(t, pexpect(t, dev, "POST", actionPath, appclient.Action{Action: "verify", Revision: v.Client.Revision, Reason: "Check renewed proof"}, key(), 200))
	issued := pexpect(t, dev, "POST", actionPath, appclient.Action{Action: "rotate_secret", Revision: v.Client.Revision, Reason: "Issue"}, key(), 200)
	secret := issued["secret"].(string)
	clientCheck(t, f, v.Client.ID, secret, 200)
	_, err = f.pool.Exec(context.Background(), `UPDATE platform_oauth.app_clients SET verified_until=now()-interval '1 second',revision=revision+1 WHERE id=$1`, v.Client.ID)
	if err != nil {
		t.Fatal(err)
	}
	clientCheck(t, f, v.Client.ID, secret, 401)
	v = clientView(t, pexpect(t, dev, "GET", path, nil, "", 200))
	if v.Ready {
		t.Fatal("expired proof still ready")
	}
	pexpect(t, dev, "POST", actionPath, appclient.Action{Action: "rotate_secret", Revision: v.Client.Revision, Reason: "Expired evidence"}, key(), 409)
}
