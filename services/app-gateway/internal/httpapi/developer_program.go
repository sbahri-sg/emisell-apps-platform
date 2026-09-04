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

type createDeveloperApplicationRequest struct {
	CompanyName      string                  `json:"companyName"`
	CompanyDomain    string                  `json:"companyDomain"`
	ContactName      string                  `json:"contactName"`
	ContactEmail     string                  `json:"contactEmail"`
	RequestedAppName string                  `json:"requestedAppName"`
	AppType          domain.DeveloperAppType `json:"appType"`
	UseCase          string                  `json:"useCase"`
	RequestedScopes  []string                `json:"requestedScopes"`
}

type reviewDeveloperApplicationRequest struct {
	Revision int64   `json:"revision"`
	Notes    *string `json:"notes"`
}

type approveDeveloperApplicationRequest struct {
	Revision    int64 `json:"revision"`
	MaxApps     int   `json:"maxApps"`
	MaxWebhooks int   `json:"maxWebhooks"`
}

type rejectDeveloperApplicationRequest struct {
	Revision int64  `json:"revision"`
	Notes    string `json:"notes"`
}

type rotateDeveloperInvitationRequest struct {
	Revision int64 `json:"revision"`
}

type acceptDeveloperInvitationRequest struct {
	Token string `json:"token"`
}

func (h *handlers) listDeveloperApplications(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	filter, err := developerApplicationFilter(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	applications, meta, err := h.developerProgram.List(request.Context(), actor.OrganizationID, filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, pageEnvelope[any]{Data: applications, Meta: meta})
}

func (h *handlers) createDeveloperApplication(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	var input createDeveloperApplicationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	application, err := h.developerProgram.Create(request.Context(), application.CreateDeveloperApplicationCommand{
		PlatformOrgID: actor.OrganizationID, ActorID: actor.UserID, CompanyName: input.CompanyName,
		CompanyDomain: input.CompanyDomain, ContactName: input.ContactName, ContactEmail: input.ContactEmail,
		RequestedAppName: input.RequestedAppName, AppType: input.AppType, UseCase: input.UseCase,
		RequestedScopes: input.RequestedScopes,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: application})
}

func (h *handlers) getDeveloperApplication(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	application, err := h.developerProgram.Get(request.Context(), actor.OrganizationID, request.PathValue("applicationId"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: application})
}

func (h *handlers) reviewDeveloperApplication(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	var input reviewDeveloperApplicationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	application, err := h.developerProgram.StartReview(request.Context(), application.ReviewDeveloperApplicationCommand{
		PlatformOrgID: actor.OrganizationID, ActorID: actor.UserID,
		ApplicationID: request.PathValue("applicationId"), Revision: input.Revision, Notes: input.Notes,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: application})
}

func (h *handlers) approveDeveloperApplication(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	var input approveDeveloperApplicationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	result, err := h.developerProgram.Approve(request.Context(), application.ApproveDeveloperApplicationCommand{
		PlatformOrgID: actor.OrganizationID, ActorID: actor.UserID,
		ApplicationID: request.PathValue("applicationId"), Revision: input.Revision,
		MaxApps: input.MaxApps, MaxWebhooks: input.MaxWebhooks,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: result})
}

func (h *handlers) rejectDeveloperApplication(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	var input rejectDeveloperApplicationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	result, err := h.developerProgram.Reject(request.Context(), application.RejectDeveloperApplicationCommand{
		PlatformOrgID: actor.OrganizationID, ActorID: actor.UserID,
		ApplicationID: request.PathValue("applicationId"), Revision: input.Revision, Notes: input.Notes,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: result})
}

func (h *handlers) rotateDeveloperInvitation(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	var input rotateDeveloperInvitationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	result, err := h.developerProgram.RotateInvitation(request.Context(), application.RotateDeveloperInvitationCommand{
		PlatformOrgID: actor.OrganizationID, ActorID: actor.UserID,
		ApplicationID: request.PathValue("applicationId"), Revision: input.Revision,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: result})
}

func (h *handlers) revokeDeveloperInvitation(writer http.ResponseWriter, request *http.Request) {
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	invitation, err := h.developerProgram.RevokeInvitation(request.Context(), actor.OrganizationID, request.PathValue("invitationId"), actor.UserID)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: invitation})
}

func (h *handlers) acceptDeveloperInvitation(writer http.ResponseWriter, request *http.Request) {
	var input acceptDeveloperInvitationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	result, err := h.developerProgram.AcceptInvitation(request.Context(), input.Token, actor.UserID, actor.Email, actor.DisplayName)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: result})
}

func developerApplicationFilter(request *http.Request) (ports.DeveloperApplicationFilter, error) {
	filter := ports.DeveloperApplicationFilter{
		Cursor: request.URL.Query().Get("cursor"), Search: request.URL.Query().Get("search"),
		Status: domain.DeveloperApplicationStatus(request.URL.Query().Get("status")), Limit: 25,
	}
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return ports.DeveloperApplicationFilter{}, fmt.Errorf("%w: limit must be between 1 and 100", domain.ErrValidation)
		}
		filter.Limit = limit
	}
	return filter, nil
}
