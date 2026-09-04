package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

func (h *handlers) getAppIntegrationReadiness(w http.ResponseWriter, r *http.Request) {
	if err := requireCapability(r, "app.read"); err != nil {
		writeError(w, r, err)
		return
	}
	h.writeIntegrationReadiness(w, r, actorFromContext(r.Context()).OrganizationID)
}

func (h *handlers) getInternalIntegrationReadiness(w http.ResponseWriter, r *http.Request) {
	if err := requirePlatformOperator(r); err != nil {
		writeError(w, r, err)
		return
	}
	h.writeIntegrationReadiness(w, r, r.PathValue("organizationId"))
}

func (h *handlers) writeIntegrationReadiness(w http.ResponseWriter, r *http.Request, organizationID string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.URL.RawQuery != "" {
		writeError(w, r, fmt.Errorf("%w: query parameters are not accepted", domain.ErrValidation))
		return
	}
	if h.catalog == nil {
		writeError(w, r, domain.ErrNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	report, err := h.catalog.IntegrationReadiness(ctx, organizationID, r.PathValue("appId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope[any]{Data: report})
}
