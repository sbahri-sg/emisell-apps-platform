package httpapi

import (
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
)

type createDevelopmentInstallRequest struct {
	MerchantID string `json:"merchantId"`
}

func (h *handlers) listDevelopmentInstallRequests(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	items, err := h.developmentInstalls.List(request.Context(), actor.OrganizationID, request.PathValue("appId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: items})
}

func (h *handlers) createDevelopmentInstallRequest(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "installation.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input createDevelopmentInstallRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	created, err := h.developmentInstalls.Create(request.Context(), application.CreateDevelopmentInstallRequestCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: key,
		AppID: request.PathValue("appId"), MerchantID: input.MerchantID,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: created})
}
