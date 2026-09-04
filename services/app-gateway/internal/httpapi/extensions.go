package httpapi

import (
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type createExtensionRequest struct {
	Name          string                 `json:"name"`
	Type          domain.ExtensionType   `json:"type"`
	RuntimeURL    *string                `json:"runtimeUrl"`
	Configuration map[string]interface{} `json:"configuration"`
}

type updateExtensionRequest struct {
	Name          *string                 `json:"name"`
	RuntimeURL    optionalString          `json:"runtimeUrl"`
	Configuration *map[string]interface{} `json:"configuration"`
	Revision      int64                   `json:"revision"`
}

func (h *handlers) listExtensions(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	extensions, err := h.extensions.List(request.Context(), actor.OrganizationID, request.PathValue("appId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: extensions})
}

func (h *handlers) createExtension(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input createExtensionRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	extension, err := h.extensions.Create(request.Context(), application.CreateExtensionCommand{
		OrganizationID: actor.OrganizationID,
		ActorID:        actor.UserID,
		IdempotencyKey: key,
		AppID:          request.PathValue("appId"),
		Name:           input.Name,
		Type:           input.Type,
		RuntimeURL:     input.RuntimeURL,
		Configuration:  input.Configuration,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: extension})
}

func (h *handlers) updateExtension(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	var input updateExtensionRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	command := application.UpdateExtensionCommand{
		OrganizationID: actor.OrganizationID,
		ActorID:        actor.UserID,
		AppID:          request.PathValue("appId"),
		ExtensionID:    request.PathValue("extensionId"),
		Name:           input.Name,
		Configuration:  input.Configuration,
		Revision:       input.Revision,
	}
	if input.RuntimeURL.Set {
		command.RuntimeURL = &input.RuntimeURL.Value
	}
	extension, err := h.extensions.Update(request.Context(), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: extension})
}

func (h *handlers) disableExtension(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	if err := h.extensions.Disable(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), request.PathValue("extensionId")); err != nil {
		writeError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
