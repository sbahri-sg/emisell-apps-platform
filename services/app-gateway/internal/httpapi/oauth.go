package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type authorizeOAuthRequest struct {
	ClientID       string             `json:"clientId"`
	RedirectURI    string             `json:"redirectUri"`
	State          string             `json:"state"`
	CodeChallenge  string             `json:"codeChallenge"`
	MerchantID     string             `json:"merchantId"`
	MerchantName   string             `json:"merchantName"`
	MerchantDomain *string            `json:"merchantDomain"`
	Environment    domain.Environment `json:"environment"`
	GrantedScopes  []string           `json:"grantedScopes"`
}

func (h *handlers) authorizeOAuth(writer http.ResponseWriter, request *http.Request) {
	if err := requireCapability(request, "installation.manage"); err != nil {
		writeError(writer, request, err)
		return
	}
	if err := requirePlatformOperator(request); err != nil {
		writeError(writer, request, err)
		return
	}
	var input authorizeOAuthRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	actor := actorFromContext(request.Context())
	response, err := h.oauth.Authorize(request.Context(), application.AuthorizeOAuthCommand{
		OrganizationID: actor.OrganizationID, ActorID: actor.UserID, ClientID: input.ClientID,
		RedirectURI: input.RedirectURI, State: input.State, CodeChallenge: input.CodeChallenge,
		MerchantID: input.MerchantID, MerchantName: input.MerchantName, MerchantDomain: input.MerchantDomain,
		Environment: input.Environment, GrantedScopes: input.GrantedScopes,
	})
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
	writeJSON(writer, http.StatusCreated, envelope[any]{Data: response})
}

func (h *handlers) exchangeOAuthToken(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if err := request.ParseForm(); err != nil {
		writeOAuthError(writer, http.StatusBadRequest, "invalid_request", "The token request is malformed.")
		return
	}
	clientID, clientSecret, ok := request.BasicAuth()
	if !ok || strings.TrimSpace(clientID) == "" || clientSecret == "" {
		writer.Header().Set("WWW-Authenticate", `Basic realm="Emisell OAuth token"`)
		writeOAuthError(writer, http.StatusUnauthorized, "invalid_client", "HTTP Basic client authentication is required.")
		return
	}
	if formClientID := request.PostForm.Get("client_id"); formClientID != "" && formClientID != clientID {
		writeOAuthError(writer, http.StatusUnauthorized, "invalid_client", "Client credentials do not match.")
		return
	}
	response, err := h.oauth.Exchange(request.Context(), application.ExchangeOAuthCommand{
		GrantType: request.PostForm.Get("grant_type"), Code: request.PostForm.Get("code"),
		ClientID: clientID, ClientSecret: clientSecret, RedirectURI: request.PostForm.Get("redirect_uri"),
		CodeVerifier: request.PostForm.Get("code_verifier"),
	})
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			writer.Header().Set("WWW-Authenticate", `Basic realm="Emisell OAuth token"`)
			writeOAuthError(writer, http.StatusUnauthorized, "invalid_client", "Client authentication failed.")
			return
		}
		if errors.Is(err, domain.ErrInvalidGrant) {
			writeOAuthError(writer, http.StatusBadRequest, "invalid_grant", "The authorization grant is invalid, expired, consumed, or failed PKCE verification.")
			return
		}
		writeError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
	writeJSON(writer, http.StatusOK, response)
}

func writeOAuthError(writer http.ResponseWriter, status int, code, description string) {
	writeJSON(writer, status, map[string]string{"error": code, "error_description": description})
}
