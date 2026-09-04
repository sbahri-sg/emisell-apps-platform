package httpapi

import (
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
)

type createVersionRequest struct {
	Version     string  `json:"version"`
	ReleaseNote *string `json:"releaseNote"`
}

type releaseVersionRequest struct {
	ExpectedActiveVersionID *string `json:"expectedActiveVersionId"`
}

func (h *handlers) listVersions(writer http.ResponseWriter, request *http.Request) {
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
	versions, meta, err := h.versions.List(request.Context(), actor.OrganizationID, request.PathValue("appId"), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: versions, Meta: meta})
}

func (h *handlers) createVersion(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "release.create"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input createVersionRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	version, err := h.versions.Create(request.Context(), application.CreateVersionCommand{
		OrganizationID: actor.OrganizationID,
		ActorID:        actor.UserID,
		IdempotencyKey: key,
		AppID:          request.PathValue("appId"),
		Version:        input.Version,
		ReleaseNote:    input.ReleaseNote,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: version})
}

func (h *handlers) getVersion(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	version, err := h.versions.Get(request.Context(), actor.OrganizationID, request.PathValue("appId"), request.PathValue("versionId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: version})
}

func (h *handlers) releaseVersion(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "release.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input releaseVersionRequest
	if err := decodeOptionalJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	version, err := h.versions.Release(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), request.PathValue("versionId"), key, input.ExpectedActiveVersionID)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: version})
}

func (h *handlers) rollbackVersion(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "release.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	version, err := h.versions.Rollback(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), request.PathValue("versionId"), key)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: version})
}
