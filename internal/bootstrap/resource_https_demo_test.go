package bootstrap_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/webhookconfig"
)

// Demo-only signed enrollment. No catalog, merchant or client is provisioned in
// a real store. The production source still needs its persisted client lifecycle.
type signedResourceDemo struct {
	merchant     string
	release      domain.IntentRelease
	public       ed25519.PublicKey
	signature    []byte
	clientActive bool
}

func (s *signedResourceDemo) ReadyResource(_ context.Context, r domain.IntentRelease) error {
	raw, _ := json.Marshal(r)
	if !s.clientActive || !ed25519.Verify(s.public, raw, s.signature) {
		return fault.Forbidden
	}
	return nil
}
func (s *signedResourceDemo) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if merchant != s.merchant || app != s.release.AppID || version != s.release.Version {
		return fault.Forbidden
	}
	if err := s.ReadyResource(ctx, s.release); err != nil {
		return err
	}
	return fn(s.release)
}

func TestResourceHTTPSDemoEndToEnd(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	_, principal, _, _ := lifecycleClient(t, f)
	const secret = "synthetic-demo-webhook-secret-only"
	var attempts atomic.Int32
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		if !appapi.VerifyWebhook(secret, r.Header.Get("X-Emisell-Tenant"), r.Header.Get("X-Emisell-Installation"), r.Header.Get("X-Emisell-Delivery"), r.Header.Get("X-Emisell-Timestamp"), r.Header.Get("X-Emisell-Signature"), raw, time.Now()) {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("X-Emisell-Tenant") != f.tenant {
			t.Error("wrong merchant")
		}
		if attempts.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(204)
	}))
	defer receiver.Close()
	// TLS remains enabled and verified. The explicit test client trusts only this
	// generated test certificate. Production SendHTTPS never allows loopback.
	client := receiver.Client()
	client.Timeout = 5 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	r := domain.IntentRelease{AppID: "app_demo_" + key(), Name: "Synthetic HTTPS demo", DeveloperID: "demo", Version: "1.0.0", ManifestDigest: service.RequestHash("synthetic-release"), InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy, Scopes: []string{"read_products"}, Capabilities: []string{}, ResourceBinding: &domain.ResourceBinding{ReleaseID: "demo_release", ClientID: "demo_client", AccessScopes: accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{"read_orders"}}, Webhooks: &webhookconfig.Config{APIVersion: webhookconfig.APIVersion, Subscriptions: []webhookconfig.Subscription{{Topics: []string{"products.created"}, URI: "https://demo.emisell.com/events"}}}}}
	raw, _ := json.Marshal(r)
	source := &signedResourceDemo{merchant: f.tenant, release: r, public: public, signature: ed25519.Sign(private, raw), clientActive: true}
	repo := installrepo.Repository{Pool: f.pool}
	intents := service.Intents{Repo: repo, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, Managed: source}
	lifecycle := service.Lifecycle{Repo: repo, Intents: intents, Resources: source}
	source.signature[0] ^= 1
	if _, err := intents.Prepare(ctx, principal, "demo-staff", key(), service.PrepareIntent{AppID: r.AppID, Version: r.Version}); err == nil {
		t.Fatal("invalid release signature accepted")
	}
	source.signature[0] ^= 1
	source.clientActive = false
	if _, err := intents.Prepare(ctx, principal, "demo-staff", key(), service.PrepareIntent{AppID: r.AppID, Version: r.Version}); err == nil {
		t.Fatal("inactive demo client accepted")
	}
	source.clientActive = true
	intent, err := intents.Prepare(ctx, principal, "demo-staff", key(), service.PrepareIntent{AppID: r.AppID, Version: r.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.Consume(ctx, principal, "demo-staff", key(), intent.ID, intent.ConsentDigest); err == nil {
		t.Fatal("consent bypass")
	}
	if _, err = intents.Decide(ctx, principal, "demo-staff", key(), service.DecideIntent{ID: intent.ID, Digest: intent.ConsentDigest, Decision: "consent"}); err != nil {
		t.Fatal(err)
	}
	consumed, err := lifecycle.Consume(ctx, principal, "demo-staff", key(), intent.ID, intent.ConsentDigest)
	if err != nil {
		t.Fatal(err)
	}
	id := consumed.Access.Installation.ID
	if _, err = lifecycle.Execute(ctx, principal, "demo-staff", key(), id, "activate"); err != nil {
		t.Fatal(err)
	}

	// Connection-scoped PostgreSQL temporary tables are destroyed on disconnect.
	// Producer writes its synthetic product and event in the same transaction.
	conn, err := f.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Conn().Close(context.Background()); conn.Release() }()
	if _, err = conn.Exec(ctx, `CREATE TEMP TABLE demo_products(id text PRIMARY KEY);
 CREATE TEMP TABLE demo_jobs(id text PRIMARY KEY,product_id text NOT NULL REFERENCES demo_products(id),status text NOT NULL,attempts int NOT NULL DEFAULT 0);`); err != nil {
		t.Fatal(err)
	}
	produce := func(eventID, productID string, rollback bool) error {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `INSERT INTO demo_products VALUES($1) ON CONFLICT DO NOTHING`, productID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO demo_jobs(id,product_id,status) VALUES($1,$2,'pending') ON CONFLICT DO NOTHING`, eventID, productID); err != nil {
			return err
		}
		var existing string
		if err = tx.QueryRow(ctx, `SELECT product_id FROM demo_jobs WHERE id=$1`, eventID).Scan(&existing); err != nil {
			return err
		}
		if existing != productID {
			return fault.Conflict
		}
		if rollback {
			return nil
		}
		return tx.Commit(ctx)
	}
	if err = produce("event_rolled_back", "product_rollback", true); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = produce("event_demo", "product_demo", false); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = produce("event_demo", "conflicting_product", false); err == nil {
		t.Fatal("event ID reused for different payload")
	}
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM demo_jobs`).Scan(&count); err != nil || count != 1 {
		t.Fatal("outbox atomicity/dedup", count, err)
	}
	dispatch := func(eventID string) error {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		var product, status string
		if err = tx.QueryRow(ctx, `SELECT product_id,status FROM demo_jobs WHERE id=$1 FOR UPDATE`, eventID).Scan(&product, &status); err != nil {
			return err
		}
		if status != "pending" {
			return nil
		}
		next := "pending"
		sent := 0
		err = lifecycle.WithResourceAccess(ctx, principal, "demo-staff", id, []string{"read_products"}, func(a domain.Access) error {
			hooks := a.Release.ResourceBinding.Webhooks
			if hooks == nil || len(hooks.Subscriptions) != 1 || hooks.Subscriptions[0].URI != "https://demo.emisell.com/events" {
				return fault.Forbidden
			}
			body, _ := json.Marshal(map[string]string{"eventId": eventID, "topic": "products.created", "productId": product, "apiVersion": hooks.APIVersion})
			request, err := http.NewRequestWithContext(ctx, "POST", receiver.URL, bytes.NewReader(body))
			if err != nil {
				return err
			}
			stamp := strconv.FormatInt(time.Now().Unix(), 10)
			request.Header.Set("X-Emisell-Tenant", f.tenant)
			request.Header.Set("X-Emisell-Installation", id)
			request.Header.Set("X-Emisell-Delivery", eventID)
			request.Header.Set("X-Emisell-Timestamp", stamp)
			request.Header.Set("X-Emisell-Signature", appapi.SignWebhook(secret, f.tenant, id, eventID, stamp, body))
			sent = 1
			response, err := client.Do(request)
			if err != nil {
				return err
			}
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				next = "delivered"
			}
			return nil
		})
		if err != nil {
			next = "cancelled"
		}
		if _, err = tx.Exec(ctx, `UPDATE demo_jobs SET status=$2,attempts=attempts+$3 WHERE id=$1`, eventID, next, sent); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	for range 3 {
		if err = dispatch("event_demo"); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	var tried int
	if err = conn.QueryRow(ctx, `SELECT status,attempts FROM demo_jobs WHERE id='event_demo'`).Scan(&status, &tried); err != nil || status != "delivered" || tried != 2 || attempts.Load() != 2 {
		t.Fatal("retry/dedup", status, tried, attempts.Load(), err)
	}
	if err = produce("event_after_revoke", "product_after_revoke", false); err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.Execute(ctx, principal, "demo-staff", key(), id, "uninstall"); err != nil {
		t.Fatal(err)
	}
	if err = dispatch("event_after_revoke"); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(ctx, `SELECT status,attempts FROM demo_jobs WHERE id='event_after_revoke'`).Scan(&status, &tried); err != nil || status != "cancelled" || tried != 0 || attempts.Load() != 2 {
		t.Fatal("revocation delivered data", status, tried, err)
	}
	t.Log("PASS: signed demo release → consent → persistent grant → transactional synthetic producer → TLS delivery → retry/dedup → uninstall cancellation")
}
