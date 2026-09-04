package httpapi

import (
	"context"
	"net"
	"net/http"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func connectionSelector(r *http.Request) ports.ExtensionConnectionSelector {
	return ports.ExtensionConnectionSelector{OrganizationID: r.PathValue("organizationId"), AppID: r.PathValue("appId"), InstallationID: r.PathValue("installationId"), ExtensionID: r.PathValue("extensionId")}
}

func (h *handlers) extensionConnectionsReady(w http.ResponseWriter, r *http.Request, operator bool) bool {
	setInstallationResponseHeaders(w)
	if operator {
		if err := requirePlatformOperator(r); err != nil {
			writeError(w, r, err)
			return false
		}
	}
	if h.extensionConnections == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "extension_connections_disabled", Message: "Managed extension credentials are not enabled.", RequestID: requestIDFromContext(r.Context())}})
		return false
	}
	return true
}

// Credential input is never echoed in decoder errors, even if a field name is secret-bearing.
func decodeConnectionRequest(w http.ResponseWriter, r *http.Request, input any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if r.URL.RawQuery != "" || decodeJSON(r, input) != nil {
		writeError(w, r, domain.ErrValidation)
		return false
	}
	return true
}

func (h *handlers) getExtensionConnection(w http.ResponseWriter, r *http.Request) {
	if !h.extensionConnectionsReady(w, r, true) {
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, r, domain.ErrValidation)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.extensionConnections.Get(ctx, connectionSelector(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope[any]{Data: result})
}

func (h *handlers) provisionExtensionConnection(w http.ResponseWriter, r *http.Request) {
	if !h.extensionConnectionsReady(w, r, true) {
		return
	}
	var input struct {
		Revision         int64             `json:"revision"`
		RuntimeName      string            `json:"runtimeName"`
		Scopes           []string          `json:"scopes"`
		Secret           map[string]string `json:"secret"`
		RuntimeExpiresAt time.Time         `json:"runtimeExpiresAt"`
	}
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.extensionConnections.Provision(ctx, application.ProvisionExtensionConnection{
		Selector: connectionSelector(r), ActorID: actorFromContext(ctx).UserID,
		Revision: input.Revision, RuntimeName: input.RuntimeName, Scopes: input.Scopes,
		Secret: input.Secret, RuntimeExpiresAt: input.RuntimeExpiresAt,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope[any]{Data: result})
}

func (h *handlers) rotateExtensionConnection(w http.ResponseWriter, r *http.Request) {
	if !h.extensionConnectionsReady(w, r, true) {
		return
	}
	var input struct {
		Revision         int64     `json:"revision"`
		RuntimeExpiresAt time.Time `json:"runtimeExpiresAt"`
	}
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.extensionConnections.Rotate(ctx, connectionSelector(r), actorFromContext(ctx).UserID, input.Revision, input.RuntimeExpiresAt)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope[any]{Data: result})
}

func (h *handlers) revokeExtensionConnection(w http.ResponseWriter, r *http.Request) {
	if !h.extensionConnectionsReady(w, r, true) {
		return
	}
	var input struct {
		Revision int64 `json:"revision"`
	}
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.extensionConnections.Revoke(ctx, connectionSelector(r), actorFromContext(ctx).UserID, input.Revision); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) resolveExtensionCredential(w http.ResponseWriter, r *http.Request) {
	setInstallationResponseHeaders(w)
	// This endpoint is machine-only: browser sessions and control-plane tokens are not runtime identities.
	if len(r.Header.Values("Authorization")) != 1 || len(r.Header.Values("Cookie")) != 0 || len(r.Header.Values("Origin")) != 0 {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}
	token, err := installationBearerToken(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !h.extensionConnectionsReady(w, r, false) {
		return
	}
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remote = r.RemoteAddr
	}
	// Bound unauthenticated attempts as well. Forwarded IP headers are not trusted.
	if !h.extensionCredentialRate.allow(remote) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, errorEnvelope{Error: apiError{Code: "rate_limited", Message: "Too many runtime requests.", RequestID: requestIDFromContext(r.Context())}})
		return
	}
	var input struct {
		Scope string `json:"scope"`
	}
	if !decodeConnectionRequest(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.extensionConnections.Resolve(ctx, token, input.Scope, requestIDFromContext(ctx))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope[any]{Data: result})
}
