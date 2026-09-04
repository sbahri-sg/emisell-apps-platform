package httpapi

import (
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type createWebhookRequest struct {
	Event       string `json:"event"`
	EndpointURL string `json:"endpointUrl"`
}

type updateWebhookRequest struct {
	EndpointURL *string               `json:"endpointUrl"`
	Status      *domain.WebhookStatus `json:"status"`
	Revision    int64                 `json:"revision"`
}

type publishWebhookEventRequest struct {
	Event   string         `json:"event"`
	Payload map[string]any `json:"payload"`
}

func (h *handlers) listWebhookEventCatalog(writer http.ResponseWriter, request *http.Request) {
	items, err := h.webhooks.ListEventCatalog(request.Context())
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: items})
}

func (h *handlers) listWebhooks(writer http.ResponseWriter, request *http.Request) {
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
	items, meta, err := h.webhooks.List(request.Context(), actor.OrganizationID, request.PathValue("appId"), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: items, Meta: meta})
}

func (h *handlers) createWebhook(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input createWebhookRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	secret, err := h.webhooks.Create(request.Context(), application.CreateWebhookCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: key,
		AppID: request.PathValue("appId"), Event: input.Event, EndpointURL: input.EndpointURL,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: secret})
}

func (h *handlers) updateWebhook(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	var input updateWebhookRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	subscription, err := h.webhooks.Update(request.Context(), application.UpdateWebhookCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, AppID: request.PathValue("appId"),
		WebhookID: request.PathValue("webhookId"), EndpointURL: input.EndpointURL, Status: input.Status, Revision: input.Revision,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: subscription})
}

func (h *handlers) disableWebhook(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	if err := h.webhooks.Disable(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), request.PathValue("webhookId")); err != nil {
		writeError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handlers) publishWebhookEvent(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "webhook.dispatch"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input publishWebhookEventRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	result, err := h.webhooks.Publish(request.Context(), application.PublishWebhookCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: key,
		AppID: request.PathValue("appId"), Event: input.Event, Payload: input.Payload,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, envelope[any]{Data: result})
}

func (h *handlers) listWebhookDeliveries(writer http.ResponseWriter, request *http.Request) {
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
	items, meta, err := h.webhooks.ListDeliveries(request.Context(), actor.OrganizationID, request.PathValue("appId"), request.PathValue("webhookId"), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: items, Meta: meta})
}
