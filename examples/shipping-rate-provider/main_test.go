package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

const fixtureKey = "fixture-only-key-never-a-real-provider-key"

func contractExample(t *testing.T) (map[string]any, map[string]any) {
	t.Helper()
	data, err := os.ReadFile("../../docs/shipping-provider.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err = json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	operation := spec["paths"].(map[string]any)["/rates"].(map[string]any)["post"].(map[string]any)
	media := func(value any) map[string]any {
		return value.(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	}
	return media(operation["requestBody"])["example"].(map[string]any), media(operation["responses"].(map[string]any)["200"])["example"].(map[string]any)
}

func TestHTTPRoundTripMatchesPublishedFixtureAndHostedConsumerShape(t *testing.T) {
	handler, err := newHandler(fixtureKey)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	input, expected := contractExample(t)
	body, _ := json.Marshal(input)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/rates", bytes.NewReader(body))
	req.Header.Set("key", fixtureKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emisell-Execution-Mode", "sandbox")
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d", response.StatusCode)
	}
	encoded, _ := io.ReadAll(response.Body)
	var actual map[string]any
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("response differs from OpenAPI fixture")
	}
	// Match the actual API Kurir hosted rate consumer, not the older starter's
	// data array. This proves wire decoding, not API Kurir integration/approval.
	var consumer struct {
		Data struct {
			Quotes []struct {
				ProviderCode string `json:"provider_code"`
				CourierCode  string `json:"courier_code"`
				ServiceCode  string `json:"service_code"`
				Price        int64  `json:"price"`
				ETD          string `json:"etd"`
			} `json:"quotes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(encoded, &consumer); err != nil || len(consumer.Data.Quotes) != 1 || consumer.Data.Quotes[0].Price != 10000 {
		t.Fatal("hosted consumer cannot decode fixture")
	}
	if strings.Contains(string(encoded), fixtureKey) {
		t.Fatal("key leaked")
	}
}

func TestFixtureRejectsUnauthorizedLiveTenantOverridesAndInvalidInputs(t *testing.T) {
	input, _ := contractExample(t)
	valid, _ := json.Marshal(input)
	handler, _ := newHandler(fixtureKey)
	cases := []struct {
		name, body string
		headers    map[string]string
		status     int
	}{
		{"missing key", string(valid), map[string]string{"key": ""}, 401},
		{"wrong key", string(valid), map[string]string{"key": "incorrect"}, 401},
		{"live", string(valid), map[string]string{"X-Emisell-Execution-Mode": "live"}, 403},
		{"missing mode", string(valid), map[string]string{"X-Emisell-Execution-Mode": ""}, 403},
		{"browser", string(valid), map[string]string{"Origin": "https://untrusted.example"}, 403},
		{"tenant header", string(valid), map[string]string{"X-Emisell-Merchant-ID": "merchant-other"}, 400},
		{"tenant body", strings.TrimSuffix(string(valid), "}") + `,"merchantId":"merchant-other"}`, nil, 400},
		{"media type", string(valid), map[string]string{"Content-Type": "text/plain"}, 415},
		{"trailing object", string(valid) + `{}`, nil, 400},
		{"unknown field", `{"unknown":true}`, nil, 400},
		{"oversized", strings.Repeat(" ", 65537), nil, 413},
		{"missing fields", `{}`, nil, 422},
		{"fractional grams", strings.Replace(string(valid), "1000", "1000.5", 1), nil, 400},
		{"negative grams", strings.Replace(string(valid), "1000", "-1", 1), nil, 422},
		{"real location", strings.Replace(string(valid), "fixture-origin", "1391", 1), nil, 422},
		{"non-fixture weight", strings.Replace(string(valid), "1000", "2000", 1), nil, 422},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/rates", strings.NewReader(item.body))
			req.Header.Set("key", fixtureKey)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Emisell-Execution-Mode", "sandbox")
			for key, value := range item.headers {
				req.Header.Set(key, value)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != item.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, item.status, w.Body.String())
			}
			if strings.Contains(w.Body.String(), fixtureKey) {
				t.Fatal("key leaked")
			}
		})
	}
}

func TestFixtureEmptyFilterResultAndKeyRequirements(t *testing.T) {
	if _, err := newHandler(""); err == nil {
		t.Fatal("empty key accepted")
	}
	input, _ := contractExample(t)
	input["service_groups"] = []string{"cargo"}
	body, _ := json.Marshal(input)
	handler, _ := newHandler(fixtureKey)
	req := httptest.NewRequest(http.MethodPost, "/rates", bytes.NewReader(body))
	req.Header.Set("key", fixtureKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emisell-Execution-Mode", "sandbox")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"quotes":[]`) {
		t.Fatalf("unexpected empty result: %s", w.Body.String())
	}
}
