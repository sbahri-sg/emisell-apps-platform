package httpapi

import (
	"context"
	"net/http"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

func (h *handlers) billingReady(w http.ResponseWriter, r *http.Request) bool {
	setInstallationResponseHeaders(w)
	if h.appBilling == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "app_billing_disabled", Message: "App plans and subscriptions are not enabled. No app fees are collected by this module.", RequestID: requestIDFromContext(r.Context())}})
		return false
	}
	if r.URL.RawQuery != "" {
		writeError(w, r, domain.ErrValidation)
		return false
	}
	return true
}

func billingResult(w http.ResponseWriter, r *http.Request, result any, err error) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope[any]{Data: result})
}

func (h *handlers) listAppPlans(w http.ResponseWriter, r *http.Request) {
	if err := requireCapability(r, "app.read"); err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	actor := actorFromContext(r.Context())
	result, err := h.appBilling.Plans(r.Context(), actor.OrganizationID, r.PathValue("appId"))
	billingResult(w, r, result, err)
}

func (h *handlers) createAppPlan(w http.ResponseWriter, r *http.Request) {
	if err := requireCapability(r, "app.manage"); err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	var input application.CreateAppPlan
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	actor := actorFromContext(r.Context())
	result, err := h.appBilling.CreatePlan(r.Context(), actor.OrganizationID, r.PathValue("appId"), key, input)
	billingResult(w, r, result, err)
}

func (h *handlers) archiveAppPlan(w http.ResponseWriter, r *http.Request) {
	if err := requireCapability(r, "app.manage"); err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	actor := actorFromContext(r.Context())
	if err := h.appBilling.ArchivePlan(r.Context(), actor.OrganizationID, r.PathValue("appId"), r.PathValue("planId")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) getMerchantInstallationBilling(w http.ResponseWriter, r *http.Request) {
	merchant, err := h.authenticateMerchantRequest(r, false)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	result, err := h.appBilling.Installation(r.Context(), merchant.Identity.MerchantID, merchant.Identity.Environment, r.PathValue("installationId"))
	billingResult(w, r, result, err)
}

func (h *handlers) quoteAppSubscription(w http.ResponseWriter, r *http.Request) {
	merchant, err := h.authenticateMerchantRequest(r, true)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	var input struct {
		PlanID string `json:"planId"`
	}
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	result, err := h.appBilling.Quote(r.Context(), merchant.Identity.MerchantID, merchant.Identity.Environment, r.PathValue("installationId"), input.PlanID)
	billingResult(w, r, result, err)
}

func (h *handlers) approveAppSubscription(w http.ResponseWriter, r *http.Request) {
	merchant, err := h.authenticateMerchantRequest(r, true)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	var input struct {
		QuoteID               string `json:"quoteId"`
		AcceptRecurringCharge bool   `json:"acceptRecurringCharge"`
	}
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	result, err := h.appBilling.Approve(r.Context(), merchant.Identity, r.PathValue("installationId"), input.QuoteID, input.AcceptRecurringCharge)
	billingResult(w, r, result, err)
}

func (h *handlers) cancelAppSubscription(w http.ResponseWriter, r *http.Request) {
	merchant, err := h.authenticateMerchantRequest(r, true)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	if err := h.appBilling.Cancel(r.Context(), merchant.Identity, r.PathValue("installationId"), r.PathValue("subscriptionId")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) getInstallationBilling(w http.ResponseWriter, r *http.Request) {
	setInstallationResponseHeaders(w)
	if r.Header.Get("Cookie") != "" || r.Header.Get("Origin") != "" || h.installationAccess == nil {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}
	token, err := installationBearerToken(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	access, err := h.installationAccess.Authenticate(r.Context(), token)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !h.billingReady(w, r) {
		return
	}
	result, err := h.appBilling.Installation(r.Context(), access.MerchantID, access.Environment, access.InstallationID)
	if result.Subscription != nil {
		copy := *result.Subscription
		copy.ApprovedBy = ""
		result.Subscription = &copy
	}
	billingResult(w, r, result, err)
}

func (h *handlers) billingBackend(w http.ResponseWriter, r *http.Request) (application.EmisellBackendPrincipal, bool) {
	setInstallationResponseHeaders(w)
	if r.Header.Get("Cookie") != "" || r.Header.Get("Origin") != "" || h.emisellBackendAuthenticator == nil {
		writeError(w, r, domain.ErrUnauthorized)
		return application.EmisellBackendPrincipal{}, false
	}
	principal, err := h.emisellBackendAuthenticator.AuthenticateEmisellBackend(r)
	if err != nil {
		writeError(w, r, domain.ErrUnauthorized)
		return principal, false
	}
	return principal, h.billingReady(w, r)
}

func (h *handlers) syncAppBillingAccount(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.billingBackend(w, r)
	if !ok {
		return
	}
	var input application.SyncAppBillingAccount
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.appBilling.SyncAccount(ctx, principal, input)
	billingResult(w, r, result, err)
}

func (h *handlers) issueAppBillingInvoice(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.billingBackend(w, r)
	if !ok {
		return
	}
	var input application.IssueAppInvoice
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.appBilling.IssueInvoice(ctx, principal, input)
	billingResult(w, r, result, err)
}

func (h *handlers) recordAppInvoicePayment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.billingBackend(w, r)
	if !ok {
		return
	}
	var input application.AppInvoicePayment
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.appBilling.Payment(ctx, principal, r.PathValue("invoiceId"), input)
	billingResult(w, r, result, err)
}
