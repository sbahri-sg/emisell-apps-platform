package bootstrap_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/apptoken"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/sdk"
	v1 "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	pay "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
)

func lifecycleClient(t *testing.T, f *fixture) (*sdk.Client, identity.ServicePrincipal, string, string) {
	t.Helper()
	admin := portalAccount(t, f, "admin", "administrator")
	created := pexpect(t, admin, "POST", "/api/v1/admin/platform-keys", map[string]any{"name": "Lifecycle test"}, key(), 200)
	token := created["secret"].(string)
	p, err := (identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}).Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	p, err = p.BindTenant(f.tenant)
	if err != nil {
		t.Fatal(err)
	}
	rpc := httptest.NewServer(bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(rpc.Close)
	c, err := sdk.NewLocalClient(rpc.URL, token)
	if err != nil {
		t.Fatal(err)
	}
	return c, p, token, rpc.URL
}

func lifecycleIntent(t *testing.T, c *sdk.Client, tenant, app string, decision v1.ConsentDecision) *v1.InstallIntent {
	t.Helper()
	ctx := context.Background()
	r, err := c.InstallIntents.Prepare(ctx, connect.NewRequest(&v1.PrepareRequest{TenantId: tenant, CoreActorId: "staff", IdempotencyKey: key(), AppId: app, Version: "1.0.0"}))
	if err != nil {
		t.Fatal(err)
	}
	if r.Msg.Intent.InstallationPolicy != domain.InstallPolicy {
		t.Fatal("missing policy binding")
	}
	if decision == v1.ConsentDecision_CONSENT_DECISION_UNSPECIFIED {
		return r.Msg.Intent
	}
	d := intentDecision(r.Msg.Intent, decision)
	d.Msg.TenantId = tenant
	v, err := c.InstallIntents.Decide(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	return v.Msg.Intent
}
func consumption(v *v1.InstallIntent) *connect.Request[v1.ConsumeRequest] {
	return connect.NewRequest(&v1.ConsumeRequest{TenantId: v.TenantId, CoreActorId: v.CoreActorId, IdempotencyKey: key(), IntentId: v.Id, ConsentDigest: v.ConsentDigest})
}
func installationRequest(tenant, id string) *v1.InstallationRequest {
	return &v1.InstallationRequest{TenantId: tenant, CoreActorId: "staff", InstallationId: id, IdempotencyKey: key()}
}
func issueTokenRequest(tenant, id string) *connect.Request[v1.IssueTokenRequest] {
	return connect.NewRequest(&v1.IssueTokenRequest{Target: installationRequest(tenant, id)})
}
func activateRequest(tenant, id string) *connect.Request[v1.ActivateRequest] {
	return connect.NewRequest(&v1.ActivateRequest{Target: installationRequest(tenant, id)})
}
func getInstallationRequest(tenant, id string) *connect.Request[v1.GetInstallationRequest] {
	return connect.NewRequest(&v1.GetInstallationRequest{Target: installationRequest(tenant, id)})
}
func uninstallRequest(tenant, id string) *connect.Request[v1.UninstallRequest] {
	return connect.NewRequest(&v1.UninstallRequest{Target: installationRequest(tenant, id)})
}
func consume(t *testing.T, c *sdk.Client, tenant, app string) *v1.InstallationAccess {
	t.Helper()
	v := lifecycleIntent(t, c, tenant, app, v1.ConsentDecision_CONSENT_DECISION_CONSENT)
	r, err := c.Installations.Consume(context.Background(), consumption(v))
	if err != nil {
		t.Fatal(err)
	}
	return r.Msg.Result.Installation
}
func appCheck(t *testing.T, f *fixture, token, tenant, app, id string, extra map[string]string) int {
	t.Helper()
	r, _ := http.NewRequest("GET", f.server.URL+"/api/v1/app/installation-access", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Emisell-Tenant-ID", tenant)
	r.Header.Set("X-Emisell-App-ID", app)
	r.Header.Set("X-Emisell-Installation-ID", id)
	for k, v := range extra {
		r.Header.Set(k, v)
	}
	res, err := f.server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if strings.Contains(string(body), token) || strings.Contains(string(body), "serviceId") || strings.Contains(string(body), "coreActorId") {
		t.Fatal("app DTO leaks privileged data")
	}
	return res.StatusCode
}

func TestConsentInstallationTokenUninstallEndToEnd(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, _, coreToken, rpcURL := lifecycleClient(t, f)
	for _, app := range []string{"emisell-pay", "parcel"} {
		t.Run(app, func(t *testing.T) {
			v := lifecycleIntent(t, c, f.tenant, app, v1.ConsentDecision_CONSENT_DECISION_CONSENT)
			req := consumption(v)
			res, err := c.Installations.Consume(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			a := res.Msg.Result.Installation
			if a.Status != "pending" || a.GrantState != "pending" || len(a.GrantedScopes) != 0 || res.Msg.Result.AppToken != "" {
				t.Fatal("consume activated prematurely")
			}
			_, err = c.Installations.IssueToken(ctx, issueTokenRequest(f.tenant, a.InstallationId))
			rpcCode(t, err, connect.CodeAlreadyExists)
			active, err := c.Installations.Activate(ctx, activateRequest(f.tenant, a.InstallationId))
			if err != nil {
				t.Fatal(err)
			}
			if active.Msg.Result.Installation.Status != "active" || !slices.Equal(active.Msg.Result.Installation.GrantedScopes, v.Scopes) {
				t.Fatal("grant mismatch")
			}
			// The existing provider-neutral resolver observes the actual installation.
			if app == "emisell-pay" {
				p, e := c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "lifecycle-pilot", AmountMinor: 1000, Currency: "IDR"}))
				if e != nil || p.Msg.InstallationId != a.InstallationId {
					t.Fatal("payment route", e)
				}
			} else {
				s, e := c.Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{TenantId: f.tenant, IdempotencyKey: key(), WeightGrams: 500, DestinationZone: "ID-JKT"}))
				if e != nil || s.Msg.InstallationId != a.InstallationId {
					t.Fatal("shipping route", e)
				}
			}
			issue := issueTokenRequest(f.tenant, a.InstallationId)
			token, err := c.Installations.IssueToken(ctx, issue)
			if err != nil {
				t.Fatal(err)
			}
			secret := token.Msg.Result.AppToken
			if !apptoken.Valid(secret) || token.Msg.Result.TokenAudience != apptoken.Audience {
				t.Fatal("token format/audience")
			}
			if got := appCheck(t, f, secret, f.tenant, app, a.InstallationId, nil); got != 200 {
				t.Fatal("self-check", got)
			}
			for _, tc := range []struct {
				token, tenant, app, id string
				headers                map[string]string
				want                   int
			}{
				{secret, f.other, app, a.InstallationId, nil, 401}, {secret, f.tenant, "another-app", a.InstallationId, nil, 401},
				{secret, f.tenant, app, "ins_other", nil, 401}, {coreToken, f.tenant, app, a.InstallationId, nil, 401},
				{secret, f.tenant, app, a.InstallationId, map[string]string{"Origin": origin}, 403},
				{secret, f.tenant, app, a.InstallationId, map[string]string{"Cookie": "browser=1"}, 403},
			} {
				if got := appCheck(t, f, tc.token, tc.tenant, tc.app, tc.id, tc.headers); got != tc.want {
					t.Fatal("identity/audience isolation", got, tc.want)
				}
			}
			// Raw app bearer is not a Core credential, even without the SDK prefix check.
			r, _ := http.NewRequest("POST", rpcURL+"/emisell.integration.v1.ConnectionService/Check", strings.NewReader(`{}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+secret)
			response, e := http.DefaultClient.Do(r)
			if e != nil {
				t.Fatal(e)
			}
			response.Body.Close()
			if response.StatusCode != 401 {
				t.Fatal("app token accepted by Core")
			}
			retry, err := c.Installations.IssueToken(ctx, issue)
			if err != nil || !retry.Msg.Result.Replayed || retry.Msg.Result.AppToken != "" || retry.Msg.Result.TokenId != token.Msg.Result.TokenId {
				t.Fatal("secret replay", err)
			}
			rotated, err := c.Installations.IssueToken(ctx, issueTokenRequest(f.tenant, a.InstallationId))
			if err != nil {
				t.Fatal(err)
			}
			if appCheck(t, f, secret, f.tenant, app, a.InstallationId, nil) != 401 || appCheck(t, f, rotated.Msg.Result.AppToken, f.tenant, app, a.InstallationId, nil) != 200 {
				t.Fatal("rotation not immediate")
			}
			// Compatibility endpoint cannot activate or mutate this managed aggregate.
			f.expect(t, "POST", f.installPath(f.tenant), action("uninstall", app, nil), key(), 403)
			uninstall := uninstallRequest(f.tenant, a.InstallationId)
			u, err := c.Installations.Uninstall(ctx, uninstall)
			if err != nil || u.Msg.Result.Installation.Status != "uninstalled" || u.Msg.Result.Installation.GrantState != "revoked" || len(u.Msg.Result.Installation.GrantedScopes) != 0 {
				t.Fatal("uninstall incomplete", err)
			}
			if appCheck(t, f, rotated.Msg.Result.AppToken, f.tenant, app, a.InstallationId, nil) != 401 {
				t.Fatal("uninstalled token remains valid")
			}
			for range 2 {
				if _, err = c.Installations.Uninstall(ctx, uninstallRequest(f.tenant, a.InstallationId)); err != nil {
					t.Fatal(err)
				}
			}
			replayed, err := c.Installations.Consume(ctx, req)
			if err != nil || replayed.Msg.Result.Installation.Status != "uninstalled" || !replayed.Msg.Result.Replayed {
				t.Fatal("consume retry resurrected installation", err)
			}
			retry, err = c.Installations.IssueToken(ctx, issue)
			if err != nil || !retry.Msg.Result.TokenInvalid || retry.Msg.Result.AppToken != "" {
				t.Fatal("token retry not current", err)
			}
			_, err = c.Installations.Activate(ctx, activateRequest(f.tenant, a.InstallationId))
			rpcCode(t, err, connect.CodeAlreadyExists)
			var audits int
			if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.access_audit WHERE tenant_id=$1 AND installation_id=$2`, f.tenant, a.InstallationId).Scan(&audits); err != nil || audits != 5 {
				t.Fatal("audit not exactly once", audits, err)
			}
			var hash string
			if err = f.pool.QueryRow(ctx, `SELECT token_hash FROM platform_installation.app_tokens WHERE id=$1`, token.Msg.Result.TokenId).Scan(&hash); err != nil || hash != apptoken.Hash(secret) {
				t.Fatal("token not hash-only", err)
			}
			// Fresh consent creates a NEW installation, and historical retries never touch it.
			newIns := consume(t, c, f.tenant, app)
			if newIns.InstallationId == a.InstallationId {
				t.Fatal("reinstall reused identity")
			}
			_, err = c.Installations.Uninstall(ctx, uninstall)
			if err != nil {
				t.Fatal(err)
			}
			fresh, err := c.Installations.GetInstallation(ctx, getInstallationRequest(f.tenant, newIns.InstallationId))
			if err != nil || fresh.Msg.Result.Installation.Status != "pending" {
				t.Fatal("old uninstall affected new installation", err)
			}
		})
	}
}

func TestConsentInstallationIsolationConcurrencyAndDeny(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, _, _, _ := lifecycleClient(t, f)
	for _, decision := range []v1.ConsentDecision{v1.ConsentDecision_CONSENT_DECISION_UNSPECIFIED, v1.ConsentDecision_CONSENT_DECISION_DENY} {
		v := lifecycleIntent(t, c, f.tenant, "emisell-pay", decision)
		_, err := c.Installations.Consume(ctx, consumption(v))
		rpcCode(t, err, connect.CodeAlreadyExists)
	}
	v := lifecycleIntent(t, c, f.tenant, "emisell-pay", v1.ConsentDecision_CONSENT_DECISION_CONSENT)
	wrong := consumption(v)
	wrong.Msg.ConsentDigest = strings.Repeat("0", 64)
	_, err := c.Installations.Consume(ctx, wrong)
	rpcCode(t, err, connect.CodeAlreadyExists)
	for _, tc := range []struct{ tenant, actor string }{{f.other, "staff"}, {f.tenant, "another-staff"}} {
		r := consumption(v)
		r.Msg.TenantId = tc.tenant
		r.Msg.CoreActorId = tc.actor
		_, err = c.Installations.Consume(ctx, r)
		rpcCode(t, err, connect.CodeNotFound)
	}
	other, _, _, _ := lifecycleClient(t, f)
	_, err = other.Installations.Consume(ctx, consumption(v))
	rpcCode(t, err, connect.CodeNotFound)
	legacy, _, _ := serviceClient(t, f, f.tenant, intentScopes())
	_, err = legacy.Installations.Consume(ctx, consumption(v))
	rpcCode(t, err, connect.CodePermissionDenied)
	requestKey := key()
	var wg sync.WaitGroup
	results := make(chan *v1.LifecycleResponse, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			r := consumption(v)
			r.Msg.IdempotencyKey = requestKey
			res, e := c.Installations.Consume(ctx, r)
			if res != nil {
				results <- res.Msg.Result
			} else {
				results <- nil
			}
			errs <- e
		})
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var ins string
	for r := range results {
		if ins == "" {
			ins = r.Installation.InstallationId
		}
		if ins != r.Installation.InstallationId {
			t.Fatal("duplicate concurrent consume")
		}
	}
	_, err = c.Installations.Consume(ctx, consumption(v))
	rpcCode(t, err, connect.CodeAlreadyExists)
	_, err = other.Installations.GetInstallation(ctx, getInstallationRequest(f.tenant, ins))
	rpcCode(t, err, connect.CodeNotFound)
	_, err = c.Installations.GetInstallation(ctx, getInstallationRequest(f.other, ins))
	rpcCode(t, err, connect.CodeNotFound)
	for _, header := range []string{"Origin", "Cookie"} {
		r := activateRequest(f.tenant, ins)
		r.Header().Set(header, "browser")
		if _, err = c.Installations.Activate(ctx, r); err == nil {
			t.Fatal("browser context accepted")
		}
	}
	if _, err = c.Installations.Activate(ctx, activateRequest(f.tenant, ins)); err != nil {
		t.Fatal(err)
	}
	// Concurrent identical issuance delivers its secret only once.
	requestKey = key()
	results = make(chan *v1.LifecycleResponse, 8)
	errs = make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			r := issueTokenRequest(f.tenant, ins)
			r.Msg.Target.IdempotencyKey = requestKey
			res, e := c.Installations.IssueToken(ctx, r)
			if res != nil {
				results <- res.Msg.Result
			} else {
				results <- nil
			}
			errs <- e
		})
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	secrets := 0
	var token string
	for r := range results {
		if r.AppToken != "" {
			secrets++
			token = r.AppToken
		}
	}
	if secrets != 1 {
		t.Fatal("secret released more than once")
	}
	// Test expiry through a controlled test-only clock update within the TTL constraint.
	_, err = f.pool.Exec(ctx, `UPDATE platform_installation.app_tokens SET created_at=now()-interval '16 minutes',expires_at=now()-interval '1 minute' WHERE tenant_id=$1 AND installation_id=$2`, f.tenant, ins)
	if err != nil {
		t.Fatal(err)
	}
	if appCheck(t, f, token, f.tenant, "emisell-pay", ins, nil) != 401 {
		t.Fatal("expired token accepted")
	}
	// Same capability cannot be activated twice; rollback leaves the second grant pending.
	second := consume(t, c, f.tenant, "emisell-pay-alt")
	_, err = c.Installations.Activate(ctx, activateRequest(f.tenant, second.InstallationId))
	rpcCode(t, err, connect.CodeAlreadyExists)
	got, err := c.Installations.GetInstallation(ctx, getInstallationRequest(f.tenant, second.InstallationId))
	if err != nil || got.Msg.Result.Installation.GrantState != "pending" {
		t.Fatal("failed activation left active grant", err)
	}
}

func TestConsentInstallationPolicyExpiryAndRollback(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, p, _, _ := lifecycleClient(t, f)
	registry := &changingIntentRegistry{Registry: appservice.Registry{Repo: apprepo.Postgres{Pool: f.caps}}}
	intents := installservice.Intents{Repo: installrepo.Repository{Pool: f.pool}, Apps: registry, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}}
	s := installservice.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}, Intents: intents}
	v := lifecycleIntent(t, c, f.tenant, "emisell-pay", v1.ConsentDecision_CONSENT_DECISION_CONSENT)
	registry.changed = true
	if _, err := s.Consume(ctx, p, "staff", key(), v.Id, v.ConsentDigest); !errors.Is(err, fault.Conflict) {
		t.Fatal("changed manifest accepted", err)
	}
	registry.changed = false
	owner := domain.IntentOwner{TenantID: p.TenantID, ServiceID: p.ID, ActorID: "staff"}
	original, err := intents.Get(ctx, p, "staff", v.Id)
	if err != nil {
		t.Fatal(err)
	}
	// Old consent-only policy and expired consent cannot be silently upgraded.
	for _, historical := range []bool{true, false} {
		release := original.Release
		now := time.Now()
		if historical {
			release.InstallPolicy = ""
		} else {
			now = now.Add(-11 * time.Minute)
		}
		old := domain.NewInstallIntent(ids.New("intent"), owner, release, now)
		old, err = old.Decide(old.ConsentDigest, "consent", now.Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(old)
		_, err = f.pool.Exec(ctx, `INSERT INTO platform_installation.install_intents(id,tenant_id,service_id,actor_id,snapshot,state,created_at,expires_at,decided_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, old.ID, p.TenantID, p.ID, "staff", raw, old.State, old.CreatedAt, old.ExpiresAt, old.DecidedAt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Consume(ctx, p, "staff", key(), old.ID, old.ConsentDigest); !errors.Is(err, fault.Conflict) {
			t.Fatal("historical or expired consent promoted", err)
		}
	}
	// Force an audit write failure after aggregate writes; no installation, grant,
	// consumption or receipt may survive. Only isolated test data is affected.
	repo := installrepo.Repository{Pool: f.pool}
	// Instead inject the digest after repository validation through the change port.
	a := consume(t, c, f.tenant, "emisell-pay")
	_, err = repo.ChangeAccess(ctx, owner, key(), "audit-failure", a.InstallationId, "activate", "", "", func(v domain.Access, _ time.Time) (domain.Access, error) {
		v.Installation.Status = "active"
		v.GrantState = "active"
		v.GrantedScopes = v.Release.Scopes
		v.ConsentDigest = "\x00"
		return v, nil
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	got, err := s.Get(ctx, p, "staff", a.InstallationId)
	if err != nil || got.Installation.Status != "pending" || got.GrantState != "pending" {
		t.Fatal("atomic rollback failed", err)
	}
}

func TestConsentInstallationRemoteHandshakeAndCleanup(t *testing.T) {
	f := remoteSetup(t)
	ctx := context.Background()
	c, _, coreToken, _ := lifecycleClient(t, f.fixture)
	a := consume(t, c, f.tenant, "remote-pay")
	_, err := c.Installations.Activate(ctx, activateRequest(f.tenant, a.InstallationId))
	rpcCode(t, err, connect.CodeUnavailable)
	// Retain the existing local OAuth fixture, not a new merchant portal.
	f.connect(t, a.InstallationId)
	server := httptest.NewServer(bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil)), f.connections))
	defer server.Close()
	c, err = sdk.NewLocalClient(server.URL, coreToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Installations.Activate(ctx, activateRequest(f.tenant, a.InstallationId)); err != nil {
		t.Fatal("remote handshake", err)
	}
	token, err := c.Installations.IssueToken(ctx, issueTokenRequest(f.tenant, a.InstallationId))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "consent-remote", AmountMinor: 1000, Currency: "IDR"})); err != nil {
		t.Fatal("remote capability", err)
	}
	f.downRevoke.Store(true)
	u, err := c.Installations.Uninstall(ctx, uninstallRequest(f.tenant, a.InstallationId))
	if err != nil || u.Msg.Result.Installation.Status != "disabling" || u.Msg.Result.Installation.GrantState != "revoked" {
		t.Fatal("remote disabling", err)
	}
	if appCheck(t, f.fixture, token.Msg.Result.AppToken, f.tenant, "remote-pay", a.InstallationId, nil) != 401 {
		t.Fatal("token remained valid during remote outage")
	}
	repo := installrepo.Repository{Pool: f.pool}
	cleanup := installrepo.Cleanup{Tenant: f.tenant, ID: a.InstallationId, App: "remote-pay"}
	if err = repo.CompleteCleanup(ctx, cleanup, f.connections.Revoke); err == nil {
		t.Fatal("cleanup outage ignored")
	}
	f.downRevoke.Store(false)
	if err = repo.CompleteCleanup(ctx, cleanup, f.connections.Revoke); err != nil {
		t.Fatal("cleanup retry", err)
	}
	got, err := c.Installations.GetInstallation(ctx, getInstallationRequest(f.tenant, a.InstallationId))
	if err != nil || got.Msg.Result.Installation.Status != "uninstalled" {
		t.Fatal("remote cleanup incomplete", err)
	}
}

func TestConsentInstallationTokenRaceAndRotationRollback(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, p, _, _ := lifecycleClient(t, f)
	a := consume(t, c, f.tenant, "emisell-pay")
	if _, err := c.Installations.Activate(ctx, activateRequest(f.tenant, a.InstallationId)); err != nil {
		t.Fatal(err)
	}
	issued, err := c.Installations.IssueToken(ctx, issueTokenRequest(f.tenant, a.InstallationId))
	if err != nil {
		t.Fatal(err)
	}
	token := issued.Msg.Result.AppToken
	o := domain.IntentOwner{TenantID: p.TenantID, ServiceID: p.ID, ActorID: "staff"}
	repo := installrepo.Repository{Pool: f.pool}
	failedKey := key()
	_, err = repo.ChangeAccess(ctx, o, failedKey, "failure", a.InstallationId, "issue_token", ids.New("apptoken"), "invalid-hash", func(v domain.Access, _ time.Time) (domain.Access, error) { return v, nil })
	if err == nil {
		t.Fatal("expected token constraint failure")
	}
	if appCheck(t, f, token, f.tenant, a.AppId, a.InstallationId, nil) != 200 {
		t.Fatal("failed rotation revoked old token")
	}
	var receipts int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.access_requests WHERE tenant_id=$1 AND request_key=$2`, f.tenant, failedKey).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal("failed rotation left receipt", err)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	tokenCh := make(chan string, 1)
	wg.Go(func() {
		r, e := c.Installations.IssueToken(ctx, issueTokenRequest(f.tenant, a.InstallationId))
		if r != nil {
			tokenCh <- r.Msg.Result.AppToken
		}
		errCh <- e
	})
	wg.Go(func() {
		_, e := c.Installations.Uninstall(ctx, uninstallRequest(f.tenant, a.InstallationId))
		errCh <- e
	})
	wg.Wait()
	close(errCh)
	close(tokenCh)
	for e := range errCh {
		if e != nil && connect.CodeOf(e) != connect.CodeAlreadyExists {
			t.Fatal(e)
		}
	}
	for secret := range tokenCh {
		if appCheck(t, f, secret, f.tenant, a.AppId, a.InstallationId, nil) != 401 {
			t.Fatal("issuance raced past revocation")
		}
	}
	if appCheck(t, f, token, f.tenant, a.AppId, a.InstallationId, nil) != 401 {
		t.Fatal("original token not revoked")
	}
	if _, err = c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "after-uninstall", AmountMinor: 1000, Currency: "IDR"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("routing remained active", err)
	}
	// A scope grant is checked at invocation time, not inferred from installation status.
	b := consume(t, c, f.tenant, "emisell-pay")
	if _, err = c.Installations.Activate(ctx, activateRequest(f.tenant, b.InstallationId)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE platform_installation.access_grants SET state='revoked',scopes='[]',revoked_at=now() WHERE tenant_id=$1 AND installation_id=$2`, f.tenant, b.InstallationId); err != nil {
		t.Fatal(err)
	}
	_, err = c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "revoked-grant", AmountMinor: 1000, Currency: "IDR"}))
	rpcCode(t, err, connect.CodePermissionDenied)
	_, err = c.Installations.Activate(ctx, activateRequest(f.tenant, b.InstallationId))
	rpcCode(t, err, connect.CodePermissionDenied)
}
