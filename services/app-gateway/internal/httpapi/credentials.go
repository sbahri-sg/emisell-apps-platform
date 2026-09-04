package httpapi

import (
	"net/http"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type createCredentialRequest struct {
	Environment domain.Environment `json:"environment"`
	ExpiresAt   *time.Time         `json:"expiresAt"`
}

func (h *handlers) listCredentials(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "credential.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	credentials, err := h.credentials.List(request.Context(), actor.OrganizationID, request.PathValue("appId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: credentials})
}

func (h *handlers) createCredential(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "credential.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input createCredentialRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	secret, err := h.credentials.Create(request.Context(), application.CreateCredentialCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: key,
		AppID: request.PathValue("appId"), Environment: input.Environment, ExpiresAt: input.ExpiresAt,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: secret})
}

func (h *handlers) rotateCredential(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "credential.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	secret, err := h.credentials.Rotate(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), request.PathValue("credentialId"), key)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: secret})
}

func (h *handlers) revokeCredential(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "credential.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	if err := h.credentials.Revoke(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), request.PathValue("credentialId"), key); err != nil {
		writeError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
