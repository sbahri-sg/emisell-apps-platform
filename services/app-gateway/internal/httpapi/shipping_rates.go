package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

func (h *handlers) calculateEmisellShippingRates(writer http.ResponseWriter, request *http.Request) {
	if h.emisellBackendAuthenticator == nil || h.shippingRates == nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	principal, err := h.emisellBackendAuthenticator.AuthenticateEmisellBackend(request)
	if err != nil {
		writeError(writer, request, domain.ErrUnauthorized)
		return
	}
	var input application.ShippingRateRequest
	if err := decodeJSON(request, &input); err != nil {
		writeShippingRateError(writer, request, &application.ShippingRateError{Status: 400, Code: "invalid_rate_request"})
		return
	}
	result, err := h.shippingRates.Calculate(request.Context(), principal, input, requestIDFromContext(request.Context()))
	if err != nil {
		writeShippingRateError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func writeShippingRateError(writer http.ResponseWriter, request *http.Request, err error) {
	var rateError *application.ShippingRateError
	if !errors.As(err, &rateError) {
		writeJSON(writer, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "Shipping rate calculation could not be completed.", RequestID: requestIDFromContext(request.Context())}})
		return
	}
	if rateError.RetryAfter > 0 {
		writer.Header().Set("Retry-After", strconv.Itoa(rateError.RetryAfter))
	}
	message := map[string]string{
		"invalid_merchant_context":       "The authenticated merchant context is invalid.",
		"shipping_rate_forbidden":        "The Emisell Backend token cannot calculate shipping rates.",
		"invalid_rate_request":           "Origin, destination, or weight is invalid.",
		"shipping_runtime_disabled":      "The API Kurir rate bridge is not enabled.",
		"shipping_runtime_unavailable":   "The API Kurir rate bridge is temporarily unavailable.",
		"shipping_extension_unavailable": "No active installed shipping extension supports rate calculation.",
		"shipping_extension_ambiguous":   "More than one active shipping extension supports rate calculation.",
		"shipping_disabled":              "Shipping is not enabled for this merchant.",
		"rate_not_available":             "No shipping rate is available for this request.",
		"shipping_rate_limited":          "Shipping rate calculation is temporarily rate limited.",
		"provider_quota_exhausted":       "The configured shipping provider quota is exhausted.",
		"shipping_provider_unavailable":  "The configured shipping provider is unavailable.",
	}[rateError.Code]
	if message == "" {
		message = "Shipping rate calculation could not be completed."
	}
	writeJSON(writer, rateError.Status, errorEnvelope{Error: apiError{Code: rateError.Code, Message: message, RequestID: requestIDFromContext(request.Context())}})
}
