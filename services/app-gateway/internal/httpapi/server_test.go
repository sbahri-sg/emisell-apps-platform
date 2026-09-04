package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/emisell"
	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
	"emisell-app-platform/services/app-gateway/internal/security"
)

const (
	testToken          = "test-bearer-token"
	testOrganizationID = "01995f72-0000-7000-8000-000000000001"
	testUserID         = "01995f72-0000-7000-8000-000000000002"
)

func TestAppAndVersionLifecycle(t *testing.T) {
	t.Parallel()
	handler, repository := testServer()

	createdApp := postApp(t, handler, "create-app-request-0001")
	replayedApp := postApp(t, handler, "create-app-request-0001")
	if replayedApp.ID != createdApp.ID {
		t.Fatalf("idempotent replay returned app %q, want %q", replayedApp.ID, createdApp.ID)
	}

	versionOne := postVersion(t, handler, createdApp.ID, "1.0.0", "create-version-request-0001")
	releaseVersion(t, handler, createdApp.ID, versionOne.ID, "release-version-request-0001", nil, http.StatusOK, domain.RoleOwner)

	activeApp := getApp(t, handler, testOrganizationID, createdApp.ID, http.StatusOK)
	if activeApp.Status != domain.AppStatusActive || activeApp.ActiveVersionID == nil || *activeApp.ActiveVersionID != versionOne.ID {
		t.Fatalf("first release did not activate version: %#v", activeApp)
	}

	versionTwo := postVersion(t, handler, createdApp.ID, "1.1.0", "create-version-request-0002")
	releaseVersion(t, handler, createdApp.ID, versionTwo.ID, "release-version-request-0002", &versionOne.ID, http.StatusOK, domain.RoleOwner)
	releaseVersion(t, handler, createdApp.ID, versionOne.ID, "rollback-version-request-0001", nil, http.StatusForbidden, domain.RoleDeveloper)
	rollbackVersion(t, handler, createdApp.ID, versionOne.ID, "rollback-version-request-0002")

	rolledBackApp := getApp(t, handler, testOrganizationID, createdApp.ID, http.StatusOK)
	if rolledBackApp.ActiveVersionID == nil || *rolledBackApp.ActiveVersionID != versionOne.ID {
		t.Fatalf("rollback activated %v, want %s", rolledBackApp.ActiveVersionID, versionOne.ID)
	}

	getApp(t, handler, "01995f72-0000-7000-8000-000000000099", createdApp.ID, http.StatusNotFound)
	auditEvents, err := repository.ListAuditEvents(t.Context(), testOrganizationID)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(auditEvents) != 6 {
		t.Fatalf("got %d audit events, want 6", len(auditEvents))
	}
}

func TestCreateAppRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()
	response := request(t, handler, http.MethodPost, "/v1/apps", `{"name":"Test App","distribution":"custom","unknown":true}`, testOrganizationID, domain.RoleOwner, "unknown-field-request-0001")
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}
}

func TestCreateAppRejectsPublicDistribution(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()
	response := request(t, handler, http.MethodPost, "/v1/apps", `{"name":"Public App","distribution":"public"}`, testOrganizationID, domain.RoleOwner, "public-app-denied-0001")
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}
}

func TestScopeCatalogAndUnknownScopeValidation(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()

	catalogResponse := request(t, handler, http.MethodGet, "/v1/scope-catalog", "", "", domain.RoleAnalyst, "")
	catalog := decodeData[[]domain.ScopeDefinition](t, catalogResponse)
	if catalogResponse.Code != http.StatusOK || len(catalog) == 0 || catalog[0].Scope != "read_merchant" || catalog[0].Availability != domain.ScopeAvailabilityAvailable {
		t.Fatalf("unexpected scope catalog: status=%d data=%#v", catalogResponse.Code, catalog)
	}
	eventCatalogResponse := request(t, handler, http.MethodGet, "/v1/webhook-event-catalog", "", "", domain.RoleAnalyst, "")
	eventCatalog := decodeData[[]domain.WebhookEventDefinition](t, eventCatalogResponse)
	if eventCatalogResponse.Code != http.StatusOK || len(eventCatalog) < 2 || eventCatalog[0].Event != "app/uninstalled" || eventCatalog[0].Availability != domain.WebhookEventAvailabilityAvailable {
		t.Fatalf("unexpected webhook event catalog: status=%d data=%#v", eventCatalogResponse.Code, eventCatalog)
	}

	app := postApp(t, handler, "unknown-scope-app-0001")
	unknown := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{"scopes":[{"scope":"read_everything","access":"required"}]}`, testOrganizationID, domain.RoleDeveloper, "")
	if unknown.Code != http.StatusUnprocessableEntity || !strings.Contains(unknown.Body.String(), "not registered") {
		t.Fatalf("unknown scope status=%d body=%s", unknown.Code, unknown.Body.String())
	}
	plannedEvent := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhooks", `{"event":"orders/created","endpointUrl":"https://hooks.example.com/orders"}`, testOrganizationID, domain.RoleDeveloper, "planned-webhook-0001")
	if plannedEvent.Code != http.StatusConflict || !strings.Contains(plannedEvent.Body.String(), "planned") {
		t.Fatalf("planned webhook status=%d body=%s", plannedEvent.Code, plannedEvent.Body.String())
	}
	unknownEvent := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhooks", `{"event":"imaginary/created","endpointUrl":"https://hooks.example.com/imaginary"}`, testOrganizationID, domain.RoleDeveloper, "unknown-webhook-0001")
	if unknownEvent.Code != http.StatusUnprocessableEntity || !strings.Contains(unknownEvent.Body.String(), "event catalog") {
		t.Fatalf("unknown webhook status=%d body=%s", unknownEvent.Code, unknownEvent.Body.String())
	}
}

func TestExtensionScopeAndSnapshotLifecycle(t *testing.T) {
	t.Parallel()
	handler, repository := testServer()
	app := postApp(t, handler, "extension-app-request-0001")

	createBody := `{"name":"Payments Runtime","type":"payment","runtimeUrl":"https://extensions.example.com/payments","configuration":{"mode":"sandbox"}}`
	createdResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/extensions", createBody, testOrganizationID, domain.RoleDeveloper, "create-extension-request-0001")
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create extension status %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	extension := decodeData[domain.AppExtension](t, createdResponse)
	replayedResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/extensions", createBody, testOrganizationID, domain.RoleDeveloper, "create-extension-request-0001")
	if replayedResponse.Code != http.StatusCreated || decodeData[domain.AppExtension](t, replayedResponse).ID != extension.ID {
		t.Fatalf("extension idempotency replay failed: %s", replayedResponse.Body.String())
	}
	missingScopesResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{}`, testOrganizationID, domain.RoleDeveloper, "")
	if missingScopesResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing scopes status %d, want 422: %s", missingScopesResponse.Code, missingScopesResponse.Body.String())
	}

	scopesBody := `{"scopes":[{"scope":"write_products","access":"required"},{"scope":"read_orders","access":"optional"}]}`
	scopesResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", scopesBody, testOrganizationID, domain.RoleDeveloper, "")
	if scopesResponse.Code != http.StatusOK || len(decodeData[[]domain.AppScope](t, scopesResponse)) != 2 {
		t.Fatalf("replace scopes status %d: %s", scopesResponse.Code, scopesResponse.Body.String())
	}

	version := postVersion(t, handler, app.ID, "1.0.0", "extension-version-request-0001")
	if len(version.Snapshot.Extensions) != 1 || version.Snapshot.Extensions[0].ExtensionID != extension.ID {
		t.Fatalf("version snapshot missing extension: %#v", version.Snapshot.Extensions)
	}
	if len(version.Snapshot.Scopes) != 2 {
		t.Fatalf("version snapshot scopes = %d, want 2", len(version.Snapshot.Scopes))
	}
	releaseVersion(t, handler, app.ID, version.ID, "extension-release-request-0001", nil, http.StatusOK, domain.RoleOwner)

	listResponse := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID+"/extensions", "", testOrganizationID, domain.RoleAnalyst, "")
	listed := decodeData[[]domain.AppExtension](t, listResponse)
	if listResponse.Code != http.StatusOK || len(listed) != 1 || listed[0].Status != domain.ExtensionStatusActive {
		t.Fatalf("unexpected extension list: status=%d data=%#v", listResponse.Code, listed)
	}

	updateBody := fmt.Sprintf(`{"runtimeUrl":"https://extensions.example.com/payments/v2","configuration":{"mode":"live"},"revision":%d}`, listed[0].Revision)
	updateResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/extensions/"+extension.ID, updateBody, testOrganizationID, domain.RoleDeveloper, "")
	updated := decodeData[domain.AppExtension](t, updateResponse)
	if updateResponse.Code != http.StatusOK || updated.Status != domain.ExtensionStatusDraft || updated.Revision != listed[0].Revision+1 {
		t.Fatalf("unexpected extension update: status=%d data=%#v", updateResponse.Code, updated)
	}
	staleResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/extensions/"+extension.ID, updateBody, testOrganizationID, domain.RoleDeveloper, "")
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale extension update status %d, want 409: %s", staleResponse.Code, staleResponse.Body.String())
	}

	disableResponse := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/extensions/"+extension.ID, "", testOrganizationID, domain.RoleDeveloper, "")
	if disableResponse.Code != http.StatusNoContent {
		t.Fatalf("disable extension status %d: %s", disableResponse.Code, disableResponse.Body.String())
	}
	disabled, err := repository.GetExtension(t.Context(), testOrganizationID, app.ID, extension.ID)
	if err != nil || disabled.Status != domain.ExtensionStatusDisabled {
		t.Fatalf("extension was not disabled: status=%q err=%v", disabled.Status, err)
	}

	auditEvents, err := repository.ListAuditEvents(t.Context(), testOrganizationID)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(auditEvents) != 7 {
		t.Fatalf("got %d audit events, want 7", len(auditEvents))
	}
}

func TestCredentialAndWebhookLifecycle(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()
	app := postApp(t, handler, "security-app-request-0001")

	forbidden := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"production"}`, testOrganizationID, domain.RoleDeveloper, "credential-forbidden-0001")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("developer credential create status %d, want 403", forbidden.Code)
	}
	productionResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"production"}`, testOrganizationID, domain.RoleOwner, "credential-production-denied-0001")
	if productionResponse.Code != http.StatusForbidden {
		t.Fatalf("production credential create status %d, want 403: %s", productionResponse.Code, productionResponse.Body.String())
	}
	createdResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "credential-create-0001")
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create credential status %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	created := decodeData[application.CredentialSecret](t, createdResponse)
	if created.ClientSecret == "" || created.Credential.SecretFingerprint == "" {
		t.Fatalf("credential secret response is incomplete: %#v", created)
	}
	listResponse := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID+"/credentials", "", testOrganizationID, domain.RoleAdmin, "")
	if listResponse.Code != http.StatusOK || bytes.Contains(listResponse.Body.Bytes(), []byte(created.ClientSecret)) || bytes.Contains(listResponse.Body.Bytes(), []byte("ciphertext")) {
		t.Fatalf("credential list exposed secret material: %s", listResponse.Body.String())
	}
	rotatedResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials/"+created.Credential.ID+"/rotate", "", testOrganizationID, domain.RoleOwner, "credential-rotate-0001")
	rotated := decodeData[application.CredentialSecret](t, rotatedResponse)
	if rotatedResponse.Code != http.StatusOK || rotated.ClientSecret == created.ClientSecret {
		t.Fatalf("credential rotation failed: status=%d data=%#v", rotatedResponse.Code, rotated)
	}
	revokeResponse := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/credentials/"+created.Credential.ID, "", testOrganizationID, domain.RoleOwner, "credential-revoke-0001")
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke credential status %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}

	webhookBody := `{"event":"app/uninstalled","endpointUrl":"https://hooks.example.com/lifecycle"}`
	privateWebhook := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhooks", `{"event":"app/uninstalled","endpointUrl":"https://127.0.0.1/hooks"}`, testOrganizationID, domain.RoleDeveloper, "webhook-private-0001")
	if privateWebhook.Code != http.StatusUnprocessableEntity {
		t.Fatalf("private webhook endpoint status %d, want 422: %s", privateWebhook.Code, privateWebhook.Body.String())
	}
	webhookResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhooks", webhookBody, testOrganizationID, domain.RoleDeveloper, "webhook-create-0001")
	createdWebhook := decodeData[application.WebhookSecret](t, webhookResponse)
	if webhookResponse.Code != http.StatusCreated || createdWebhook.SigningSecret == "" {
		t.Fatalf("create webhook status %d: %s", webhookResponse.Code, webhookResponse.Body.String())
	}
	version := postVersion(t, handler, app.ID, "1.0.0", "security-version-0001")
	if len(version.Snapshot.WebhookSubscriptions) != 1 || version.Snapshot.WebhookSubscriptions[0].SubscriptionID != createdWebhook.Subscription.ID {
		t.Fatalf("version snapshot missing active webhook: %#v", version.Snapshot.WebhookSubscriptions)
	}
	pauseBody := fmt.Sprintf(`{"status":"paused","revision":%d}`, createdWebhook.Subscription.Revision)
	pauseResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/webhooks/"+createdWebhook.Subscription.ID, pauseBody, testOrganizationID, domain.RoleDeveloper, "")
	paused := decodeData[domain.WebhookSubscription](t, pauseResponse)
	if pauseResponse.Code != http.StatusOK || paused.Status != domain.WebhookStatusPaused {
		t.Fatalf("pause webhook status %d: %s", pauseResponse.Code, pauseResponse.Body.String())
	}
	staleResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/webhooks/"+createdWebhook.Subscription.ID, pauseBody, testOrganizationID, domain.RoleDeveloper, "")
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale webhook update status %d, want 409", staleResponse.Code)
	}
	deleteResponse := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/webhooks/"+createdWebhook.Subscription.ID, "", testOrganizationID, domain.RoleDeveloper, "")
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete webhook status %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestInstallationLifecycle(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()
	app := postApp(t, handler, "installation-app-request-0001")
	version := postVersion(t, handler, app.ID, "1.0.0", "installation-version-request-0001")
	releaseVersion(t, handler, app.ID, version.ID, "installation-release-request-0001", nil, http.StatusOK, domain.RoleOwner)

	productionBody := `{"merchantId":"01995f72-0000-7000-8000-000000000098","merchantName":"Production Merchant","environment":"production"}`
	productionResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations", productionBody, testOrganizationID, domain.RoleOwner, "installation-production-denied-0001")
	if productionResponse.Code != http.StatusForbidden {
		t.Fatalf("production installation status %d, want 403: %s", productionResponse.Code, productionResponse.Body.String())
	}

	body := `{"merchantId":"01995f72-0000-7000-8000-000000000099","merchantName":"Demo Merchant","merchantDomain":"demo.emisell.test","environment":"sandbox"}`
	forbidden := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations", body, testOrganizationID, domain.RoleDeveloper, "installation-forbidden-0001")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("developer installation create status %d, want 403", forbidden.Code)
	}
	createResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations", body, testOrganizationID, domain.RoleOwner, "installation-create-0001")
	installation := decodeData[domain.AppInstallation](t, createResponse)
	if createResponse.Code != http.StatusCreated || installation.Status != domain.InstallationStatusActive || installation.InstalledVersionID != version.ID {
		t.Fatalf("create installation status=%d data=%#v", createResponse.Code, installation)
	}
	listResponse := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID+"/installations", "", testOrganizationID, domain.RoleAnalyst, "")
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list installations status %d: %s", listResponse.Code, listResponse.Body.String())
	}
	suspendBody := fmt.Sprintf(`{"status":"suspended","revision":%d}`, installation.Revision)
	suspendResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/installations/"+installation.ID, suspendBody, testOrganizationID, domain.RoleAdmin, "")
	suspended := decodeData[domain.AppInstallation](t, suspendResponse)
	if suspendResponse.Code != http.StatusOK || suspended.Status != domain.InstallationStatusSuspended || suspended.Revision != 2 {
		t.Fatalf("suspend installation status=%d data=%#v", suspendResponse.Code, suspended)
	}
	staleResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/installations/"+installation.ID, suspendBody, testOrganizationID, domain.RoleOwner, "")
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale installation update status %d, want 409", staleResponse.Code)
	}
	uninstallResponse := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/installations/"+installation.ID, "", testOrganizationID, domain.RoleOwner, "installation-uninstall-0001")
	if uninstallResponse.Code != http.StatusNoContent {
		t.Fatalf("uninstall status %d: %s", uninstallResponse.Code, uninstallResponse.Body.String())
	}
	replayResponse := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/installations/"+installation.ID, "", testOrganizationID, domain.RoleOwner, "installation-uninstall-0001")
	if replayResponse.Code != http.StatusNoContent {
		t.Fatalf("uninstall replay status %d", replayResponse.Code)
	}
}

func TestInstallationVersionUpgradeLifecycle(t *testing.T) {
	t.Parallel()
	handler, repository := testServer()
	app := postApp(t, handler, "upgrade-app-request-0001")

	v1Scopes := `{"scopes":[{"scope":"read_orders","access":"required"},{"scope":"write_orders","access":"optional"}]}`
	v1ScopesResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", v1Scopes, testOrganizationID, domain.RoleDeveloper, "")
	if v1ScopesResponse.Code != http.StatusOK {
		t.Fatalf("configure v1 scopes status %d: %s", v1ScopesResponse.Code, v1ScopesResponse.Body.String())
	}
	v1 := postVersion(t, handler, app.ID, "1.0.0", "upgrade-version-request-0001")
	releaseVersion(t, handler, app.ID, v1.ID, "upgrade-release-request-0001", nil, http.StatusOK, domain.RoleOwner)

	installBody := `{"merchantId":"01995f72-0000-7000-8000-000000000096","merchantName":"Upgrade Sandbox","environment":"sandbox","grantedScopes":["read_orders","write_orders"]}`
	installResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations", installBody, testOrganizationID, domain.RoleOwner, "upgrade-installation-create-0001")
	installation := decodeData[domain.AppInstallation](t, installResponse)
	if installResponse.Code != http.StatusCreated || len(installation.GrantedScopes) != 2 {
		t.Fatalf("create upgrade installation status=%d data=%#v", installResponse.Code, installation)
	}

	v2Scopes := `{"scopes":[{"scope":"read_orders","access":"required"},{"scope":"read_inventory","access":"required"},{"scope":"write_inventory","access":"optional"}]}`
	v2ScopesResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", v2Scopes, testOrganizationID, domain.RoleDeveloper, "")
	if v2ScopesResponse.Code != http.StatusOK {
		t.Fatalf("configure v2 scopes status %d: %s", v2ScopesResponse.Code, v2ScopesResponse.Body.String())
	}
	v2 := postVersion(t, handler, app.ID, "2.0.0", "upgrade-version-request-0002")
	releaseVersion(t, handler, app.ID, v2.ID, "upgrade-release-request-0002", &v1.ID, http.StatusOK, domain.RoleOwner)

	upgradeBody := fmt.Sprintf(`{"targetVersionId":%q,"expectedInstalledVersionId":%q,"revision":%d}`, v2.ID, v1.ID, installation.Revision)
	forbidden := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations/"+installation.ID+"/upgrade", upgradeBody, testOrganizationID, domain.RoleDeveloper, "upgrade-installation-forbidden-0001")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("developer upgrade status %d, want 403: %s", forbidden.Code, forbidden.Body.String())
	}
	upgradeResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations/"+installation.ID+"/upgrade", upgradeBody, testOrganizationID, domain.RoleAdmin, "upgrade-installation-request-0001")
	upgraded := decodeData[domain.AppInstallation](t, upgradeResponse)
	if upgradeResponse.Code != http.StatusOK || upgraded.InstalledVersionID != v2.ID || upgraded.Revision != installation.Revision+1 {
		t.Fatalf("upgrade status=%d data=%#v", upgradeResponse.Code, upgraded)
	}
	if strings.Join(upgraded.GrantedScopes, ",") != "read_inventory,read_orders" {
		t.Fatalf("upgraded scopes = %v, want required scopes only", upgraded.GrantedScopes)
	}

	replayResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations/"+installation.ID+"/upgrade", upgradeBody, testOrganizationID, domain.RoleAdmin, "upgrade-installation-request-0001")
	replayed := decodeData[domain.AppInstallation](t, replayResponse)
	if replayResponse.Code != http.StatusOK || replayed.ID != upgraded.ID || replayed.Revision != upgraded.Revision {
		t.Fatalf("upgrade replay status=%d data=%#v", replayResponse.Code, replayed)
	}
	staleResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations/"+installation.ID+"/upgrade", upgradeBody, testOrganizationID, domain.RoleOwner, "upgrade-installation-stale-0001")
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale upgrade status %d, want 409: %s", staleResponse.Code, staleResponse.Body.String())
	}
	crossTenant := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations/"+installation.ID+"/upgrade", upgradeBody, "01995f72-0000-7000-8000-000000000099", domain.RoleOwner, "upgrade-installation-tenant-0001")
	if crossTenant.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant upgrade status %d, want 404: %s", crossTenant.Code, crossTenant.Body.String())
	}

	auditEvents, err := repository.ListAuditEvents(t.Context(), testOrganizationID)
	if err != nil {
		t.Fatalf("list upgrade audit events: %v", err)
	}
	upgradeAudits := 0
	for _, event := range auditEvents {
		if event.Action == "installation.upgraded" && event.ResourceID == installation.ID {
			upgradeAudits++
			if event.Metadata["fromVersionId"] != v1.ID || event.Metadata["toVersionId"] != v2.ID {
				t.Fatalf("upgrade audit metadata = %#v", event.Metadata)
			}
		}
	}
	if upgradeAudits != 1 {
		t.Fatalf("upgrade audit events = %d, want 1", upgradeAudits)
	}
}

func TestOAuthAuthorizationCodeLifecycle(t *testing.T) {
	t.Parallel()
	handler, repository := testServer()
	app := postApp(t, handler, "oauth-app-request-0001")
	credentialResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "oauth-credential-0001")
	credential := decodeData[application.CredentialSecret](t, credentialResponse)
	scopeResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{"scopes":[{"scope":"read_orders","access":"required"},{"scope":"read_merchant","access":"optional"},{"scope":"write_orders","access":"optional"}]}`, testOrganizationID, domain.RoleDeveloper, "")
	if scopeResponse.Code != http.StatusOK {
		t.Fatalf("configure OAuth scopes status %d: %s", scopeResponse.Code, scopeResponse.Body.String())
	}
	version := postVersion(t, handler, app.ID, "1.0.0", "oauth-version-request-0001")
	releaseVersion(t, handler, app.ID, version.ID, "oauth-release-request-0001", nil, http.StatusOK, domain.RoleOwner)

	verifier := strings.Repeat("a", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	authorizeBody := fmt.Sprintf(`{"clientId":%q,"redirectUri":"https://apps.emisell.com/loyalty","state":"state-value-0000001","codeChallenge":%q,"merchantId":"01995f72-0000-7000-8000-000000000099","merchantName":"OAuth Merchant","merchantDomain":"oauth.example.test","environment":"sandbox","grantedScopes":["read_orders"]}`, credential.Credential.ClientID, challenge)
	forbiddenAuthorize := request(t, handler, http.MethodPost, "/v1/oauth/authorizations", authorizeBody, testOrganizationID, domain.RoleDeveloper, "")
	if forbiddenAuthorize.Code != http.StatusForbidden {
		t.Fatalf("developer authorize status %d, want 403: %s", forbiddenAuthorize.Code, forbiddenAuthorize.Body.String())
	}
	wrongRedirectBody := strings.Replace(authorizeBody, "https://apps.emisell.com/loyalty", "https://attacker.example/callback", 1)
	wrongRedirect := request(t, handler, http.MethodPost, "/v1/oauth/authorizations", wrongRedirectBody, testOrganizationID, domain.RoleOwner, "")
	if wrongRedirect.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong redirect status %d, want 422: %s", wrongRedirect.Code, wrongRedirect.Body.String())
	}
	authorizeResponse := request(t, handler, http.MethodPost, "/v1/oauth/authorizations", authorizeBody, testOrganizationID, domain.RoleOwner, "")
	if authorizeResponse.Code != http.StatusCreated {
		t.Fatalf("authorize status %d: %s", authorizeResponse.Code, authorizeResponse.Body.String())
	}
	if authorizeResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("authorization response cache control = %q, want no-store", authorizeResponse.Header().Get("Cache-Control"))
	}
	authorization := decodeData[application.OAuthAuthorizationResponse](t, authorizeResponse)
	if authorization.Code == "" || !strings.Contains(authorization.RedirectTo, "state=state-value-0000001") || strings.Join(authorization.Authorization.GrantedScopes, ",") != "read_orders" {
		t.Fatalf("authorization response incomplete: %#v", authorization)
	}

	exchange := func(code, codeVerifier string) *httptest.ResponseRecorder {
		form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://apps.emisell.com/loyalty"}, "code_verifier": {codeVerifier}}
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(credential.Credential.ClientID, credential.ClientSecret)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	badVerifier := exchange(authorization.Code, strings.Repeat("b", 43))
	if badVerifier.Code != http.StatusBadRequest || !strings.Contains(badVerifier.Body.String(), "invalid_grant") {
		t.Fatalf("bad verifier status %d: %s", badVerifier.Code, badVerifier.Body.String())
	}
	tokenResponse := exchange(authorization.Code, verifier)
	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("exchange status %d: %s", tokenResponse.Code, tokenResponse.Body.String())
	}
	if tokenResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("token response cache control = %q, want no-store", tokenResponse.Header().Get("Cache-Control"))
	}
	var token application.OAuthTokenResponse
	if err := json.Unmarshal(tokenResponse.Body.Bytes(), &token); err != nil || token.AccessToken == "" || token.Installation.Status != domain.InstallationStatusActive || token.Scope != "read_orders" {
		t.Fatalf("invalid token response: err=%v data=%#v", err, token)
	}
	missingToken := installationRequest(t, handler, "")
	if missingToken.Code != http.StatusUnauthorized || !strings.Contains(missingToken.Header().Get("WWW-Authenticate"), "invalid_token") {
		t.Fatalf("missing installation token status=%d authenticate=%q", missingToken.Code, missingToken.Header().Get("WWW-Authenticate"))
	}
	contextResponse := installationRequest(t, handler, token.AccessToken)
	access := decodeData[domain.InstallationAccessContext](t, contextResponse)
	if contextResponse.Code != http.StatusOK || contextResponse.Header().Get("Cache-Control") != "no-store" || access.InstallationID != token.Installation.ID || access.InstalledVersionID != version.ID || strings.Join(access.Scopes, ",") != "read_orders" {
		t.Fatalf("installation context status=%d data=%#v", contextResponse.Code, access)
	}
	if strings.Contains(contextResponse.Body.String(), token.AccessToken) || strings.Contains(contextResponse.Body.String(), credential.ClientSecret) {
		t.Fatal("installation context exposed protected token material")
	}
	if err := application.RequireInstallationScopes(access, "read_orders"); err != nil {
		t.Fatalf("granted scope rejected: %v", err)
	}
	if err := application.RequireInstallationScopes(access, "write_orders"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("missing scope error = %v, want forbidden", err)
	}
	deniedProfile := merchantProfileRequest(t, handler, token.AccessToken)
	if deniedProfile.Code != http.StatusForbidden || !strings.Contains(deniedProfile.Header().Get("WWW-Authenticate"), "insufficient_scope") || !strings.Contains(deniedProfile.Header().Get("WWW-Authenticate"), application.ScopeReadMerchant) {
		t.Fatalf("merchant profile without scope status=%d authenticate=%q body=%s", deniedProfile.Code, deniedProfile.Header().Get("WWW-Authenticate"), deniedProfile.Body.String())
	}

	profileAuthorizeBody := strings.NewReplacer(
		`"state":"state-value-0000001"`, `"state":"state-value-profile-0002"`,
		`"merchantId":"01995f72-0000-7000-8000-000000000099"`, `"merchantId":"01995f72-0000-7000-8000-000000000098"`,
		`"merchantName":"OAuth Merchant"`, `"merchantName":"Scoped Merchant"`,
		`"merchantDomain":"oauth.example.test"`, `"merchantDomain":"scoped.example.test"`,
		`"grantedScopes":["read_orders"]`, `"grantedScopes":["read_orders","read_merchant"]`,
	).Replace(authorizeBody)
	profileAuthorizeResponse := request(t, handler, http.MethodPost, "/v1/oauth/authorizations", profileAuthorizeBody, testOrganizationID, domain.RoleOwner, "")
	if profileAuthorizeResponse.Code != http.StatusCreated {
		t.Fatalf("scoped authorize status %d: %s", profileAuthorizeResponse.Code, profileAuthorizeResponse.Body.String())
	}
	profileAuthorization := decodeData[application.OAuthAuthorizationResponse](t, profileAuthorizeResponse)
	profileTokenResponse := exchange(profileAuthorization.Code, verifier)
	if profileTokenResponse.Code != http.StatusOK {
		t.Fatalf("scoped token status %d: %s", profileTokenResponse.Code, profileTokenResponse.Body.String())
	}
	var profileToken application.OAuthTokenResponse
	if err := json.Unmarshal(profileTokenResponse.Body.Bytes(), &profileToken); err != nil {
		t.Fatalf("decode scoped token: %v", err)
	}
	profileResponse := merchantProfileRequest(t, handler, profileToken.AccessToken)
	profile := decodeData[domain.MerchantProfile](t, profileResponse)
	if profileResponse.Code != http.StatusOK || profileResponse.Header().Get("Cache-Control") != "no-store" || profile.ID != "01995f72-0000-7000-8000-000000000098" || profile.Name != "Scoped Merchant" || profile.Domain == nil || *profile.Domain != "scoped.example.test" || profile.Environment != domain.EnvironmentSandbox || profile.InstallationID != profileToken.Installation.ID {
		t.Fatalf("merchant profile status=%d data=%#v", profileResponse.Code, profile)
	}
	if strings.Contains(profileResponse.Body.String(), profileToken.AccessToken) || strings.Contains(profileResponse.Body.String(), credential.ClientSecret) {
		t.Fatal("merchant profile exposed protected token material")
	}
	expiredAccess := application.NewInstallationAccessService(repository, func() time.Time {
		return time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC)
	})
	if _, err := expiredAccess.Authenticate(t.Context(), token.AccessToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expired token error = %v, want unauthorized", err)
	}

	suspendBody := fmt.Sprintf(`{"status":"suspended","revision":%d}`, token.Installation.Revision)
	suspendResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/installations/"+token.Installation.ID, suspendBody, testOrganizationID, domain.RoleAdmin, "")
	suspended := decodeData[domain.AppInstallation](t, suspendResponse)
	if suspendResponse.Code != http.StatusOK || suspended.Status != domain.InstallationStatusSuspended {
		t.Fatalf("suspend OAuth installation status=%d data=%#v", suspendResponse.Code, suspended)
	}
	if response := installationRequest(t, handler, token.AccessToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("suspended installation token status %d, want 401", response.Code)
	}
	resumeBody := fmt.Sprintf(`{"status":"active","revision":%d}`, suspended.Revision)
	resumeResponse := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/installations/"+token.Installation.ID, resumeBody, testOrganizationID, domain.RoleAdmin, "")
	if resumeResponse.Code != http.StatusOK || installationRequest(t, handler, token.AccessToken).Code != http.StatusOK {
		t.Fatalf("resumed installation token was not restored: %s", resumeResponse.Body.String())
	}
	replay := exchange(authorization.Code, verifier)
	if replay.Code != http.StatusBadRequest || !strings.Contains(replay.Body.String(), "invalid_grant") {
		t.Fatalf("authorization code replay status %d: %s", replay.Code, replay.Body.String())
	}
	if strings.Contains(tokenResponse.Body.String(), credential.ClientSecret) {
		t.Fatal("token response exposed the client secret")
	}
	revokeResponse := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/credentials/"+credential.Credential.ID, "", testOrganizationID, domain.RoleOwner, "oauth-credential-revoke-0001")
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke OAuth credential status %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	if response := installationRequest(t, handler, token.AccessToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked credential token status %d, want 401", response.Code)
	}
	if response := merchantProfileRequest(t, handler, profileToken.AccessToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked credential merchant profile status %d, want 401", response.Code)
	}
}

func TestSandboxMerchantConsentAndUninstallLifecycle(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()
	app := postApp(t, handler, "merchant-consent-app-0001")
	credentialResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "merchant-consent-credential-0001")
	credential := decodeData[application.CredentialSecret](t, credentialResponse)
	scopeResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{"scopes":[{"scope":"read_orders","access":"required"},{"scope":"read_merchant","access":"optional"}]}`, testOrganizationID, domain.RoleDeveloper, "")
	if scopeResponse.Code != http.StatusOK {
		t.Fatalf("configure merchant consent scopes status %d: %s", scopeResponse.Code, scopeResponse.Body.String())
	}
	version := postVersion(t, handler, app.ID, "1.0.0", "merchant-consent-version-0001")
	releaseVersion(t, handler, app.ID, version.ID, "merchant-consent-release-0001", nil, http.StatusOK, domain.RoleOwner)

	missingSession := merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/session", "", nil, "", "")
	if missingSession.Code != http.StatusUnauthorized {
		t.Fatalf("missing merchant session status %d, want 401", missingSession.Code)
	}
	login := merchantSessionRequest(t, handler, http.MethodPost, "/auth/sandbox-merchant-login", `{"merchantId":"cmmerchantconsent000000001","merchantName":"Consent Sandbox","merchantDomain":"consent.example.test"}`, nil, "", "")
	if login.Code != http.StatusCreated {
		t.Fatalf("merchant login status %d: %s", login.Code, login.Body.String())
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, cookie := range login.Result().Cookies() {
		switch cookie.Name {
		case "emisell_merchant_session":
			sessionCookie = cookie
			if !cookie.HttpOnly {
				t.Fatal("merchant session cookie must be HttpOnly")
			}
		case "emisell_merchant_csrf":
			csrfCookie = cookie
			if cookie.HttpOnly {
				t.Fatal("merchant CSRF cookie must be browser readable")
			}
		}
	}
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatalf("merchant cookies missing: %#v", login.Result().Cookies())
	}
	cookies := []*http.Cookie{sessionCookie, csrfCookie}
	sessionResponse := merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/session", "", cookies, "", "")
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), "cmmerchantconsent000000001") || strings.Contains(sessionResponse.Body.String(), "csrf") {
		t.Fatalf("merchant session status=%d body=%s", sessionResponse.Code, sessionResponse.Body.String())
	}

	verifier := strings.Repeat("m", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	previewBody := fmt.Sprintf(`{"clientId":%q,"redirectUri":"https://apps.emisell.com/loyalty","state":"merchant-state-value-0001","codeChallenge":%q,"requestedScopes":["read_merchant"]}`, credential.Credential.ClientID, challenge)
	badCSRF := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/preview", previewBody, cookies, "wrong-csrf-token", "")
	if badCSRF.Code != http.StatusForbidden {
		t.Fatalf("merchant preview bad CSRF status %d, want 403: %s", badCSRF.Code, badCSRF.Body.String())
	}
	previewResponse := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/preview", previewBody, cookies, csrfCookie.Value, "")
	if previewResponse.Code != http.StatusOK || !strings.Contains(previewResponse.Body.String(), app.Name) || !strings.Contains(previewResponse.Body.String(), "read_orders") || !strings.Contains(previewResponse.Body.String(), "read_merchant") {
		t.Fatalf("merchant preview status=%d body=%s", previewResponse.Code, previewResponse.Body.String())
	}

	tamperedAuthorizeBody := strings.TrimSuffix(previewBody, "}") + `,"grantedScopes":["read_orders","read_merchant"],"merchantId":"cmmerchanttampered00000001"}`
	tampered := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/authorize", tamperedAuthorizeBody, cookies, csrfCookie.Value, "")
	if tampered.Code != http.StatusUnprocessableEntity {
		t.Fatalf("tampered merchant identity status %d, want 422: %s", tampered.Code, tampered.Body.String())
	}
	authorizeBody := strings.TrimSuffix(previewBody, "}") + `,"grantedScopes":["read_orders","read_merchant"]}`
	authorizeResponse := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/authorize", authorizeBody, cookies, csrfCookie.Value, "")
	if authorizeResponse.Code != http.StatusCreated || authorizeResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("merchant authorize status %d: %s", authorizeResponse.Code, authorizeResponse.Body.String())
	}
	authorization := decodeData[application.OAuthAuthorizationResponse](t, authorizeResponse)
	if authorization.Authorization.MerchantID != "cmmerchantconsent000000001" || authorization.Authorization.MerchantName != "Consent Sandbox" || !strings.Contains(authorization.RedirectTo, "state=merchant-state-value-0001") {
		t.Fatalf("merchant authorization did not use session identity: %#v", authorization)
	}

	form := url.Values{"grant_type": {"authorization_code"}, "code": {authorization.Code}, "redirect_uri": {"https://apps.emisell.com/loyalty"}, "code_verifier": {verifier}}
	exchangeRequest := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	exchangeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	exchangeRequest.SetBasicAuth(credential.Credential.ClientID, credential.ClientSecret)
	exchangeResponse := httptest.NewRecorder()
	handler.ServeHTTP(exchangeResponse, exchangeRequest)
	if exchangeResponse.Code != http.StatusOK {
		t.Fatalf("merchant token exchange status %d: %s", exchangeResponse.Code, exchangeResponse.Body.String())
	}
	var token application.OAuthTokenResponse
	if err := json.Unmarshal(exchangeResponse.Body.Bytes(), &token); err != nil || token.Installation.MerchantID != "cmmerchantconsent000000001" {
		t.Fatalf("merchant token response err=%v data=%#v", err, token)
	}
	connected := merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/installations", "", cookies, "", "")
	apps := decodeData[[]domain.MerchantInstalledApp](t, connected)
	if connected.Code != http.StatusOK || len(apps) != 1 || apps[0].AppID != app.ID || apps[0].Version != "1.0.0" {
		t.Fatalf("merchant connected apps status=%d data=%#v", connected.Code, apps)
	}
	uninstall := merchantSessionRequest(t, handler, http.MethodDelete, "/v1/merchant/installations/"+token.Installation.ID, "", cookies, csrfCookie.Value, "merchant-uninstall-0001")
	if uninstall.Code != http.StatusNoContent {
		t.Fatalf("merchant uninstall status %d: %s", uninstall.Code, uninstall.Body.String())
	}
	if response := installationRequest(t, handler, token.AccessToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("merchant uninstall token status %d, want 401", response.Code)
	}
	connected = merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/installations", "", cookies, "", "")
	apps = decodeData[[]domain.MerchantInstalledApp](t, connected)
	if connected.Code != http.StatusOK || len(apps) != 0 {
		t.Fatalf("merchant connected apps after uninstall status=%d data=%#v", connected.Code, apps)
	}
	logout := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/session/logout", "", cookies, csrfCookie.Value, "")
	if logout.Code != http.StatusNoContent {
		t.Fatalf("merchant logout status %d: %s", logout.Code, logout.Body.String())
	}
	if response := merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/session", "", cookies, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked merchant session status %d, want 401", response.Code)
	}
}

func TestDevelopmentInstallRequestBindsMerchantConsent(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()
	app := postApp(t, handler, "development-install-app-0001")
	credentialResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "development-install-credential-0001")
	credential := decodeData[application.CredentialSecret](t, credentialResponse)
	scopeResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{"scopes":[{"scope":"read_merchant","access":"required"}]}`, testOrganizationID, domain.RoleDeveloper, "")
	if scopeResponse.Code != http.StatusOK {
		t.Fatalf("configure development scope status=%d body=%s", scopeResponse.Code, scopeResponse.Body.String())
	}
	version := postVersion(t, handler, app.ID, "1.0.0", "development-install-version-0001")
	releaseVersion(t, handler, app.ID, version.ID, "development-install-release-0001", nil, http.StatusOK, domain.RoleOwner)

	login := merchantSessionRequest(t, handler, http.MethodPost, "/auth/sandbox-merchant-login", `{"merchantId":"cmmerchantdevelopment00001","merchantName":"Development Merchant","merchantDomain":"development.example.test"}`, nil, "", "")
	if login.Code != http.StatusCreated {
		t.Fatalf("sandbox merchant login status=%d body=%s", login.Code, login.Body.String())
	}
	var csrfCookie *http.Cookie
	cookies := login.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "emisell_merchant_csrf" {
			csrfCookie = cookie
		}
	}
	if csrfCookie == nil {
		t.Fatal("merchant CSRF cookie is missing")
	}

	createResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/test-install-requests", `{"merchantId":"cmmerchantdevelopment00001"}`, testOrganizationID, domain.RoleOwner, "development-install-request-0001")
	created := decodeData[domain.DevelopmentInstallRequest](t, createResponse)
	if createResponse.Code != http.StatusCreated || created.Status != domain.DevelopmentInstallRequestStatusPending || !strings.Contains(created.LaunchURL, "emisell_test_install_request="+created.ID) || strings.Contains(created.LaunchURL, "emisell_environment") || strings.Contains(createResponse.Body.String(), `"environment"`) || created.MerchantName != "Development Merchant" {
		t.Fatalf("development request status=%d data=%#v", createResponse.Code, created)
	}
	duplicate := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/test-install-requests", `{"merchantId":"cmmerchantdevelopment00001"}`, testOrganizationID, domain.RoleOwner, "development-install-request-0002")
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate request status=%d want 409 body=%s", duplicate.Code, duplicate.Body.String())
	}

	verifier := strings.Repeat("d", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	previewBody := fmt.Sprintf(`{"clientId":%q,"redirectUri":"https://apps.emisell.com/loyalty","state":"development-state-000001","codeChallenge":%q,"requestedScopes":[],"testInstallRequestId":%q}`, credential.Credential.ClientID, challenge, created.ID)
	otherLogin := merchantSessionRequest(t, handler, http.MethodPost, "/auth/sandbox-merchant-login", `{"merchantId":"cmmerchantdevelopmentother1","merchantName":"Other Merchant","merchantDomain":"other.example.test"}`, nil, "", "")
	var otherCSRF *http.Cookie
	otherCookies := otherLogin.Result().Cookies()
	for _, cookie := range otherCookies {
		if cookie.Name == "emisell_merchant_csrf" {
			otherCSRF = cookie
		}
	}
	if otherLogin.Code != http.StatusCreated || otherCSRF == nil {
		t.Fatalf("other merchant login status=%d cookies=%#v", otherLogin.Code, otherCookies)
	}
	crossMerchant := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/preview", previewBody, otherCookies, otherCSRF.Value, "")
	if crossMerchant.Code != http.StatusNotFound {
		t.Fatalf("cross-merchant request status=%d want 404 body=%s", crossMerchant.Code, crossMerchant.Body.String())
	}
	preview := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/preview", previewBody, cookies, csrfCookie.Value, "")
	consent := decodeData[application.MerchantOAuthConsent](t, preview)
	if preview.Code != http.StatusOK || !consent.DevelopmentInstall || consent.Merchant.MerchantID != created.MerchantID {
		t.Fatalf("development consent status=%d data=%#v body=%s", preview.Code, consent, preview.Body.String())
	}

	authorizeBody := strings.TrimSuffix(previewBody, "}") + `,"grantedScopes":["read_merchant"]}`
	authorized := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/authorize", authorizeBody, cookies, csrfCookie.Value, "")
	if authorized.Code != http.StatusCreated {
		t.Fatalf("development authorize status=%d body=%s", authorized.Code, authorized.Body.String())
	}
	listedResponse := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID+"/test-install-requests", "", testOrganizationID, domain.RoleAnalyst, "")
	listed := decodeData[[]domain.DevelopmentInstallRequest](t, listedResponse)
	if listedResponse.Code != http.StatusOK || len(listed) != 1 || listed[0].Status != domain.DevelopmentInstallRequestStatusAuthorized || listed[0].AuthorizationID == nil {
		t.Fatalf("authorized request status=%d data=%#v", listedResponse.Code, listed)
	}
	replay := merchantSessionRequest(t, handler, http.MethodPost, "/v1/merchant/oauth/preview", previewBody, cookies, csrfCookie.Value, "")
	if replay.Code != http.StatusNotFound {
		t.Fatalf("authorized request replay status=%d want 404 body=%s", replay.Code, replay.Body.String())
	}
	publishBody := `{"category":"custom","status":"published","featured":false,"revision":0}`
	published := request(t, handler, http.MethodPut, "/v1/internal/organizations/"+testOrganizationID+"/apps/"+app.ID+"/catalog-listing", publishBody, testOrganizationID, domain.RoleOwner, "development-install-publish-0001")
	if published.Code != http.StatusOK {
		t.Fatalf("publish development app status=%d body=%s", published.Code, published.Body.String())
	}
	releasedRequest := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/test-install-requests", `{"merchantId":"cmmerchantdevelopment00001"}`, testOrganizationID, domain.RoleOwner, "development-install-request-0003")
	if releasedRequest.Code != http.StatusConflict {
		t.Fatalf("released app test install status=%d want 409 body=%s", releasedRequest.Code, releasedRequest.Body.String())
	}
}

type webhookClientFunc func(*http.Request) (*http.Response, error)

func (function webhookClientFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestSandboxDeveloperJourneyAcceptance(t *testing.T) {
	t.Parallel()
	handler, repository := testServer()
	app := postApp(t, handler, "acceptance-app-0001")

	extensionResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/extensions", `{"name":"Order Sync","type":"custom","runtimeUrl":"https://runtime.example.com/order-sync","configuration":{"mode":"sandbox"}}`, testOrganizationID, domain.RoleDeveloper, "acceptance-extension-0001")
	if extensionResponse.Code != http.StatusCreated {
		t.Fatalf("create extension status %d: %s", extensionResponse.Code, extensionResponse.Body.String())
	}

	scopeResponse := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{"scopes":[{"scope":"read_orders","access":"required"},{"scope":"write_orders","access":"optional"}]}`, testOrganizationID, domain.RoleDeveloper, "")
	if scopeResponse.Code != http.StatusOK {
		t.Fatalf("replace scopes status %d: %s", scopeResponse.Code, scopeResponse.Body.String())
	}

	webhookResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhooks", `{"event":"app/uninstalled","endpointUrl":"https://hooks.example.com/lifecycle"}`, testOrganizationID, domain.RoleDeveloper, "acceptance-webhook-0001")
	if webhookResponse.Code != http.StatusCreated {
		t.Fatalf("create webhook status %d: %s", webhookResponse.Code, webhookResponse.Body.String())
	}
	webhook := decodeData[application.WebhookSecret](t, webhookResponse)

	version := postVersion(t, handler, app.ID, "1.0.0", "acceptance-version-0001")
	if len(version.Snapshot.Extensions) != 1 || len(version.Snapshot.Scopes) != 2 || len(version.Snapshot.WebhookSubscriptions) != 1 {
		t.Fatalf("version snapshot is incomplete: %#v", version.Snapshot)
	}
	releaseVersion(t, handler, app.ID, version.ID, "acceptance-release-0001", nil, http.StatusOK, domain.RoleOwner)

	credentialResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "acceptance-credential-0001")
	credential := decodeData[application.CredentialSecret](t, credentialResponse)
	if credentialResponse.Code != http.StatusCreated || credential.ClientSecret == "" || credential.Credential.Environment != domain.EnvironmentSandbox {
		t.Fatalf("sandbox credential status=%d data=%#v", credentialResponse.Code, credential)
	}

	installationResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations", `{"merchantId":"01995f72-0000-7000-8000-000000000097","merchantName":"Acceptance Sandbox","merchantDomain":"acceptance.example.test","environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "acceptance-installation-0001")
	installation := decodeData[domain.AppInstallation](t, installationResponse)
	if installationResponse.Code != http.StatusCreated || installation.Environment != domain.EnvironmentSandbox || installation.InstalledVersionID != version.ID || len(installation.GrantedScopes) != 2 {
		t.Fatalf("sandbox installation status=%d data=%#v", installationResponse.Code, installation)
	}

	publishResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhook-events", `{"event":"app/uninstalled","payload":{"resourceId":"test_installation","developmentTest":true}}`, testOrganizationID, domain.RoleOwner, "acceptance-event-0001")
	published := decodeData[application.PublishWebhookResponse](t, publishResponse)
	if publishResponse.Code != http.StatusAccepted || published.Event.ID == "" || len(published.Deliveries) != 1 || published.Deliveries[0].SubscriptionID != webhook.Subscription.ID || published.Deliveries[0].Status != domain.WebhookDeliveryStatusPending {
		t.Fatalf("sandbox webhook publish status=%d data=%#v", publishResponse.Code, published)
	}

	deliveriesResponse := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID+"/webhooks/"+webhook.Subscription.ID+"/deliveries", "", testOrganizationID, domain.RoleAnalyst, "")
	deliveries := decodeData[[]domain.WebhookDelivery](t, deliveriesResponse)
	if deliveriesResponse.Code != http.StatusOK || len(deliveries) != 1 || deliveries[0].EventID != published.Event.ID {
		t.Fatalf("sandbox delivery log status=%d data=%#v", deliveriesResponse.Code, deliveries)
	}
	uninstallResponse := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/installations/"+installation.ID, "", testOrganizationID, domain.RoleOwner, "acceptance-uninstall-0001")
	if uninstallResponse.Code != http.StatusNoContent {
		t.Fatalf("uninstall status=%d body=%s", uninstallResponse.Code, uninstallResponse.Body.String())
	}
	afterUninstall := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID+"/webhooks/"+webhook.Subscription.ID+"/deliveries", "", testOrganizationID, domain.RoleAnalyst, "")
	lifecycleDeliveries := decodeData[[]domain.WebhookDelivery](t, afterUninstall)
	if afterUninstall.Code != http.StatusOK || len(lifecycleDeliveries) != 2 {
		t.Fatalf("uninstall delivery status=%d data=%#v", afterUninstall.Code, lifecycleDeliveries)
	}
	claimed, err := repository.ClaimWebhookDeliveries(t.Context(), 10, time.Date(2026, time.August, 31, 7, 0, 1, 0, time.UTC), time.Time{})
	if err != nil {
		t.Fatalf("claim uninstall delivery: %v", err)
	}
	foundLifecycleContext := false
	for _, delivery := range claimed {
		if delivery.EventSource == domain.WebhookEventSourceAppPlatform && delivery.MerchantID != nil && *delivery.MerchantID == installation.MerchantID && delivery.InstallationID != nil && *delivery.InstallationID == installation.ID {
			foundLifecycleContext = true
		}
	}
	if !foundLifecycleContext {
		t.Fatalf("app/uninstalled delivery is missing merchant installation context: %#v", claimed)
	}
}

func TestWebhookDeliveryIsSignedAndPersisted(t *testing.T) {
	t.Parallel()
	handler, repository := testServer()
	app := postApp(t, handler, "delivery-app-request-0001")
	webhookResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhooks", `{"event":"app/uninstalled","endpointUrl":"https://hooks.example.com/lifecycle"}`, testOrganizationID, domain.RoleDeveloper, "delivery-webhook-0001")
	webhook := decodeData[application.WebhookSecret](t, webhookResponse)
	version := postVersion(t, handler, app.ID, "1.0.0", "delivery-version-0001")
	releaseVersion(t, handler, app.ID, version.ID, "delivery-release-0001", nil, http.StatusOK, domain.RoleOwner)
	publish := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/webhook-events", `{"event":"app/uninstalled","payload":{"resourceId":"test_installation"}}`, testOrganizationID, domain.RoleOwner, "delivery-event-0001")
	if publish.Code != http.StatusAccepted {
		t.Fatalf("publish status %d: %s", publish.Code, publish.Body.String())
	}

	var signature string
	calls := 0
	client := webhookClientFunc(func(request *http.Request) (*http.Response, error) {
		signature = request.Header.Get("X-Emisell-Webhook-Signature")
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("temporary receiver failure")
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	secretBox, err := security.NewSecretBox([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	currentTime := time.Date(2026, time.August, 31, 7, 0, 0, 0, time.UTC)
	now := func() time.Time { return currentTime }
	dispatcher := application.NewWebhookDispatcher(repository, secretBox, client, func() (string, error) { return "01995f72-0000-7000-8000-999999999999", nil }, now, slog.New(slog.NewTextHandler(io.Discard, nil)), application.WebhookDispatcherOptions{BatchSize: 10, MaxAttempts: 3, BaseRetry: time.Second, MaxRetry: time.Minute})
	processed, err := dispatcher.ProcessBatch(context.Background())
	if err != nil || processed != 1 || !strings.HasPrefix(signature, "t=") || !strings.Contains(signature, ",v1=") {
		t.Fatalf("dispatch failed: processed=%d signature=%q err=%v", processed, signature, err)
	}
	currentTime = currentTime.Add(2 * time.Second)
	processed, err = dispatcher.ProcessBatch(context.Background())
	if err != nil || processed != 1 || calls != 2 {
		t.Fatalf("retry dispatch failed: processed=%d calls=%d err=%v", processed, calls, err)
	}
	deliveries := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID+"/webhooks/"+webhook.Subscription.ID+"/deliveries", "", testOrganizationID, domain.RoleAnalyst, "")
	listed := decodeData[[]domain.WebhookDelivery](t, deliveries)
	if deliveries.Code != http.StatusOK || len(listed) != 2 || listed[0].Status != domain.WebhookDeliveryStatusDelivered || listed[1].Status != domain.WebhookDeliveryStatusFailed {
		t.Fatalf("unexpected delivery list: status=%d data=%#v", deliveries.Code, listed)
	}
	if strings.Contains(deliveries.Body.String(), webhook.SigningSecret) || strings.Contains(deliveries.Body.String(), "payload") {
		t.Fatal("delivery observability exposed protected webhook material")
	}
}

func testServer() (http.Handler, *memory.Repository) {
	return testServerWithProducts(nil, func() time.Time { return time.Date(2026, time.August, 31, 7, 0, 0, 0, time.UTC) })
}

func testServerWithProducts(products *emisell.Products, now func() time.Time) (http.Handler, *memory.Repository) {
	return testServerWithIntegrations(products, nil, now)
}

func testServerWithShippingRates(shippingClient application.ShippingRateClient, now func() time.Time) (http.Handler, *memory.Repository) {
	return testServerWithIntegrations(nil, shippingClient, now)
}

func testServerWithIntegrations(products *emisell.Products, shippingClient application.ShippingRateClient, now func() time.Time) (http.Handler, *memory.Repository) {
	var mu sync.Mutex
	sequence := 0
	id := func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		sequence++
		return fmt.Sprintf("01995f72-0000-7000-8000-%012d", sequence), nil
	}
	repository := memory.NewRepository(id, now)
	apps := application.NewAppService(repository, id, now)
	versions := application.NewVersionService(repository, application.CurrentConfigurationSnapshotBuilder{Repository: repository}, id, now)
	extensions := application.NewExtensionService(repository, id, now)
	scopes := application.NewScopeService(repository)
	secretBox, err := security.NewSecretBox([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		panic(err)
	}
	credentials := application.NewCredentialService(repository, secretBox, 1, id, now)
	webhooks := application.NewWebhookService(repository, secretBox, 1, id, now)
	installations := application.NewInstallationService(repository, id, now)
	developmentInstalls := application.NewDevelopmentInstallService(repository, repository, id, now, 7*24*time.Hour)
	installationAccess := application.NewInstallationAccessService(repository, now)
	oauth := application.NewOAuthService(repository, secretBox, id, now, 5*time.Minute, time.Hour)
	identity := application.NewIdentityService(repository, id, now, 12*time.Hour, 2*time.Hour)
	merchants := application.NewMerchantService(repository, repository, repository, identity, id, now)
	catalog := application.NewCatalogService(repository, repository, now)
	emisellIntegration := application.NewEmisellIntegrationService(repository, repository, repository, identity, id, now, 2*time.Minute, "http://localhost:8081")
	shippingRates := application.NewShippingRateService(repository, shippingClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpapi.NewServer(httpapi.Dependencies{
		// Handler tests inject an operator principal. Real cookie/password
		// boundaries are exercised separately by admin_login tests.
		AdminAuthenticator:  httpapi.DevelopmentAuthenticator{BearerToken: testToken, DefaultUserID: testUserID, DefaultRole: domain.RoleOwner, PlatformOperator: true},
		Products:            products,
		ShippingRates:       shippingRates,
		Apps:                apps,
		Versions:            versions,
		Extensions:          extensions,
		Scopes:              scopes,
		Credentials:         credentials,
		Webhooks:            webhooks,
		Installations:       installations,
		DevelopmentInstalls: developmentInstalls,
		InstallationAccess:  installationAccess,
		OAuth:               oauth,
		Identity:            identity,
		Merchants:           merchants,
		Catalog:             catalog,
		EmisellIntegration:  emisellIntegration,
		EmisellBackendAuth:  httpapi.DevelopmentEmisellBackendAuthenticator{BearerToken: "emisell-backend-test-token"},
		IdentityHTTP: httpapi.IdentityHTTPOptions{
			FrontendURL:       "http://localhost:3003",
			SessionCookieName: "emisell_session", CSRFCookieName: "emisell_csrf",
			MerchantSessionCookieName: "emisell_merchant_session", MerchantCSRFCookieName: "emisell_merchant_csrf",
			AllowedOrigins: []string{"http://localhost:3003"}, Development: &httpapi.DevelopmentSessionIdentity{},
		},
		Authenticator: httpapi.DevelopmentAuthenticator{
			BearerToken:   testToken,
			DefaultUserID: testUserID,
			DefaultRole:   domain.RoleOwner, PlatformOperator: true,
		},
		Logger: logger,
	})
	return handler, repository
}

func postApp(t *testing.T, handler http.Handler, key string) domain.App {
	t.Helper()
	response := request(t, handler, http.MethodPost, "/v1/apps", `{"name":"Loyalty Connect","distribution":"custom","appUrl":"https://apps.emisell.com/loyalty"}`, testOrganizationID, domain.RoleOwner, key)
	if response.Code != http.StatusCreated {
		t.Fatalf("create app status %d: %s", response.Code, response.Body.String())
	}
	return decodeData[domain.App](t, response)
}

func postVersion(t *testing.T, handler http.Handler, appID, version, key string) domain.AppVersion {
	t.Helper()
	body := fmt.Sprintf(`{"version":%q,"releaseNote":"Test release"}`, version)
	response := request(t, handler, http.MethodPost, "/v1/apps/"+appID+"/versions", body, testOrganizationID, domain.RoleDeveloper, key)
	if response.Code != http.StatusCreated {
		t.Fatalf("create version status %d: %s", response.Code, response.Body.String())
	}
	return decodeData[domain.AppVersion](t, response)
}

func releaseVersion(t *testing.T, handler http.Handler, appID, versionID, key string, expectedActiveVersionID *string, expectedStatus int, role domain.Role) {
	t.Helper()
	body := `{}`
	if expectedActiveVersionID != nil {
		body = fmt.Sprintf(`{"expectedActiveVersionId":%q}`, *expectedActiveVersionID)
	}
	response := request(t, handler, http.MethodPost, "/v1/apps/"+appID+"/versions/"+versionID+"/release", body, testOrganizationID, role, key)
	if response.Code != expectedStatus {
		t.Fatalf("release version status %d, want %d: %s", response.Code, expectedStatus, response.Body.String())
	}
}

func rollbackVersion(t *testing.T, handler http.Handler, appID, versionID, key string) {
	t.Helper()
	response := request(t, handler, http.MethodPost, "/v1/apps/"+appID+"/versions/"+versionID+"/rollback", "", testOrganizationID, domain.RoleOwner, key)
	if response.Code != http.StatusOK {
		t.Fatalf("rollback version status %d: %s", response.Code, response.Body.String())
	}
}

func getApp(t *testing.T, handler http.Handler, organizationID, appID string, expectedStatus int) domain.App {
	t.Helper()
	response := request(t, handler, http.MethodGet, "/v1/apps/"+appID, "", organizationID, domain.RoleAnalyst, "")
	if response.Code != expectedStatus {
		t.Fatalf("get app status %d, want %d: %s", response.Code, expectedStatus, response.Body.String())
	}
	if expectedStatus != http.StatusOK {
		return domain.App{}
	}
	return decodeData[domain.App](t, response)
}

func request(t *testing.T, handler http.Handler, method, path, body, organizationID string, role domain.Role, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+testToken)
	request.Header.Set("X-Organization-Id", organizationID)
	request.Header.Set("X-Emisell-Role", string(role))
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func installationRequest(t *testing.T, handler http.Handler, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/installation-context", nil)
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func merchantProfileRequest(t *testing.T, handler http.Handler, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/merchant/profile", nil)
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func merchantSessionRequest(t *testing.T, handler http.Handler, method, path, body string, cookies []*http.Cookie, csrfToken, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if csrfToken != "" {
		request.Header.Set("X-CSRF-Token", csrfToken)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeData[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var payload struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload.Data
}
