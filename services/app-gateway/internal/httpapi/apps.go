package httpapi

import (
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type createAppRequest struct {
	Name         string              `json:"name"`
	Description  *string             `json:"description"`
	Distribution domain.Distribution `json:"distribution"`
	AppURL       *string             `json:"appUrl"`
	ContactEmail *string             `json:"contactEmail"`
}

type updateAppRequest struct {
	Name         *string              `json:"name"`
	Description  optionalString       `json:"description"`
	Distribution *domain.Distribution `json:"distribution"`
	AppURL       optionalString       `json:"appUrl"`
	ContactEmail optionalString       `json:"contactEmail"`
	Revision     int64                `json:"revision"`
}

func (h *handlers) listApps(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	filter, err := pageFilter(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	apps, meta, err := h.apps.List(request.Context(), actor.OrganizationID, filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: apps, Meta: meta})
}

func (h *handlers) createApp(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input createAppRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	app, err := h.apps.Create(request.Context(), application.CreateAppCommand{
		OrganizationID: actor.OrganizationID,
		ActorID:        actor.UserID,
		IdempotencyKey: key,
		Name:           input.Name,
		Description:    input.Description,
		Distribution:   input.Distribution,
		AppURL:         input.AppURL,
		ContactEmail:   input.ContactEmail,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: app})
}

func (h *handlers) getApp(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	app, err := h.apps.Get(request.Context(), actor.OrganizationID, request.PathValue("appId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: app})
}

func (h *handlers) updateApp(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	var input updateAppRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	command := application.UpdateAppCommand{
		OrganizationID: actor.OrganizationID,
		ActorID:        actor.UserID,
		AppID:          request.PathValue("appId"),
		Name:           input.Name,
		Distribution:   input.Distribution,
		Revision:       input.Revision,
	}
	if input.Description.Set {
		command.Description = &input.Description.Value
	}
	if input.AppURL.Set {
		command.AppURL = &input.AppURL.Value
	}
	if input.ContactEmail.Set {
		command.ContactEmail = &input.ContactEmail.Value
	}
	app, err := h.apps.Update(request.Context(), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: app})
}

func (h *handlers) archiveApp(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	if err := h.apps.Archive(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), key); err != nil {
		writeError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
