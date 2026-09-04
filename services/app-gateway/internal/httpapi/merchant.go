package httpapi

import (
	"net/http"
	"slices"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type sandboxMerchantLoginRequest struct {
	MerchantID     string  `json:"merchantId"`
	MerchantName   string  `json:"merchantName"`
	MerchantDomain *string `json:"merchantDomain"`
}

type merchantOAuthRequest struct {
	ClientID             string   `json:"clientId"`
	RedirectURI          string   `json:"redirectUri"`
	State                string   `json:"state"`
	CodeChallenge        string   `json:"codeChallenge"`
	RequestedScopes      []string `json:"requestedScopes"`
	TestInstallRequestID string   `json:"testInstallRequestId"`
}

type authorizeMerchantOAuthRequest struct {
	ClientID             string   `json:"clientId"`
	RedirectURI          string   `json:"redirectUri"`
	State                string   `json:"state"`
	CodeChallenge        string   `json:"codeChallenge"`
	RequestedScopes      []string `json:"requestedScopes"`
	GrantedScopes        []string `json:"grantedScopes"`
	TestInstallRequestID string   `json:"testInstallRequestId"`
}

func (h *handlers) sandboxMerchantLogin(writer http.ResponseWriter, request *http.Request) {
	if h.identityHTTP.Development == nil || h.merchants == nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	if origin := request.Header.Get("Origin"); origin != "" && !slices.Contains(h.identityHTTP.AllowedOrigins, origin) {
		writeError(writer, request, domain.ErrForbidden)
		return
	}
	var input sandboxMerchantLoginRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	created, err := h.merchants.CreateSandboxSession(request.Context(), application.CreateSandboxMerchantSessionCommand{
		MerchantID: input.MerchantID, MerchantName: input.MerchantName, MerchantDomain: input.MerchantDomain,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	h.setMerchantCookies(writer, created)
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: merchantSessionResponse(created.Identity, created.Session.Session)})
}

func (h *handlers) getMerchantSession(writer http.ResponseWriter, request *http.Request) {
	merchant, err := h.authenticateMerchantRequest(request, false)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: merchantSessionResponse(merchant.Identity, merchant.Session)})
}

func (h *handlers) logoutMerchantSession(writer http.ResponseWriter, request *http.Request) {
	if _, err := h.authenticateMerchantRequest(request, true); err != nil {
		writeError(writer, request, err)
		return
	}
	if cookie, err := request.Cookie(h.identityHTTP.MerchantSessionCookieName); err == nil {
		if err := h.merchants.Revoke(request.Context(), cookie.Value); err != nil {
			writeError(writer, request, err)
			return
		}
	}
	h.clearMerchantCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handlers) previewMerchantOAuth(writer http.ResponseWriter, request *http.Request) {
	merchant, err := h.authenticateMerchantRequest(request, true)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input merchantOAuthRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	consent, err := h.oauth.PreviewMerchantConsent(request.Context(), merchant.Identity, application.MerchantOAuthRequest{
		ClientID: input.ClientID, RedirectURI: input.RedirectURI, State: input.State,
		CodeChallenge: input.CodeChallenge, RequestedScopes: input.RequestedScopes,
		TestInstallRequestID: input.TestInstallRequestID,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: consent})
}

func (h *handlers) authorizeMerchantOAuth(writer http.ResponseWriter, request *http.Request) {
	merchant, err := h.authenticateMerchantRequest(request, true)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	var input authorizeMerchantOAuthRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	authorization, err := h.oauth.AuthorizeMerchant(request.Context(), merchant.Identity, application.AuthorizeMerchantOAuthCommand{
		Request: application.MerchantOAuthRequest{
			ClientID: input.ClientID, RedirectURI: input.RedirectURI, State: input.State,
			CodeChallenge: input.CodeChallenge, RequestedScopes: input.RequestedScopes,
			TestInstallRequestID: input.TestInstallRequestID,
		},
		GrantedScopes: input.GrantedScopes,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: authorization})
}

func (h *handlers) listMerchantInstallations(writer http.ResponseWriter, request *http.Request) {
	merchant, err := h.authenticateMerchantRequest(request, false)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	items, err := h.merchants.ListInstalledApps(request.Context(), merchant)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: items})
}

func (h *handlers) uninstallMerchantInstallation(writer http.ResponseWriter, request *http.Request) {
	merchant, err := h.authenticateMerchantRequest(request, true)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	key, err := idempotencyKey(request)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	if err := h.merchants.Uninstall(request.Context(), merchant, request.PathValue("installationId"), key); err != nil {
		writeError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *handlers) authenticateMerchantRequest(request *http.Request, requireCSRF bool) (application.MerchantSessionContext, error) {
	if h.merchants == nil {
		return application.MerchantSessionContext{}, domain.ErrUnauthorized
	}
	cookie, err := request.Cookie(h.identityHTTP.MerchantSessionCookieName)
	if err != nil || cookie.Value == "" {
		return application.MerchantSessionContext{}, domain.ErrUnauthorized
	}
	merchant, err := h.merchants.Authenticate(request.Context(), cookie.Value)
	if err != nil {
		return application.MerchantSessionContext{}, err
	}
	if requireCSRF {
		if err := h.merchants.ValidateCSRF(merchant.Session, request.Header.Get("X-CSRF-Token")); err != nil {
			return application.MerchantSessionContext{}, err
		}
	}
	return merchant, nil
}

func merchantSessionResponse(identity domain.MerchantIdentity, session domain.IdentitySession) map[string]any {
	return map[string]any{
		"merchant":             identity,
		"sessionExpiresAt":     session.ExpiresAt,
		"authenticationMethod": "merchant_session",
	}
}

func (h *handlers) setMerchantCookies(writer http.ResponseWriter, created application.CreatedMerchantSession) {
	maxAge := max(0, int(time.Until(created.Session.Session.ExpiresAt).Seconds()))
	http.SetCookie(writer, &http.Cookie{
		Name: h.identityHTTP.MerchantSessionCookieName, Value: created.Session.SessionToken, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.identityHTTP.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(writer, &http.Cookie{
		Name: h.identityHTTP.MerchantCSRFCookieName, Value: created.Session.CSRFToken, Path: "/", MaxAge: maxAge,
		HttpOnly: false, Secure: h.identityHTTP.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *handlers) clearMerchantCookies(writer http.ResponseWriter) {
	for _, cookieName := range []string{h.identityHTTP.MerchantSessionCookieName, h.identityHTTP.MerchantCSRFCookieName} {
		http.SetCookie(writer, &http.Cookie{
			Name: cookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0),
			HttpOnly: cookieName == h.identityHTTP.MerchantSessionCookieName,
			Secure:   h.identityHTTP.CookieSecure, SameSite: http.SameSiteLaxMode,
		})
	}
}
