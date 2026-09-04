package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type updateCatalogListingRequest struct {
	Category                domain.CatalogCategory      `json:"category"`
	Status                  domain.CatalogListingStatus `json:"status"`
	Featured                bool                        `json:"featured"`
	Revision                int64                       `json:"revision"`
	ExpectedAppRevision     *int64                      `json:"expectedAppRevision"`
	ExpectedActiveVersionID *string                     `json:"expectedActiveVersionId"`
}

func (h *handlers) listCatalogApps(writer http.ResponseWriter, request *http.Request) {
	filter, err := catalogFilter(request, false)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	items, meta, err := h.catalog.List(request.Context(), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: items, Meta: meta})
}

func (h *handlers) getCatalogApp(writer http.ResponseWriter, request *http.Request) {
	item, err := h.catalog.Get(request.Context(), request.PathValue("appId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: item})
}

func (h *handlers) listCatalogCandidates(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	filter, err := catalogFilter(request, true)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	items, meta, err := h.catalog.ListCandidates(request.Context(), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: items, Meta: meta})
}

func (h *handlers) getCatalogListing(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	listing, err := h.catalog.GetListing(request.Context(), request.PathValue("organizationId"), request.PathValue("appId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: listing})
}

func (h *handlers) updateCatalogListing(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	var input updateCatalogListingRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	listing, err := h.catalog.UpdateListing(request.Context(), application.UpdateCatalogListingCommand{
		OrganizationID: request.PathValue("organizationId"), AppID: request.PathValue("appId"),
		ActorID: actor.UserID, Category: input.Category, Status: input.Status,
		Featured: input.Featured, Revision: input.Revision,
		ExpectedAppRevision: input.ExpectedAppRevision, ExpectedActiveVersionID: input.ExpectedActiveVersionID,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: listing})
}

func catalogFilter(request *http.Request, includeStatus bool) (ports.CatalogFilter, error) {
	filter := ports.CatalogFilter{
		Cursor: request.URL.Query().Get("cursor"), Search: request.URL.Query().Get("search"),
		Category: domain.CatalogCategory(request.URL.Query().Get("category")), Limit: 25,
	}
	if includeStatus {
		filter.Status = domain.CatalogListingStatus(request.URL.Query().Get("status"))
	}
	if raw := strings.TrimSpace(request.URL.Query().Get("featured")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return ports.CatalogFilter{}, fmt.Errorf("%w: featured must be true or false", domain.ErrValidation)
		}
		filter.Featured = &value
	}
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return ports.CatalogFilter{}, fmt.Errorf("%w: limit must be between 1 and 100", domain.ErrValidation)
		}
		filter.Limit = limit
	}
	return filter, nil
}
