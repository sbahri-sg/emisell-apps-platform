package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/application"
)

type SessionAuthenticator struct {
	Identity   *application.IdentityService
	CookieName string
}

func (a SessionAuthenticator) Authenticate(request *http.Request) (Actor, error) {
	cookie, err := request.Cookie(a.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return Actor{}, fmt.Errorf("missing session cookie")
	}
	session, membership, err := a.Identity.Authenticate(request.Context(), cookie.Value)
	if err != nil {
		return Actor{}, err
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodOptions {
		if err := a.Identity.ValidateCSRF(session, request.Header.Get("X-CSRF-Token")); err != nil {
			return Actor{}, err
		}
	}
	actor := Actor{
		UserID: session.UserID, Email: session.Email, DisplayName: session.DisplayName,
		PlatformOperator: session.PlatformOperator, AuthenticationMethod: "session",
		SessionID: session.ID, SessionExpiresAt: session.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if membership != nil {
		actor.OrganizationID = membership.OrganizationID
		actor.OrganizationName = membership.Name
		actor.OrganizationSlug = membership.Slug
		actor.Role = membership.Role
	}
	return actor, nil
}

type CompositeAuthenticator struct {
	Session SessionAuthenticator
	Bearer  Authenticator
}

func (a CompositeAuthenticator) Authenticate(request *http.Request) (Actor, error) {
	if cookie, err := request.Cookie(a.Session.CookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return a.Session.Authenticate(request)
	}
	return a.Bearer.Authenticate(request)
}
