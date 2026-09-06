// Package kurir adapts the API-Kurir merchant gateway to the local shipping
// reference. It cannot connect to production or accept real service credentials.
package kurir

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/localhttp"
	"emisell.app/platform/pkg/appmanifest"
)

const FixtureKey = "kurir-local-fixture-only-NOT-A-SERVICE-KEY"
const FixtureHeader = "X-Emisell-Kurir-Fixture"

var identity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var serviceCode = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

type Client struct {
	origin string
	http   *http.Client
	zones  map[string]int
	slots  chan struct{}
}

// NewLocalFixture takes a server-owned mapping from neutral zones to API-Kurir
// district IDs. No URL, key, provider or credential ID is accepted from callers.
func NewLocalFixture(origin string, zones map[string]int) (*Client, error) {
	h, err := localhttp.Client(origin)
	if err != nil {
		return nil, fault.Invalid
	}
	u, _ := url.Parse(origin)
	switch u.Port() {
	case "3000", "4317", "4318", "4319", "8087", "8088":
		return nil, fault.Invalid
	}
	if len(zones) == 0 || len(zones) > 1000 {
		return nil, fault.Invalid
	}
	for zone, district := range zones {
		if !identity.MatchString(zone) || district < 1 || district > 1_000_000_000 {
			return nil, fault.Invalid
		}
	}
	return &Client{origin: origin, http: h, zones: maps.Clone(zones), slots: make(chan struct{}, 4)}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

// Ready never activates a provider or changes credentials. The app wraps the
// existing API-Kurir account; the provider selection remains owned by API-Kurir.
func (c *Client) Ready(ctx context.Context, merchant, installation string) error {
	if !identity.MatchString(merchant) || !identity.MatchString(installation) {
		return fault.Invalid
	}
	var result struct {
		Data *struct {
			ActiveProviderCode *string `json:"active_provider_code"`
			Version            *int64  `json:"version"`
			Providers          []struct {
				Code      string `json:"code"`
				Available bool   `json:"available"`
				Installed bool   `json:"installed"`
				Active    bool   `json:"active"`
			} `json:"providers"`
		} `json:"data"`
	}
	if err := c.call(ctx, merchant, http.MethodGet, "/api/v1/integrations/providers", nil, &result); err != nil {
		return err
	}
	if result.Data == nil || result.Data.Version == nil || *result.Data.Version < 0 || len(result.Data.Providers) > 100 {
		return fault.Unavailable
	}
	if result.Data.ActiveProviderCode == nil {
		return fault.Conflict
	}
	active := 0
	seen := make(map[string]bool)
	for _, p := range result.Data.Providers {
		if !serviceCode.MatchString(p.Code) || seen[p.Code] {
			return fault.Unavailable
		}
		seen[p.Code] = true
		if p.Active {
			if p.Code != *result.Data.ActiveProviderCode || !p.Available || !p.Installed {
				return fault.Conflict
			}
			active++
		}
	}
	if active != 1 {
		return fault.Conflict
	}
	return nil
}

func (c *Client) Execute(ctx context.Context, merchant, actor, installation, cap, key string, r capability.Request) (*capability.Resource, []capability.Rate, error) {
	if cap != "shipping/v1" || r.Operation != "get_rates" {
		return nil, nil, fault.Forbidden
	}
	origin, originOK := c.zones[r.OriginZone]
	destination, destinationOK := c.zones[r.DestinationZone]
	if !identity.MatchString(merchant) || !identity.MatchString(actor) || !identity.MatchString(installation) || !identity.MatchString(key) || !originOK || !destinationOK || r.WeightGrams < 1 || r.WeightGrams > 1_000_000 || r.ResourceID != "" || r.Reference != "" || r.AmountMinor != 0 || r.Currency != "" {
		return nil, nil, fault.Invalid
	}
	// Cek ongkir only, never shipping-quotes/shipments. Canonical identifiers are
	// requested explicitly; raw provider service codes are never substituted.
	form := url.Values{"origin": {strconv.Itoa(origin)}, "destination": {strconv.Itoa(destination)}, "weight": {strconv.Itoa(r.WeightGrams)}, "include_group": {"true"}}
	var result struct {
		Meta struct {
			Code   int    `json:"code"`
			Status string `json:"status"`
		} `json:"meta"`
		Data *[]struct {
			Code             string `json:"code"`
			CanonicalService string `json:"canonical_service"`
			Cost             *int64 `json:"cost"`
		} `json:"data"`
	}
	if err := c.call(ctx, merchant, http.MethodPost, "/api/v1/calculate/district/domestic-cost", form, &result); err != nil {
		return nil, nil, err
	}
	if result.Meta.Code != 200 || result.Meta.Status != "success" || result.Data == nil || len(*result.Data) > 100 {
		return nil, nil, fault.Unavailable
	}
	rates := make([]capability.Rate, 0, len(*result.Data))
	seen := make(map[string]bool)
	for _, item := range *result.Data {
		code := item.Code + ":" + item.CanonicalService
		if !serviceCode.MatchString(item.Code) || !serviceCode.MatchString(item.CanonicalService) || item.Cost == nil || *item.Cost < 0 || *item.Cost > 1_000_000_000_000 || seen[code] {
			return nil, nil, fault.Unavailable
		}
		seen[code] = true
		rates = append(rates, capability.Rate{Service: code, AmountMinor: *item.Cost, Currency: "IDR"})
	}
	return nil, rates, nil
}

func (c *Client) call(ctx context.Context, merchant, method, path string, form url.Values, target any) error {
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return fault.Unavailable
	default:
		return fault.Unavailable
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r, err := http.NewRequestWithContext(ctx, method, c.origin+path, body)
	if err != nil {
		return fault.Unavailable
	}
	r.Header.Set("key", FixtureKey)
	r.Header.Set("X-Emisell-Merchant-ID", merchant)
	r.Header.Set(FixtureHeader, appmanifest.KurirFixtureProfile)
	r.Header.Set("Accept", "application/json")
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := c.http.Do(r)
	if err != nil {
		return fault.Unavailable
	}
	defer response.Body.Close()
	// Deliberate handshake marker prevents treating an unrelated local API as a
	// simulator. This marker is not production authentication or origin proof.
	if response.Header.Get(FixtureHeader) != appmanifest.KurirFixtureProfile {
		return fault.Unavailable
	}
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return fault.Invalid
	case http.StatusConflict, http.StatusUnprocessableEntity:
		return fault.Conflict
	default:
		return fault.Unavailable
	}
	decoder := json.NewDecoder(response.Body)
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return fault.Unavailable
	}
	return nil
}
