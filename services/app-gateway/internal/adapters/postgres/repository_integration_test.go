package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	postgresadapter "emisell-app-platform/services/app-gateway/internal/adapters/postgres"
	"emisell-app-platform/services/app-gateway/internal/database"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func TestRepositoryPersistsAppVersionLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := database.Open(ctx, database.Options{
		URL:             databaseURL,
		MaxConnections:  5,
		MinConnections:  0,
		ConnectTimeout:  5 * time.Second,
		ApplicationName: "emisell-app-gateway-integration-test",
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	organizationID := mustID(t)
	userID := mustID(t)
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, organizationID, userID, "owner", userID+"@integration.emisell.test", "Local Developer"); err != nil {
		t.Fatalf("bootstrap identity: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM audit_events WHERE organization_id = $1::uuid`, organizationID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM idempotency_keys WHERE organization_id = $1::uuid`, organizationID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM organizations WHERE id = $1::uuid`, organizationID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	now := func() time.Time { return time.Now().UTC() }
	repository := postgresadapter.NewRepository(pool, ids.NewUUIDv7, now)
	appID := mustID(t)
	app := domain.App{
		ID:             appID,
		OrganizationID: organizationID,
		Name:           "Integration App",
		Slug:           "integration-app",
		Distribution:   domain.DistributionCustom,
		Status:         domain.AppStatusDraft,
		CreatedBy:      userID,
		CreatedAt:      now(),
		UpdatedAt:      now(),
		Revision:       1,
	}
	created, err := repository.CreateApp(ctx, app, ports.MutationMeta{
		ActorID:        userID,
		Action:         "app.created",
		IdempotencyKey: "integration-create-app-0001",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	replayed, err := repository.CreateApp(ctx, app, ports.MutationMeta{
		ActorID:        userID,
		Action:         "app.created",
		IdempotencyKey: "integration-create-app-0001",
	})
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("replay app: id=%q err=%v", replayed.ID, err)
	}

	extensionID := mustID(t)
	runtimeURL := "https://extensions.example.com/payments"
	extension := domain.AppExtension{
		ID: extensionID, AppID: appID, Name: "Payments Runtime", Type: domain.ExtensionTypePayment,
		Status: domain.ExtensionStatusDraft, RuntimeURL: &runtimeURL,
		Configuration: map[string]interface{}{"mode": "sandbox"}, CreatedAt: now(), UpdatedAt: now(), Revision: 1,
	}
	createdExtension, err := repository.CreateExtension(ctx, organizationID, extension, ports.MutationMeta{
		ActorID: userID, Action: "extension.created", IdempotencyKey: "integration-create-extension-0001",
	})
	if err != nil {
		t.Fatalf("create extension: %v", err)
	}
	replayedExtension, err := repository.CreateExtension(ctx, organizationID, extension, ports.MutationMeta{
		ActorID: userID, Action: "extension.created", IdempotencyKey: "integration-create-extension-0001",
	})
	if err != nil || replayedExtension.ID != createdExtension.ID {
		t.Fatalf("replay extension: id=%q err=%v", replayedExtension.ID, err)
	}
	scopes, err := repository.ReplaceScopes(ctx, organizationID, appID, []domain.AppScope{
		{AppID: appID, Scope: "write_products", Access: domain.ScopeAccessRequired},
		{AppID: appID, Scope: "read_orders", Access: domain.ScopeAccessOptional},
	}, ports.MutationMeta{ActorID: userID, Action: "scopes.replaced"})
	if err != nil || len(scopes) != 2 {
		t.Fatalf("replace scopes: count=%d err=%v", len(scopes), err)
	}

	versionOne := createVersion(t, ctx, repository, organizationID, userID, appID, "1.0.0", "integration-version-0001")
	activeOne, err := repository.ActivateVersion(ctx, organizationID, appID, versionOne.ID, domain.VersionStatusDraft, nil, ports.MutationMeta{
		ActorID:        userID,
		Action:         "version.released",
		IdempotencyKey: "integration-release-0001",
	})
	if err != nil || activeOne.Status != domain.VersionStatusActive {
		t.Fatalf("release first version: status=%q err=%v", activeOne.Status, err)
	}

	versionTwo := createVersion(t, ctx, repository, organizationID, userID, appID, "1.1.0", "integration-version-0002")
	activeTwo, err := repository.ActivateVersion(ctx, organizationID, appID, versionTwo.ID, domain.VersionStatusDraft, &versionOne.ID, ports.MutationMeta{
		ActorID:        userID,
		Action:         "version.released",
		IdempotencyKey: "integration-release-0002",
	})
	if err != nil || activeTwo.Status != domain.VersionStatusActive {
		t.Fatalf("release second version: status=%q err=%v", activeTwo.Status, err)
	}

	rolledBack, err := repository.ActivateVersion(ctx, organizationID, appID, versionOne.ID, domain.VersionStatusReleased, nil, ports.MutationMeta{
		ActorID:        userID,
		Action:         "version.rolled_back",
		IdempotencyKey: "integration-rollback-0001",
	})
	if err != nil || rolledBack.Status != domain.VersionStatusActive {
		t.Fatalf("rollback first version: status=%q err=%v", rolledBack.Status, err)
	}
	createdExtension.Name = "Payments Runtime v2"
	createdExtension.Status = domain.ExtensionStatusDraft
	updatedExtension, err := repository.UpdateExtension(ctx, organizationID, createdExtension, createdExtension.Revision, ports.MutationMeta{ActorID: userID, Action: "extension.updated"})
	if err != nil || updatedExtension.Revision != 2 {
		t.Fatalf("update extension: revision=%d err=%v", updatedExtension.Revision, err)
	}
	if err := repository.DisableExtension(ctx, organizationID, appID, extensionID, ports.MutationMeta{ActorID: userID, Action: "extension.disabled"}); err != nil {
		t.Fatalf("disable extension: %v", err)
	}
	disabledExtension, err := repository.GetExtension(ctx, organizationID, appID, extensionID)
	if err != nil || disabledExtension.Status != domain.ExtensionStatusDisabled {
		t.Fatalf("get disabled extension: status=%q err=%v", disabledExtension.Status, err)
	}

	persisted, err := repository.GetApp(ctx, organizationID, appID)
	if err != nil {
		t.Fatalf("get persisted app: %v", err)
	}
	if persisted.ActiveVersionID == nil || *persisted.ActiveVersionID != versionOne.ID || persisted.Revision != 4 {
		t.Fatalf("unexpected persisted app state: %#v", persisted)
	}
	if _, err := repository.GetApp(ctx, mustID(t), appID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant read error = %v, want not found", err)
	}
	auditEvents, err := repository.ListAuditEvents(ctx, organizationID)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(auditEvents) != 10 {
		t.Fatalf("got %d audit events, want 10", len(auditEvents))
	}
}

func createVersion(t *testing.T, ctx context.Context, repository *postgresadapter.Repository, organizationID, userID, appID, number, key string) domain.AppVersion {
	t.Helper()
	version := domain.AppVersion{
		ID:        mustID(t),
		AppID:     appID,
		Version:   number,
		Status:    domain.VersionStatusDraft,
		Snapshot:  domain.VersionSnapshot{Extensions: []domain.SnapshotExtension{}, Scopes: []domain.SnapshotScope{}, WebhookSubscriptions: []domain.SnapshotWebhook{}, RedirectURLs: []string{}, ConfigurationHash: "integration-test"},
		CreatedBy: userID,
		CreatedAt: time.Now().UTC(),
	}
	created, err := repository.CreateVersion(ctx, organizationID, version, ports.MutationMeta{
		ActorID:        userID,
		Action:         "version.created",
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("create version %s: %v", number, err)
	}
	return created
}

func mustID(t *testing.T) string {
	t.Helper()
	id, err := ids.NewUUIDv7()
	if err != nil {
		t.Fatalf("generate id: %v", err)
	}
	return id
}

func TestRepositoryPersistsIdentitySessionAndMembershipSwitch(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := database.Open(ctx, database.Options{URL: databaseURL, MaxConnections: 5, ConnectTimeout: 5 * time.Second, ApplicationName: "emisell-identity-integration-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	firstOrg, secondOrg, userID := mustID(t), mustID(t), mustID(t)
	email := userID + "@identity.integration.test"
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, firstOrg, userID, "owner", email, "Identity Test"); err != nil {
		t.Fatal(err)
	}
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, secondOrg, userID, "developer", email, "Identity Test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM organizations WHERE id = ANY($1::uuid[])`, []string{firstOrg, secondOrg})
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})
	now := time.Now().UTC()
	repository := postgresadapter.NewRepository(pool, ids.NewUUIDv7, func() time.Time { return now })
	compactUserID := strings.ReplaceAll(userID, "-", "")
	session := domain.IdentitySession{
		ID: mustID(t), TokenHash: compactUserID + compactUserID,
		CSRFTokenHash: compactUserID + strings.Repeat("b", 32),
		UserID:        userID, ActiveOrgID: &firstOrg, PlatformOperator: true, CreatedAt: now, LastSeenAt: now,
		ExpiresAt: now.Add(12 * time.Hour), IdleExpiresAt: now.Add(2 * time.Hour),
	}
	if err := repository.CreateIdentitySession(ctx, session); err != nil {
		t.Fatal(err)
	}
	loaded, membership, err := repository.GetIdentitySessionByTokenHash(ctx, session.TokenHash, now.Add(time.Minute), 2*time.Hour)
	if err != nil || membership == nil || membership.OrganizationID != firstOrg || loaded.CSRFTokenHash != session.CSRFTokenHash {
		t.Fatalf("loaded=%+v membership=%+v err=%v", loaded, membership, err)
	}
	_, switched, err := repository.SwitchIdentitySessionOrganization(ctx, session.ID, userID, secondOrg, now.Add(2*time.Minute))
	if err != nil || switched.OrganizationID != secondOrg || switched.Role != domain.RoleDeveloper {
		t.Fatalf("switched=%+v err=%v", switched, err)
	}
	if _, _, err := repository.SwitchIdentitySessionOrganization(ctx, session.ID, userID, mustID(t), now.Add(3*time.Minute)); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("non-member switch error=%v", err)
	}
	if err := repository.RevokeIdentitySession(ctx, session.TokenHash, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.GetIdentitySessionByTokenHash(ctx, session.TokenHash, now.Add(5*time.Minute), 2*time.Hour); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("revoked session error=%v", err)
	}
}

func TestRepositoryReadsDeveloperOrganizationInventory(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := database.Open(ctx, database.Options{URL: databaseURL, MaxConnections: 5, ConnectTimeout: 5 * time.Second, ApplicationName: "emisell-organization-integration-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	platformOrgID, developerOrgID, userID, appID := mustID(t), mustID(t), mustID(t), mustID(t)
	email := userID + "@organizations.integration.test"
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, platformOrgID, userID, "owner", email, "Organization Owner"); err != nil {
		t.Fatal(err)
	}
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, developerOrgID, userID, "owner", email, "Organization Owner"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO organization_entitlements (organization_id, sandbox_access, production_access, max_apps, max_webhooks, created_at, updated_at)
		VALUES ($1::uuid, true, false, 5, 30, $2, $2)`, developerOrgID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO apps (id, organization_id, name, slug, distribution, status, created_by, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, 'Organization Inventory App', 'organization-inventory-app', 'custom', 'draft', $3::uuid, $4, $4)`, appID, developerOrgID, userID, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM apps WHERE id = $1::uuid`, appID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM organizations WHERE id = ANY($1::uuid[])`, []string{platformOrgID, developerOrgID})
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})
	repository := postgresadapter.NewRepository(pool, ids.NewUUIDv7, func() time.Time { return now })
	organizations, meta, err := repository.ListDeveloperOrganizations(ctx, ports.DeveloperOrganizationFilter{Search: "local-" + strings.ReplaceAll(developerOrgID, "-", ""), Status: domain.OrganizationStatusActive})
	if err != nil || meta.HasMore || len(organizations) != 1 {
		t.Fatalf("organizations=%#v meta=%#v err=%v", organizations, meta, err)
	}
	if organizations[0].ID != developerOrgID || organizations[0].AppCount != 1 || organizations[0].MembershipCount != 1 || organizations[0].Entitlement.MaxApps != 5 {
		t.Fatalf("unexpected organization summary: %#v", organizations[0])
	}
	detail, err := repository.GetDeveloperOrganization(ctx, developerOrgID)
	if err != nil || len(detail.Memberships) != 1 || detail.Memberships[0].Email != email {
		t.Fatalf("unexpected organization detail: %#v err=%v", detail, err)
	}
}
