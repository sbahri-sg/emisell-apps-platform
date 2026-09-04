package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

func TestBrowserSessionOrganizationSwitchRequiresCSRFAndMembership(t *testing.T) {
	now := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	sequence := 0
	id := func() (string, error) {
		sequence++
		return []string{"01995f72-0000-7000-8000-000000000101", "01995f72-0000-7000-8000-000000000102", "01995f72-0000-7000-8000-000000000103"}[sequence-1], nil
	}
	repository := memory.NewRepository(id, func() time.Time { return now })
	userID := "01995f72-0000-7000-8000-000000000002"
	firstOrg := "01995f72-0000-7000-8000-000000000001"
	secondOrg := "01995f72-0000-7000-8000-000000000003"
	repository.SeedIdentityMemberships(userID,
		domain.OrganizationMembership{OrganizationID: firstOrg, Name: "Emisell", Slug: "emisell", Status: "active", Role: domain.RoleOwner},
		domain.OrganizationMembership{OrganizationID: secondOrg, Name: "Partner", Slug: "partner", Status: "active", Role: domain.RoleDeveloper},
	)
	identity := application.NewIdentityService(repository, id, func() time.Time { return now }, 12*time.Hour, 2*time.Hour)
	sessionAuthenticator := SessionAuthenticator{Identity: identity, CookieName: "emisell_session"}
	handler := NewServer(Dependencies{
		Identity: identity, Authenticator: CompositeAuthenticator{Session: sessionAuthenticator, Bearer: DevelopmentAuthenticator{
			BearerToken: "dev-token", DefaultUserID: userID, DefaultRole: domain.RoleOwner,
			DefaultEmail: "developer@example.test", DefaultDisplayName: "Developer", PlatformOperator: true,
		}},
		IdentityHTTP: IdentityHTTPOptions{
			FrontendURL: "http://localhost:3003", SessionCookieName: "emisell_session", CSRFCookieName: "emisell_csrf",
			AllowedOrigins: []string{"http://localhost:3003"}, Development: &DevelopmentSessionIdentity{
				UserID: userID, Email: "developer@example.test", DisplayName: "Developer", PreferredOrganizationID: firstOrg, PlatformOperator: true,
			},
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), AllowedOrigins: []string{"http://localhost:3003"},
	})

	login := httptest.NewRequest(http.MethodPost, "/auth/development-login", nil)
	login.Header.Set("Origin", "http://localhost:3003")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusCreated {
		t.Fatalf("development login status=%d body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		switch cookie.Name {
		case "emisell_session":
			sessionCookie = cookie
		case "emisell_csrf":
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || csrfCookie == nil || csrfCookie.HttpOnly {
		t.Fatal("expected HttpOnly session cookie and browser-readable CSRF cookie")
	}

	switchBody, _ := json.Marshal(map[string]string{"organizationId": secondOrg})
	withoutCSRF := httptest.NewRequest(http.MethodPost, "/v1/session/organization", bytes.NewReader(switchBody))
	withoutCSRF.AddCookie(sessionCookie)
	withoutCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(withoutCSRFResponse, withoutCSRF)
	if withoutCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d body=%s", withoutCSRFResponse.Code, withoutCSRFResponse.Body.String())
	}

	withCSRF := httptest.NewRequest(http.MethodPost, "/v1/session/organization", bytes.NewReader(switchBody))
	withCSRF.AddCookie(sessionCookie)
	withCSRF.Header.Set("X-CSRF-Token", csrfCookie.Value)
	withCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(withCSRFResponse, withCSRF)
	if withCSRFResponse.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", withCSRFResponse.Code, withCSRFResponse.Body.String())
	}

	readSession := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
	readSession.AddCookie(sessionCookie)
	readSessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(readSessionResponse, readSession)
	if readSessionResponse.Code != http.StatusOK || !bytes.Contains(readSessionResponse.Body.Bytes(), []byte(secondOrg)) {
		t.Fatalf("session status=%d body=%s", readSessionResponse.Code, readSessionResponse.Body.String())
	}
}
