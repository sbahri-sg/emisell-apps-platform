package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func TestAppBillingHTTPTrustBoundaries(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC) }
	id := func() string {
		value, err := ids.NewUUIDv7()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	repo := memory.NewRepository(ids.NewUUIDv7, now)
	org, user, appID, instID, versionID := id(), id(), id(), id(), id()
	meta := ports.MutationMeta{ActorID: user, Action: "app.created", IdempotencyKey: id()}
	_, err := repo.CreateApp(t.Context(), domain.App{ID: appID, OrganizationID: org, Name: "HTTP billing app", Status: domain.AppStatusActive, ActiveVersionID: &versionID}, meta)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateInstallation(t.Context(), org, domain.AppInstallation{ID: instID, AppID: appID, MerchantID: "http-merchant-a", Environment: domain.EnvironmentSandbox, Status: domain.InstallationStatusActive, InstalledVersionID: versionID}, versionID, ports.MutationMeta{ActorID: user, Action: "installation.created", IdempotencyKey: id()})
	if err != nil {
		t.Fatal(err)
	}
	identity := application.NewIdentityService(repo, ids.NewUUIDv7, now, time.Hour, time.Hour)
	merchants := application.NewMerchantService(repo, repo, repo, identity, ids.NewUUIDv7, now)
	merchant, err := merchants.CreateSandboxSession(t.Context(), application.CreateSandboxMerchantSessionCommand{MerchantID: "http-merchant-a", MerchantName: "HTTP Merchant A"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := merchants.CreateSandboxSession(t.Context(), application.CreateSandboxMerchantSessionCommand{MerchantID: "http-merchant-b", MerchantName: "HTTP Merchant B"})
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewAppBillingService(repo, ids.NewUUIDv7, now, false)
	dependencies := httpapi.Dependencies{AppBilling: service, Merchants: merchants, Authenticator: httpapi.DevelopmentAuthenticator{BearerToken: "billing-owner-token", DefaultUserID: user, DefaultRole: domain.RoleOwner}, EmisellBackendAuth: httpapi.DevelopmentEmisellBackendAuthenticator{BearerToken: "billing-backend-token"}, IdentityHTTP: httpapi.IdentityHTTPOptions{MerchantSessionCookieName: "merchant", MerchantCSRFCookieName: "merchant_csrf"}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	server := httpapi.NewServer(dependencies)
	call := func(handler http.Handler, method, path string, body any, headers map[string]string, want int) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: status %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w.Body.Bytes()
	}
	owner := map[string]string{"Authorization": "Bearer billing-owner-token", "X-Organization-Id": org, "Idempotency-Key": "http-create-plan-00000001"}
	planInput := map[string]any{"name": "Free test", "currency": "IDR", "amountMinor": 0, "interval": "free", "features": []string{}}
	planPath := "/v1/apps/" + appID + "/plans"
	developer := map[string]string{"Authorization": "Bearer billing-owner-token", "X-Organization-Id": org, "X-Emisell-Role": "developer", "Idempotency-Key": "http-create-plan-00000001"}
	call(server, "POST", planPath, planInput, developer, 403)
	var plan struct{ Data domain.AppPlan }
	if err := json.Unmarshal(call(server, "POST", planPath, planInput, owner, 200), &plan); err != nil {
		t.Fatal(err)
	}
	merchantHeaders := map[string]string{"Cookie": "merchant=" + merchant.Session.SessionToken, "X-CSRF-Token": merchant.Session.CSRFToken}
	billingPath := "/v1/merchant/installations/" + instID + "/billing"
	call(server, "GET", billingPath, nil, nil, 401)
	call(server, "GET", billingPath, nil, map[string]string{"Cookie": "merchant=" + other.Session.SessionToken}, 404)
	call(server, "POST", billingPath+"/quotes", map[string]any{"planId": plan.Data.ID}, map[string]string{"Cookie": "merchant=" + merchant.Session.SessionToken}, 403)
	call(server, "POST", billingPath+"/quotes", map[string]any{"planId": plan.Data.ID, "merchantId": "http-merchant-b"}, merchantHeaders, 422)
	var quote struct{ Data domain.AppSubscriptionQuote }
	if err := json.Unmarshal(call(server, "POST", billingPath+"/quotes", map[string]any{"planId": plan.Data.ID}, merchantHeaders, 200), &quote); err != nil {
		t.Fatal(err)
	}
	call(server, "POST", billingPath+"/approve", map[string]any{"quoteId": quote.Data.ID, "acceptRecurringCharge": false}, merchantHeaders, 422)
	call(server, "POST", billingPath+"/approve", map[string]any{"quoteId": quote.Data.ID, "acceptRecurringCharge": true}, merchantHeaders, 200)
	call(server, "POST", billingPath+"/approve", map[string]any{"quoteId": quote.Data.ID, "acceptRecurringCharge": true}, merchantHeaders, 200)
	backendPath := "/v1/integrations/emisell/billing/account"
	backendHeaders := map[string]string{"Authorization": "Bearer billing-backend-token", "X-Emisell-Subject": "http-billing-service", "X-Emisell-Token-Id": "http-billing-jti-0000001", "X-Emisell-Store-Id": "http-merchant-a", "X-Emisell-Environment": "sandbox", "X-Emisell-Permissions": "apps.billing.write"}
	account := map[string]any{"currency": "IDR", "cycleStart": "2026-09-01T00:00:00Z", "cycleEnd": "2026-10-01T00:00:00Z", "enabled": true, "revision": 0}
	call(server, "PUT", backendPath, account, owner, 401)
	backendHeaders["Cookie"] = "merchant=" + merchant.Session.SessionToken
	call(server, "PUT", backendPath, account, backendHeaders, 401)
	delete(backendHeaders, "Cookie")
	backendHeaders["Origin"] = "https://example.test"
	call(server, "PUT", backendPath, account, backendHeaders, 401)
	delete(backendHeaders, "Origin")
	backendHeaders["X-Emisell-Permissions"] = "apps.install"
	call(server, "PUT", backendPath, account, backendHeaders, 403)
	backendHeaders["X-Emisell-Permissions"] = "apps.billing.write"
	call(server, "PUT", backendPath, account, backendHeaders, 200)
	account["merchantId"] = "http-merchant-b"
	call(server, "PUT", backendPath, account, backendHeaders, 422)
	dependencies.AppBilling = nil
	disabled := httpapi.NewServer(dependencies)
	call(disabled, "GET", planPath, nil, owner, 503)
	call(disabled, "GET", billingPath, nil, merchantHeaders, 503)
}
