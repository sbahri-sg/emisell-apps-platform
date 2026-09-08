package resourceclient

import "encoding/json"

type Location struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	IsPrimary            bool   `json:"isPrimary"`
	IsActive             bool   `json:"isActive"`
	IsPhysicalStorefront bool   `json:"isPhysicalStorefront"`
	IsFulfillment        bool   `json:"isFulfillment"`
	ShippingIsActive     bool   `json:"shippingIsActive"`
	DeliveryIsActive     bool   `json:"deliveryIsActive"`
	PickUpIsActive       bool   `json:"pickUpIsActive"`
}
type InventoryLevel struct {
	LocationID string `json:"locationId"`
	Available  int64  `json:"available"`
}
type InventoryItem struct {
	ID     string           `json:"id"`
	Type   string           `json:"type"`
	Stock  int64            `json:"stock"`
	Levels []InventoryLevel `json:"levels"`
}
type InventoryProduct struct {
	ID                            string          `json:"id"`
	TrackInventory                bool            `json:"trackInventory"`
	ContinueSellingWhenOutOfStock bool            `json:"continueSellingWhenOutOfStock"`
	Stock                         int64           `json:"stock"`
	Items                         []InventoryItem `json:"items"`
}

func decodeInventoryLocations(body []byte, scope, id string, limit int) (any, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
		Meta json.RawMessage `json:"meta"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return nil, unavailable()
	}
	decode := func(raw json.RawMessage) (any, error) {
		if scope == "read_locations" {
			var v Location
			if !requiredFields(raw, "id", "name", "isPrimary", "isActive", "isPhysicalStorefront", "isFulfillment", "shippingIsActive", "deliveryIsActive", "pickUpIsActive") || json.Unmarshal(raw, &v) != nil || !Identifier.MatchString(v.ID) || v.Name == "" || (id != "" && v.ID != id) {
				return nil, unavailable()
			}
			return v, nil
		}
		var v InventoryProduct
		var nested struct {
			Items []json.RawMessage `json:"items"`
		}
		if !requiredFields(raw, "id", "trackInventory", "continueSellingWhenOutOfStock", "stock", "items") || json.Unmarshal(raw, &v) != nil || json.Unmarshal(raw, &nested) != nil || !Identifier.MatchString(v.ID) || (id != "" && v.ID != id) || len(v.Items) == 0 || len(v.Items) > 100 {
			return nil, unavailable()
		}
		var total int64
		count := 0
		seen := map[string]bool{}
		for i, item := range v.Items {
			var n struct {
				Levels []json.RawMessage `json:"levels"`
			}
			if !requiredFields(nested.Items[i], "id", "type", "stock", "levels") || json.Unmarshal(nested.Items[i], &n) != nil || !Identifier.MatchString(item.ID) || seen[item.ID] || item.Levels == nil || (item.Type != "PRODUCT" && item.Type != "VARIANT") || (item.Type == "PRODUCT" && (item.ID != v.ID || len(v.Items) != 1)) {
				return nil, unavailable()
			}
			seen[item.ID] = true
			locations := map[string]bool{}
			var sum int64
			for j, level := range item.Levels {
				if !requiredFields(n.Levels[j], "locationId", "available") || !Identifier.MatchString(level.LocationID) || locations[level.LocationID] || level.Available < -2147483648 || level.Available > 2147483647 {
					return nil, unavailable()
				}
				count++
				if count > 100 {
					return nil, unavailable()
				}
				locations[level.LocationID] = true
				sum += level.Available
			}
			if sum != item.Stock {
				return nil, unavailable()
			}
			total += sum
		}
		if total != v.Stock {
			return nil, unavailable()
		}
		return v, nil
	}
	if id != "" {
		v, e := decode(envelope.Data)
		if e != nil {
			return nil, e
		}
		return struct {
			Data any `json:"data"`
		}{v}, nil
	}
	var rows []json.RawMessage
	var meta PageMeta
	var fields map[string]json.RawMessage
	if json.Unmarshal(envelope.Data, &rows) != nil || rows == nil || len(rows) > limit || json.Unmarshal(envelope.Meta, &fields) != nil || len(fields["nextCursor"]) == 0 || json.Unmarshal(envelope.Meta, &meta) != nil || (meta.NextCursor != nil && (*meta.NextCursor == "" || len(*meta.NextCursor) > 1024)) {
		return nil, unavailable()
	}
	values := make([]any, 0, len(rows))
	for _, row := range rows {
		v, e := decode(row)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return struct {
		Data []any    `json:"data"`
		Meta PageMeta `json:"meta"`
	}{values, meta}, nil
}
