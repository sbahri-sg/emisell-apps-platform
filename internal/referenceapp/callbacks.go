package referenceapp

import (
	"bytes"
	"context"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/localhttp"
	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/sdk/events"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strconv"
	"time"
)

// Simulate is an explicit local operator command, never an HTTP/browser route.
// It changes only this fixture's resource, then commits a durable callback.
func (s Server) Simulate(ctx context.Context, tenant, ins, resource, operation, key string) (appapi.Response, error) {
	if !events.Token.MatchString(tenant) || !events.Token.MatchString(ins) || !events.Token.MatchString(resource) || !events.Token.MatchString(key) || len(key) < 16 || (operation != "capture" && operation != "refund") {
		return appapi.Response{}, fault.Invalid
	}
	g := grant{Tenant: tenant, Installation: ins, Local: true}
	var sealed []byte
	err := s.Pool.QueryRow(ctx, "SELECT id,hook FROM reference_remote.grants WHERE tenant_id=$1 AND installation_id=$2 AND NOT revoked AND expires_at>now()", tenant, ins).Scan(&g.ID, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return appapi.Response{}, fault.NotFound
	}
	if err != nil {
		return appapi.Response{}, err
	}
	if _, err = s.Box.Open("hook:"+tenant+":"+ins, sealed); err != nil {
		return appapi.Response{}, fault.Forbidden
	}
	return s.execute(ctx, g, appapi.Invocation{TenantID: tenant, InstallationID: ins, Capability: "payment/v1", IdempotencyKey: hash("local-simulation:" + key), Request: appapi.Request{Operation: operation, ResourceID: resource}})
}

// DeliverOne leases only the queue row, not the grant/installation gate, while
// sending. A crash before commit resends the SAME delivery/body; receiver inbox
// deduplicates. No callback URL is accepted from manifests or request payloads.
func (s Server) DeliverOne(ctx context.Context, platformOrigin string) (string, error) {
	client, err := localhttp.Client(platformOrigin)
	if err != nil {
		return "", err
	}
	defer client.CloseIdleConnections()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id, tenant, ins string
	var body []byte
	var attempts int
	var expired bool
	err = tx.QueryRow(ctx, `SELECT id,tenant_id,installation_id,body,attempts,created_at<now()-interval '24 hours' FROM reference_remote.callbacks WHERE owner_key=$1 AND status='pending' AND next_at<=now() ORDER BY next_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, hash(s.Config.EncryptionKey)).Scan(&id, &tenant, &ins, &body, &attempts, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return "idle", nil
	}
	if err != nil {
		return "", err
	}
	status, reason := "pending", "receiver_unavailable"
	var sealed []byte
	err = tx.QueryRow(ctx, "SELECT hook FROM reference_remote.grants WHERE tenant_id=$1 AND installation_id=$2 AND NOT revoked AND expires_at>now()", tenant, ins).Scan(&sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		status, reason = "cancelled", "grant_inactive"
	} else if err != nil {
		return "", err
	} else if expired || attempts >= 12 {
		status, reason = "dead", "retry_budget_exhausted"
	} else {
		secret, err := s.Box.Open("hook:"+tenant+":"+ins, sealed)
		if err != nil {
			return "", fault.Unavailable
		}
		stamp := strconv.FormatInt(time.Now().Unix(), 10)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, platformOrigin+"/api/v1/app-callbacks/payment/v1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Emisell-Tenant", tenant)
		req.Header.Set("X-Emisell-Installation", ins)
		req.Header.Set("X-Emisell-Delivery", id)
		req.Header.Set("X-Emisell-Timestamp", stamp)
		req.Header.Set("X-Emisell-Signature", appapi.SignCallback(string(secret), tenant, ins, id, stamp, body))
		attempts++
		resp, sendErr := client.Do(req)
		if sendErr == nil {
			var result struct {
				Outcome string `json:"outcome"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()
			if resp.StatusCode == 200 && decodeErr == nil && (result.Outcome == "applied" || result.Outcome == "unchanged" || result.Outcome == "duplicate" || result.Outcome == "stale") {
				status, reason = "delivered", ""
			} else if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 409 || resp.StatusCode == 413 || resp.StatusCode == 415 {
				status, reason = "dead", "callback_rejected"
			}
		}
		if status == "pending" && attempts >= 12 {
			status, reason = "dead", "retry_budget_exhausted"
		}
	}
	_, err = tx.Exec(ctx, `UPDATE reference_remote.callbacks SET status=$2,attempts=$3,last_error=$4,next_at=now()+make_interval(secs=>$5),completed_at=CASE WHEN $2='pending' THEN NULL ELSE now() END WHERE id=$1`, id, status, attempts, reason, float64(uint64(1)<<uint(min(attempts, 8))))
	if err != nil {
		return "", err
	}
	return status, tx.Commit(ctx)
}
