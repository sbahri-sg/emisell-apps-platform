package httpapi

import (
	"fmt"
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type replaceScopesRequest struct {
	Scopes *[]struct {
		Scope  string             `json:"scope"`
		Access domain.ScopeAccess `json:"access"`
	} `json:"scopes"`
}

func (h *handlers) listScopeCatalog(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, envelope[any]{Data: h.scopes.Catalog()})
}

func (h *handlers) listScopes(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	scopes, err := h.scopes.List(request.Context(), actor.OrganizationID, request.PathValue("appId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: scopes})
}

func (h *handlers) replaceScopes(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.write"); err != nil {
		writeError(writer, request, err)
		return
	}
	var input replaceScopesRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	if input.Scopes == nil {
		writeError(writer, request, fmt.Errorf("%w: scopes is required", domain.ErrValidation))
		return
	}
	items := make([]domain.AppScope, 0, len(*input.Scopes))
	for _, scope := range *input.Scopes {
		items = append(items, domain.AppScope{Scope: scope.Scope, Access: scope.Access})
	}
	actor := actorFromContext(request.Context())
	scopes, err := h.scopes.Replace(request.Context(), application.ReplaceScopesCommand{
		OrganizationID: actor.OrganizationID,
		ActorID:        actor.UserID,
		AppID:          request.PathValue("appId"),
		Scopes:         items,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: scopes})
}
