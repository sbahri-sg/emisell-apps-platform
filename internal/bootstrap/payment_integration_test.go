package bootstrap_test

import (
	"bytes"
	"context"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/appapi"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

func paymentSetup(t *testing.T) (*remoteFixture, string, appapi.Resource) {
	t.Helper()
	f := remoteSetup(t)
	ins := f.install(t)
	f.connect(t, ins)
	f.expect(t, "POST", f.installPath(f.tenant), action("activate", "remote-pay", nil), key(), 200)
	data := f.expect(t, "POST", f.capPath(f.tenant, "payment"), map[string]any{"operation": "create", "reference": "phase5-order", "amountMinor": 12000, "currency": "IDR"}, key(), 200)
	raw, _ := json.Marshal(data["resource"])
	var resource appapi.Resource
	if err := json.Unmarshal(raw, &resource); err != nil {
		t.Fatal(err)
	}
	return f, ins, resource
}
func callback(t *testing.T, f *remoteFixture, tenant, ins, delivery, secret string, update appapi.PaymentUpdate, want int) string {
	t.Helper()
	raw, _ := json.Marshal(update)
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	req, _ := http.NewRequest("POST", f.server.URL+"/api/v1/app-callbacks/payment/v1", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emisell-Tenant", tenant)
	req.Header.Set("X-Emisell-Installation", ins)
	req.Header.Set("X-Emisell-Delivery", delivery)
	req.Header.Set("X-Emisell-Timestamp", stamp)
	req.Header.Set("X-Emisell-Signature", appapi.SignCallback(secret, tenant, ins, delivery, stamp, raw))
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("callback got %d want %d", resp.StatusCode, want)
	}
	var result map[string]string
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result["outcome"]
}
func TestPaymentCallbacksIsolationOrderingAndUninstall(t *testing.T) {
	f, ins, r := paymentSetup(t)
	ctx := context.Background()
	secret := f.access(t, ins).WebhookSecret
	update := appapi.PaymentUpdate{Type: appapi.PaymentUpdateType, Resource: r, OccurredAt: time.Now().UTC()}
	update.Resource.Status = "captured"
	update.Resource.Revision = 2
	callback(t, f, f.tenant, ins, key(), "forged", update, 401)
	callback(t, f, f.other, ins, key(), secret, update, 404)
	bad := update
	bad.Resource.ID = ids.New("foreign")
	callback(t, f, f.tenant, ins, key(), secret, bad, 404)
	bad = update
	bad.Resource.AmountMinor++
	callback(t, f, f.tenant, ins, key(), secret, bad, 409)
	bad = update
	bad.OccurredAt = time.Now().Add(time.Minute)
	callback(t, f, f.tenant, ins, key(), secret, bad, 400)
	delivery := key()
	if got := callback(t, f, f.tenant, ins, delivery, secret, update, 200); got != "applied" {
		t.Fatal(got)
	}
	if got := callback(t, f, f.tenant, ins, delivery, secret, update, 200); got != "duplicate" {
		t.Fatal(got)
	}
	bad = update
	bad.Resource.Status = "refunded"
	callback(t, f, f.tenant, ins, delivery, secret, bad, 409)
	callback(t, f, f.tenant, ins, key(), secret, bad, 409) // same version, different state
	bad = update
	bad.Resource = r
	if got := callback(t, f, f.tenant, ins, key(), secret, bad, 200); got != "stale" {
		t.Fatal(got)
	}
	bad.Resource.Revision = 9
	callback(t, f, f.tenant, ins, key(), secret, bad, 409) // higher revision cannot regress
	path := "/api/v1/workspaces/" + f.tenant + "/payments/" + r.ID
	d := f.expect(t, "GET", path, nil, "", 200)
	if d["status"] != "captured" || len(d["history"].([]any)) != 2 {
		t.Fatal("incorrect projection/history", d)
	}
	f.expect(t, "GET", "/api/v1/workspaces/"+f.other+"/payments/"+r.ID, nil, "", 404)
	f.expect(t, "GET", "/api/v1/workspaces/"+ids.New("foreign")+"/payments", nil, "", 404)
	f.expect(t, "GET", "/api/v1/workspaces/"+f.tenant+"/payments?status=failed", nil, "", 400)
	page := f.expect(t, "GET", "/api/v1/workspaces/"+f.other+"/payments", nil, "", 200)
	if len(page["items"].([]any)) != 0 {
		t.Fatal("cross tenant list leak")
	}
	// Browser/session credentials are not callback authentication.
	f.expect(t, "POST", "/api/v1/app-callbacks/payment/v1", update, "", 403)
	// Storage outage cannot acknowledge a status it has not persisted.
	deadPool, err := pgxpool.New(ctx, os.Getenv("EMISELL_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	deadPool.Close()
	dead := httptest.NewServer(bootstrap.Handler(f.pool, deadPool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), f.connections))
	defer dead.Close()
	live := f.server
	f.server = dead
	callback(t, f, f.tenant, ins, key(), secret, update, 500)
	f.server = live
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_capability.events WHERE tenant_id=$1 AND envelope->>'type'='emisell.payment.status_changed.v1'", f.tenant).Scan(&count); err != nil || count != 2 {
		t.Fatal("duplicated business event", count, err)
	}
	f.expect(t, "POST", f.installPath(f.tenant), action("uninstall", "remote-pay", nil), key(), 200)
	callback(t, f, f.tenant, ins, key(), secret, update, 409)
	f.expect(t, "GET", path, nil, "", 200) // audit retained even after disable
}

func TestRemotePaymentCallbackRetryIsDurable(t *testing.T) {
	f, ins, r := paymentSetup(t)
	ctx := context.Background()
	// Initial notification agrees with the synchronous create response.
	if got, err := f.reference.DeliverOne(ctx, f.server.URL); err != nil || got != "delivered" {
		t.Fatal(got, err)
	}
	result, err := f.reference.Simulate(ctx, f.tenant, ins, r.ID, "capture", "phase5-local-simulation-key")
	if err != nil || result.Resource.Status != "captured" {
		t.Fatal(result, err)
	}
	if _, err = f.reference.Simulate(ctx, f.tenant, ins, r.ID, "capture", "phase5-local-simulation-key"); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/workspaces/" + f.tenant + "/payments/" + r.ID
	if got := f.expect(t, "GET", path, nil, "", 200)["status"]; got != "authorized" {
		t.Fatal("platform changed before callback", got)
	}
	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer unavailable.Close()
	if got, err := f.reference.DeliverOne(ctx, unavailable.URL); err != nil || got != "pending" {
		t.Fatal(got, err)
	}
	forceDue := func() {
		t.Helper()
		if _, err := f.pool.Exec(ctx, "UPDATE reference_remote.callbacks SET next_at=now() WHERE tenant_id=$1 AND status='pending'", f.tenant); err != nil {
			t.Fatal(err)
		}
	}
	forceDue()
	// Receiver commits but the response is lost. Only the transport must retry.
	lost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), f.connections).ServeHTTP(rec, r)
		if rec.Code != 200 {
			t.Errorf("receiver failed before lost ACK: %d", rec.Code)
		}
		w.WriteHeader(503)
	}))
	defer lost.Close()
	if got, err := f.reference.DeliverOne(ctx, lost.URL); err != nil || got != "pending" {
		t.Fatal(got, err)
	}
	forceDue()
	if got, err := f.reference.DeliverOne(ctx, f.server.URL); err != nil || got != "delivered" {
		t.Fatal(got, err)
	}
	d := f.expect(t, "GET", path, nil, "", 200)
	if d["status"] != "captured" || len(d["history"].([]any)) != 2 {
		t.Fatal("lost ACK duplicated status event", d)
	}
	var count int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM reference_remote.callbacks WHERE tenant_id=$1", f.tenant).Scan(&count); err != nil || count != 2 {
		t.Fatal("simulation retry duplicated callback", count, err)
	}
	// Refund may reach the receiver before a status poll; a later captured
	// snapshot cannot overwrite it, and notifications persist independently.
	if _, err = f.reference.Simulate(ctx, f.tenant, ins, r.ID, "refund", "phase5-refund-simulation-key"); err != nil {
		t.Fatal(err)
	}
	if got, err := f.reference.DeliverOne(ctx, f.server.URL); err != nil || got != "delivered" {
		t.Fatal(got, err)
	}
	if got := f.expect(t, "GET", path, nil, "", 200)["status"]; got != "refunded" {
		t.Fatal(got)
	}
}
