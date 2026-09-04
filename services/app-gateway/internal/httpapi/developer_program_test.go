package httpapi_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
)

func TestDeveloperQueueRequiresPlatformOperatorClaim(t *testing.T) {
	var sequence int
	id := func() (string, error) {
		sequence++
		return fmt.Sprintf("01995f72-0000-7000-8000-%012d", sequence), nil
	}
	now := func() time.Time { return time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC) }
	repository := memory.NewRepository(id, now)
	program := application.NewDeveloperProgramService(repository, id, now, 48*time.Hour)
	organizations := application.NewDeveloperOrganizationService(repository)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	build := func(platformOperator bool) http.Handler {
		return httpapi.NewServer(httpapi.Dependencies{
			AdminAuthenticator:     httpapi.DevelopmentAuthenticator{BearerToken: "test-token", DefaultUserID: testUserID, DefaultRole: domain.RoleOwner, DefaultEmail: "operator@example.test", PlatformOperator: platformOperator},
			DeveloperProgram:       program,
			DeveloperOrganizations: organizations,
			Authenticator: httpapi.DevelopmentAuthenticator{
				BearerToken: "test-token", DefaultUserID: testUserID, DefaultRole: domain.RoleOwner,
				DefaultEmail: "operator@example.test", PlatformOperator: platformOperator,
			},
			Logger: logger,
		})
	}
	request := func(handler http.Handler) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/internal/developer-applications", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Organization-Id", testOrganizationID)
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	if response := request(build(false)); response.Code != http.StatusForbidden {
		t.Fatalf("non-operator status = %d, want 403: %s", response.Code, response.Body.String())
	}
	if response := request(build(true)); response.Code != http.StatusOK {
		t.Fatalf("operator status = %d, want 200: %s", response.Code, response.Body.String())
	}
	organizationRequest := func(handler http.Handler) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/internal/organizations", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Organization-Id", testOrganizationID)
		handler.ServeHTTP(recorder, req)
		return recorder
	}
	if response := organizationRequest(build(false)); response.Code != http.StatusForbidden {
		t.Fatalf("non-operator organization status = %d, want 403: %s", response.Code, response.Body.String())
	}
	if response := organizationRequest(build(true)); response.Code != http.StatusOK {
		t.Fatalf("operator organization status = %d, want 200: %s", response.Code, response.Body.String())
	}

	for _, path := range []string{
		"/v1/apps/01995f72-0000-7000-8000-000000000099/installations",
		"/v1/oauth/authorizations",
	} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Organization-Id", testOrganizationID)
		build(false).ServeHTTP(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("non-operator simulator %s status = %d, want 403: %s", path, recorder.Code, recorder.Body.String())
		}
	}
}
