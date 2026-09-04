package postgres_test

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	postgresadapter "emisell-app-platform/services/app-gateway/internal/adapters/postgres"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/database"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"emisell-app-platform/services/app-gateway/internal/security"
)

func TestIntegrationReadinessPostgres(t *testing.T) {
	raw := os.Getenv("INTEGRATION_READINESS_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("run npm run test:integration-readiness for isolated PostgreSQL verification")
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil || u.User.Username() != "readiness_test" || u.Hostname() != "127.0.0.1" || !regexp.MustCompile(`^/emisell_readiness_test_[a-f0-9]{24}$`).MatchString(u.Path) {
		t.Fatal("refusing non-disposable inspection database")
	}
	ctx := t.Context()
	pool, err := database.Open(ctx, database.Options{URL: raw, MaxConnections: 5, ConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal("cannot open isolated inspection database")
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	org, user := mustID(t), mustID(t)
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, org, user, "owner", user+"@example.test", "Readiness test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO organization_entitlements(organization_id,sandbox_access,production_access,max_apps,max_webhooks,created_at,updated_at) VALUES($1,true,false,20,20,now(),now())`, org); err != nil {
		t.Fatal(err)
	}
	repo := postgresadapter.NewRepository(pool, ids.NewUUIDv7, clock)
	apps := application.NewAppService(repo, ids.NewUUIDv7, clock)
	launch := "https://provider.example.test/oauth/start"
	app, err := apps.Create(ctx, application.CreateAppCommand{OrganizationID: org, ActorID: user, IdempotencyKey: mustID(t), Name: "Readiness Test", Distribution: domain.DistributionCustom, AppURL: &launch})
	if err != nil {
		t.Fatal(err)
	}
	versions := application.NewVersionService(repo, application.CurrentConfigurationSnapshotBuilder{Repository: repo}, ids.NewUUIDv7, clock)
	createVersion := func(number string, previous *string) domain.AppVersion {
		t.Helper()
		v, err := versions.Create(ctx, application.CreateVersionCommand{OrganizationID: org, ActorID: user, IdempotencyKey: mustID(t), AppID: app.ID, Version: number})
		if err != nil {
			t.Fatal(err)
		}
		v, err = versions.Release(ctx, org, user, app.ID, v.ID, mustID(t), previous)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	v1 := createVersion("1.0.0", nil)
	box, err := security.NewSecretBox([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	credentials := application.NewCredentialService(repo, box, 1, ids.NewUUIDv7, clock)
	credential, err := credentials.Create(ctx, application.CreateCredentialCommand{OrganizationID: org, ActorID: user, IdempotencyKey: mustID(t), AppID: app.ID, Environment: domain.EnvironmentSandbox})
	if err != nil {
		t.Fatal(err)
	}
	catalog := application.NewCatalogService(repo, repo, clock)
	beforeAudit, err := repo.ListAuditEvents(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	report, err := catalog.IntegrationReadiness(ctx, org, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(report)
	if report.EndToEndVerified || report.ActiveVersionID == nil || *report.ActiveVersionID != v1.ID || strings.Contains(string(encoded), credential.ClientSecret) || strings.Contains(string(encoded), credential.Credential.ClientID) {
		t.Fatal("incorrect or sensitive inspection")
	}
	statuses := map[string]string{}
	for _, check := range report.Checks {
		statuses[check.Code] = check.Status
	}
	if statuses["development_credentials"] != "pass" || statuses["production_credentials"] != "attention" || statuses["draft_changes"] != "pass" {
		t.Fatal("incorrect configuration results", statuses)
	}
	afterAudit, err := repo.ListAuditEvents(ctx, org)
	if err != nil || len(beforeAudit) != len(afterAudit) {
		t.Fatal("read-only inspection created audit writes", err)
	}
	if _, err := catalog.IntegrationReadiness(ctx, mustID(t), app.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("tenant boundary failed", err)
	}
	v2 := createVersion("1.1.0", &v1.ID)
	_, err = repo.UpsertCatalogListing(ctx, domain.AppCatalogListing{AppID: app.ID, OrganizationID: org, Category: domain.CatalogCategoryCustom, Status: domain.CatalogListingStatusPublished, PublishedBy: &user, PublishedAt: &now, UpdatedAt: now}, 0, ports.MutationMeta{ActorID: user, Action: "app_catalog_listing.published", ExpectedAppRevision: &report.AppRevision, ExpectedActiveVersionID: report.ActiveVersionID})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatal("transaction accepted a stale reviewed app/version", err)
	}
	if _, err := repo.GetCatalogListing(ctx, org, app.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("failed publication changed listing", err)
	}
	latest, err := catalog.IntegrationReadiness(ctx, org, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	listing, err := catalog.UpdateListing(ctx, application.UpdateCatalogListingCommand{OrganizationID: org, AppID: app.ID, ActorID: user, Category: domain.CatalogCategoryCustom, Status: domain.CatalogListingStatusPublished, ExpectedAppRevision: &latest.AppRevision, ExpectedActiveVersionID: &v2.ID})
	if err != nil || listing.Revision != 1 {
		t.Fatal("fresh inspection failed to publish", err)
	}
	restarted := postgresadapter.NewRepository(pool, ids.NewUUIDv7, clock)
	persisted, err := application.NewCatalogService(restarted, restarted, clock).IntegrationReadiness(ctx, org, app.ID)
	if err != nil || persisted.ListingStatus != domain.CatalogListingStatusPublished || persisted.ListingRevision != 1 {
		t.Fatal("inspection did not read persisted state", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE organizations SET status='suspended' WHERE id=$1`, org); err != nil {
		t.Fatal(err)
	}
	suspended, err := catalog.IntegrationReadiness(ctx, org, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, check := range suspended.Checks {
		if check.Code == "organization" && check.Status == "blocked" {
			found = true
		}
	}
	if !found {
		t.Fatal("suspended organization was not flagged")
	}
}
