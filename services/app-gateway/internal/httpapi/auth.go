package httpapi

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type Actor struct {
	UserID               string
	OrganizationID       string
	Role                 domain.Role
	Email                string
	DisplayName          string
	PlatformOperator     bool
	AuthenticationMethod string
	SessionID            string
	SessionExpiresAt     string
	OrganizationName     string
	OrganizationSlug     string
}

type Authenticator interface {
	Authenticate(request *http.Request) (Actor, error)
}

type DevelopmentAuthenticator struct {
	BearerToken        string
	DefaultUserID      string
	DefaultRole        domain.Role
	DefaultEmail       string
	DefaultDisplayName string
	PlatformOperator   bool
}

func (a DevelopmentAuthenticator) Authenticate(request *http.Request) (Actor, error) {
	token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	if token == request.Header.Get("Authorization") || subtle.ConstantTimeCompare([]byte(token), []byte(a.BearerToken)) != 1 {
		return Actor{}, fmt.Errorf("invalid bearer token")
	}
	organizationID := strings.TrimSpace(request.Header.Get("X-Organization-Id"))
	if organizationID == "" {
		return Actor{}, fmt.Errorf("organization header is required")
	}
	userID := strings.TrimSpace(request.Header.Get("X-Emisell-User-Id"))
	if userID == "" {
		userID = a.DefaultUserID
	}
	role := domain.Role(strings.TrimSpace(request.Header.Get("X-Emisell-Role")))
	if role == "" {
		role = a.DefaultRole
	}
	if !validRole(role) {
		return Actor{}, fmt.Errorf("invalid role")
	}
	return Actor{
		UserID: userID, OrganizationID: organizationID, Role: role,
		Email: a.DefaultEmail, DisplayName: a.DefaultDisplayName,
		PlatformOperator: a.PlatformOperator, AuthenticationMethod: "bearer",
	}, nil
}

func validRole(role domain.Role) bool {
	return role == domain.RoleOwner || role == domain.RoleAdmin || role == domain.RoleDeveloper || role == domain.RoleAnalyst
}

func can(role domain.Role, capability string) bool {
	switch role {
	case domain.RoleOwner:
		return true
	case domain.RoleAdmin:
		return capability != "organization.manage"
	case domain.RoleDeveloper:
		return capability == "app.read" || capability == "app.write" || capability == "release.create" || capability == "analytics.read"
	case domain.RoleAnalyst:
		return capability == "app.read" || capability == "analytics.read"
	default:
		return false
	}
}
