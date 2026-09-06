package kurir

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
)

const readyJSON = `{"data":{"active_provider_code":"rajaongkir","version":1,"providers":[{"code":"rajaongkir","active":true,"installed":true,"available":true}]}}`
const ratesJSON = `{"meta":{"code":200,"status":"success"},"data":[{"code":"jne","canonical_service":"REG","cost":12000,"service":"provider-private-code"}]}`

func fixtureClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	c, err := NewLocalFixture(s.URL, map[string]int{"origin": 442, "destination": 1354})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func rateRequest() capability.Request {
	return capability.Request{Operation: "get_rates", OriginZone: "origin", DestinationZone: "destination", WeightGrams: 1200}
}

func TestLocalFixtureRejectsProductionAndMutableConfiguration(t *testing.T) {
	for _, address := range []string{"https://api-kurir.emisell.com", "http://localhost:9090", "http://127.0.0.1:4317", "http://127.0.0.1:8087", "http://127.0.0.1:9090/path", "http://user:secret@127.0.0.1:9090", "http://127.0.0.1:9090?key=value", "http://[::1]:9090"} {
		if _, err := NewLocalFixture(address, map[string]int{"origin": 442}); err == nil {
			t.Fatal("accepted non-fixture address", address)
		}
	}
	for _, zones := range []map[string]int{nil, {"bad zone": 1}, {"zone": 0}} {
		if _, err := NewLocalFixture("http://127.0.0.1:9090", zones); err == nil {
			t.Fatal("invalid zone map accepted")
		}
	}
	zones := map[string]int{"origin": 442}
	c, err := NewLocalFixture("http://127.0.0.1:9090", zones)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	zones["origin"] = 1
	if c.zones["origin"] != 442 {
		t.Fatal("configuration retained caller-owned map")
	}
}

func TestGatewayPayloadAndCanonicalRates(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("key") != FixtureKey || r.Header.Get("X-Emisell-Merchant-ID") != "merchant_1" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get(FixtureHeader) != appmanifest.KurirFixtureProfile {
			t.Error("wrong authentication boundary")
		}
		w.Header().Set(FixtureHeader, appmanifest.KurirFixtureProfile)
		switch r.URL.Path {
		case "/api/v1/integrations/providers":
			if r.Method != "GET" {
				t.Error("readiness must not mutate")
			}
			_, _ = w.Write([]byte(readyJSON))
		case "/api/v1/calculate/district/domestic-cost":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.ParseForm() != nil || len(r.PostForm) != 4 || r.PostForm.Get("origin") != "442" || r.PostForm.Get("destination") != "1354" || r.PostForm.Get("weight") != "1200" || r.PostForm.Get("include_group") != "true" {
				t.Error("wrong rates contract")
			}
			_, _ = w.Write([]byte(ratesJSON))
		default:
			t.Error("unexpected endpoint", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	if err := c.Ready(context.Background(), "merchant_1", "ins_1"); err != nil {
		t.Fatal(err)
	}
	resource, rates, err := c.Execute(context.Background(), "merchant_1", "staff", "ins_1", "shipping/v1", "request_1234567890", rateRequest())
	if err != nil || resource != nil || len(rates) != 1 || rates[0].Service != "jne:REG" || rates[0].AmountMinor != 12000 || rates[0].Currency != "IDR" {
		t.Fatal("invalid normalized rates", rates, err)
	}
}

func TestInvalidOperationsNeverReachGateway(t *testing.T) {
	var hits atomic.Int64
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(500) })
	for _, r := range []capability.Request{
		{Operation: "create"}, {Operation: "track"}, {Operation: "get_rates", OriginZone: "origin", DestinationZone: "destination"},
		{Operation: "get_rates", OriginZone: "unknown", DestinationZone: "destination", WeightGrams: 100},
		{Operation: "get_rates", OriginZone: "origin", DestinationZone: "destination", WeightGrams: 100, ResourceID: "shipment"},
	} {
		if _, _, err := c.Execute(context.Background(), "merchant", "staff", "ins", "shipping/v1", "request", r); err == nil {
			t.Fatal("invalid operation accepted")
		}
	}
	if err := c.Ready(context.Background(), "bad\r\nmerchant", "ins"); err == nil {
		t.Fatal("invalid merchant accepted")
	}
	if hits.Load() != 0 {
		t.Fatal("invalid input reached upstream")
	}
}

func TestFailClosedGatewayResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		ready  bool
		want   error
	}{
		{"missing data", 200, `{}`, true, fault.Unavailable},
		{"not configured", 200, `{"data":{"version":0,"active_provider_code":null,"providers":[]}}`, true, fault.Conflict},
		{"not installed", 200, strings.Replace(readyJSON, `"installed":true`, `"installed":false`, 1), true, fault.Conflict},
		{"wrong active", 200, strings.Replace(readyJSON, `"active_provider_code":"rajaongkir"`, `"active_provider_code":"other"`, 1), true, fault.Conflict},
		{"duplicate JSON", 200, readyJSON + readyJSON, true, fault.Unavailable},
		{"invalid JSON", 200, `{`, true, fault.Unavailable},
		{"oversized", 200, strings.Repeat(" ", 33<<10) + readyJSON, true, fault.Unavailable},
		{"bad request", 400, `{"secret":"must-not-leak"}`, true, fault.Invalid},
		{"credential rejected", 401, `{"secret":"must-not-leak"}`, true, fault.Unavailable},
		{"disabled", 409, `{}`, true, fault.Conflict},
		{"missing credential", 422, `{}`, true, fault.Conflict},
		{"throttled", 429, `{}`, true, fault.Unavailable},
		{"unavailable", 503, `{}`, true, fault.Unavailable},
		{"negative price", 200, strings.Replace(ratesJSON, `12000`, `-1`, 1), false, fault.Unavailable},
		{"fractional price", 200, strings.Replace(ratesJSON, `12000`, `12.5`, 1), false, fault.Unavailable},
		{"no canonical ID", 200, strings.Replace(ratesJSON, `"canonical_service":"REG",`, ``, 1), false, fault.Unavailable},
		{"null rates", 200, `{"meta":{"code":200,"status":"success"},"data":null}`, false, fault.Unavailable},
		{"empty rates", 200, `{"meta":{"code":200,"status":"success"},"data":[]}`, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set(FixtureHeader, appmanifest.KurirFixtureProfile)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			var err error
			if tc.ready {
				err = c.Ready(context.Background(), "merchant", "ins")
			} else {
				_, _, err = c.Execute(context.Background(), "merchant", "staff", "ins", "shipping/v1", "request", rateRequest())
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if calls.Load() != 1 {
				t.Fatal("hidden retry")
			}
		})
	}
}

func TestNoRedirectProxyOrNonFixtureResponse(t *testing.T) {
	var escaped atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { escaped.Add(1) }))
	defer target.Close()
	t.Setenv("HTTP_PROXY", target.URL)
	t.Setenv("HTTPS_PROXY", target.URL)
	for _, redirect := range []bool{true, false} {
		c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
			if redirect {
				w.Header().Set(FixtureHeader, appmanifest.KurirFixtureProfile)
				http.Redirect(w, r, target.URL, 307)
				return
			}
			_, _ = w.Write([]byte(readyJSON))
		})
		if err := c.Ready(context.Background(), "merchant", "ins"); !errors.Is(err, fault.Unavailable) {
			t.Fatal("unsafe target accepted", err)
		}
	}
	if escaped.Load() != 0 {
		t.Fatal("credential escaped configured origin")
	}
}

func TestCancellationAndConcurrencyBound(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := c.Ready(ctx, "merchant", "ins"); !errors.Is(err, fault.Unavailable) {
		t.Fatal(err)
	}
	for range 4 {
		c.slots <- struct{}{}
	}
	if err := c.Ready(context.Background(), "merchant", "ins"); !errors.Is(err, fault.Unavailable) {
		t.Fatal("concurrency limit ignored")
	}
	for range 4 {
		<-c.slots
	}
}
