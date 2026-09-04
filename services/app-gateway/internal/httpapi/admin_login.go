package httpapi

import (
	"crypto/subtle"
	"errors"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

const AdminSessionCookie = "emisell_admin_session"
const AdminCSRFCookie = "emisell_admin_csrf"

type AdminSessionAuthenticator struct {
	Service *application.AdminLoginService
}

func (a AdminSessionAuthenticator) Authenticate(request *http.Request) (Actor, error) {
	if a.Service == nil {
		return Actor{}, domain.ErrUnauthorized
	}
	cookie, err := request.Cookie(AdminSessionCookie)
	if err != nil {
		return Actor{}, domain.ErrUnauthorized
	}
	s, err := a.Service.Authenticate(request.Context(), cookie.Value)
	if err != nil {
		return Actor{}, err
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodOptions {
		csrf := request.Header.Get("X-CSRF-Token")
		if csrf == "" || subtle.ConstantTimeCompare([]byte(application.IdentityTokenDigest(csrf)), []byte(s.CSRFTokenHash)) != 1 {
			return Actor{}, domain.ErrForbidden
		}
	}
	if s.ActiveOrgID == nil {
		return Actor{}, domain.ErrUnauthorized
	}
	return Actor{UserID: s.UserID, OrganizationID: *s.ActiveOrgID, Email: s.Email, DisplayName: s.DisplayName, PlatformOperator: true,
		AuthenticationMethod: "session", SessionID: s.ID, SessionExpiresAt: s.ExpiresAt.UTC().Format(time.RFC3339)}, nil
}

func (h *handlers) adminLogin(writer http.ResponseWriter, request *http.Request) {
	// Require a configured frontend Origin, including for local curl/Postman.
	// This protects login itself from CSRF before a session exists.
	if request.Header.Get("Origin") == "" || request.Header.Get("Origin") != strings.TrimRight(h.identityHTTP.FrontendURL, "/") {
		writeError(writer, request, domain.ErrForbidden)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(writer, request, domain.ErrValidation)
		return
	}
	if h.adminLoginService == nil {
		writeJSON(writer, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "admin_login_unavailable", Message: "Admin login requires PostgreSQL.", RequestID: requestIDFromContext(request.Context())}})
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, request, err)
		return
	}
	if len(input.Email) > 254 || len(input.Password) > 256 {
		writeError(writer, request, domain.ErrValidation)
		return
	}
	// Do not trust X-Forwarded-For from an arbitrary caller. Deploy behind an
	// edge limiter as well; proxy connections share this transport-IP budget.
	ip, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		ip = request.RemoteAddr
	}
	created, err := h.adminLoginService.Login(request.Context(), input.Email, input.Password, ip)
	if errors.Is(err, application.ErrAdminLoginLimited) {
		writer.Header().Set("Retry-After", "900")
		writeJSON(writer, http.StatusTooManyRequests, errorEnvelope{Error: apiError{Code: "admin_login_rate_limited", Message: "Too many sign-in attempts. Try again later.", RequestID: requestIDFromContext(request.Context())}})
		return
	}
	if err != nil {
		writeError(writer, request, err)
		return
	}
	if old, err := request.Cookie(AdminSessionCookie); err == nil {
		if err := h.adminLoginService.Logout(request.Context(), old.Value); err != nil {
			writeError(writer, request, err)
			return
		}
	}
	for name, value := range map[string]string{AdminSessionCookie: created.SessionToken, AdminCSRFCookie: created.CSRFToken} {
		http.SetCookie(writer, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: int(application.AdminSessionTTL.Seconds()),
			HttpOnly: name == AdminSessionCookie, Secure: h.identityHTTP.CookieSecure, SameSite: http.SameSiteStrictMode})
	}
	writeJSON(writer, http.StatusOK, envelope[any]{Data: map[string]string{"status": "authenticated"}})
}

func (h *handlers) adminLogout(writer http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie(AdminSessionCookie); err == nil {
		if err := h.adminLoginService.Logout(request.Context(), cookie.Value); err != nil {
			writeError(writer, request, err)
			return
		}
	}
	for _, name := range []string{AdminSessionCookie, AdminCSRFCookie} {
		http.SetCookie(writer, &http.Cookie{Name: name, Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: name == AdminSessionCookie, Secure: h.identityHTTP.CookieSecure, SameSite: http.SameSiteStrictMode})
	}
	writer.WriteHeader(http.StatusNoContent)
}
