package httpapi

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type DevelopmentSessionIdentity struct {
	UserID                  string
	Email                   string
	DisplayName             string
	PreferredOrganizationID string
	PlatformOperator        bool
}

type IdentityHTTPOptions struct {
	FrontendURL               string
	SessionCookieName         string
	CSRFCookieName            string
	MerchantSessionCookieName string
	MerchantCSRFCookieName    string
	CookieSecure              bool
	AllowedOrigins            []string
	Development               *DevelopmentSessionIdentity
}

type switchOrganizationRequest struct {
	OrganizationID string `json:"organizationId"`
}

func (h *handlers) getSession(writer http.ResponseWriter, request *http.Request) {
	actor := actorFromContext(request.Context())
	var activeOrganization any
	if actor.OrganizationID != "" {
		activeOrganization = map[string]any{
			"organizationId": actor.OrganizationID, "name": actor.OrganizationName,
			"slug": actor.OrganizationSlug, "role": actor.Role,
		}
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: map[string]any{
		"userId": actor.UserID, "organizationId": actor.OrganizationID, "role": actor.Role,
		"email": actor.Email, "displayName": actor.DisplayName, "platformOperator": actor.PlatformOperator,
		"authenticationMethod": actor.AuthenticationMethod, "sessionExpiresAt": nullableString(actor.SessionExpiresAt),
		"activeOrganization": activeOrganization,
	}})
}

func (h *handlers) listSessionOrganizations(writer http.ResponseWriter, request *http.Request) {
	actor := actorFromContext(request.Context())
	memberships, err := h.identity.ListOrganizations(request.Context(), actor.UserID)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: memberships})
}

func (h *handlers) switchSessionOrganization(writer http.ResponseWriter, request *http.Request) {
	actor := actorFromContext(request.Context())
	if actor.AuthenticationMethod != "session" || actor.SessionID == "" {
		writeError(writer, request, domain.ErrConflict)
		return
	}
	var input switchOrganizationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	_, membership, err := h.identity.SwitchOrganization(request.Context(), actor.SessionID, actor.UserID, input.OrganizationID)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: membership})
}

func (h *handlers) logout(writer http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie(h.identityHTTP.SessionCookieName); err == nil {
		if err := h.identity.Revoke(request.Context(), cookie.Value); err != nil {
			writeError(writer, request, err)
			return
		}
	}
	h.clearIdentityCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handlers) startOIDCLogin(writer http.ResponseWriter, request *http.Request) {
	if h.oidc == nil {
		writeJSON(writer, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "oidc_not_configured", Message: "The identity provider is not configured.", RequestID: requestIDFromContext(request.Context())}})
		return
	}
	location, err := h.oidc.StartLogin(request.Context(), request.URL.Query().Get("return_to"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	http.Redirect(writer, request, location, http.StatusFound)
}

func (h *handlers) completeOIDCLogin(writer http.ResponseWriter, request *http.Request) {
	if h.oidc == nil {
		writeError(writer, request, domain.ErrInvalidGrant)
		return
	}
	created, returnTo, err := h.oidc.CompleteLogin(request.Context(), request.URL.Query().Get("state"), request.URL.Query().Get("code"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	h.setIdentityCookies(writer, created)
	http.Redirect(writer, request, strings.TrimRight(h.identityHTTP.FrontendURL, "/")+returnTo, http.StatusFound)
}

func (h *handlers) developmentLogin(writer http.ResponseWriter, request *http.Request) {
	if h.identityHTTP.Development == nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	if origin := request.Header.Get("Origin"); origin != "" && !slices.Contains(h.identityHTTP.AllowedOrigins, origin) {
		writeError(writer, request, domain.ErrForbidden)
		return
	}
	identity := h.identityHTTP.Development
	created, err := h.identity.CreateSession(request.Context(), identity.UserID, identity.Email, identity.DisplayName, identity.PlatformOperator, identity.PreferredOrganizationID)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	h.setIdentityCookies(writer, created)
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: map[string]any{"status": "authenticated"}})
}

func (h *handlers) setIdentityCookies(writer http.ResponseWriter, created application.CreatedIdentitySession) {
	maxAge := max(0, int(time.Until(created.Session.ExpiresAt).Seconds()))
	http.SetCookie(writer, &http.Cookie{
		Name: h.identityHTTP.SessionCookieName, Value: created.SessionToken, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.identityHTTP.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(writer, &http.Cookie{
		Name: h.identityHTTP.CSRFCookieName, Value: created.CSRFToken, Path: "/", MaxAge: maxAge,
		HttpOnly: false, Secure: h.identityHTTP.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *handlers) clearIdentityCookies(writer http.ResponseWriter) {
	for _, cookieName := range []string{h.identityHTTP.SessionCookieName, h.identityHTTP.CSRFCookieName} {
		http.SetCookie(writer, &http.Cookie{
			Name: cookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0),
			HttpOnly: cookieName == h.identityHTTP.SessionCookieName, Secure: h.identityHTTP.CookieSecure, SameSite: http.SameSiteLaxMode,
		})
	}
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
