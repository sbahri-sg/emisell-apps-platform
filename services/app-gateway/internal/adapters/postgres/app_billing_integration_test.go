package postgres_test

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	postgresadapter "emisell-app-platform/services/app-gateway/internal/adapters/postgres"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/database"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func TestAppBillingPostgres(t *testing.T) {
	raw := os.Getenv("APP_BILLING_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("run npm run test:billing for isolated PostgreSQL billing tests")
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil || u.User.Username() != "billing_test" || u.Hostname() != "127.0.0.1" || !regexp.MustCompile(`^/emisell_billing_test_[a-f0-9]{24}$`).MatchString(u.Path) {
		t.Fatal("refusing non-disposable billing database")
	}
	ctx := t.Context()
	pool, err := database.Open(ctx, database.Options{URL: raw, MaxConnections: 10, ConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal("cannot open isolated billing database")
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal("migration replay", err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	org, user := mustID(t), mustID(t)
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, org, user, "owner", user+"@example.test", "Billing Test"); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO organization_entitlements(organization_id,sandbox_access,production_access,max_apps,max_webhooks,created_at,updated_at) VALUES($1,true,false,20,20,now(),now()) ON CONFLICT(organization_id) DO UPDATE SET sandbox_access=true`, org)
	repo := postgresadapter.NewRepository(pool, ids.NewUUIDv7, clock)
	meta := func(action string) ports.MutationMeta {
		return ports.MutationMeta{ActorID: user, Action: action, IdempotencyKey: mustID(t)}
	}
	app, err := repo.CreateApp(ctx, domain.App{ID: mustID(t), OrganizationID: org, Name: "Billing Test", Slug: "billing-test", Status: domain.AppStatusDraft, Distribution: domain.DistributionCustom, CreatedBy: user, CreatedAt: now, UpdatedAt: now, Revision: 1}, meta("app.created"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := repo.CreateVersion(ctx, org, domain.AppVersion{ID: mustID(t), AppID: app.ID, Version: "1.0.0", Status: domain.VersionStatusDraft, CreatedBy: user, CreatedAt: now, Snapshot: domain.VersionSnapshot{Extensions: []domain.SnapshotExtension{}, Scopes: []domain.SnapshotScope{}, WebhookSubscriptions: []domain.SnapshotWebhook{}, RedirectURLs: []string{}, ConfigurationHash: "billing-test"}}, meta("version.created"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ActivateVersion(ctx, org, app.ID, version.ID, domain.VersionStatusDraft, nil, meta("version.released")); err != nil {
		t.Fatal(err)
	}
	install := func(merchant string) domain.AppInstallation {
		t.Helper()
		inst, err := repo.CreateInstallation(ctx, org, domain.AppInstallation{ID: mustID(t), AppID: app.ID, MerchantID: merchant, MerchantName: "Billing Merchant", Environment: domain.EnvironmentSandbox, Status: domain.InstallationStatusActive, InstalledVersionID: version.ID, GrantedScopes: []string{}, InstalledBy: user, InstalledAt: now, CreatedAt: now, UpdatedAt: now, Revision: 1}, version.ID, meta("installation.created"))
		if err != nil {
			t.Fatal(err)
		}
		return inst
	}
	a, b := install("billing-merchant-a"), install("billing-merchant-b")
	s := application.NewAppBillingService(repo, ids.NewUUIDv7, clock, false)
	plan, err := s.CreatePlan(ctx, org, app.ID, "pg-plan-create-000001", application.CreateAppPlan{Name: "Paid test", Currency: "IDR", Interval: "monthly", AmountMinor: 300000, Features: []string{"Feature"}})
	if err != nil {
		t.Fatal(err)
	}
	principal := application.EmisellBackendPrincipal{Subject: "billing-worker", MerchantID: a.MerchantID, Environment: a.Environment, JTI: "billing-pg-request-000001", Permissions: []string{"apps.billing.write"}}
	account, err := s.SyncAccount(ctx, principal, application.SyncAppBillingAccount{Currency: "IDR", CycleStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), CycleEnd: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.Quote(ctx, a.MerchantID, a.Environment, a.ID, plan.ID)
	if err != nil || q.AmountMinor != 150000 {
		t.Fatal(q, err)
	}
	identity := domain.MerchantIdentity{MerchantID: a.MerchantID, Environment: a.Environment, UserID: user}
	if _, err := s.Approve(ctx, domain.MerchantIdentity{MerchantID: b.MerchantID, Environment: b.Environment, UserID: user}, b.ID, q.ID, true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign quote accepted", err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 12)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Approve(ctx, identity, a.ID, q.ID, true)
			if err != nil {
				failures <- err
			}
		}()
	}
	wg.Wait()
	invoiceInput := application.IssueAppInvoice{InvoiceID: "pg-invoice-one", Currency: "IDR", CycleStart: account.CycleStart, CycleEnd: account.CycleEnd}
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.IssueInvoice(ctx, principal, invoiceInput)
			if err != nil {
				failures <- err
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app_subscription_charges`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate charges", count, err)
	}
	invoice, err := s.IssueInvoice(ctx, principal, invoiceInput)
	if err != nil || len(invoice.Lines) != 1 || invoice.AmountMinor != 150000 {
		t.Fatal(invoice, err)
	}
	// A fresh repository must reconstruct everything from durable records.
	s = application.NewAppBillingService(postgresadapter.NewRepository(pool, ids.NewUUIDv7, clock), ids.NewUUIDv7, clock, false)
	payment := application.AppInvoicePayment{EventID: "pg-payment-event-000001", PaymentID: "pg-payment-one", Currency: "IDR", AmountMinor: invoice.AmountMinor, Status: "paid"}
	exec(`CREATE FUNCTION fail_billing_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'simulated audit failure'; END $$`)
	exec(`CREATE TRIGGER fail_billing_audit BEFORE INSERT ON app_billing_events FOR EACH ROW EXECUTE FUNCTION fail_billing_audit()`)
	if _, err := s.Payment(ctx, principal, invoice.InvoiceID, payment); err == nil {
		t.Fatal("audit failure committed payment")
	}
	access, err := s.Installation(ctx, a.MerchantID, a.Environment, a.ID)
	if err != nil || access.PaidAccess {
		t.Fatal("failed transaction granted access", err)
	}
	exec(`DROP TRIGGER fail_billing_audit ON app_billing_events`)
	exec(`DROP FUNCTION fail_billing_audit()`)
	if _, err := s.Payment(ctx, principal, invoice.InvoiceID, payment); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Payment(ctx, principal, invoice.InvoiceID, payment); err != nil {
		t.Fatal("callback replay", err)
	}
	access, err = s.Installation(ctx, a.MerchantID, a.Environment, a.ID)
	if err != nil || !access.PaidAccess {
		t.Fatal("paid access missing", err)
	}
	if _, err := s.Installation(ctx, b.MerchantID, b.Environment, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign installation leaked", err)
	}
	// A malicious direct database row cannot bind another merchant's installation.
	otherQ := q
	otherQ.ID = mustID(t)
	data, _ := json.Marshal(otherQ)
	exec(`INSERT INTO merchant_app_billing(merchant_id,environment) VALUES($1,'sandbox') ON CONFLICT DO NOTHING`, b.MerchantID)
	if _, err := pool.Exec(ctx, `INSERT INTO app_billing_quotes(id,merchant_id,environment,installation_id,document) VALUES($1,$2,'sandbox',$3,$4)`, otherQ.ID, b.MerchantID, a.ID, data); err == nil {
		t.Fatal("database accepted cross-merchant quote")
	}
	// Real uninstall entrypoint invokes the trigger, including late callback retries.
	if err := repo.UninstallInstallation(ctx, org, app.ID, a.ID, meta("installation.uninstalled")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Payment(ctx, principal, invoice.InvoiceID, payment); err != nil {
		t.Fatal(err)
	}
	access, err = s.Installation(ctx, a.MerchantID, a.Environment, a.ID)
	if err != nil || access.PaidAccess || access.Subscription.Status != "cancelled" {
		t.Fatal("uninstall did not cancel billing", err)
	}
	now = account.CycleEnd
	account, err = s.SyncAccount(ctx, principal, application.SyncAppBillingAccount{Currency: "IDR", CycleStart: now, CycleEnd: now.AddDate(0, 1, 0), Enabled: true, Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	invoice, err = s.IssueInvoice(ctx, principal, application.IssueAppInvoice{InvoiceID: "pg-invoice-two", Currency: "IDR", CycleStart: account.CycleStart, CycleEnd: account.CycleEnd})
	if err != nil || invoice.AmountMinor != 0 {
		t.Fatal("uninstall billed renewal", err)
	}
}
