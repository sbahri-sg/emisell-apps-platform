package resourceclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
)

var orderDetail = regexp.MustCompile(`^/v1/orders/([A-Za-z0-9_-]{1,128})$`)
var profileDetail = regexp.MustCompile(`^/v1/settings/shipping/profile/([A-Za-z0-9_-]{1,128})$`)
var groupingPath = regexp.MustCompile(`^/v1/(catalogs|collections)(?:/([A-Za-z0-9_-]{1,128}))?$`)
var locationPath = regexp.MustCompile(`^/v1/settings/location(?:/([A-Za-z0-9_-]{1,128}))?$`)
var inventoryPath = regexp.MustCompile(`^/v1/products(?:/([A-Za-z0-9_-]{1,128}))?$`)
var reservedOrders = map[string]bool{"draft": true, "refund": true, "returns": true, "restocks": true, "analytics": true, "customer": true, "hold-reasons": true, "export": true, "edit": true, "update": true, "bulk": true, "print": true, "fulfillment": true, "fulfill": true, "selected": true, "tags": true, "cancel": true, "order-timelines": true, "mark-as-fullfiled": true, "mark-as-delivered": true, "change-customers": true}
var orderStatuses = map[string]bool{"UNPAID": true, "PAID": true, "PENDING": true, "PROCESSING": true, "REJECTED": true, "SHIPPING": true, "FAILED": true, "REFUNDED": true, "COMPLETED": true, "CANCELED": true}

// Paths are existing Emisell business endpoints, never a wildcard proxy.
func ResourceOperation(path string, queries ...url.Values) (scope, id string) {
	if len(queries) > 1 {
		return "", ""
	}
	if len(queries) == 1 && queries[0].Has("view") {
		if len(queries[0]["view"]) != 1 || queries[0].Get("view") != "inventory" {
			return "", ""
		}
		if m := inventoryPath.FindStringSubmatch(path); m != nil {
			reserved := map[string]bool{"general-info": true, "analytics": true, "get-categories": true, "categories": true, "vendor": true, "type": true, "export": true, "import": true, "sample-csv": true, "stock": true, "recommendations": true, "first": true, "variants": true}
			if !reserved[strings.ToLower(m[1])] {
				return "read_inventory", m[1]
			}
		}
		return "", ""
	}
	if m := locationPath.FindStringSubmatch(path); m != nil && strings.ToLower(m[1]) != "countries" && strings.ToLower(m[1]) != "first" {
		return "read_locations", m[1]
	}
	if m := groupingPath.FindStringSubmatch(path); m != nil {
		reserved := strings.ToLower(m[2])
		if reserved == "first" || reserved == "bulk-delete" || reserved == "export" || reserved == "import" {
			return "", ""
		}
		return "read_" + m[1], m[2]
	}
	switch path {
	case "/v1/products":
		return ReadProducts, ""
	case "/v1/orders":
		return "read_orders", ""
	case "/v1/settings/shipping":
		return "read_shipping", ""
	}
	if m := orderDetail.FindStringSubmatch(path); m != nil && !reservedOrders[strings.ToLower(m[1])] {
		return "read_orders", m[1]
	}
	if m := profileDetail.FindStringSubmatch(path); m != nil {
		return "read_shipping", m[1]
	}
	return "", ""
}

func ValidateResourceQuery(path string, query url.Values) error {
	scope, id := ResourceOperation(path, query)
	bad := &Error{Status: 400, Code: "invalid_request"}
	if scope == "" {
		return bad
	}
	if scope == ReadProducts {
		return ValidateQuery(query, false)
	}
	if id != "" && len(query) != 0 && !(scope == "read_inventory" && len(query) == 1 && query.Get("view") == "inventory") {
		return bad
	}
	for key, values := range query {
		if len(values) != 1 || values[0] == "" {
			return bad
		}
		value := values[0]
		switch key {
		case "view":
			if scope != "read_inventory" || value != "inventory" {
				return bad
			}
		case "q":
			if (scope != "read_catalogs" && scope != "read_collections" && scope != "read_locations") || len(value) > 100 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
				return bad
			}
		case "limit":
			n, e := strconv.Atoi(value)
			if e != nil || n < 1 || n > 100 || strconv.Itoa(n) != value {
				return bad
			}
		case "cursor":
			if len(value) > 1024 {
				return bad
			}
		case "status":
			if scope != "read_orders" || !orderStatuses[value] {
				return bad
			}
		case "updatedAfter":
			if scope != "read_orders" || !utcTimestamp.MatchString(value) {
				return bad
			}
			if _, e := time.Parse(time.RFC3339Nano, value); e != nil {
				return bad
			}
		default:
			return bad
		}
	}
	return nil
}

// Reuses the same release/client/installation locks as product access. The
// operation, not caller-supplied scope, chooses a single down-scoped assertion.
func (p *Products) ReadExistingForApp(ctx context.Context, lifecycle service.Lifecycle, principal identity.ServicePrincipal, actor, installation, app, client, path string, query url.Values, requestID string) (any, error) {
	if p == nil {
		return nil, fault.Unavailable
	}
	if app == "" || client == "" {
		return nil, fault.Forbidden
	}
	if err := ValidateResourceQuery(path, query); err != nil {
		return nil, err
	}
	scope, id := ResourceOperation(path, query)
	if scope == ReadProducts {
		return p.ReadForApp(ctx, lifecycle, principal, actor, installation, app, client, query, requestID)
	}
	var result any
	err := lifecycle.WithResourceAccess(ctx, principal, actor, installation, []string{scope}, func(a domain.Access) error {
		if a.Release.AppID != app || a.Release.ResourceBinding == nil || a.Release.ResourceBinding.ClientID != client {
			return fault.Forbidden
		}
		var e error
		result, e = p.readExisting(ctx, delegation{MerchantID: a.Owner.TenantID, InstallationID: a.Installation.ID, AppID: a.Installation.AppID, Environment: p.environment}, path, scope, id, query, requestID)
		return e
	})
	return result, err
}

type OrderItem struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	SKU         *string `json:"sku"`
	VariantName *string `json:"variantName"`
	Quantity    int     `json:"quantity"`
	Price       string  `json:"price"`
	LineTotal   string  `json:"lineTotal"`
}
type Order struct {
	ID                string      `json:"id"`
	Number            int         `json:"number"`
	NumberFormat      string      `json:"numberFormat"`
	Status            string      `json:"status"`
	FulfillmentStatus string      `json:"fulfillmentStatus"`
	Subtotal          string      `json:"subtotal"`
	ShippingFee       string      `json:"shippingFee"`
	TotalTax          string      `json:"totalTax"`
	Currency          *string     `json:"currency"`
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
	Items             []OrderItem `json:"items,omitempty"`
}
type ShippingZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ShippingProfile struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	IsPrimary bool           `json:"isPrimary"`
	Zones     []ShippingZone `json:"zones,omitempty"`
}
type ShippingSettings struct {
	ID              string            `json:"id"`
	SplitShipping   bool              `json:"splitShipping"`
	EstimatedShow   string            `json:"estimatedShow"`
	FulfillmentTime string            `json:"fulfillmentTime"`
	CustomTime      *int              `json:"customTime"`
	Unit            string            `json:"unit"`
	ShopPromise     bool              `json:"shopPromise"`
	Profiles        []ShippingProfile `json:"profiles"`
}

func requiredFields(raw json.RawMessage, fields ...string) bool {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return false
	}
	for _, key := range fields {
		if len(value[key]) == 0 || (string(value[key]) == "null" && key != "currency" && key != "customTime") {
			return false
		}
	}
	return true
}
func validOrder(v Order) bool {
	return Identifier.MatchString(v.ID) && v.NumberFormat != "" && orderStatuses[v.Status] && v.FulfillmentStatus != "" && decimal.MatchString(v.Subtotal) && decimal.MatchString(v.ShippingFee) && decimal.MatchString(v.TotalTax) && !v.CreatedAt.IsZero() && !v.UpdatedAt.IsZero() && (v.Currency == nil || regexp.MustCompile(`^[A-Z]{3}$`).MatchString(*v.Currency))
}

func decodeExisting(body []byte, scope, id string, limit int) (any, error) {
	if scope == "read_inventory" || scope == "read_locations" {
		return decodeInventoryLocations(body, scope, id, limit)
	}
	if scope == "read_catalogs" || scope == "read_collections" {
		return decodeGroupings(body, scope, id, limit)
	}
	var envelope struct {
		Data json.RawMessage            `json:"data"`
		Meta map[string]json.RawMessage `json:"meta"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.Data) == 0 {
		return nil, unavailable()
	}
	var meta PageMeta
	if id == "" {
		if len(envelope.Meta["nextCursor"]) == 0 || json.Unmarshal(envelope.Meta["nextCursor"], &meta.NextCursor) != nil || (meta.NextCursor != nil && (*meta.NextCursor == "" || len(*meta.NextCursor) > 1024)) {
			return nil, unavailable()
		}
	}
	decodeOrder := func(raw json.RawMessage) (Order, error) {
		var v Order
		if !requiredFields(raw, "id", "number", "numberFormat", "status", "fulfillmentStatus", "subtotal", "shippingFee", "totalTax", "currency", "createdAt", "updatedAt") || json.Unmarshal(raw, &v) != nil || !validOrder(v) {
			return v, unavailable()
		}
		return v, nil
	}
	if scope == "read_orders" {
		if id != "" {
			v, e := decodeOrder(envelope.Data)
			if e != nil || v.ID != id || !requiredFields(envelope.Data, "items") || v.Items == nil || len(v.Items) > 100 {
				return nil, unavailable()
			}
			var rawItems struct {
				Items []json.RawMessage `json:"items"`
			}
			if json.Unmarshal(envelope.Data, &rawItems) != nil {
				return nil, unavailable()
			}
			for i, item := range v.Items {
				if !requiredFields(rawItems.Items[i], "id", "quantity", "price", "lineTotal") || !Identifier.MatchString(item.ID) || item.Quantity < 0 || !decimal.MatchString(item.Price) || !decimal.MatchString(item.LineTotal) {
					return nil, unavailable()
				}
			}
			// Preserve an empty detail items array (the summary intentionally omits it).
			return struct {
				Data any `json:"data"`
			}{struct {
				Order
				Items []OrderItem `json:"items"`
			}{v, v.Items}}, nil
		}
		var raw []json.RawMessage
		if json.Unmarshal(envelope.Data, &raw) != nil || raw == nil || len(raw) > limit {
			return nil, unavailable()
		}
		values := make([]Order, 0, len(raw))
		for _, item := range raw {
			v, e := decodeOrder(item)
			if e != nil {
				return nil, e
			}
			v.Items = nil
			values = append(values, v)
		}
		return struct {
			Data []Order  `json:"data"`
			Meta PageMeta `json:"meta"`
		}{values, meta}, nil
	}
	if id != "" {
		var v ShippingProfile
		if !requiredFields(envelope.Data, "id", "name", "isPrimary", "zones") || json.Unmarshal(envelope.Data, &v) != nil || v.ID != id || v.Name == "" || v.Zones == nil || len(v.Zones) > 100 {
			return nil, unavailable()
		}
		for _, zone := range v.Zones {
			if !Identifier.MatchString(zone.ID) || zone.Name == "" {
				return nil, unavailable()
			}
		}
		return struct {
			Data any `json:"data"`
		}{struct {
			ShippingProfile
			Zones []ShippingZone `json:"zones"`
		}{v, v.Zones}}, nil
	}
	if string(envelope.Data) == "null" {
		if meta.NextCursor != nil {
			return nil, unavailable()
		}
		return struct {
			Data any      `json:"data"`
			Meta PageMeta `json:"meta"`
		}{nil, meta}, nil
	}
	var v ShippingSettings
	if !requiredFields(envelope.Data, "id", "splitShipping", "estimatedShow", "fulfillmentTime", "customTime", "unit", "shopPromise", "profiles") || json.Unmarshal(envelope.Data, &v) != nil || !Identifier.MatchString(v.ID) || v.EstimatedShow == "" || v.FulfillmentTime == "" || v.Unit == "" || v.Profiles == nil || len(v.Profiles) > limit {
		return nil, unavailable()
	}
	var rawProfiles struct {
		Profiles []json.RawMessage `json:"profiles"`
	}
	if json.Unmarshal(envelope.Data, &rawProfiles) != nil {
		return nil, unavailable()
	}
	for i, profile := range v.Profiles {
		if !requiredFields(rawProfiles.Profiles[i], "id", "name", "isPrimary") || !Identifier.MatchString(profile.ID) || profile.Name == "" {
			return nil, unavailable()
		}
		v.Profiles[i].Zones = nil
	}
	return struct {
		Data ShippingSettings `json:"data"`
		Meta PageMeta         `json:"meta"`
	}{v, meta}, nil
}

func (p *Products) readExisting(ctx context.Context, a delegation, path, scope, id string, query url.Values, requestID string) (any, error) {
	actual, actualID := ResourceOperation(path, query)
	if actual != scope || actualID != id || ValidateResourceQuery(path, query) != nil || !correlationID.MatchString(requestID) {
		return nil, unavailable()
	}
	assertion, e := p.scopedAssertion(a, scope)
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, p.origin+path+"?"+query.Encode(), nil)
	if e != nil {
		return nil, unavailable()
	}
	req.Header.Set("Authorization", "Bearer "+assertion)
	req.Header.Set("X-Emisell-Merchant-ID", a.MerchantID)
	req.Header.Set("X-Emisell-Installation-ID", a.InstallationID)
	req.Header.Set("X-Request-ID", requestID)
	req.Header.Set("X-Emisell-App-Access", "resource-v1")
	req.Header.Set("Accept", "application/json")
	res, e := p.client.Do(req)
	if e != nil {
		return nil, unavailable()
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == 404 && id != "" {
			return nil, &Error{Status: 404, Code: "not_found"}
		}
		return nil, unavailable()
	}
	if values := res.Header.Values("X-Emisell-App-Access"); len(values) != 1 || values[0] != "resource-v1" {
		return nil, unavailable()
	}
	body, e := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if e != nil || len(body) > 4<<20 {
		return nil, unavailable()
	}
	limit := 50
	if query.Get("limit") != "" {
		limit, _ = strconv.Atoi(query.Get("limit"))
	}
	return decodeExisting(body, scope, id, limit)
}
