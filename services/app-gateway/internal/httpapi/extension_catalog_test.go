package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

func TestPublicExtensionCatalogAndNoImplicitGrants(t *testing.T) {
	handler, _ := testServer()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/extension-catalog", nil))
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d", w.Code)
	}
	catalog := decodeData[domain.ExtensionCatalog](t, w)
	if len(catalog.Families) != 3 {
		t.Fatal("families absent")
	}
	// OpenAPI drift is checked by sync-extension-catalog --check, using the
	// same Go registry. Keep core API tests runnable in the backend-only Docker
	// mount without depending on frontend documentation files outside /app.
	app := postApp(t, handler, "catalog-no-grant-app")
	for _, id := range []string{"shipping.rates.calculate", "shipping.rates.quote", "payment.session.create", "merchant.profile.read", "rates:read"} {
		response := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{"scopes":[{"scope":"`+id+`","access":"required"}]}`, testOrganizationID, domain.RoleDeveloper, "")
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("capability %s granted as OAuth scope", id)
		}
	}
}
