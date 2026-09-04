package ports

import (
	"context"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type AppBillingState struct {
	Account       *domain.AppBillingAccount
	Installations []domain.AppInstallation
	Apps          []domain.App
	AvailableApps map[string]bool
	Plans         []domain.AppPlan
	Quotes        []domain.AppSubscriptionQuote
	Subscriptions []domain.AppSubscription
	Charges       []domain.AppSubscriptionCharge
	Invoices      []domain.AppBillingInvoice
	Events        []domain.AppBillingEvent
}

type AppBillingRepository interface {
	// Callbacks run under lifecycle locks and must not make network requests.
	WithAppPlans(context.Context, string, string, func(domain.App, *[]domain.AppPlan) error) error
	WithMerchantBilling(context.Context, string, domain.Environment, func(*AppBillingState) error) error
}
