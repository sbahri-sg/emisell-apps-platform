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
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/appmanifest"
	"emisell.app/platform/pkg/sdk"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	intentconnect "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1/installationv1connect"
	"github.com/jackc/pgx/v5/pgxpool"
)

func intentScopes() []string {
	return []string{identity.ScopeInstallIntentsRead, identity.ScopeInstallIntentsWrite, identity.ScopeInstallIntentsConsent}
}

func prepareIntent(t *testing.T, c *sdk.Client, actor, app string) (*intent.InstallIntent, *connect.Request[intent.PrepareRequest]) {
	t.Helper()
	r := connect.NewRequest(&intent.PrepareRequest{CoreActorId: actor, IdempotencyKey: key(), AppId: app, Version: "1.0.0"})
	v, err := c.InstallIntents.Prepare(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return v.Msg.Intent, r
}

func intentDecision(v *intent.InstallIntent, decision intent.ConsentDecision) *connect.Request[intent.DecideRequest] {
	return connect.NewRequest(&intent.DecideRequest{CoreActorId: v.CoreActorId, IdempotencyKey: key(), IntentId: v.Id, ConsentDigest: v.ConsentDigest, Decision: decision})
}

func TestInstallIntentRPCConsentIsNotInstallation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// One lifecycle connection must still make progress: registry reads use caps.
	cfg := f.pool.Config()
	cfg.MaxConns = 1
	small, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(small.Close)
	limitedFixture := *f
	limitedFixture.pool = small
	c, _, _ := serviceClient(t, &limitedFixture, f.tenant, intentScopes())
	v, prepare := prepareIntent(t, c, "core-staff-1", "emisell-pay")
	if v.State != intent.IntentState_INTENT_STATE_PENDING || v.TenantId != f.tenant || v.ExecutionAllowed || !slices.Equal(v.Scopes, payScopes) || v.ExpiresAt.AsTime().Sub(v.CreatedAt.AsTime()) != domain.IntentTTL || len(v.ManifestDigest) != 64 {
		t.Fatal("incorrect consent snapshot", v)
	}
	again, err := c.InstallIntents.Prepare(ctx, prepare)
	if err != nil || again.Msg.Intent.Id != v.Id {
		t.Fatal("prepare retry duplicated intent", err)
	}
	prepare.Msg.Version = "2.0.0"
	_, err = c.InstallIntents.Prepare(ctx, prepare)
	rpcCode(t, err, connect.CodeAlreadyExists)
	prepare.Msg.Version = "1.0.0"
	wrong := intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_CONSENT)
	wrong.Msg.ConsentDigest = strings.Repeat("0", 64)
	_, err = c.InstallIntents.Decide(ctx, wrong)
	rpcCode(t, err, connect.CodeAlreadyExists)
	wrong.Msg.ConsentDigest = v.ConsentDigest
	wrong.Msg.Decision = intent.ConsentDecision(99)
	_, err = c.InstallIntents.Decide(ctx, wrong)
	rpcCode(t, err, connect.CodeInvalidArgument)

	// Concurrent retries use independent request objects, but the same business key.
	decision := intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_CONSENT)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			r := intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_CONSENT)
			r.Msg.IdempotencyKey = decision.Msg.IdempotencyKey
			response, err := c.InstallIntents.Decide(ctx, r)
			if err == nil && (response.Msg.Intent.State != intent.IntentState_INTENT_STATE_CONSENTED || response.Msg.Intent.ExecutionAllowed) {
				err = errors.New("invalid consent response")
			}
			results <- err
		})
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	again, err = c.InstallIntents.Prepare(ctx, prepare)
	if err != nil || again.Msg.Intent.State != intent.IntentState_INTENT_STATE_CONSENTED {
		t.Fatal("retry did not return current state", err)
	}
	_, err = c.InstallIntents.Decide(ctx, intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_DENY))
	rpcCode(t, err, connect.CodeAlreadyExists)
	decision.Msg.Decision = intent.ConsentDecision_CONSENT_DECISION_DENY
	_, err = c.InstallIntents.Decide(ctx, decision)
	rpcCode(t, err, connect.CodeAlreadyExists)
	var audits, requests int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_installation.intent_audit WHERE intent_id=$1", v.Id).Scan(&audits); err != nil || audits != 2 {
		t.Fatal("audit not exactly once", audits, err)
	}
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_installation.intent_requests WHERE intent_id=$1", v.Id).Scan(&requests); err != nil || requests != 2 {
		t.Fatal("idempotency not exactly once", requests, err)
	}
	for _, table := range []string{"platform_installation.installations", "platform_installation.events", "platform_capability.resources"} {
		var count int
		if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE tenant_id=$1", f.tenant).Scan(&count); err != nil || count != 0 {
			t.Fatal("consent caused an installation/capability side effect", table, count, err)
		}
	}
	// A second app exercises the shipping reference and competing opposite decisions.
	shipping, _ := prepareIntent(t, c, "core-staff-1", "parcel")
	if !slices.Equal(shipping.Scopes, shippingScopes) {
		t.Fatal("shipping permission snapshot missing")
	}
	outcomes := make(chan error, 2)
	for _, d := range []intent.ConsentDecision{intent.ConsentDecision_CONSENT_DECISION_CONSENT, intent.ConsentDecision_CONSENT_DECISION_DENY} {
		wg.Go(func() { _, err := c.InstallIntents.Decide(ctx, intentDecision(shipping, d)); outcomes <- err })
	}
	wg.Wait()
	close(outcomes)
	success, conflict := 0, 0
	for err := range outcomes {
		if err == nil {
			success++
		} else if connect.CodeOf(err) == connect.CodeAlreadyExists {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("competing decisions were not serialized", success, conflict)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE platform_installation.install_intents SET snapshot=snapshot || '{\"tampered\":true}'::jsonb WHERE id=$1", v.Id); err == nil {
		t.Fatal("database allowed immutable snapshot update")
	}
}

func TestInstallIntentRPCIdentityAndValidation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, principal, token := serviceClient(t, f, f.tenant, intentScopes())
	v, _ := prepareIntent(t, c, "core-staff-1", "emisell-pay")
	get := connect.NewRequest(&intent.GetRequest{CoreActorId: v.CoreActorId, IntentId: v.Id})
	if _, err := c.InstallIntents.Get(ctx, get); err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []string{f.tenant, f.other} {
		other, _, _ := serviceClient(t, f, tenant, intentScopes())
		_, err := other.InstallIntents.Get(ctx, get)
		rpcCode(t, err, connect.CodeNotFound)
		_, err = other.InstallIntents.Decide(ctx, intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_CONSENT))
		rpcCode(t, err, connect.CodeNotFound)
	}
	get.Msg.CoreActorId = "core-staff-other"
	_, err := c.InstallIntents.Get(ctx, get)
	rpcCode(t, err, connect.CodeNotFound)
	wrongActor := intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_CONSENT)
	wrongActor.Msg.CoreActorId = "core-staff-other"
	_, err = c.InstallIntents.Decide(ctx, wrongActor)
	rpcCode(t, err, connect.CodeNotFound)
	get.Msg.CoreActorId = v.CoreActorId
	for _, scope := range []string{"payments.write", identity.ScopeInstallIntentsRead, identity.ScopeInstallIntentsWrite} {
		limited, _, _ := serviceClient(t, f, f.tenant, []string{scope})
		_, err = limited.InstallIntents.Decide(ctx, intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_CONSENT))
		rpcCode(t, err, connect.CodePermissionDenied)
		if scope != identity.ScopeInstallIntentsWrite {
			_, err = limited.InstallIntents.Prepare(ctx, connect.NewRequest(&intent.PrepareRequest{CoreActorId: "staff", IdempotencyKey: key(), AppId: "parcel", Version: "1.0.0"}))
			rpcCode(t, err, connect.CodePermissionDenied)
		}
		if scope != identity.ScopeInstallIntentsRead {
			_, err = limited.InstallIntents.Get(ctx, get)
			rpcCode(t, err, connect.CodePermissionDenied)
		}
	}
	for _, tc := range []struct {
		app, version, actor, key string
		code                     connect.Code
	}{
		{"catalog-not-executable", "1.0.0", "staff", key(), connect.CodeNotFound},
		{"emisell-pay", "2.0.0", "staff", key(), connect.CodeAlreadyExists},
		{"emisell-pay", "1.0.0", "", key(), connect.CodeInvalidArgument},
		{"emisell-pay", "1.0.0", "staff", "short", connect.CodeInvalidArgument},
		{"https://example.invalid/app", "1.0.0", "staff", key(), connect.CodeInvalidArgument},
	} {
		_, err = c.InstallIntents.Prepare(ctx, connect.NewRequest(&intent.PrepareRequest{AppId: tc.app, Version: tc.version, CoreActorId: tc.actor, IdempotencyKey: tc.key}))
		rpcCode(t, err, tc.code)
	}
	// Credential scope changes are read on every request, not cached in the SDK.
	if _, err = f.pool.Exec(ctx, "UPDATE platform_identity.service_accounts SET scopes=$2 WHERE id=$1", principal.ID, []string{identity.ScopeInstallIntentsRead}); err != nil {
		t.Fatal(err)
	}
	_, err = c.InstallIntents.Decide(ctx, intentDecision(v, intent.ConsentDecision_CONSENT_DECISION_CONSENT))
	rpcCode(t, err, connect.CodePermissionDenied)
	if _, err = c.InstallIntents.Get(ctx, get); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE platform_identity.service_accounts SET expires_at=now()-interval '1 second' WHERE id=$1", principal.ID); err != nil {
		t.Fatal(err)
	}
	_, err = c.InstallIntents.Get(ctx, get)
	rpcCode(t, err, connect.CodeUnauthenticated)
	accounts := identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}
	rotated, err := accounts.Issue(ctx, principal, true, "test-operator")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.InstallIntents.Get(ctx, get)
	rpcCode(t, err, connect.CodeUnauthenticated)

	server := httptest.NewServer(bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()
	rotatedClient, err := sdk.NewLocalClient(server.URL, rotated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = rotatedClient.InstallIntents.Get(ctx, get); err != nil {
		t.Fatal("rotation lost intent owner", err)
	}
	if err = accounts.Revoke(ctx, principal.ID, "test-operator"); err != nil {
		t.Fatal(err)
	}
	_, err = rotatedClient.InstallIntents.Get(ctx, get)
	rpcCode(t, err, connect.CodeUnauthenticated)
	for _, header := range []string{"Origin", "Cookie"} {
		req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+intentconnect.InstallIntentServiceGetProcedure, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set(header, "browser-context")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatal("browser context accepted", res.StatusCode)
		}
	}
	browser, err := sdk.NewLocalClient(server.URL, f.cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	_, err = browser.InstallIntents.Get(ctx, get)
	rpcCode(t, err, connect.CodeUnauthenticated)
}

type changingIntentRegistry struct {
	installservice.Registry
	changed bool
}

func (r *changingIntentRegistry) Get(ctx context.Context, id string) (appmanifest.Manifest, error) {
	m, err := r.Registry.Get(ctx, id)
	if r.changed {
		m.Description += " modified release"
	}
	return m, err
}

func TestInstallIntentExpirySnapshotRevalidationAndAtomicAudit(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c, p, _ := serviceClient(t, f, f.tenant, intentScopes())
	owner := domain.IntentOwner{TenantID: p.TenantID, ServiceID: p.ID, ActorID: "core-staff-1"}
	// Historical fixture, inserted directly only in the isolated test DB. No clock sleeps.
	var now time.Time
	if err := f.pool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"pending", "consented", "denied"} {
		v := domain.NewInstallIntent(ids.New("intent"), owner, domain.IntentRelease{AppID: "emisell-pay", Version: "1.0.0"}, now.Add(-11*time.Minute))
		raw, _ := json.Marshal(v)
		var decided *time.Time
		if state != "pending" {
			when := v.CreatedAt.Add(time.Second)
			decided = &when
		}
		_, err := f.pool.Exec(ctx, "INSERT INTO platform_installation.install_intents(id,tenant_id,service_id,actor_id,snapshot,state,created_at,expires_at,decided_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", v.ID, owner.TenantID, owner.ServiceID, owner.ActorID, raw, state, v.CreatedAt, v.ExpiresAt, decided)
		if err != nil {
			t.Fatal(err)
		}
		got, err := c.InstallIntents.Get(ctx, connect.NewRequest(&intent.GetRequest{CoreActorId: owner.ActorID, IntentId: v.ID}))
		want := intent.IntentState_INTENT_STATE_EXPIRED
		if state == "denied" {
			want = intent.IntentState_INTENT_STATE_DENIED
		}
		if err != nil || got.Msg.Intent.State != want || got.Msg.Intent.ExecutionAllowed {
			t.Fatal("expired/denied snapshot accepted", err)
		}
		decision := intentDecision(got.Msg.Intent, intent.ConsentDecision_CONSENT_DECISION_CONSENT)
		_, err = c.InstallIntents.Decide(ctx, decision)
		rpcCode(t, err, connect.CodeAlreadyExists)
		// Seed a historical successful prepare receipt; retry must not return stale pending.
		prepare := installservice.PrepareIntent{AppID: "emisell-pay", Version: "1.0.0"}
		hash := installservice.RequestHash(struct {
			Operation string
			Request   installservice.PrepareIntent
		}{"prepare", prepare})
		k := key()
		if _, err = f.pool.Exec(ctx, "INSERT INTO platform_installation.intent_requests(tenant_id,service_id,actor_id,request_key,request_hash,intent_id) VALUES($1,$2,$3,$4,$5,$6)", owner.TenantID, owner.ServiceID, owner.ActorID, k, hash, v.ID); err != nil {
			t.Fatal(err)
		}
		retry, err := c.InstallIntents.Prepare(ctx, connect.NewRequest(&intent.PrepareRequest{CoreActorId: owner.ActorID, AppId: prepare.AppID, Version: prepare.Version, IdempotencyKey: k}))
		if err != nil || retry.Msg.Intent.State != want {
			t.Fatal("retry revived expired intent", err)
		}
		if state == "consented" {
			d := installservice.DecideIntent{ID: v.ID, Digest: v.ConsentDigest, Decision: "consent"}
			hash = installservice.RequestHash(struct {
				Operation string
				Request   installservice.DecideIntent
			}{"decide", d})
			k = key()
			if _, err = f.pool.Exec(ctx, "INSERT INTO platform_installation.intent_requests(tenant_id,service_id,actor_id,request_key,request_hash,intent_id) VALUES($1,$2,$3,$4,$5,$6)", owner.TenantID, owner.ServiceID, owner.ActorID, k, hash, v.ID); err != nil {
				t.Fatal(err)
			}
			decision.Msg.IdempotencyKey = k
			replayed, err := c.InstallIntents.Decide(ctx, decision)
			if err != nil || replayed.Msg.Intent.State != intent.IntentState_INTENT_STATE_EXPIRED || replayed.Msg.Intent.ExecutionAllowed {
				t.Fatal("decision replay revived expired consent", err)
			}
		}
	}

	registry := &changingIntentRegistry{Registry: appservice.Registry{Repo: apprepo.Postgres{Pool: f.caps}}}
	s := installservice.Intents{Repo: installrepo.Repository{Pool: f.pool}, Apps: registry, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}}
	v, err := s.Prepare(ctx, p, owner.ActorID, key(), installservice.PrepareIntent{AppID: "emisell-pay", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	registry.changed = true
	decision := installservice.DecideIntent{ID: v.ID, Digest: v.ConsentDigest, Decision: "consent"}
	if _, err = s.Decide(ctx, p, owner.ActorID, key(), decision); !errors.Is(err, fault.Conflict) {
		t.Fatal("changed release accepted", err)
	}
	decision.Decision = "deny"
	if _, err = s.Decide(ctx, p, owner.ActorID, key(), decision); err != nil {
		t.Fatal("deny must not depend on release", err)
	}

	// Force an audit text-encoding failure via the repository port; neither state nor
	// receipt can survive. This does not change any database trigger/schema.
	before, err := s.Prepare(ctx, p, owner.ActorID, key(), installservice.PrepareIntent{AppID: "parcel", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	k := key()
	_, err = (installrepo.Repository{Pool: f.pool}).ChangeIntent(ctx, owner, k, "test-audit-failure", before.ID, func(current *domain.InstallIntent, now time.Time) (domain.InstallIntent, error) {
		next, err := current.Decide(current.ConsentDigest, "consent", now)
		// PostgreSQL text rejects NUL when inserting the audit, after state UPDATE.
		next.ConsentDigest = string([]byte{0})
		return next, err
	})
	if err == nil {
		t.Fatal("expected audit encoding failure")
	}
	after, err := s.Get(ctx, p, owner.ActorID, before.ID)
	if err != nil || after.State != "pending" {
		t.Fatal("failed transaction changed state", err)
	}
	var receipts int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_installation.intent_requests WHERE tenant_id=$1 AND request_key=$2", owner.TenantID, k).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal("failed transaction left receipt", err)
	}
}
