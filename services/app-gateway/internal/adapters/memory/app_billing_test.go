package memory

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type billingFixture struct {
	repo                 *Repository
	service              *application.AppBillingService
	now                  time.Time
	org, app, inst, user string
	merchant             domain.MerchantIdentity
	backend              application.EmisellBackendPrincipal
	plan                 domain.AppPlan
	account              domain.AppBillingAccount
}

func newBillingFixture(t *testing.T) *billingFixture {
	t.Helper()
	id := func() string {
		value, err := ids.NewUUIDv7()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	f := &billingFixture{now: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), org: id(), app: id(), inst: id(), user: id()}
	f.repo = NewRepository(ids.NewUUIDv7, func() time.Time { return f.now })
	f.repo.apps[f.app] = domain.App{ID: f.app, OrganizationID: f.org, Status: domain.AppStatusActive}
	f.repo.organizationEntitlements[f.org] = domain.OrganizationEntitlement{SandboxAccess: true}
	f.merchant = domain.MerchantIdentity{MerchantID: "merchant-billing-a", Environment: domain.EnvironmentSandbox, UserID: f.user}
	f.repo.installations[f.app] = map[string]domain.AppInstallation{f.inst: {ID: f.inst, AppID: f.app, MerchantID: f.merchant.MerchantID, Environment: domain.EnvironmentSandbox, Status: domain.InstallationStatusActive}}
	f.service = application.NewAppBillingService(f.repo, ids.NewUUIDv7, func() time.Time { return f.now }, false)
	f.backend = application.EmisellBackendPrincipal{Subject: "emisell-billing", MerchantID: f.merchant.MerchantID, Environment: domain.EnvironmentSandbox, JTI: "billing-request-000000001", Permissions: []string{"apps.billing.write"}}
	var err error
	f.plan, err = f.service.CreatePlan(t.Context(), f.org, f.app, "create-plan-request-0001", application.CreateAppPlan{Name: "Reviews Pro", AmountMinor: 300000, Currency: "IDR", Interval: "monthly", Features: []string{"Unlimited reviews"}})
	if err != nil {
		t.Fatal(err)
	}
	f.account, err = f.service.SyncAccount(t.Context(), f.backend, application.SyncAppBillingAccount{Currency: "IDR", CycleStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), CycleEnd: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *billingFixture) quote(t *testing.T) domain.AppSubscriptionQuote {
	t.Helper()
	q, err := f.service.Quote(t.Context(), f.merchant.MerchantID, f.merchant.Environment, f.inst, f.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func (f *billingFixture) approve(t *testing.T) domain.AppSubscription {
	t.Helper()
	q := f.quote(t)
	sub, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	return sub
}
func (f *billingFixture) issue(t *testing.T, id string) domain.AppBillingInvoice {
	t.Helper()
	invoice, err := f.service.IssueInvoice(t.Context(), f.backend, application.IssueAppInvoice{InvoiceID: id, Currency: "IDR", CycleStart: f.account.CycleStart, CycleEnd: f.account.CycleEnd})
	if err != nil {
		t.Fatal(err)
	}
	return invoice
}
func (f *billingFixture) pay(t *testing.T, invoice domain.AppBillingInvoice, status string) {
	t.Helper()
	_, err := f.service.Payment(t.Context(), f.backend, invoice.InvoiceID, application.AppInvoicePayment{EventID: "payment-event-" + invoice.InvoiceID + "-" + status, PaymentID: "payment-" + invoice.InvoiceID, Currency: invoice.Currency, AmountMinor: invoice.AmountMinor, Status: status})
	if err != nil {
		t.Fatal(err)
	}
}
func (f *billingFixture) advance(t *testing.T) {
	t.Helper()
	f.now = f.account.CycleEnd
	account, err := f.service.SyncAccount(t.Context(), f.backend, application.SyncAppBillingAccount{Currency: "IDR", CycleStart: f.now, CycleEnd: f.now.AddDate(0, 1, 0), Enabled: true, Revision: f.account.Revision})
	if err != nil {
		t.Fatal(err)
	}
	f.account = account
}

func TestAppBillingConsentProrationRenewalAndCancellation(t *testing.T) {
	f := newBillingFixture(t)
	q := f.quote(t)
	if q.AmountMinor != 150000 {
		t.Fatalf("half-month prorata = %d", q.AmountMinor)
	}
	if _, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, false); !errors.Is(err, domain.ErrValidation) {
		t.Fatal("accepted without consent", err)
	}
	sub, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, true)
	if err != nil || sub.Status != "pending_payment" {
		t.Fatal(sub, err)
	}
	replay, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, true)
	if err != nil || replay.ID != sub.ID {
		t.Fatal("consent replay", err)
	}
	first := f.issue(t, "invoice-first")
	if first.AmountMinor != 150000 || len(first.Lines) != 1 {
		t.Fatal(first)
	}
	if again := f.issue(t, "invoice-first"); again.AmountMinor != first.AmountMinor || again.Lines[0].ID != first.Lines[0].ID {
		t.Fatal("unstable invoice replay")
	}
	if empty := f.issue(t, "invoice-other"); len(empty.Lines) != 0 {
		t.Fatal("double charge")
	}
	f.pay(t, first, "failed")
	access, err := f.service.Installation(t.Context(), f.merchant.MerchantID, f.merchant.Environment, f.inst)
	if err != nil || access.PaidAccess || access.Subscription.Status != "past_due" {
		t.Fatal(access, err)
	}
	f.pay(t, first, "paid")
	f.pay(t, first, "paid")
	access, err = f.service.Installation(t.Context(), f.merchant.MerchantID, f.merchant.Environment, f.inst)
	if err != nil || !access.PaidAccess {
		t.Fatal(access, err)
	}
	if err := f.service.ArchivePlan(t.Context(), f.org, f.app, f.plan.ID); err != nil {
		t.Fatal(err)
	}
	f.advance(t)
	second := f.issue(t, "invoice-second")
	if second.AmountMinor != 300000 || len(second.Lines) != 1 {
		t.Fatal("archived plan must renew at approved price", second)
	}
	f.pay(t, second, "paid")
	if err := f.service.Cancel(t.Context(), f.merchant, f.inst, sub.ID); err != nil {
		t.Fatal(err)
	}
	access, _ = f.service.Installation(t.Context(), f.merchant.MerchantID, f.merchant.Environment, f.inst)
	if !access.PaidAccess || access.Subscription.Status != "cancelled" {
		t.Fatal("cancel must preserve prepaid access")
	}
	f.advance(t)
	if invoice := f.issue(t, "invoice-third"); len(invoice.Lines) != 0 {
		t.Fatal("cancelled subscription renewed")
	}
	access, _ = f.service.Installation(t.Context(), f.merchant.MerchantID, f.merchant.Environment, f.inst)
	if access.PaidAccess {
		t.Fatal("expired paid access")
	}
}

func TestAppBillingIsolationAndTamperResistance(t *testing.T) {
	f := newBillingFixture(t)
	q := f.quote(t)
	other := f.merchant
	other.MerchantID = "merchant-billing-b"
	if _, err := f.service.Installation(t.Context(), other.MerchantID, other.Environment, f.inst); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("tenant data leak", err)
	}
	if _, err := f.service.Approve(t.Context(), other, f.inst, q.ID, true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign quote approval", err)
	}
	if _, err := f.service.Plans(t.Context(), f.user, f.app); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign org pricing", err)
	}
	if _, err := f.service.CreatePlan(t.Context(), f.org, f.app, "create-plan-request-0001", application.CreateAppPlan{Name: "Reviews Pro", AmountMinor: 1, Currency: "IDR", Interval: "monthly"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("idempotency body changed", err)
	}
	if _, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, true); err != nil {
		t.Fatal(err)
	}
	invoice := f.issue(t, "invoice-validation")
	for name, change := range map[string]func(*application.AppInvoicePayment){"amount": func(p *application.AppInvoicePayment) { p.AmountMinor++ }, "currency": func(p *application.AppInvoicePayment) { p.Currency = "USD" }, "status": func(p *application.AppInvoicePayment) { p.Status = "refunded" }} {
		t.Run(name, func(t *testing.T) {
			p := application.AppInvoicePayment{EventID: "payment-test-00000001", PaymentID: "payment-validation", Status: "paid", Currency: "IDR", AmountMinor: invoice.AmountMinor}
			change(&p)
			if _, err := f.service.Payment(t.Context(), f.backend, invoice.InvoiceID, p); err == nil {
				t.Fatal("invalid callback accepted")
			}
		})
	}
	backend := f.backend
	backend.MerchantID = other.MerchantID
	if _, err := f.service.Payment(t.Context(), backend, invoice.InvoiceID, application.AppInvoicePayment{EventID: "payment-test-00000002", PaymentID: "payment-validation", Status: "paid", Currency: "IDR", AmountMinor: invoice.AmountMinor}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross merchant callback", err)
	}
	f.pay(t, invoice, "paid")
	if _, err := f.service.Payment(t.Context(), f.backend, invoice.InvoiceID, application.AppInvoicePayment{EventID: "payment-test-late-failure", PaymentID: "payment-validation", Status: "failed", Currency: "IDR", AmountMinor: invoice.AmountMinor}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("late failure regressed payment", err)
	}
	backend = f.backend
	backend.Permissions = []string{"apps.install"}
	if _, err := f.service.IssueInvoice(t.Context(), backend, application.IssueAppInvoice{InvoiceID: "unauthorized-invoice"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("install permission billed merchant", err)
	}
	backend = f.backend
	backend.Environment = domain.EnvironmentProduction
	if _, err := f.service.IssueInvoice(t.Context(), backend, application.IssueAppInvoice{InvoiceID: "live-invoice"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("live billing enabled implicitly", err)
	}
}

func TestAppBillingFreeQuoteExpiryAndCycleGuards(t *testing.T) {
	f := newBillingFixture(t)
	q := f.quote(t)
	f.now = f.now.Add(11 * time.Minute)
	if _, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, true); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("expired quote accepted", err)
	}
	if _, err := f.service.SyncAccount(t.Context(), f.backend, application.SyncAppBillingAccount{Currency: "IDR", CycleStart: f.account.CycleStart, CycleEnd: f.account.CycleEnd.AddDate(1, 0, 0), Enabled: true, Revision: 1}); !errors.Is(err, domain.ErrValidation) {
		t.Fatal("annual period accepted for monthly app", err)
	}
	f.repo.appBilling[f.merchant.MerchantID+":sandbox"] = ports.AppBillingState{}
	free, err := f.service.CreatePlan(t.Context(), f.org, f.app, "create-free-plan-00001", application.CreateAppPlan{Name: "Forever Free", Currency: "IDR", Interval: "free"})
	if err != nil {
		t.Fatal(err)
	}
	f.plan = free
	sub := f.approve(t)
	if sub.Status != "active" {
		t.Fatal(sub)
	}
	state := f.repo.appBilling[f.merchant.MerchantID+":sandbox"]
	if len(state.Charges) != 0 {
		t.Fatal("free plan charged")
	}
	if err := f.service.Cancel(t.Context(), f.merchant, f.inst, sub.ID); err != nil {
		t.Fatal(err)
	}
	q = f.quote(t)
	if err := f.service.ArchivePlan(t.Context(), f.org, f.app, free.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, true); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("archived quote approved", err)
	}
}

func TestAppBillingParallelRequestsDoNotDuplicateCharges(t *testing.T) {
	f := newBillingFixture(t)
	q := f.quote(t)
	var wg sync.WaitGroup
	errorsFound := make(chan error, 20)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.service.Approve(t.Context(), f.merchant, f.inst, q.ID, true)
			if err != nil {
				errorsFound <- err
			}
		}()
	}
	wg.Wait()
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.service.IssueInvoice(t.Context(), f.backend, application.IssueAppInvoice{InvoiceID: fmt.Sprintf("concurrent-invoice-%d", i), Currency: "IDR", CycleStart: f.account.CycleStart, CycleEnd: f.account.CycleEnd})
			if err != nil {
				errorsFound <- err
			}
		}()
	}
	wg.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
	state := f.repo.appBilling[f.merchant.MerchantID+":sandbox"]
	if len(state.Subscriptions) != 1 || len(state.Charges) != 1 {
		t.Fatal("duplicate subscriptions or charges")
	}
	var total int64
	for _, invoice := range state.Invoices {
		total += invoice.AmountMinor
	}
	if total != 150000 {
		t.Fatal("charged more than once", total)
	}
}

func TestAppBillingUninstallAndSuspendAreDifferent(t *testing.T) {
	f := newBillingFixture(t)
	f.approve(t)
	invoice := f.issue(t, "invoice-installation")
	f.pay(t, invoice, "paid")
	inst := f.repo.installations[f.app][f.inst]
	inst.Status = domain.InstallationStatusSuspended
	f.repo.installations[f.app][f.inst] = inst
	access, _ := f.service.Installation(t.Context(), f.merchant.MerchantID, f.merchant.Environment, f.inst)
	if access.PaidAccess {
		t.Fatal("suspended installation has paid access")
	}
	f.advance(t)
	renewal := f.issue(t, "invoice-suspended")
	if renewal.AmountMinor != 300000 {
		t.Fatal("deactivation unexpectedly cancelled billing")
	}
	inst.Status = domain.InstallationStatusUninstalled
	f.repo.installations[f.app][f.inst] = inst
	f.repo.cancelAppBillingLocked(inst)
	f.pay(t, renewal, "paid")
	access, _ = f.service.Installation(t.Context(), f.merchant.MerchantID, f.merchant.Environment, f.inst)
	if access.PaidAccess || access.Subscription.Status != "cancelled" {
		t.Fatal("payment reactivated uninstalled app")
	}
	f.advance(t)
	if next := f.issue(t, "invoice-uninstalled"); next.AmountMinor != 0 {
		t.Fatal("uninstalled subscription charged")
	}
}
