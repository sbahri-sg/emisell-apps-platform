package httpapi

import (
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type createInstallationRequest struct {
	MerchantID     string             `json:"merchantId"`
	MerchantName   string             `json:"merchantName"`
	MerchantDomain *string            `json:"merchantDomain"`
	Environment    domain.Environment `json:"environment"`
	GrantedScopes  []string           `json:"grantedScopes"`
}

type updateInstallationRequest struct {
	Status   domain.InstallationStatus `json:"status"`
	Revision int64                     `json:"revision"`
}

type upgradeInstallationRequest struct {
	TargetVersionID            string `json:"targetVersionId"`
	ExpectedInstalledVersionID string `json:"expectedInstalledVersionId"`
	Revision                   int64  `json:"revision"`
}

func (h *handlers) listInstallations(writer http.ResponseWriter, request *http.Request) {
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
	items, meta, err := h.installations.List(request.Context(), actor.OrganizationID, request.PathValue("appId"), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: items, Meta: meta})
}

func (h *handlers) createInstallation(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "installation.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input createInstallationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	installation, err := h.installations.Create(request.Context(), application.CreateInstallationCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: key,
		AppID: request.PathValue("appId"), MerchantID: input.MerchantID, MerchantName: input.MerchantName,
		MerchantDomain: input.MerchantDomain, Environment: input.Environment, GrantedScopes: input.GrantedScopes,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: installation})
}

func (h *handlers) getInstallation(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "app.read"); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	installation, err := h.installations.Get(request.Context(), actor.OrganizationID, request.PathValue("appId"), request.PathValue("installationId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: installation})
}

func (h *handlers) updateInstallation(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "installation.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	var input updateInstallationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	installation, err := h.installations.Update(request.Context(), application.UpdateInstallationCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, AppID: request.PathValue("appId"),
		InstallationID: request.PathValue("installationId"), Status: input.Status, Revision: input.Revision,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: installation})
}

func (h *handlers) upgradeInstallation(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "installation.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input upgradeInstallationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	installation, err := h.installations.Upgrade(request.Context(), application.UpgradeInstallationCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: key,
		AppID: request.PathValue("appId"), InstallationID: request.PathValue("installationId"),
		TargetVersionID: input.TargetVersionID, ExpectedInstalledVersionID: input.ExpectedInstalledVersionID,
		Revision: input.Revision,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: installation})
}

func (h *handlers) uninstallInstallation(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "installation.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	if err := h.installations.Uninstall(request.Context(), actor.OrganizationID, actor.UserID, request.PathValue("appId"), request.PathValue("installationId"), key); err != nil {
		writeError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
