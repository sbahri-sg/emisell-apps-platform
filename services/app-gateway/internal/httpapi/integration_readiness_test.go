package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func readinessCheck(t *testing.T, report domain.IntegrationReadiness, code string) domain.IntegrationCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.Code == code {
			return check
		}
	}
	t.Fatalf("missing check %s", code)
	return domain.IntegrationCheck{}
}

func TestIntegrationReadinessEvidenceAndRedaction(t *testing.T) {
	handler, repo := testServer()
	app := postApp(t, handler, "readiness-app-create-001")
	path := "/v1/apps/" + app.ID + "/integration-readiness"
	load := func() domain.IntegrationReadiness {
		t.Helper()
		res := request(t, handler, http.MethodGet, path, "", testOrganizationID, domain.RoleAnalyst, "")
		if res.Code != 200 {
			t.Fatalf("readiness %d: %s", res.Code, res.Body.String())
		}
		if res.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("readiness can be cached")
		}
		for _, secret := range []string{"secretFingerprint", "signingSecret", "clientId", "ciphertext", "merchantId", "secret-in-configuration", "https://", "endpointUrl"} {
			if strings.Contains(res.Body.String(), secret) {
				t.Fatalf("sensitive detail leaked: %s", secret)
			}
		}
		return decodeData[domain.IntegrationReadiness](t, res)
	}
	initial := load()
	if readinessCheck(t, initial, "active_version").Status != "blocked" || initial.EndToEndVerified {
		t.Fatal("draft marked ready")
	}
	ctx := context.Background()
	_, err := repo.CreateExtension(ctx, testOrganizationID, domain.AppExtension{ID: "11111111-1111-7111-8111-111111111112", AppID: app.ID, Name: "Private extension", Type: domain.ExtensionTypeShipping, Status: domain.ExtensionStatusActive, Configuration: map[string]any{"apiKey": "secret-in-configuration"}}, ports.MutationMeta{ActorID: testUserID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.ReplaceScopes(ctx, testOrganizationID, app.ID, []domain.AppScope{{AppID: app.ID, Scope: "read_merchant", Access: domain.ScopeAccessRequired}}, ports.MutationMeta{ActorID: testUserID})
	if err != nil {
		t.Fatal(err)
	}
	version := postVersion(t, handler, app.ID, "1.0.0", "readiness-version-create-001")
	releaseVersion(t, handler, app.ID, version.ID, "readiness-version-release-001", nil, 200, domain.RoleOwner)
	active := load()
	if readinessCheck(t, active, "draft_changes").Status != "pass" || readinessCheck(t, active, "installation_evidence").Status != "attention" {
		t.Fatal("incorrect active evidence", active)
	}
	if len(active.Scopes) != 1 || len(active.Scopes[0].Endpoints) != 1 {
		t.Fatal("scope catalog not resolved")
	}
	_, err = repo.ReplaceScopes(ctx, testOrganizationID, app.ID, []domain.AppScope{{AppID: app.ID, Scope: "read_orders", Access: domain.ScopeAccessRequired}}, ports.MutationMeta{ActorID: testUserID})
	if err != nil {
		t.Fatal(err)
	}
	if report := load(); readinessCheck(t, report, "draft_changes").Status != "attention" || report.Scopes[0].Scope != "read_merchant" {
		t.Fatal("draft edits incorrectly represented as active")
	}
	now := time.Date(2026, 8, 31, 7, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	_, err = repo.CreateCredential(ctx, testOrganizationID, domain.AppCredential{ID: "11111111-1111-7111-8111-111111111113", AppID: app.ID, Environment: domain.EnvironmentSandbox, ClientID: "private-client", Status: domain.CredentialStatusActive, ExpiresAt: &expired}, ports.MutationMeta{ActorID: testUserID, Action: "credential.created", IdempotencyKey: "readiness-expired-credential"})
	if err != nil {
		t.Fatal(err)
	}
	if readinessCheck(t, load(), "development_credentials").Status != "attention" {
		t.Fatal("expired credential treated as usable")
	}
	storedCredentials, err := repo.ListCredentials(ctx, testOrganizationID, app.ID)
	if err != nil || len(storedCredentials) != 1 || storedCredentials[0].ExpiresAt == nil {
		t.Fatal("expired credential fixture was not persisted")
	}
	installation, err := repo.CreateInstallation(ctx, testOrganizationID, domain.AppInstallation{ID: "11111111-1111-7111-8111-111111111114", AppID: app.ID, MerchantID: "private-merchant", MerchantName: "Private merchant", Environment: domain.EnvironmentSandbox, Status: domain.InstallationStatusActive, InstalledVersionID: version.ID, InstalledBy: testUserID, InstalledAt: now, CreatedAt: now, UpdatedAt: now, Revision: 1}, version.ID, ports.MutationMeta{ActorID: testUserID, Action: "installation.created", IdempotencyKey: "readiness-installation-001"})
	if err != nil {
		t.Fatal(err)
	}
	installed := load()
	if installed.Installations.ActiveCurrentVersion != 1 || readinessCheck(t, installed, "installation_evidence").Status != "pass" || installed.EndToEndVerified {
		t.Fatal("installation is not evidence of full end-to-end verification")
	}
	if err := repo.UninstallInstallation(ctx, testOrganizationID, app.ID, installation.ID, ports.MutationMeta{ActorID: testUserID}); err != nil {
		t.Fatal(err)
	}
	if load().Installations.ActiveCurrentVersion != 0 {
		t.Fatal("uninstalled app counted as active evidence")
	}
	v2 := postVersion(t, handler, app.ID, "1.1.0", "readiness-planned-version-001")
	releaseVersion(t, handler, app.ID, v2.ID, "readiness-planned-release-001", &version.ID, 200, domain.RoleOwner)
	planned := load()
	if readinessCheck(t, planned, "scopes").Status != "attention" || planned.Scopes[0].Availability != domain.ScopeAvailabilityPlanned || len(planned.Scopes[0].Endpoints) != 0 {
		t.Fatal("planned scope presented as available")
	}
}

func TestIntegrationReadinessAuthenticationAndTenantBoundary(t *testing.T) {
	handler, repo := testServer()
	app := postApp(t, handler, "readiness-auth-app-create")
	path := "/v1/apps/" + app.ID + "/integration-readiness"
	other := "33333333-3333-7333-8333-333333333333"
	for _, tc := range []struct {
		path, org string
		status    int
	}{{path, other, 404}, {path + "?organizationId=" + testOrganizationID, testOrganizationID, 422}, {"/v1/apps/not-a-uuid/integration-readiness", testOrganizationID, 422}} {
		res := request(t, handler, "GET", tc.path, "", tc.org, domain.RoleOwner, "")
		if res.Code != tc.status {
			t.Fatalf("%s got %d: %s", tc.path, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
	if res.Code != 401 {
		t.Fatal("anonymous inspection allowed", res.Code)
	}
	internal := "/v1/internal/organizations/" + testOrganizationID + "/apps/" + app.ID + "/integration-readiness"
	req := httptest.NewRequest("GET", internal, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Organization-Id", testOrganizationID)
	req.Header.Set("X-Emisell-Role", "owner")
	req.Header.Set("X-Emisell-Platform-Operator", "true")
	res = httptest.NewRecorder()
	nonOperator := httpapi.NewServer(httpapi.Dependencies{Catalog: application.NewCatalogService(repo, repo, time.Now), AdminAuthenticator: httpapi.DevelopmentAuthenticator{BearerToken: testToken, DefaultUserID: testUserID, DefaultRole: domain.RoleOwner}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	nonOperator.ServeHTTP(res, req)
	if res.Code != 403 {
		t.Fatalf("organization owner bypassed operator boundary: %d %s", res.Code, res.Body.String())
	}
	res = request(t, handler, "GET", internal, "", other, domain.RoleOwner, "")
	if res.Code != 200 {
		t.Fatalf("operator cannot inspect selected org: %d %s", res.Code, res.Body.String())
	}
}

func TestCatalogPublicationRejectsStaleInspection(t *testing.T) {
	handler, _ := testServer()
	app := postApp(t, handler, "readiness-publish-app-001")
	v1 := postVersion(t, handler, app.ID, "1.0.0", "readiness-publish-version-001")
	releaseVersion(t, handler, app.ID, v1.ID, "readiness-publish-release-001", nil, 200, domain.RoleOwner)
	res := request(t, handler, "GET", "/v1/apps/"+app.ID+"/integration-readiness", "", testOrganizationID, domain.RoleOwner, "")
	report := decodeData[domain.IntegrationReadiness](t, res)
	v2 := postVersion(t, handler, app.ID, "1.1.0", "readiness-publish-version-002")
	releaseVersion(t, handler, app.ID, v2.ID, "readiness-publish-release-002", &v1.ID, 200, domain.RoleOwner)
	path := "/v1/internal/organizations/" + testOrganizationID + "/apps/" + app.ID + "/catalog-listing"
	body := fmt.Sprintf(`{"category":"custom","status":"published","featured":false,"revision":0,"expectedAppRevision":%d,"expectedActiveVersionId":%q}`, report.AppRevision, *report.ActiveVersionID)
	res = request(t, handler, "PUT", path, body, testOrganizationID, domain.RoleOwner, "")
	if res.Code != 409 {
		t.Fatalf("stale inspection accepted: %d %s", res.Code, res.Body.String())
	}
	for _, invalid := range []string{`{"expectedAppRevision":1}`, `{"expectedActiveVersionId":"bad"}`} {
		var payload map[string]any
		_ = json.Unmarshal([]byte(invalid), &payload)
		payload["category"], payload["status"], payload["revision"] = "custom", "published", 0
		encoded, _ := json.Marshal(payload)
		res = request(t, handler, "PUT", path, string(encoded), testOrganizationID, domain.RoleOwner, "")
		if res.Code != 422 {
			t.Fatal("partial precondition accepted", res.Code)
		}
	}
}
