package apikurir

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

const testServiceKey = "api-kurir-sandbox-service-key-000001"

func TestCalculateUsesMerchantGatewayWithoutProviderSelectors(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != calculatePath || request.URL.RawQuery != "" ||
			request.Header.Get("key") != testServiceKey || request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" ||
			request.Header.Get("X-Emisell-Merchant-ID") != "merchant.sandbox:1" || request.Header.Get("X-Emisell-Execution-Mode") != "sandbox" ||
			request.Header.Get("X-Request-ID") != "request-shipping-000001" {
			t.Fatalf("unsafe API Kurir request: %s %#v", request.URL, request.Header)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("origin") != "442" || request.Form.Get("destination") != "1354" || request.Form.Get("weight") != "1200" || request.Form.Get("include_group") != "true" {
			t.Fatalf("unexpected form: %#v", request.Form)
		}
		for _, forbidden := range []string{"courier", "merchantId", "credential_id", "installation_id", "extension_id", "runtime_url"} {
			if request.Form.Has(forbidden) {
				t.Fatalf("caller/provider selector leaked: %s", forbidden)
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"meta":{"message":"Success Calculate Domestic Shipping cost","code":200,"status":"success"},"data":[{"name":"JNE","code":"jne","logo":"https://assets.example.test/jne.webp?v=1","service":"JTR>130","canonical_service":"JTR","service_group":"cargo","service_type":"cargo","description":"JNE Trucking","cost":75000,"etd":"3-4 day"}]}`))
	}))
	defer peer.Close()
	client, err := NewRates(Options{Origin: peer.URL, ServiceKey: testServiceKey, AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Calculate(t.Context(), "merchant.sandbox:1", domain.EnvironmentSandbox, "request-shipping-000001", application.ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 1200})
	if err != nil || len(result.Data) != 1 || result.Data[0].CanonicalService != "JTR" || result.Data[0].Cost != 75000 {
		t.Fatalf("unexpected result=%#v err=%v", result, err)
	}
}

func TestCalculateMapsUpstreamFailuresWithoutLeakingDetails(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		body       string
		retry      string
		wantStatus int
		wantCode   string
	}{
		{"bad request", 400, errorBody(400), "", 400, "invalid_rate_request"},
		{"shipping disabled", 409, errorBody(409), "", 409, "shipping_disabled"},
		{"not available", 422, errorBody(422), "", 422, "rate_not_available"},
		{"rate limited", 429, errorBody(429), "999", 429, "shipping_rate_limited"},
		{"provider auth", 401, errorBody(401), "", 502, "shipping_provider_unavailable"},
		{"provider", 502, errorBody(502), "", 502, "shipping_provider_unavailable"},
		{"quota", 503, errorBody(503), "", 503, "provider_quota_exhausted"},
		{"redirect", 302, "", "", 503, "shipping_runtime_unavailable"},
		{"malformed error", 500, "private database detail", "", 503, "shipping_runtime_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Location", "https://attacker.invalid/")
				writer.Header().Set("Retry-After", test.retry)
				writer.WriteHeader(test.status)
				writer.Write([]byte(test.body))
			}))
			defer peer.Close()
			client, err := NewRates(Options{Origin: peer.URL, ServiceKey: testServiceKey, AllowHTTP: true})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Calculate(t.Context(), "merchant-a", domain.EnvironmentSandbox, "request-shipping-000001", application.ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 1000})
			var rateError *application.ShippingRateError
			if !errors.As(err, &rateError) || rateError.Status != test.wantStatus || rateError.Code != test.wantCode || strings.Contains(err.Error(), "private") {
				t.Fatalf("unexpected error: %#v", err)
			}
			if test.status == 429 && rateError.RetryAfter != 60 {
				t.Fatalf("unsafe Retry-After: %d", rateError.RetryAfter)
			}
		})
	}
}

func TestCalculateRejectsMalformedSuccessAndUnsafeConfiguration(t *testing.T) {
	for _, body := range []string{
		`not-json`,
		`{"meta":{"message":"ok","code":200,"status":"success"}}`,
		`{"meta":{"message":"ok","code":200,"status":"success"},"data":[{"name":"JNE","code":"jne","logo":"https://assets.example.test/jne.webp","service":"REG","description":"Regular","cost":1,"etd":"1 day"}]}`,
		`{"meta":{"message":"ok","code":200,"status":"success"},"data":[{"name":"JNE","code":"jne","logo":"javascript:alert(1)","service":"REG","canonical_service":"REG","service_group":"regular","service_type":"parcel","description":"Regular","cost":1,"etd":"1 day"}]}`,
		`{"meta":{"message":"ok","code":200,"status":"success"},"data":[{"name":"JNE","code":"jne","logo":"https://assets.example.test/jne.webp","service":"REG","canonical_service":"REG","service_group":"regular","service_type":"parcel","description":"Regular","cost":-1,"etd":"1 day"}]}`,
	} {
		peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.Write([]byte(body)) }))
		client, err := NewRates(Options{Origin: peer.URL, ServiceKey: testServiceKey, AllowHTTP: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Calculate(t.Context(), "merchant-a", domain.EnvironmentSandbox, "request-shipping-000001", application.ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 1000})
		peer.Close()
		var rateError *application.ShippingRateError
		if !errors.As(err, &rateError) || rateError.Code != "shipping_runtime_unavailable" {
			t.Fatalf("malformed success accepted: %s err=%v", body, err)
		}
	}
	for _, options := range []Options{
		{Origin: "http://api-kurir.example.test", ServiceKey: testServiceKey},
		{Origin: "https://user:pass@api-kurir.example.test", ServiceKey: testServiceKey},
		{Origin: "https://api-kurir.example.test/admin", ServiceKey: testServiceKey},
		{Origin: "https://api-kurir.example.test?target=other", ServiceKey: testServiceKey},
		{Origin: "file:///tmp/socket", ServiceKey: testServiceKey},
		{Origin: "https://api-kurir.example.test", ServiceKey: "short"},
		{Origin: "https://api-kurir.example.test", ServiceKey: testServiceKey, Timeout: 6 * time.Second},
	} {
		if _, err := NewRates(options); err == nil {
			t.Fatalf("unsafe options accepted: %#v", options)
		}
	}
}

func errorBody(status int) string {
	return `{"meta":{"message":"MUST-NOT-LEAK","code":` + strconv.Itoa(status) + `,"status":"error"},"data":null}`
}
