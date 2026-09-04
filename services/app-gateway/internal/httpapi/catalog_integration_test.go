package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

func TestCatalogRequiresExplicitOperatorPublication(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()
	app := postApp(t, handler, "catalog-app-create-0001")
	version := postVersion(t, handler, app.ID, "1.0.0", "catalog-version-create-0001")
	releaseVersion(t, handler, app.ID, version.ID, "catalog-version-release-0001", nil, http.StatusOK, domain.RoleOwner)
	developmentResponse := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID, "", testOrganizationID, domain.RoleAnalyst, "")
	if development := decodeData[domain.App](t, developmentResponse); developmentResponse.Code != http.StatusOK || development.ReleaseStatus != domain.AppReleaseStatusDevelopment {
		t.Fatalf("unpublished app release status=%d data=%#v", developmentResponse.Code, development)
	}

	publicBefore := request(t, handler, http.MethodGet, "/v1/catalog/apps", "", "", "", "")
	if publicBefore.Code != http.StatusOK || len(decodeData[[]domain.CatalogApp](t, publicBefore)) != 0 {
		t.Fatalf("unpublished app leaked into catalog: status=%d body=%s", publicBefore.Code, publicBefore.Body.String())
	}

	candidates := request(t, handler, http.MethodGet, "/v1/internal/catalog/apps", "", testOrganizationID, domain.RoleOwner, "")
	items := decodeData[[]domain.CatalogCandidate](t, candidates)
	if candidates.Code != http.StatusOK || len(items) != 1 || !items[0].Eligible {
		t.Fatalf("unexpected catalog candidates: status=%d data=%#v", candidates.Code, items)
	}

	publishBody := `{"category":"marketing","status":"published","featured":true,"revision":0}`
	published := request(t, handler, http.MethodPut, "/v1/internal/organizations/"+testOrganizationID+"/apps/"+app.ID+"/catalog-listing", publishBody, testOrganizationID, domain.RoleOwner, "catalog-publish-0001")
	listing := decodeData[domain.AppCatalogListing](t, published)
	if published.Code != http.StatusOK || listing.Status != domain.CatalogListingStatusPublished || listing.Revision != 1 || listing.PublishedAt == nil {
		t.Fatalf("publish catalog listing status=%d data=%#v", published.Code, listing)
	}
	releasedResponse := request(t, handler, http.MethodGet, "/v1/apps/"+app.ID, "", testOrganizationID, domain.RoleAnalyst, "")
	if released := decodeData[domain.App](t, releasedResponse); releasedResponse.Code != http.StatusOK || released.ReleaseStatus != domain.AppReleaseStatusReleased {
		t.Fatalf("published app release status=%d data=%#v", releasedResponse.Code, released)
	}

	publicAfter := request(t, handler, http.MethodGet, "/v1/catalog/apps?featured=true", "", "", "", "")
	catalog := decodeData[[]domain.CatalogApp](t, publicAfter)
	if publicAfter.Code != http.StatusOK || len(catalog) != 1 || catalog[0].AppID != app.ID || catalog[0].LaunchURL == "" || catalog[0].Version != "1.0.0" {
		t.Fatalf("published app missing from catalog: status=%d data=%#v", publicAfter.Code, catalog)
	}

	hideBody := `{"category":"marketing","status":"hidden","featured":false,"revision":1}`
	hidden := request(t, handler, http.MethodPut, "/v1/internal/organizations/"+testOrganizationID+"/apps/"+app.ID+"/catalog-listing", hideBody, testOrganizationID, domain.RoleOwner, "catalog-hide-0001")
	if hidden.Code != http.StatusOK || decodeData[domain.AppCatalogListing](t, hidden).Status != domain.CatalogListingStatusHidden {
		t.Fatalf("hide catalog listing status=%d body=%s", hidden.Code, hidden.Body.String())
	}
	publicHidden := request(t, handler, http.MethodGet, "/v1/catalog/apps", "", "", "", "")
	if len(decodeData[[]domain.CatalogApp](t, publicHidden)) != 0 {
		t.Fatalf("hidden app remained public: %s", publicHidden.Body.String())
	}
}

func TestEmisellBackendOneTimeMerchantSessionBridge(t *testing.T) {
	t.Parallel()
	handler, _ := testServer()

	grantRequest := httptest.NewRequest(http.MethodPost, "/v1/integrations/emisell/merchant-session-grants", strings.NewReader(`{"returnTo":"/merchant/app-store"}`))
	grantRequest.Header.Set("Authorization", "Bearer emisell-backend-test-token")
	grantRequest.Header.Set("Content-Type", "application/json")
	grantRequest.Header.Set("X-Emisell-Subject", "11111111-1111-7111-8111-111111111111")
	grantRequest.Header.Set("X-Emisell-Token-Id", "bridge-request-00000001")
	grantRequest.Header.Set("X-Emisell-Email", "owner@example.com")
	grantRequest.Header.Set("X-Emisell-Display-Name", "Store Owner")
	grantRequest.Header.Set("X-Emisell-Store-Id", "22222222-2222-7222-8222-222222222222")
	grantRequest.Header.Set("X-Emisell-Store-Name", "Production Store")
	grantRequest.Header.Set("X-Emisell-Store-Domain", "store.example.com")
	grantRequest.Header.Set("X-Emisell-Environment", "production")
	grantRequest.Header.Set("X-Emisell-Permissions", "apps.install")
	grantResponse := httptest.NewRecorder()
	handler.ServeHTTP(grantResponse, grantRequest)
	if grantResponse.Code != http.StatusCreated {
		t.Fatalf("create merchant session grant status=%d body=%s", grantResponse.Code, grantResponse.Body.String())
	}
	var payload struct {
		Data struct {
			ExchangeURL string `json:"exchangeUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(grantResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode grant response: %v", err)
	}
	exchangeURL, err := url.Parse(payload.Data.ExchangeURL)
	if err != nil || exchangeURL.Query().Get("code") == "" {
		t.Fatalf("invalid exchange URL %q: %v", payload.Data.ExchangeURL, err)
	}

	exchangeRequest := httptest.NewRequest(http.MethodGet, exchangeURL.RequestURI(), nil)
	exchangeResponse := httptest.NewRecorder()
	handler.ServeHTTP(exchangeResponse, exchangeRequest)
	if exchangeResponse.Code != http.StatusSeeOther || exchangeResponse.Header().Get("Location") != "http://localhost:3003/merchant/app-store" {
		t.Fatalf("exchange status=%d location=%q body=%s", exchangeResponse.Code, exchangeResponse.Header().Get("Location"), exchangeResponse.Body.String())
	}
	cookies := exchangeResponse.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("exchange cookies=%d, want 2", len(cookies))
	}
	sessionResponse := merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/session", "", cookies, "", "")
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), `"environment":"production"`) || !strings.Contains(sessionResponse.Body.String(), `"authenticationMethod":"merchant_session"`) {
		t.Fatalf("merchant session status=%d body=%s", sessionResponse.Code, sessionResponse.Body.String())
	}

	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, httptest.NewRequest(http.MethodGet, exchangeURL.RequestURI(), nil))
	if replayResponse.Code != http.StatusBadRequest {
		t.Fatalf("grant replay status=%d, want 400: %s", replayResponse.Code, replayResponse.Body.String())
	}

	secondGrantRequest := httptest.NewRequest(http.MethodPost, "/v1/integrations/emisell/merchant-session-grants", strings.NewReader(`{"returnTo":"/merchant/apps"}`))
	for name, values := range grantRequest.Header {
		for _, value := range values {
			secondGrantRequest.Header.Add(name, value)
		}
	}
	secondGrantRequest.Header.Set("X-Emisell-Token-Id", "bridge-request-00000002")
	secondGrantRequest.Header.Set("X-Emisell-Store-Id", "33333333-3333-7333-8333-333333333333")
	secondGrantRequest.Header.Set("X-Emisell-Store-Name", "Second Store")
	secondGrantResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondGrantResponse, secondGrantRequest)
	if secondGrantResponse.Code != http.StatusCreated {
		t.Fatalf("create second store grant status=%d body=%s", secondGrantResponse.Code, secondGrantResponse.Body.String())
	}
	if err := json.Unmarshal(secondGrantResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode second grant response: %v", err)
	}
	secondExchangeURL, _ := url.Parse(payload.Data.ExchangeURL)
	secondExchangeResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondExchangeResponse, httptest.NewRequest(http.MethodGet, secondExchangeURL.RequestURI(), nil))
	secondCookies := secondExchangeResponse.Result().Cookies()
	secondSession := merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/session", "", secondCookies, "", "")
	if secondSession.Code != http.StatusOK || !strings.Contains(secondSession.Body.String(), "33333333-3333-7333-8333-333333333333") {
		t.Fatalf("second store session status=%d body=%s", secondSession.Code, secondSession.Body.String())
	}
	firstSessionAgain := merchantSessionRequest(t, handler, http.MethodGet, "/v1/merchant/session", "", cookies, "", "")
	if firstSessionAgain.Code != http.StatusOK || !strings.Contains(firstSessionAgain.Body.String(), "22222222-2222-7222-8222-222222222222") {
		t.Fatalf("first store session lost its binding: status=%d body=%s", firstSessionAgain.Code, firstSessionAgain.Body.String())
	}
}
