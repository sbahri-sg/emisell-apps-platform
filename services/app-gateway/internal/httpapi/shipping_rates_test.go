package httpapi_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/apikurir"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

func TestEmisellShippingRateBridgeUsesInstalledCapabilityAndMerchantContext(t *testing.T) {
	upstreamCalls := 0
	peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalls++
		if request.Header.Get("X-Emisell-Merchant-ID") != "merchant-shipping-a" || request.Header.Get("X-Emisell-Execution-Mode") != "sandbox" || request.Header.Get("key") != "api-kurir-sandbox-service-key-000001" {
			t.Fatalf("untrusted context forwarded: %#v", request.Header)
		}
		request.ParseForm()
		if request.Form.Get("courier") != "" || request.Form.Get("credential_id") != "" || request.Form.Get("installation_id") != "" {
			t.Fatalf("provider selector forwarded: %#v", request.Form)
		}
		writer.Write([]byte(`{"meta":{"message":"Success Calculate Domestic Shipping cost","code":200,"status":"success"},"data":[{"name":"JNE","code":"jne","logo":"https://assets.example.test/jne.webp","service":"REG","canonical_service":"REG","service_group":"regular","service_type":"parcel","description":"Regular","cost":15000,"etd":"2-3 day"}]}`))
	}))
	defer peer.Close()
	client, err := apikurir.NewRates(apikurir.Options{Origin: peer.URL, ServiceKey: "api-kurir-sandbox-service-key-000001", AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := testServerWithShippingRates(client, time.Now)
	app := postApp(t, handler, "shipping-bridge-app-0001")
	extensionResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/extensions", `{"name":"API Kurir Shipping","type":"shipping","configuration":{"capabilities":["shipping.rates.calculate"]}}`, testOrganizationID, domain.RoleDeveloper, "shipping-bridge-extension-0001")
	if extensionResponse.Code != http.StatusCreated {
		t.Fatalf("create extension: %d %s", extensionResponse.Code, extensionResponse.Body.String())
	}
	version := postVersion(t, handler, app.ID, "1.0.0", "shipping-bridge-version-0001")
	releaseVersion(t, handler, app.ID, version.ID, "shipping-bridge-release-0001", nil, http.StatusOK, domain.RoleOwner)
	installationResponse := request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/installations", `{"merchantId":"merchant-shipping-a","merchantName":"Shipping Fixture","environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "shipping-bridge-install-0001")
	if installationResponse.Code != http.StatusCreated {
		t.Fatalf("create installation: %d %s", installationResponse.Code, installationResponse.Body.String())
	}
	installation := decodeData[domain.AppInstallation](t, installationResponse)

	call := func(body, permissions string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/integrations/emisell/shipping/rates/calculate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer emisell-backend-test-token")
		req.Header.Set("X-Emisell-Store-Id", "merchant-shipping-a")
		req.Header.Set("X-Emisell-Environment", "sandbox")
		req.Header.Set("X-Emisell-Permissions", permissions)
		req.Header.Set("X-Request-Id", "request-shipping-http-0001")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	missingPermission := call(`{"origin":"442","destination":"1354","weight":1200}`, "apps.install")
	if missingPermission.Code != http.StatusForbidden || upstreamCalls != 0 {
		t.Fatalf("permission boundary failed: %d calls=%d", missingPermission.Code, upstreamCalls)
	}
	spoof := call(`{"origin":"442","destination":"1354","weight":1200,"merchantId":"merchant-b"}`, application.ShippingRatesCalculate)
	if spoof.Code != http.StatusBadRequest || upstreamCalls != 0 {
		t.Fatalf("body merchant override accepted: %d calls=%d", spoof.Code, upstreamCalls)
	}
	success := call(`{"origin":"442","destination":"1354","weight":1200}`, application.ShippingRatesCalculate)
	if success.Code != http.StatusOK || upstreamCalls != 1 || !strings.Contains(success.Body.String(), `"canonicalService":"REG"`) || strings.Contains(success.Body.String(), "installationId") {
		t.Fatalf("unexpected success: %d calls=%d body=%s", success.Code, upstreamCalls, success.Body.String())
	}

	suspend := request(t, handler, http.MethodPatch, "/v1/apps/"+app.ID+"/installations/"+installation.ID, fmt.Sprintf(`{"status":"suspended","revision":%d}`, installation.Revision), testOrganizationID, domain.RoleOwner, "")
	if suspend.Code != http.StatusOK {
		t.Fatalf("suspend: %d %s", suspend.Code, suspend.Body.String())
	}
	blocked := call(`{"origin":"442","destination":"1354","weight":1200}`, application.ShippingRatesCalculate)
	if blocked.Code != http.StatusConflict || !strings.Contains(blocked.Body.String(), "shipping_extension_unavailable") || upstreamCalls != 1 {
		t.Fatalf("suspended extension reached API Kurir: %d calls=%d body=%s", blocked.Code, upstreamCalls, blocked.Body.String())
	}
}

func TestEmisellShippingRateBridgeIsDisabledByDefault(t *testing.T) {
	handler, _ := testServer()
	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/emisell/shipping/rates/calculate", strings.NewReader(`{"origin":"442","destination":"1354","weight":1200}`))
	req.Header.Set("Authorization", "Bearer emisell-backend-test-token")
	req.Header.Set("X-Emisell-Store-Id", "merchant-a")
	req.Header.Set("X-Emisell-Environment", "sandbox")
	req.Header.Set("X-Emisell-Permissions", application.ShippingRatesCalculate)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable || !strings.Contains(res.Body.String(), "shipping_runtime_disabled") {
		t.Fatalf("default gate: %d %s", res.Code, res.Body.String())
	}
}
