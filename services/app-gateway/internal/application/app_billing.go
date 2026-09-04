package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

const AppBillingTerms = "monthly-prepaid-v1"

type AppBillingService struct {
	repo ports.AppBillingRepository
	id   ids.Generator
	now  func() time.Time
	live bool
}

func NewAppBillingService(repo ports.AppBillingRepository, id ids.Generator, now func() time.Time, live bool) *AppBillingService {
	return &AppBillingService{repo, id, now, live}
}

type CreateAppPlan struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AmountMinor int64    `json:"amountMinor"`
	Currency    string   `json:"currency"`
	Interval    string   `json:"interval"`
	Features    []string `json:"features"`
}

func billingHash(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (s *AppBillingService) Plans(ctx context.Context, org, appID string) ([]domain.AppPlan, error) {
	result := []domain.AppPlan{}
	if !uuidValue.MatchString(org) || !uuidValue.MatchString(appID) {
		return nil, domain.ErrValidation
	}
	err := s.repo.WithAppPlans(ctx, org, appID, func(_ domain.App, plans *[]domain.AppPlan) error { result = *plans; return nil })
	return result, err
}

func (s *AppBillingService) CreatePlan(ctx context.Context, org, appID, key string, input CreateAppPlan) (domain.AppPlan, error) {
	input.Name, input.Description = strings.TrimSpace(input.Name), strings.TrimSpace(input.Description)
	if !uuidValue.MatchString(org) || !uuidValue.MatchString(appID) || !extensionConnectionRequestID.MatchString(key) || len(input.Name) < 2 || len(input.Name) > 80 || len(input.Description) > 1000 || input.AmountMinor < 0 || input.AmountMinor > 1_000_000_000_000 || !slices.Contains([]string{"IDR", "USD"}, input.Currency) || len(input.Features) > 20 {
		return domain.AppPlan{}, domain.ErrValidation
	}
	if (input.Interval != "free" || input.AmountMinor != 0) && (input.Interval != "monthly" || input.AmountMinor <= 0) {
		return domain.AppPlan{}, domain.ErrValidation
	}
	for _, feature := range input.Features {
		if strings.TrimSpace(feature) == "" || len(feature) > 160 {
			return domain.AppPlan{}, domain.ErrValidation
		}
	}
	if input.Features == nil {
		input.Features = []string{}
	}
	hash := billingHash(input)
	var result domain.AppPlan
	err := s.repo.WithAppPlans(ctx, org, appID, func(app domain.App, plans *[]domain.AppPlan) error {
		for _, plan := range *plans {
			if plan.RequestKey == key {
				if plan.RequestHash != hash {
					return domain.ErrConflict
				}
				result = plan
				return nil
			}
		}
		if app.Status == domain.AppStatusArchived {
			return domain.ErrForbidden
		}
		if len(*plans) >= 100 {
			return fmt.Errorf("%w: maximum 100 immutable plans per app", domain.ErrValidation)
		}
		id, err := s.id()
		if err != nil {
			return err
		}
		result = domain.AppPlan{ID: id, AppID: appID, Name: input.Name, Description: input.Description, AmountMinor: input.AmountMinor, Currency: input.Currency, Interval: input.Interval, Features: input.Features, Status: "active", CreatedAt: s.now().UTC(), RequestKey: key, RequestHash: hash}
		result.AppName = app.Name
		*plans = append(*plans, result)
		return nil
	})
	return result, err
}

func (s *AppBillingService) ArchivePlan(ctx context.Context, org, appID, planID string) error {
	if !uuidValue.MatchString(org) || !uuidValue.MatchString(appID) || !uuidValue.MatchString(planID) {
		return domain.ErrValidation
	}
	return s.repo.WithAppPlans(ctx, org, appID, func(_ domain.App, plans *[]domain.AppPlan) error {
		for i := range *plans {
			if (*plans)[i].ID == planID {
				(*plans)[i].Status = "archived"
				return nil
			}
		}
		return domain.ErrNotFound
	})
}

func billingMerchant(merchant string, env domain.Environment) error {
	if !externalIdentityValue.MatchString(merchant) || (env != domain.EnvironmentSandbox && env != domain.EnvironmentProduction) {
		return domain.ErrValidation
	}
	return nil
}

func billingInstallation(state *ports.AppBillingState, installationID string, active bool) (domain.AppInstallation, error) {
	for _, inst := range state.Installations {
		if inst.ID != installationID {
			continue
		}
		if active && (inst.Status != domain.InstallationStatusActive || !state.AvailableApps[inst.AppID]) {
			return domain.AppInstallation{}, domain.ErrForbidden
		}
		return inst, nil
	}
	return domain.AppInstallation{}, domain.ErrNotFound
}

func currentSubscription(state *ports.AppBillingState, installationID string) *domain.AppSubscription {
	var result *domain.AppSubscription
	for i := range state.Subscriptions {
		value := &state.Subscriptions[i]
		if value.InstallationID != installationID {
			continue
		}
		if result == nil || (value.Status != "cancelled" && result.Status == "cancelled") ||
			((value.Status == "cancelled") == (result.Status == "cancelled") &&
				(value.ApprovedAt.After(result.ApprovedAt) || (value.ApprovedAt.Equal(result.ApprovedAt) && value.ID > result.ID))) {
			result = value
		}
	}
	return result
}

type InstallationBilling struct {
	Plans                []domain.AppPlan        `json:"plans"`
	Subscription         *domain.AppSubscription `json:"subscription"`
	PaidAccess           bool                    `json:"paidAccess"`
	PaidBillingAvailable bool                    `json:"paidBillingAvailable"`
	Test                 bool                    `json:"test"`
}

func (s *AppBillingService) Installation(ctx context.Context, merchant string, env domain.Environment, instID string) (InstallationBilling, error) {
	result := InstallationBilling{Plans: []domain.AppPlan{}, Test: env == domain.EnvironmentSandbox}
	if err := billingMerchant(merchant, env); err != nil {
		return result, err
	}
	if !uuidValue.MatchString(instID) {
		return result, domain.ErrValidation
	}
	err := s.repo.WithMerchantBilling(ctx, merchant, env, func(state *ports.AppBillingState) error {
		inst, err := billingInstallation(state, instID, false)
		if err != nil {
			return err
		}
		for _, plan := range state.Plans {
			if plan.AppID == inst.AppID && plan.Status == "active" {
				result.Plans = append(result.Plans, plan)
			}
		}
		result.Subscription = currentSubscription(state, instID)
		now := s.now().UTC()
		result.PaidBillingAvailable = state.Account != nil && state.Account.Enabled && !now.Before(state.Account.CycleStart) && now.Before(state.Account.CycleEnd) && (s.live || result.Test)
		if sub := result.Subscription; sub != nil {
			// Cancellation stops renewal immediately, but prepaid access survives to
			// the paid-through date unless the installation itself is suspended/uninstalled.
			result.PaidAccess = sub.Plan.AmountMinor > 0 && sub.PaidThrough != nil && now.Before(*sub.PaidThrough) && inst.Status == domain.InstallationStatusActive && state.AvailableApps[inst.AppID]
		}
		return nil
	})
	return result, err
}

func (s *AppBillingService) Quote(ctx context.Context, merchant string, env domain.Environment, instID, planID string) (domain.AppSubscriptionQuote, error) {
	var result domain.AppSubscriptionQuote
	if err := billingMerchant(merchant, env); err != nil {
		return result, err
	}
	if !uuidValue.MatchString(instID) || !uuidValue.MatchString(planID) {
		return result, domain.ErrValidation
	}
	err := s.repo.WithMerchantBilling(ctx, merchant, env, func(state *ports.AppBillingState) error {
		inst, err := billingInstallation(state, instID, true)
		if err != nil {
			return err
		}
		var plan domain.AppPlan
		for _, item := range state.Plans {
			if item.ID == planID && item.AppID == inst.AppID && item.Status == "active" {
				plan = item
			}
		}
		if plan.ID == "" {
			return domain.ErrNotFound
		}
		now := s.now().UTC().Truncate(time.Second)
		if err := canSelectAppPlan(state, instID, now); err != nil {
			return err
		}
		id, err := s.id()
		if err != nil {
			return err
		}
		result = domain.AppSubscriptionQuote{ID: id, InstallationID: instID, Plan: plan, PeriodStart: now, ExpiresAt: now.Add(10 * time.Minute), TermsVersion: AppBillingTerms}
		if plan.AmountMinor > 0 {
			account := state.Account
			if (env == domain.EnvironmentProduction && !s.live) || account == nil || !account.Enabled {
				return fmt.Errorf("%w: paid billing is not enabled for this merchant", domain.ErrForbidden)
			}
			if now.Before(account.CycleStart) || !now.Before(account.CycleEnd) || plan.Currency != account.Currency {
				return fmt.Errorf("%w: billing cycle or currency is not ready", domain.ErrConflict)
			}
			result.PeriodEnd, result.AccountRevision = account.CycleEnd, account.Revision
			result.AmountMinor = proratedAppAmount(plan.AmountMinor, int64(account.CycleEnd.Sub(now)/time.Second), int64(account.CycleEnd.Sub(account.CycleStart)/time.Second))
			if result.ExpiresAt.After(account.CycleEnd) {
				result.ExpiresAt = account.CycleEnd
			}
		}
		state.Quotes = append(state.Quotes, result)
		return nil
	})
	return result, err
}

// ceil(price * remaining / cycle), without int64 multiplication overflow.
func proratedAppAmount(price, remaining, cycle int64) int64 {
	n := new(big.Int).Mul(big.NewInt(price), big.NewInt(remaining))
	n.Add(n, big.NewInt(cycle-1))
	return n.Div(n, big.NewInt(cycle)).Int64()
}

func canSelectAppPlan(state *ports.AppBillingState, installationID string, now time.Time) error {
	if sub := currentSubscription(state, installationID); sub != nil {
		if sub.Status != "cancelled" || (sub.PaidThrough != nil && now.Before(*sub.PaidThrough)) {
			return fmt.Errorf("%w: cancel the current subscription and wait until its paid period ends before choosing another plan", domain.ErrConflict)
		}
	}
	for _, charge := range state.Charges {
		if charge.InstallationID == installationID && (charge.Status == "invoiced" || charge.Status == "failed") {
			return fmt.Errorf("%w: settle the existing app invoice first", domain.ErrConflict)
		}
	}
	return nil
}

func (s *AppBillingService) Approve(ctx context.Context, identity domain.MerchantIdentity, instID, quoteID string, accepted bool) (domain.AppSubscription, error) {
	var result domain.AppSubscription
	if err := billingMerchant(identity.MerchantID, identity.Environment); err != nil {
		return result, err
	}
	if !uuidValue.MatchString(instID) || !uuidValue.MatchString(quoteID) || !accepted || identity.UserID == "" {
		return result, domain.ErrValidation
	}
	err := s.repo.WithMerchantBilling(ctx, identity.MerchantID, identity.Environment, func(state *ports.AppBillingState) error {
		if _, err := billingInstallation(state, instID, true); err != nil {
			return err
		}
		for _, sub := range state.Subscriptions {
			if sub.QuoteID == quoteID && sub.InstallationID == instID {
				result = sub
				return nil
			}
		}
		now := s.now().UTC()
		if err := canSelectAppPlan(state, instID, now); err != nil {
			return err
		}
		var quote domain.AppSubscriptionQuote
		for _, q := range state.Quotes {
			if q.ID == quoteID && q.InstallationID == instID {
				quote = q
			}
		}
		if quote.ID == "" {
			return domain.ErrNotFound
		}
		if !now.Before(quote.ExpiresAt) {
			return fmt.Errorf("%w: quote expired; review a new quote", domain.ErrConflict)
		}
		available := false
		for _, plan := range state.Plans {
			if plan.ID == quote.Plan.ID && plan.Status == "active" {
				available = true
			}
		}
		if !available {
			return domain.ErrConflict
		}
		if quote.Plan.AmountMinor > 0 && ((identity.Environment == domain.EnvironmentProduction && !s.live) || state.Account == nil || !state.Account.Enabled || state.Account.Revision != quote.AccountRevision) {
			return domain.ErrConflict
		}
		id, err := s.id()
		if err != nil {
			return err
		}
		result = domain.AppSubscription{ID: id, InstallationID: instID, QuoteID: quoteID, Plan: quote.Plan, Status: "active", ApprovedBy: identity.UserID, ApprovedAt: now, TermsVersion: quote.TermsVersion}
		if quote.Plan.AmountMinor > 0 {
			result.Status = "pending_payment"
			charge, err := s.newCharge(result, quote.AmountMinor, quote.PeriodStart, quote.PeriodEnd)
			if err != nil {
				return err
			}
			state.Charges = append(state.Charges, charge)
		}
		state.Subscriptions = append(state.Subscriptions, result)
		return s.event(state, "subscription.approved", identity.UserID, result.ID, "")
	})
	return result, err
}

func (s *AppBillingService) Cancel(ctx context.Context, identity domain.MerchantIdentity, instID, subscriptionID string) error {
	if err := billingMerchant(identity.MerchantID, identity.Environment); err != nil {
		return err
	}
	if !uuidValue.MatchString(instID) || !uuidValue.MatchString(subscriptionID) || identity.UserID == "" {
		return domain.ErrValidation
	}
	return s.repo.WithMerchantBilling(ctx, identity.MerchantID, identity.Environment, func(state *ports.AppBillingState) error {
		if _, err := billingInstallation(state, instID, false); err != nil {
			return err
		}
		for i := range state.Subscriptions {
			sub := &state.Subscriptions[i]
			if sub.ID != subscriptionID || sub.InstallationID != instID {
				continue
			}
			if sub.Status == "cancelled" {
				return nil
			}
			now := s.now().UTC()
			sub.Status, sub.CancelledAt, sub.CancellationReason = "cancelled", &now, "merchant_cancelled"
			for j := range state.Charges {
				if state.Charges[j].SubscriptionID == sub.ID && state.Charges[j].Status == "unbilled" {
					state.Charges[j].Status = "void"
				}
			}
			return s.event(state, "subscription.cancelled", identity.UserID, sub.ID, "")
		}
		return domain.ErrNotFound
	})
}

func (s *AppBillingService) event(state *ports.AppBillingState, action, subject, resource, hash string) error {
	id, err := s.id()
	if err != nil {
		return err
	}
	state.Events = append(state.Events, domain.AppBillingEvent{ID: id, Action: action, Subject: subject, ResourceID: resource, RequestHash: hash, CreatedAt: s.now().UTC()})
	return nil
}

func (s *AppBillingService) newCharge(sub domain.AppSubscription, amount int64, start, end time.Time) (domain.AppSubscriptionCharge, error) {
	id, err := s.id()
	return domain.AppSubscriptionCharge{ID: id, SubscriptionID: sub.ID, InstallationID: sub.InstallationID, AppID: sub.Plan.AppID, PlanID: sub.Plan.ID, Description: sub.Plan.AppName + " — " + sub.Plan.Name, AmountMinor: amount, Currency: sub.Plan.Currency, PeriodStart: start, PeriodEnd: end, Status: "unbilled"}, err
}

func (s *AppBillingService) checkBackend(principal EmisellBackendPrincipal) error {
	if err := billingMerchant(principal.MerchantID, principal.Environment); err != nil {
		return err
	}
	if !externalIdentityValue.MatchString(principal.Subject) || !extensionConnectionRequestID.MatchString(principal.JTI) || !slices.Contains(principal.Permissions, "apps.billing.write") {
		return domain.ErrForbidden
	}
	if principal.Environment == domain.EnvironmentProduction && !s.live {
		return fmt.Errorf("%w: live app billing is disabled", domain.ErrForbidden)
	}
	return nil
}

type SyncAppBillingAccount struct {
	Currency   string    `json:"currency"`
	CycleStart time.Time `json:"cycleStart"`
	CycleEnd   time.Time `json:"cycleEnd"`
	Enabled    bool      `json:"enabled"`
	Revision   int64     `json:"revision"`
}

func validMonthlyCycle(start, end, now time.Time) bool {
	length := end.Sub(start)
	return length >= 27*24*time.Hour && length <= 32*24*time.Hour && !now.Before(start) && now.Before(end) && start.Nanosecond() == 0 && end.Nanosecond() == 0
}

func (s *AppBillingService) SyncAccount(ctx context.Context, principal EmisellBackendPrincipal, input SyncAppBillingAccount) (domain.AppBillingAccount, error) {
	var result domain.AppBillingAccount
	if err := s.checkBackend(principal); err != nil {
		return result, err
	}
	input.CycleStart, input.CycleEnd = input.CycleStart.UTC(), input.CycleEnd.UTC()
	if !slices.Contains([]string{"IDR", "USD"}, input.Currency) || !validMonthlyCycle(input.CycleStart, input.CycleEnd, s.now()) || input.Revision < 0 {
		return result, fmt.Errorf("%w: supply the current monthly app-billing cycle (not an annual store cycle)", domain.ErrValidation)
	}
	err := s.repo.WithMerchantBilling(ctx, principal.MerchantID, principal.Environment, func(state *ports.AppBillingState) error {
		if old := state.Account; old != nil {
			if old.Currency == input.Currency && old.CycleStart.Equal(input.CycleStart) && old.CycleEnd.Equal(input.CycleEnd) && old.Enabled == input.Enabled {
				result = *old
				return nil
			}
			if old.Revision != input.Revision || old.Currency != input.Currency {
				return domain.ErrConflict
			}
			same := old.CycleStart.Equal(input.CycleStart) && old.CycleEnd.Equal(input.CycleEnd)
			if !same && !old.CycleEnd.Equal(input.CycleStart) {
				return fmt.Errorf("%w: billing cycles must be consecutive and cannot be rewritten", domain.ErrConflict)
			}
		} else if input.Revision != 0 {
			return domain.ErrConflict
		}
		result = domain.AppBillingAccount{MerchantID: principal.MerchantID, Environment: principal.Environment, Currency: input.Currency, CycleStart: input.CycleStart, CycleEnd: input.CycleEnd, Enabled: input.Enabled, Revision: input.Revision + 1}
		state.Account = &result
		return s.event(state, "billing_account.synchronized", principal.Subject, principal.MerchantID, billingHash(input))
	})
	return result, err
}

type IssueAppInvoice struct {
	InvoiceID  string    `json:"invoiceId"`
	Currency   string    `json:"currency"`
	CycleStart time.Time `json:"cycleStart"`
	CycleEnd   time.Time `json:"cycleEnd"`
}

func (s *AppBillingService) IssueInvoice(ctx context.Context, principal EmisellBackendPrincipal, input IssueAppInvoice) (domain.AppBillingInvoice, error) {
	var result domain.AppBillingInvoice
	if err := s.checkBackend(principal); err != nil {
		return result, err
	}
	if !externalIdentityValue.MatchString(input.InvoiceID) {
		return result, domain.ErrValidation
	}
	input.CycleStart, input.CycleEnd = input.CycleStart.UTC(), input.CycleEnd.UTC()
	err := s.repo.WithMerchantBilling(ctx, principal.MerchantID, principal.Environment, func(state *ports.AppBillingState) error {
		for _, invoice := range state.Invoices {
			if invoice.InvoiceID == input.InvoiceID {
				if invoice.Currency != input.Currency || !invoice.CycleStart.Equal(input.CycleStart) || !invoice.CycleEnd.Equal(input.CycleEnd) {
					return domain.ErrConflict
				}
				result = invoice
				return nil
			}
		}
		account := state.Account
		if account == nil || !account.Enabled || input.Currency != account.Currency || !input.CycleStart.Equal(account.CycleStart) || !input.CycleEnd.Equal(account.CycleEnd) || !validMonthlyCycle(input.CycleStart, input.CycleEnd, s.now()) {
			return domain.ErrConflict
		}
		// Charge a renewal only after the previous paid period. Never turn an
		// unpaid first subscription into a growing series of automatic debts.
		for _, sub := range state.Subscriptions {
			if sub.Status != "active" || sub.Plan.AmountMinor == 0 || sub.PaidThrough == nil || sub.PaidThrough.After(account.CycleStart) {
				continue
			}
			inst, err := billingInstallation(state, sub.InstallationID, false)
			if err != nil || inst.Status == domain.InstallationStatusUninstalled || !state.AvailableApps[inst.AppID] {
				continue
			}
			already := false
			for _, charge := range state.Charges {
				if charge.SubscriptionID == sub.ID && (charge.PeriodStart.Equal(account.CycleStart) || charge.Status == "invoiced" || charge.Status == "failed" || charge.Status == "unbilled") {
					already = true
				}
			}
			if !already {
				charge, err := s.newCharge(sub, sub.Plan.AmountMinor, account.CycleStart, account.CycleEnd)
				if err != nil {
					return err
				}
				state.Charges = append(state.Charges, charge)
			}
		}
		result = domain.AppBillingInvoice{InvoiceID: input.InvoiceID, MerchantID: principal.MerchantID, Environment: principal.Environment, Currency: input.Currency, CycleStart: account.CycleStart, CycleEnd: account.CycleEnd, Lines: []domain.AppSubscriptionCharge{}, Status: "issued", CreatedAt: s.now().UTC()}
		for i := range state.Charges {
			charge := &state.Charges[i]
			if charge.Status != "unbilled" || charge.PeriodStart.After(s.now()) {
				continue
			}
			if charge.Currency != input.Currency {
				return domain.ErrConflict
			}
			charge.Status, charge.InvoiceID = "invoiced", input.InvoiceID
			result.Lines = append(result.Lines, *charge)
			if result.AmountMinor > 9_000_000_000_000-charge.AmountMinor {
				return domain.ErrValidation
			}
			result.AmountMinor += charge.AmountMinor
		}
		state.Invoices = append(state.Invoices, result)
		return s.event(state, "invoice.issued", principal.Subject, input.InvoiceID, billingHash(input))
	})
	return result, err
}

type AppInvoicePayment struct {
	EventID     string `json:"eventId"`
	Status      string `json:"status"`
	PaymentID   string `json:"paymentId"`
	Currency    string `json:"currency"`
	AmountMinor int64  `json:"amountMinor"`
}

func (s *AppBillingService) Payment(ctx context.Context, principal EmisellBackendPrincipal, invoiceID string, input AppInvoicePayment) (domain.AppBillingInvoice, error) {
	var result domain.AppBillingInvoice
	if err := s.checkBackend(principal); err != nil {
		return result, err
	}
	if !externalIdentityValue.MatchString(invoiceID) || !extensionConnectionRequestID.MatchString(input.EventID) || !slices.Contains([]string{"paid", "failed"}, input.Status) || !externalIdentityValue.MatchString(input.PaymentID) || input.AmountMinor < 0 {
		return result, domain.ErrValidation
	}
	hash := billingHash(struct {
		Invoice string
		Payment AppInvoicePayment
	}{invoiceID, input})
	err := s.repo.WithMerchantBilling(ctx, principal.MerchantID, principal.Environment, func(state *ports.AppBillingState) error {
		var invoice *domain.AppBillingInvoice
		for i := range state.Invoices {
			if state.Invoices[i].InvoiceID == invoiceID {
				invoice = &state.Invoices[i]
			}
		}
		if invoice == nil {
			return domain.ErrNotFound
		}
		for _, event := range state.Events {
			if event.ID == input.EventID {
				if event.RequestHash != hash {
					return domain.ErrConflict
				}
				result = *invoice
				return nil
			}
		}
		if input.AmountMinor != invoice.AmountMinor || input.Currency != invoice.Currency {
			return fmt.Errorf("%w: app invoice amount or currency does not match", domain.ErrConflict)
		}
		if input.Status == "paid" {
			for _, other := range state.Invoices {
				if other.InvoiceID != invoiceID && other.Status == "paid" && other.PaymentID == input.PaymentID {
					return fmt.Errorf("%w: payment is already assigned to another invoice", domain.ErrConflict)
				}
			}
		}
		if invoice.Status == "paid" {
			if input.Status != "paid" || invoice.PaymentID != input.PaymentID {
				return domain.ErrConflict
			}
		} else {
			invoice.Status, invoice.PaymentID = input.Status, input.PaymentID
			for i := range state.Charges {
				charge := &state.Charges[i]
				if charge.InvoiceID != invoiceID {
					continue
				}
				charge.Status = input.Status
				for j := range state.Subscriptions {
					sub := &state.Subscriptions[j]
					if sub.ID != charge.SubscriptionID {
						continue
					}
					if input.Status == "paid" {
						if sub.PaidThrough == nil || sub.PaidThrough.Before(charge.PeriodEnd) {
							paidThrough := charge.PeriodEnd
							sub.PaidThrough = &paidThrough
						}
						if sub.Status != "cancelled" {
							sub.Status = "active"
						}
					} else if sub.Status != "cancelled" {
						sub.Status = "past_due"
					}
				}
			}
		}
		state.Events = append(state.Events, domain.AppBillingEvent{ID: input.EventID, Action: "invoice." + input.Status, Subject: principal.Subject, ResourceID: invoiceID, RequestHash: hash, CreatedAt: s.now().UTC()})
		result = *invoice
		return nil
	})
	return result, err
}
