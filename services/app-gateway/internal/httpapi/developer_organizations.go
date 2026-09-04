package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (h *handlers) listDeveloperOrganizations(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	filter, err := developerOrganizationFilter(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	organizations, meta, err := h.developerOrganizations.List(request.Context(), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: organizations, Meta: meta})
}

func (h *handlers) getDeveloperOrganization(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	organization, err := h.developerOrganizations.Get(request.Context(), request.PathValue("organizationId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: organization})
}

func developerOrganizationFilter(request *http.Request) (ports.DeveloperOrganizationFilter, error) {
	filter := ports.DeveloperOrganizationFilter{
		Cursor: request.URL.Query().Get("cursor"), Search: request.URL.Query().Get("search"),
		Status: domain.OrganizationStatus(request.URL.Query().Get("status")), Limit: 25,
	}
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return ports.DeveloperOrganizationFilter{}, fmt.Errorf("%w: limit must be between 1 and 100", domain.ErrValidation)
		}
		filter.Limit = limit
	}
	return filter, nil
}
