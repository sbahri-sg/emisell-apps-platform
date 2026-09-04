package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

func (h *handlers) getInstallationContext(writer http.ResponseWriter, request *http.Request) {
	setInstallationResponseHeaders(writer)
	accessToken, err := installationBearerToken(request)
	if err != nil {
		writeInstallationAccessError(writer, request, err, "")
		return
	}
	access, err := h.installationAccess.Authenticate(request.Context(), accessToken)
	if err != nil {
		writeInstallationAccessError(writer, request, err, "")
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: access})
}

func (h *handlers) getMerchantProfile(writer http.ResponseWriter, request *http.Request) {
	setInstallationResponseHeaders(writer)
	accessToken, err := installationBearerToken(request)
	if err != nil {
		writeInstallationAccessError(writer, request, err, "")
		return
	}
	profile, err := h.installationAccess.GetMerchantProfile(request.Context(), accessToken)
	if err != nil {
		writeInstallationAccessError(writer, request, err, application.ScopeReadMerchant)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: profile})
}

func setInstallationResponseHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
	writer.Header().Set("Vary", "Authorization")
}

func writeInstallationAccessError(writer http.ResponseWriter, request *http.Request, err error, requiredScope string) {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		writer.Header().Set("WWW-Authenticate", `Bearer realm="Emisell installation", error="invalid_token"`)
	case errors.Is(err, domain.ErrForbidden) && requiredScope != "":
		writer.Header().Set("WWW-Authenticate", `Bearer realm="Emisell installation", error="insufficient_scope", scope="`+requiredScope+`"`)
	}
	writeError(writer, request, err)
}

func installationBearerToken(request *http.Request) (string, error) {
	parts := strings.Fields(request.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", domain.ErrUnauthorized
	}
	return parts[1], nil
}
