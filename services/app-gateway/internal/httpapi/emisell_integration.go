package httpapi

import (
	"net/http"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type createMerchantSessionGrantRequest struct {
	ReturnTo string `json:"returnTo"`
}

func (h *handlers) createEmisellMerchantSessionGrant(writer http.ResponseWriter, request *http.Request) {
	if h.emisellBackendAuthenticator == nil || h.emisellIntegration == nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	principal, err := h.emisellBackendAuthenticator.AuthenticateEmisellBackend(request)
	if err != nil {
		writeError(writer, request, domain.ErrUnauthorized)
		return
	}
	var input createMerchantSessionGrantRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	grant, err := h.emisellIntegration.CreateMerchantSessionGrant(request.Context(), principal, input.ReturnTo)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: grant})
}

func (h *handlers) exchangeEmisellMerchantSessionGrant(writer http.ResponseWriter, request *http.Request) {
	if h.emisellIntegration == nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	exchanged, err := h.emisellIntegration.ExchangeMerchantSessionGrant(request.Context(), request.URL.Query().Get("code"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	h.setMerchantCookies(writer, exchanged.Created)
	writer.Header().Set("Location", strings.TrimRight(h.identityHTTP.FrontendURL, "/")+exchanged.ReturnTo)
	writer.WriteHeader(http.StatusSeeOther)
}
