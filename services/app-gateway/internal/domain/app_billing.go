package domain

import "time"

// Money is an integer in the currency's smallest unit. Never use floating point
// or the current catalogue price to reconstruct an approved subscription.
type AppPlan struct {
	ID          string    `json:"id"`
	AppID       string    `json:"appId"`
	AppName     string    `json:"appName"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AmountMinor int64     `json:"amountMinor"`
	Currency    string    `json:"currency"`
	Interval    string    `json:"interval"`
	Features    []string  `json:"features"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	RequestKey  string    `json:"-"`
	RequestHash string    `json:"-"`
}

type AppBillingAccount struct {
	MerchantID  string      `json:"merchantId"`
	Environment Environment `json:"environment"`
	Currency    string      `json:"currency"`
	CycleStart  time.Time   `json:"cycleStart"`
	CycleEnd    time.Time   `json:"cycleEnd"`
	Enabled     bool        `json:"enabled"`
	Revision    int64       `json:"revision"`
}

type AppSubscriptionQuote struct {
	ID              string    `json:"id"`
	InstallationID  string    `json:"installationId"`
	Plan            AppPlan   `json:"plan"`
	AmountMinor     int64     `json:"amountMinor"`
	PeriodStart     time.Time `json:"periodStart"`
	PeriodEnd       time.Time `json:"periodEnd"`
	ExpiresAt       time.Time `json:"expiresAt"`
	AccountRevision int64     `json:"accountRevision"`
	TermsVersion    string    `json:"termsVersion"`
}

type AppSubscription struct {
	ID                 string     `json:"id"`
	InstallationID     string     `json:"installationId"`
	QuoteID            string     `json:"quoteId"`
	Plan               AppPlan    `json:"plan"`
	Status             string     `json:"status"`
	ApprovedBy         string     `json:"approvedBy"`
	ApprovedAt         time.Time  `json:"approvedAt"`
	TermsVersion       string     `json:"termsVersion"`
	PaidThrough        *time.Time `json:"paidThrough"`
	CancelledAt        *time.Time `json:"cancelledAt"`
	CancellationReason string     `json:"cancellationReason,omitempty"`
}

type AppSubscriptionCharge struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscriptionId"`
	InstallationID string    `json:"installationId"`
	AppID          string    `json:"appId"`
	PlanID         string    `json:"planId"`
	Description    string    `json:"description"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	PeriodStart    time.Time `json:"periodStart"`
	PeriodEnd      time.Time `json:"periodEnd"`
	Status         string    `json:"status"`
	InvoiceID      string    `json:"invoiceId,omitempty"`
}

// This is the app portion of an Emisell invoice, not a second payment request.
// Lines and amount are frozen when issued; payment updates change status only.
type AppBillingInvoice struct {
	InvoiceID   string                  `json:"invoiceId"`
	MerchantID  string                  `json:"merchantId"`
	Environment Environment             `json:"environment"`
	Currency    string                  `json:"currency"`
	CycleStart  time.Time               `json:"cycleStart"`
	CycleEnd    time.Time               `json:"cycleEnd"`
	Lines       []AppSubscriptionCharge `json:"lines"`
	AmountMinor int64                   `json:"amountMinor"`
	Status      string                  `json:"status"`
	PaymentID   string                  `json:"paymentId,omitempty"`
	CreatedAt   time.Time               `json:"createdAt"`
}

type AppBillingEvent struct {
	ID          string    `json:"id"`
	Action      string    `json:"action"`
	Subject     string    `json:"subject"`
	ResourceID  string    `json:"resourceId"`
	RequestHash string    `json:"requestHash,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}
