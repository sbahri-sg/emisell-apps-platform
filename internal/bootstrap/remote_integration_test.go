package bootstrap_test

import (
	"bytes"
	"context"
	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/event/natsbus"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/platform/secretbox"
	"emisell.app/platform/internal/referenceapp"
	"emisell.app/platform/internal/webhook"
	hookrepo "emisell.app/platform/internal/webhook/postgres"
	"emisell.app/platform/pkg/appapi"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/oauth2"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type remoteFixture struct {
	reference referenceapp.Server
	*fixture
	connections                      *oauth.Service
	hooks                            webhook.Service
	provider                         *httptest.Server
	config                           localfiles.RemoteConfig
	lostInvoke, lostHook, downRevoke atomic.Bool
}

// Each remote fixture owns a different provider/key. Limit its test worker's
// candidate selection to its tenant while retaining the real repository for
// enqueue, locked delivery, audit and replay. Concurrent browser fixtures and
// retained rows from earlier runs must not be dispatched through another fixture.
type fixtureWebhookQueue struct {
	hookrepo.Repository
	Tenant string
}

func (q fixtureWebhookQueue) Candidate(ctx context.Context) (webhook.Delivery, error) {
	var d webhook.Delivery
	err := q.Pool.QueryRow(ctx, "SELECT tenant_id,installation_id,id FROM platform_webhook.deliveries WHERE tenant_id=$1 AND status='pending' AND next_at<=now() ORDER BY next_at,id LIMIT 1", q.Tenant).Scan(&d.Tenant, &d.Installation, &d.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return d, err
}

func remoteSetup(t *testing.T) *remoteFixture {
	t.Helper()
	f := setup(t)
	ctx := context.Background()
	connPool, err := pgxpool.New(ctx, os.Getenv("EMISELL_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connPool.Close)
	if err = referenceapp.Init(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
	if err = (apprepo.Postgres{Pool: f.pool}).SeedRemote(ctx); err != nil {
		t.Fatal(err)
	}
	client, provider := localfiles.NewRemoteConfigs()
	box, err := secretbox.New(provider.EncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	result := &remoteFixture{fixture: f}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/installations/revoke" && result.downRevoke.Load() {
			w.WriteHeader(503)
			return
		}
		h := (referenceapp.Server{Pool: connPool, Config: provider, Box: &box}).Handler()
		if (r.URL.Path == "/v1/capabilities/invoke" && result.lostInvoke.Swap(false)) || (r.URL.Path == "/v1/webhooks" && result.lostHook.Swap(false)) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != 200 {
				t.Errorf("expected success before simulated lost response, got %d", rec.Code)
			}
			w.WriteHeader(503)
			return
		}
		h.ServeHTTP(w, r)
	})
	result.provider = httptest.NewServer(handler)
	t.Cleanup(result.provider.Close)
	client.Origin = result.provider.URL
	client.AuthorizationURL = strings.Replace(result.provider.URL, "127.0.0.1", "localhost", 1) + "/oauth/authorize"
	provider.Origin = client.Origin
	provider.AuthorizationURL = client.AuthorizationURL
	result.reference = referenceapp.Server{Pool: connPool, Config: provider, Box: &box}
	result.config = client
	result.connections, err = bootstrap.Connections(f.pool, connPool, client)
	if err != nil {
		t.Fatal(err)
	}
	result.hooks = webhook.Service{Repo: fixtureWebhookQueue{Repository: hookrepo.Repository{Pool: f.caps}, Tenant: f.tenant}, Connections: result.connections}
	t.Cleanup(func() {
		// Retain audit/history, but do not leave failed test queues for another run.
		_, _ = f.pool.Exec(context.Background(), "UPDATE platform_webhook.deliveries SET status='cancelled' WHERE tenant_id IN ($1,$2) AND status='pending'", f.tenant, f.other)
		_, _ = f.pool.Exec(context.Background(), "UPDATE platform_installation.installations SET cleanup_next_at='infinity' WHERE tenant_id IN ($1,$2) AND status='disabling'", f.tenant, f.other)
	})
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), result.connections))
	t.Cleanup(f.server.Close)
	return result
}
func (f *remoteFixture) install(t *testing.T) string {
	data := f.expect(t, "POST", f.installPath(f.tenant), action("install", "remote-pay", payScopes), ids.New("key"), 200)
	return data["installation"].(map[string]any)["id"].(string)
}
func (f *remoteFixture) consent(t *testing.T, ins string) (string, string) {
	t.Helper()
	data := f.expect(t, "POST", f.installPath(f.tenant)+"/"+ins+"/oauth", map[string]any{}, ids.New("key"), 200)
	target, err := url.Parse(data["authorizationUrl"].(string))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", f.provider.URL+target.RequestURI(), nil)
	response, err := f.provider.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("consent not available", response.StatusCode)
	}
	code := regexp.MustCompile(`name="code" value="([^"]+)"`).FindSubmatch(body)
	form := regexp.MustCompile(`name="form" value="([^"]+)"`).FindSubmatch(body)
	if len(code) != 2 || len(form) != 2 {
		t.Fatal("missing consent form")
	}
	browser := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ = http.NewRequest("POST", f.provider.URL+"/oauth/authorize", strings.NewReader(url.Values{"code": {string(code[1])}, "form": {string(form[1])}, "decision": {"allow"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", strings.TrimSuffix(f.config.AuthorizationURL, "/oauth/authorize"))
	response, err = browser.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 303 {
		t.Fatal("consent approval failed", response.StatusCode)
	}
	callback, _ := url.Parse(response.Header.Get("Location"))
	return callback.Query().Get("state"), callback.Query().Get("code")
}
func (f *remoteFixture) connect(t *testing.T, ins string) {
	t.Helper()
	state, code := f.consent(t, ins)
	if _, err := f.connections.Callback(context.Background(), f.user, f.cookie.Value, state, code); err != nil {
		t.Fatal("OAuth callback", err)
	}
}
func (f *remoteFixture) access(t *testing.T, ins string) oauth.Connected {
	t.Helper()
	var result oauth.Connected
	err := f.connections.Gate.WithInstallation(context.Background(), f.tenant, ins, func(domain.Installation) error {
		var err error
		result, err = f.connections.Access(context.Background(), f.tenant, ins)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestRemoteOAuthInvocationWebhooksAndCleanup(t *testing.T) {
	f := remoteSetup(t)
	ctx := context.Background()
	ins := f.install(t)
	f.expect(t, "POST", f.installPath(f.tenant), action("activate", "remote-pay", nil), ids.New("key"), 409)
	// Begin is owner/session/tenant-bound; a request for the other workspace cannot reuse the installation.
	f.expect(t, "POST", f.installPath(f.other)+"/"+ins+"/oauth", map[string]any{}, ids.New("key"), 404)
	state, code := f.consent(t, ins)
	if _, err := f.connections.Callback(ctx, f.user, "different-session", state, code); !errors.Is(err, fault.Invalid) {
		t.Fatal("cross-session callback accepted", err)
	}
	if _, err := f.connections.Callback(ctx, f.user, f.cookie.Value, state, code); err != nil {
		t.Fatal(err)
	}
	if _, err := f.connections.Callback(ctx, f.user, f.cookie.Value, state, code); !errors.Is(err, fault.Invalid) {
		t.Fatal("state replay accepted", err)
	}
	access := f.access(t, ins)
	var encrypted []byte
	if err := f.pool.QueryRow(ctx, "SELECT secret FROM platform_oauth.connections WHERE tenant_id=$1 AND installation_id=$2", f.tenant, ins).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted, []byte(access.AccessToken)) || bytes.Contains(encrypted, []byte(access.WebhookSecret)) {
		t.Fatal("plaintext connection stored")
	}
	f.expect(t, "POST", f.installPath(f.tenant), action("activate", "remote-pay", nil), ids.New("key"), 200)
	request := map[string]any{"operation": "create", "reference": "remote-reference-test", "amountMinor": 12000, "currency": "IDR"}
	key := ids.New("key")
	f.lostInvoke.Store(true)
	f.expect(t, "POST", f.capPath(f.tenant, "payment"), request, key, 503)
	result := f.expect(t, "POST", f.capPath(f.tenant, "payment"), request, key, 200)
	resource := result["resource"].(map[string]any)["id"].(string)
	repeated := f.expect(t, "POST", f.capPath(f.tenant, "payment"), request, key, 200)
	if repeated["resource"].(map[string]any)["id"] != resource {
		t.Fatal("remote retry duplicated result")
	}
	for _, op := range []string{"capture", "refund", "status"} {
		f.expect(t, "POST", f.capPath(f.tenant, "payment"), map[string]any{"operation": op, "resourceId": resource}, ids.New("key"), 200)
	}
	f.expect(t, "POST", f.capPath(f.other, "payment"), request, ids.New("key"), 404)
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM reference_remote.resources WHERE tenant_id=$1 AND installation_id=$2", f.tenant, ins).Scan(&count); err != nil || count != 1 {
		t.Fatal("remote idempotency", count, err)
	}
	var e event.Envelope
	if err := f.pool.QueryRow(ctx, "SELECT envelope FROM platform_capability.events WHERE tenant_id=$1 AND envelope->>'type'='emisell.capability.invoked.v1' ORDER BY occurred_at LIMIT 1", f.tenant).Scan(&e); err != nil {
		t.Fatal(err)
	}
	if err := f.hooks.Ingest(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := f.hooks.Ingest(ctx, e); err != nil {
		t.Fatal(err)
	}
	f.lostHook.Store(true)
	outcome, err := f.hooks.DeliverOne(ctx)
	if err != nil || outcome != "pending" {
		t.Fatal("ambiguous webhook should retry", outcome, err)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE platform_webhook.deliveries SET next_at=now() WHERE tenant_id=$1", f.tenant); err != nil {
		t.Fatal(err)
	}
	outcome, err = f.hooks.DeliverOne(ctx)
	if err != nil || outcome != "delivered" {
		t.Fatal("webhook retry", outcome, err)
	}
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM reference_remote.audit WHERE tenant_id=$1 AND installation_id=$2 AND action='webhook_received'", f.tenant, ins).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate webhook side effect", count, err)
	}
	// Force expiration of the encrypted client token, exercising actual rotating refresh.
	raw, err := f.connections.Box.Open("connection:"+f.tenant+":"+ins, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Token         oauth2.Token `json:"token"`
		WebhookSecret string       `json:"webhookSecret"`
	}
	if err = json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	oldRefresh := stored.Token.RefreshToken
	stored.Token.Expiry = time.Now().Add(-time.Minute)
	raw, _ = json.Marshal(stored)
	if err = f.connections.Repo.Save(ctx, f.tenant, ins, f.user, "connected", f.connections.Box.Seal("connection:"+f.tenant+":"+ins, raw)); err != nil {
		t.Fatal(err)
	}
	rotated := f.access(t, ins)
	if rotated.AccessToken == access.AccessToken {
		t.Fatal("token not rotated")
	}
	// Reuse an old refresh token: provider revokes the entire grant family.
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {oldRefresh}}
	req, _ := http.NewRequest("POST", f.provider.URL+"/oauth/token", strings.NewReader(form.Encode()))
	req.SetBasicAuth(f.config.ClientID, f.config.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := f.provider.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal("refresh replay accepted")
	}
	req, _ = http.NewRequest("GET", f.provider.URL+"/v1/connection", nil)
	req.Header.Set("Authorization", "Bearer "+rotated.AccessToken)
	response, err = f.provider.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("refresh replay did not revoke family")
	}
	f.connect(t, ins)
	late := event.New("emisell.capability.invoked.v1", f.tenant, f.user, ins, ids.New("req"), map[string]string{"capability": "payment/v1", "operation": "status"})
	if err = f.hooks.Ingest(ctx, late); err != nil {
		t.Fatal(err)
	}
	// Uninstall clears routing immediately even while provider cleanup is unavailable.
	result = f.expect(t, "POST", f.installPath(f.tenant), action("uninstall", "remote-pay", nil), ids.New("key"), 200)
	if result["installation"].(map[string]any)["status"] != "disabling" {
		t.Fatal("premature uninstall completion")
	}
	f.expect(t, "POST", f.capPath(f.tenant, "payment"), request, ids.New("key"), 404)
	f.expect(t, "POST", f.installPath(f.tenant)+"/"+ins+"/oauth", map[string]any{}, ids.New("key"), 409)
	f.expect(t, "POST", f.installPath(f.tenant), action("complete_uninstall", "remote-pay", nil), ids.New("key"), 400)
	cleanup := installrepo.Cleanup{Tenant: f.tenant, ID: ins, App: "remote-pay"}
	repo := installrepo.Repository{Pool: f.pool}
	f.downRevoke.Store(true)
	if err = repo.CompleteCleanup(ctx, cleanup, f.connections.Revoke); err == nil {
		t.Fatal("cleanup outage ignored")
	}
	status, _, err := f.connections.Repo.Load(ctx, f.tenant, ins)
	if err != nil || status != "revoked" {
		t.Fatal("local access not revoked", err)
	}
	f.downRevoke.Store(false)
	if err = repo.CompleteCleanup(ctx, cleanup, f.connections.Revoke); err != nil {
		t.Fatal(err)
	}
	outcome, err = f.hooks.DeliverOne(ctx)
	if err != nil || outcome != "cancelled" {
		t.Fatal("webhook sent after uninstall", outcome, err)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE platform_webhook.deliveries SET status='dead' WHERE tenant_id=$1 AND event_id=$2", f.tenant, e.ID); err != nil {
		t.Fatal(err)
	}
	deliveries, err := f.hooks.List(ctx, f.tenant)
	if err != nil {
		t.Fatal(err)
	}
	var deadID string
	for _, d := range deliveries {
		if d.Status == "dead" {
			deadID = d.ID
		}
	}
	if err = f.hooks.Replay(ctx, f.other, deadID, "wrong tenant replay"); !errors.Is(err, fault.NotFound) {
		t.Fatal("cross tenant replay", err)
	}
	if err = f.hooks.Replay(ctx, f.tenant, deadID, "verified local test retry"); err != nil {
		t.Fatal(err)
	}
	outcome, err = f.hooks.DeliverOne(ctx)
	if err != nil || outcome != "cancelled" {
		t.Fatal("replay bypassed uninstall", outcome, err)
	}
	newID := f.install(t)
	if newID == ins {
		t.Fatal("reinstall reused identity")
	}
	// Signature includes tenant, installation, delivery, timestamp and raw payload.
	body, _ := json.Marshal(e)
	timestamp := time.Now().Format(time.RFC3339)
	if appapi.VerifyWebhook(access.WebhookSecret, f.tenant, ins, "delivery", timestamp, "invalid", body, time.Now()) {
		t.Fatal("invalid signature accepted")
	}
}

func TestRemoteWebhookScopeRevokedBeforeDelivery(t *testing.T) {
	f := remoteSetup(t)
	ctx := context.Background()
	ins := f.install(t)
	f.connect(t, ins)
	f.expect(t, "POST", f.installPath(f.tenant), action("activate", "remote-pay", nil), ids.New("key"), 200)
	e := event.New("emisell.capability.invoked.v1", f.tenant, f.user, ins, ids.New("req"), map[string]string{"capability": "payment/v1", "operation": "status"})
	if err := f.hooks.Ingest(ctx, e); err != nil {
		t.Fatal(err)
	}
	// Simulate a current scope reduction after enqueue while status stays active.
	if _, err := f.pool.Exec(ctx, `UPDATE platform_installation.installations SET scopes='[]'::jsonb WHERE tenant_id=$1 AND id=$2`, f.tenant, ins); err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.hooks.DeliverOne(ctx); err != nil || outcome != "cancelled" {
		t.Fatal("scope revocation bypassed", outcome, err)
	}
	var received int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM reference_remote.audit WHERE tenant_id=$1 AND installation_id=$2 AND action='webhook_received'`, f.tenant, ins).Scan(&received); err != nil || received != 0 {
		t.Fatal("unauthorized receiver effect", received, err)
	}
	e.ID = ids.New("evt")
	if err := f.hooks.Ingest(ctx, e); err != nil {
		t.Fatal(err)
	}
	items, err := f.hooks.List(ctx, f.tenant)
	if err != nil || len(items) != 1 {
		t.Fatal("revoked scope enqueued new event", len(items), err)
	}
}

func TestRemoteWebhookJetStreamDurability(t *testing.T) {
	broker := brokerSetup(t)
	f := remoteSetup(t)
	ctx := context.Background()
	ins := f.install(t)
	f.connect(t, ins)
	f.expect(t, "POST", f.installPath(f.tenant), action("activate", "remote-pay", nil), ids.New("key"), 200)
	nc, err := natsbus.Connect(broker.config.Worker)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err = natsbus.Provision(ctx, js, []string{"local-store"}); err != nil {
		t.Fatal(err)
	}
	consumer, err := webhook.Provision(ctx, js)
	if err != nil {
		t.Fatal(err)
	}
	e := event.New("emisell.capability.invoked.v1", f.tenant, f.user, ins, ids.New("req"), map[string]string{"capability": "payment/v1", "operation": "status"})
	if err = (natsbus.Publisher{JS: js}).Publish(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err = f.hooks.RouteBatch(ctx, consumer); err != nil {
		t.Fatal("durable route/ACK", err)
	}
	broker.stop(t)
	broker.start(t)
	deadline := time.Now().Add(8 * time.Second)
	for !nc.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	consumer, err = webhook.Provision(ctx, js)
	if err != nil {
		t.Fatal(err)
	}
	info, err := consumer.Info(ctx)
	if err != nil || info.NumAckPending != 0 {
		t.Fatal("webhook ACK did not persist", err)
	}
	items, err := f.hooks.List(ctx, f.tenant)
	if err != nil || len(items) != 1 {
		t.Fatal("queue not durable", len(items), err)
	}
	outcome, err := f.hooks.DeliverOne(ctx)
	if err != nil || outcome != "delivered" {
		t.Fatal("delivery after broker restart", outcome, err)
	}
}

func TestRemoteOAuthExpiryPKCECodeReplayAndTenantBinding(t *testing.T) {
	f := remoteSetup(t)
	ctx := context.Background()
	ins := f.install(t)
	url1, err := f.connections.Begin(ctx, f.user, f.cookie.Value, f.tenant, ins, "same-oauth-begin-key")
	if err != nil {
		t.Fatal(err)
	}
	url2, err := f.connections.Begin(ctx, f.user, f.cookie.Value, f.tenant, ins, "same-oauth-begin-key")
	if err != nil || url1 != url2 {
		t.Fatal("begin retry not stable", err)
	}
	state, code := f.consent(t, ins)
	if _, err = f.pool.Exec(ctx, "UPDATE platform_oauth.states SET expires_at=now()-interval '1 second' WHERE state_hash=$1", oauth.Hash(state)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.connections.Callback(ctx, f.user, f.cookie.Value, state, code); !errors.Is(err, fault.Invalid) {
		t.Fatal("expired state accepted", err)
	}
	state, code = f.consent(t, ins)
	exchange := func(code, verifier string) int {
		form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {f.config.CallbackURL}, "code_verifier": {verifier}}
		req, _ := http.NewRequest("POST", f.provider.URL+"/oauth/token", strings.NewReader(form.Encode()))
		req.SetBasicAuth(f.config.ClientID, f.config.ClientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := f.provider.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if exchange(code, strings.Repeat("a", 43)) != 400 {
		t.Fatal("incorrect PKCE accepted")
	}
	if _, err = f.connections.Callback(ctx, f.user, f.cookie.Value, state, code); err != nil {
		t.Fatal("valid code after failed PKCE", err)
	}
	if exchange(code, strings.Repeat("a", 43)) != 400 {
		t.Fatal("used code accepted")
	}
	connection := f.access(t, ins)
	raw, _ := json.Marshal(appapi.Invocation{TenantID: f.other, InstallationID: ins, Capability: "payment/v1", IdempotencyKey: ids.New("key"), Request: appapi.Request{Operation: "create", Reference: "forbidden", AmountMinor: 1, Currency: "IDR"}})
	req, _ := http.NewRequest("POST", f.provider.URL+"/v1/capabilities/invoke", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+connection.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := f.provider.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("provider tenant binding not enforced")
	}
	req, _ = http.NewRequest("GET", f.server.URL+"/api/v1/oauth/callback?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(code), nil)
	req.AddCookie(f.cookie)
	browser := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Timeout: 3 * time.Second}
	response, err = browser.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	location := response.Header.Get("Location")
	if response.StatusCode != 303 || !strings.Contains(location, "oauth=failed") || strings.Contains(location, code) || strings.Contains(location, state) {
		t.Fatal("callback did not clean sensitive parameters")
	}
}
