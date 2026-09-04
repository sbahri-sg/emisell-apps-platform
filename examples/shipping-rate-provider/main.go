// Local fixture only. Does not call a courier, use a merchant credential,
// install an app, create a shipment, or expose a production server.
package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

type district struct {
	ID string `json:"district_id"`
}

type rateRequest struct {
	Origin        district `json:"origin"`
	Destination   district `json:"destination"`
	Weight        int64    `json:"weight_grams"`
	Couriers      []string `json:"courier_codes,omitempty"`
	ServiceGroups []string `json:"service_groups,omitempty"`
	Price         string   `json:"price,omitempty"`
}

func newHandler(key string) (http.Handler, error) {
	if len(key) < 32 || strings.TrimSpace(key) != key || strings.ContainsAny(key, "\r\n") {
		return nil, errors.New("SHIPPING_EXAMPLE_KEY must contain at least 32 characters with no surrounding whitespace")
	}
	expected := sha256.Sum256([]byte(key))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Emisell-Example", "synthetic-only")
		if r.URL.Path != "/rates" {
			fail(w, 404, "NOT_FOUND", "Only the local /rates example exists")
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			fail(w, 405, "METHOD_NOT_ALLOWED", "Use POST")
			return
		}
		actual := sha256.Sum256([]byte(r.Header.Get("key")))
		if len(r.Header.Values("key")) != 1 || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			fail(w, 401, "UNAUTHORIZED", "A valid example-only key is required")
			return
		}
		// No live fallback. Existing API Kurir defaults are deliberately not
		// copied into this teaching fixture.
		if len(r.Header.Values("X-Emisell-Execution-Mode")) != 1 || r.Header.Get("X-Emisell-Execution-Mode") != "sandbox" || r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" {
			fail(w, 403, "FIXTURE_ONLY", "Only server-side sandbox fixture requests are accepted")
			return
		}
		if r.URL.RawQuery != "" || r.Header.Get("X-Emisell-Merchant-ID") != "" || r.Header.Get("X-Merchant-Id") != "" {
			fail(w, 400, "INVALID_REQUEST", "Tenant selectors are not part of the hosted rate payload")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Use application/json")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			fail(w, 413, "BODY_TOO_LARGE", "Maximum body size is 64 KiB")
			return
		}
		if err != nil {
			fail(w, 400, "INVALID_REQUEST", "Unable to read request")
			return
		}
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.DisallowUnknownFields()
		var input rateRequest
		if err := decoder.Decode(&input); err != nil {
			fail(w, 400, "INVALID_REQUEST", "Malformed JSON, wrong field type, or unknown field")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			fail(w, 400, "INVALID_REQUEST", "Expected one JSON object")
			return
		}
		if input.Origin.ID == "" || len(input.Origin.ID) > 128 || input.Destination.ID == "" || len(input.Destination.ID) > 128 || input.Weight < 1 || input.Weight > 1000000 || len(input.Couriers) > 20 || len(input.ServiceGroups) > 4 || (input.Price != "" && input.Price != "lowest" && input.Price != "highest") {
			fail(w, 422, "INVALID_REQUEST", "Review district references, weight and filters")
			return
		}
		for _, group := range input.ServiceGroups {
			if !slices.Contains([]string{"regular", "next_day", "economy", "cargo"}, group) {
				fail(w, 422, "INVALID_REQUEST", "Unknown service group")
				return
			}
		}
		if input.Origin.ID != "fixture-origin" || input.Destination.ID != "fixture-destination" || input.Weight != 1000 || (len(input.Couriers) != 0 && (len(input.Couriers) != 1 || input.Couriers[0] != "fixture")) {
			fail(w, 422, "FIXTURE_ROUTE_ONLY", "No real locations, couriers or calculated rates are supported")
			return
		}
		quotes := []any{}
		if len(input.ServiceGroups) == 0 || slices.Contains(input.ServiceGroups, "regular") {
			quotes = append(quotes, map[string]any{
				"provider_code": "fixture_provider", "courier_code": "fixture", "courier_name": "Fixture Courier",
				"service_code": "FIXTURE_REG", "service_name": "Synthetic rate — not bookable", "service_group": "regular",
				"price": 10000, "currency": "IDR", "etd": "2-3",
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"quotes": quotes},
			"meta": map[string]any{"provider_code": "fixture_provider", "upstream": "synthetic_fixture", "product": "shipping_cost", "fixture": true},
		})
	}), nil
}

func fail(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func main() {
	handler, err := newHandler(os.Getenv("SHIPPING_EXAMPLE_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	// Intentionally fixed to loopback; do not expose this fixture as a provider.
	server := &http.Server{Addr: "127.0.0.1:3016", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
	fmt.Println("Synthetic shipping rate example: http://127.0.0.1:3016/rates (no live rates or booking)")
	log.Fatal(server.ListenAndServe())
}
