package bootstrap_test

import (
	"bytes"
	"context"
	"emisell.app/platform/internal/event"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/webhook"
	hookrepo "emisell.app/platform/internal/webhook/postgres"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestOperationsMonitorTenantIsolationAndRecovery(t *testing.T) {
	f := remoteSetup(t)
	ctx := context.Background()
	ins := f.install(t)
	f.connect(t, ins)
	f.expect(t, "POST", f.installPath(f.tenant), action("activate", "remote-pay", nil), key(), 200)
	root := "/api/v1/workspaces/" + f.tenant
	repo := hookrepo.Repository{Pool: f.caps}
	var deliveries []string
	for i := 0; i < 23; i++ {
		e := event.New("emisell.capability.invoked.v1", f.tenant, f.user, ins, ids.New("req"), map[string]any{"capability": "payment/v1", "operation": "status", "private": "payload-not-for-dashboard"})
		if err := f.hooks.Ingest(ctx, e); err != nil {
			t.Fatal(err)
		}
		var id string
		if err := f.pool.QueryRow(ctx, "SELECT id FROM platform_webhook.deliveries WHERE tenant_id=$1 AND event_id=$2", f.tenant, e.ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		deliveries = append(deliveries, id)
	}
	if _, err := f.pool.Exec(ctx, "UPDATE platform_webhook.deliveries SET status='delivered' WHERE tenant_id=$1", f.tenant); err != nil {
		t.Fatal(err)
	}
	dead := deliveries[0]
	if _, err := f.pool.Exec(ctx, "UPDATE platform_webhook.deliveries SET status='dead',attempts=12,revision=12,last_error='receiver_unavailable' WHERE tenant_id=$1 AND id=$2", f.tenant, dead); err != nil {
		t.Fatal(err)
	}
	page := f.expect(t, "GET", root+"/webhooks", nil, "", 200)
	if len(page["items"].([]any)) != 20 || page["summary"].(map[string]any)["delivered"] != float64(22) {
		t.Fatal("invalid tenant page", page)
	}
	cursor := page["nextCursor"].(string)
	next := f.expect(t, "GET", root+"/webhooks?cursor="+cursor, nil, "", 200)
	if len(next["items"].([]any)) != 3 || next["nextCursor"] != "" {
		t.Fatal("invalid next page")
	}
	seen := map[string]bool{}
	for _, p := range []map[string]any{page, next} {
		for _, item := range p["items"].([]any) {
			id := item.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatal("pagination duplicate")
			}
			seen[id] = true
		}
	}
	filtered := f.expect(t, "GET", root+"/webhooks?status=dead", nil, "", 200)
	if len(filtered["items"].([]any)) != 1 || filtered["summary"].(map[string]any)["delivered"] != float64(22) {
		t.Fatal("filter changed workspace summary")
	}
	f.expect(t, "GET", root+"/webhooks?status=unknown", nil, "", 400)
	f.expect(t, "GET", root+"/webhooks?cursor=missing", nil, "", 400)
	detail := f.expect(t, "GET", root+"/webhooks/"+dead, nil, "", 200)
	if detail["canRetry"] != true || detail["lastAttemptAt"] != nil || detail["completedAt"] != nil {
		t.Fatal("incorrect historical metadata", detail)
	}
	raw, _ := json.Marshal(detail)
	if bytes.Contains(raw, []byte("payload-not-for-dashboard")) || bytes.Contains(raw, []byte("\"body\"")) {
		t.Fatal("webhook payload leaked")
	}
	other := "/api/v1/workspaces/" + f.other
	f.expect(t, "GET", other+"/webhooks/"+dead, nil, "", 404)
	f.expect(t, "GET", other+"/webhooks?cursor="+cursor, nil, "", 400)
	if len(f.expect(t, "GET", other+"/webhooks", nil, "", 200)["items"].([]any)) != 0 {
		t.Fatal("cross-tenant list")
	}
	request := map[string]any{"reason": "Local receiver restored", "expectedRevision": 12}
	retry := root + "/webhooks/" + dead + "/retry"
	f.expect(t, "POST", other+"/webhooks/"+dead+"/retry", request, key(), 404)
	f.expect(t, "POST", root+"/webhooks/"+deliveries[1]+"/retry", request, key(), 409)
	f.expect(t, "POST", retry, request, "", 400)
	f.expect(t, "POST", retry, map[string]any{"reason": "Local receiver restored"}, key(), 400)
	f.expect(t, "POST", retry, map[string]any{"reason": "Local receiver restored", "expectedRevision": 12, "unknown": true}, key(), 400)
	f.expect(t, "POST", retry, map[string]any{"reason": "short", "expectedRevision": 12}, key(), 400)
	f.expect(t, "POST", retry, map[string]any{"reason": "New\nline reason", "expectedRevision": 12}, key(), 400)
	f.expect(t, "POST", retry, map[string]any{"reason": "Local receiver restored", "expectedRevision": 11}, key(), 409)
	code, _, _, err := f.call("POST", retry, request, key(), "https://foreign.invalid")
	if err != nil || code != 403 {
		t.Fatal("foreign-origin recovery allowed", code, err)
	}
	cookie := f.cookie
	f.cookie = nil
	f.expect(t, "GET", root+"/webhooks", nil, "", 401)
	f.expect(t, "POST", retry, request, key(), 401)
	f.cookie = cookie
	foreign := "/api/v1/workspaces/" + ids.New("tenant")
	for _, path := range []string{"/webhooks", "/webhooks/" + dead, "/connections"} {
		f.expect(t, "GET", foreign+path, nil, "", 404)
	}
	f.expect(t, "POST", foreign+"/webhooks/"+dead+"/retry", request, key(), 404)
	// A double click (same key) returns one durable result and one audit entry.
	k := key()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, data, _, err := f.call("POST", retry, request, k, origin)
			if err != nil || status != 202 {
				errs <- fmt.Errorf("concurrent retry status=%d data=%v err=%v", status, data, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	accepted := f.expect(t, "POST", retry, request, k, 202)
	if accepted["revision"] != float64(13) {
		t.Fatal("duplicate replay", accepted)
	}
	f.expect(t, "POST", retry, map[string]any{"reason": "Another valid reason", "expectedRevision": 12}, k, 409)
	f.expect(t, "POST", retry, request, key(), 409)
	current := f.expect(t, "GET", root+"/webhooks/"+dead, nil, "", 200)
	if current["status"] != "pending" || current["attempts"] != float64(0) || current["canRetry"] != false || !reflect.DeepEqual(current["enqueuedAt"], detail["enqueuedAt"]) {
		t.Fatal("retry changed immutable identity or age", current)
	}
	history := current["history"].([]any)
	if len(history) != 1 || history[0].(map[string]any)["actorId"] != f.user {
		t.Fatal("missing owner audit")
	}
	// Completing the queued delivery records timestamps and increments the revision.
	if _, err := repo.WithDelivery(ctx, dead, func(d webhook.Delivery) (webhook.Outcome, error) {
		return webhook.Outcome{Status: "delivered", Attempts: 1, NextAt: time.Now()}, nil
	}); err != nil {
		t.Fatal(err)
	}
	current = f.expect(t, "GET", root+"/webhooks/"+dead, nil, "", 200)
	if current["lastAttemptAt"] == nil || current["completedAt"] == nil || current["revision"] != float64(14) {
		t.Fatal("missing completion metadata")
	}
	// Cached acceptance is not a claim about the current outcome.
	if !reflect.DeepEqual(accepted, f.expect(t, "POST", retry, request, k, 202)) {
		t.Fatal("cached acceptance changed")
	}
	if _, err := f.pool.Exec(ctx, "UPDATE platform_webhook.deliveries SET status='dead',revision=revision+1 WHERE tenant_id=$1 AND id=$2", f.tenant, dead); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "POST", f.installPath(f.tenant), action("uninstall", "remote-pay", nil), key(), 200)
	current = f.expect(t, "GET", root+"/webhooks/"+dead, nil, "", 200)
	if current["canRetry"] != false {
		t.Fatal("uninstall reopened retry")
	}
	f.expect(t, "POST", retry, map[string]any{"reason": "Local receiver restored", "expectedRevision": 15}, key(), 409)
}

func TestOperationsCleanupExhaustionAuditAndRecovery(t *testing.T) {
	f := remoteSetup(t)
	ctx := context.Background()
	ins := f.install(t)
	f.connect(t, ins)
	root := "/api/v1/workspaces/" + f.tenant
	list := func() map[string]any {
		return f.expect(t, "GET", root+"/connections", nil, "", 200)["items"].([]any)[0].(map[string]any)
	}
	var secretBefore, secretAfter []byte
	if err := f.pool.QueryRow(ctx, "SELECT secret FROM platform_oauth.connections WHERE tenant_id=$1 AND installation_id=$2", f.tenant, ins).Scan(&secretBefore); err != nil {
		t.Fatal(err)
	}
	c := list()
	if c["connectionStatus"] != "connected" || c["canRetryCleanup"] != false {
		t.Fatal("wrong connection state")
	}
	if err := f.pool.QueryRow(ctx, "SELECT secret FROM platform_oauth.connections WHERE tenant_id=$1 AND installation_id=$2", f.tenant, ins).Scan(&secretAfter); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(secretBefore, secretAfter) {
		t.Fatal("monitor read rotated credentials")
	}
	raw, _ := json.Marshal(c)
	for _, forbidden := range []string{"accessToken", "refreshToken", "webhookSecret", "secret", "authorizationUrl"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatal("credential metadata leaked", forbidden)
		}
	}
	retry := f.installPath(f.tenant) + "/" + ins + "/cleanup/retry"
	request := map[string]any{"reason": "Revocation service restored", "expectedRevision": c["revision"]}
	f.expect(t, "POST", retry, request, key(), 409)
	f.expect(t, "POST", f.installPath(f.tenant), action("uninstall", "remote-pay", nil), key(), 200)
	c = list()
	if c["cleanupNextAt"] == nil || c["canRetryCleanup"] != false {
		t.Fatal("cleanup not scheduled")
	}
	f.downRevoke.Store(true)
	repo := installrepo.Repository{Pool: f.pool}
	var lastCandidate installrepo.Cleanup
	for i := 0; i < 12; i++ {
		if _, err := f.pool.Exec(ctx, "UPDATE platform_installation.installations SET cleanup_next_at=now() WHERE tenant_id=$1 AND id=$2", f.tenant, ins); err != nil {
			t.Fatal(err)
		}
		candidates, err := repo.PendingCleanup(ctx)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, candidate := range candidates {
			if candidate.ID == ins {
				found = true
				lastCandidate = candidate
				if err = repo.CompleteCleanup(ctx, candidate, f.connections.Revoke); err == nil {
					t.Fatal("outage ignored")
				}
			}
		}
		if !found {
			t.Fatal("missing due cleanup")
		}
	}
	c = list()
	if c["cleanupAttempts"] != float64(12) || c["canRetryCleanup"] != true || c["cleanupNextAt"] != nil || len(c["history"].([]any)) != 12 {
		t.Fatal("missing exhausted cleanup state", c)
	}
	request["expectedRevision"] = c["revision"]
	f.expect(t, "POST", f.installPath(f.other)+"/"+ins+"/cleanup/retry", request, key(), 404)
	f.expect(t, "POST", retry, request, "", 400)
	k := key()
	result := f.expect(t, "POST", retry, request, k, 202)
	if !reflect.DeepEqual(result, f.expect(t, "POST", retry, request, k, 202)) {
		t.Fatal("cleanup not idempotent")
	}
	f.expect(t, "POST", retry, request, key(), 409)
	f.expect(t, "POST", retry, map[string]any{"reason": "A different reason", "expectedRevision": c["revision"]}, k, 409)
	// A stale worker failure cannot consume the new recovery budget.
	_ = repo.CompleteCleanup(ctx, lastCandidate, f.connections.Revoke)
	c = list()
	if c["cleanupAttempts"] != float64(0) || c["canRetryCleanup"] != false || c["status"] != "disabling" || len(c["history"].([]any)) != 13 {
		t.Fatal("stale failure or recovery reopened access", c)
	}
	f.downRevoke.Store(false)
	candidates, err := repo.PendingCleanup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.ID == ins {
			if err = repo.CompleteCleanup(ctx, candidate, f.connections.Revoke); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(f.expect(t, "GET", root+"/connections", nil, "", 200)["items"].([]any)) != 0 {
		t.Fatal("completed cleanup still listed")
	}
	var status string
	var scopes, capabilities []string
	if err = f.pool.QueryRow(ctx, "SELECT status,scopes,capabilities FROM platform_installation.installations WHERE tenant_id=$1 AND id=$2", f.tenant, ins).Scan(&status, &scopes, &capabilities); err != nil || status != "uninstalled" || len(scopes) != 0 || len(capabilities) != 0 {
		t.Fatal("cleanup failed to close lifecycle", status, err)
	}
}
