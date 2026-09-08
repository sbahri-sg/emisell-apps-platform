package resourceclient

import (
	"encoding/json"
	"time"
)

// Deliberately omit catalog pricing/markets and expanded product attributes.
type Catalog struct {
	ID                     string    `json:"id"`
	Title                  string    `json:"title"`
	Status                 string    `json:"status"`
	CurrencyCode           *string   `json:"currencyCode"`
	AutoIncludeNewProducts bool      `json:"autoIncludeNewProducts"`
	CreatedAt              time.Time `json:"createdAt"`
	ProductCount           int       `json:"productCount"`
}
type Collection struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ProductCount int       `json:"productCount"`
}
type CatalogProductRef struct {
	ProductID string  `json:"productId"`
	VariantID *string `json:"variantId"`
}
type CollectionProductRef struct {
	ProductID string `json:"productId"`
}

func decodeGroupings(body []byte, scope, id string, limit int) (any, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
		Meta json.RawMessage `json:"meta"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return nil, unavailable()
	}
	decode := func(raw json.RawMessage, detail bool) (any, error) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || !requiredFields(raw, "id", "createdAt", "productCount") {
			return nil, unavailable()
		}
		if scope == "read_catalogs" {
			var v Catalog
			if !requiredFields(raw, "title", "status", "autoIncludeNewProducts") || len(fields["currencyCode"]) == 0 || json.Unmarshal(raw, &v) != nil || !Identifier.MatchString(v.ID) || (id != "" && v.ID != id) || v.Title == "" || (v.Status != "ACTIVE" && v.Status != "DRAFT" && v.Status != "ARCHIVED") || v.CreatedAt.IsZero() || v.ProductCount < 0 {
				return nil, unavailable()
			}
			if !detail {
				return v, nil
			}
			var data struct {
				Products []json.RawMessage `json:"products"`
			}
			if json.Unmarshal(raw, &data) != nil || data.Products == nil || len(data.Products) > 100 || len(data.Products) != v.ProductCount {
				return nil, unavailable()
			}
			refs := make([]CatalogProductRef, 0, len(data.Products))
			for _, item := range data.Products {
				var ref CatalogProductRef
				var f map[string]json.RawMessage
				if json.Unmarshal(item, &f) != nil || !requiredFields(item, "productId") || len(f["variantId"]) == 0 || json.Unmarshal(item, &ref) != nil || !Identifier.MatchString(ref.ProductID) || (ref.VariantID != nil && !Identifier.MatchString(*ref.VariantID)) {
					return nil, unavailable()
				}
				refs = append(refs, ref)
			}
			return struct {
				Catalog
				Products []CatalogProductRef `json:"products"`
			}{v, refs}, nil
		}
		var v Collection
		if !requiredFields(raw, "name", "updatedAt") || len(fields["description"]) == 0 || json.Unmarshal(raw, &v) != nil || !Identifier.MatchString(v.ID) || (id != "" && v.ID != id) || v.Name == "" || v.CreatedAt.IsZero() || v.UpdatedAt.IsZero() || v.ProductCount < 0 {
			return nil, unavailable()
		}
		if !detail {
			return v, nil
		}
		var data struct {
			Products []CollectionProductRef `json:"products"`
		}
		if json.Unmarshal(raw, &data) != nil || data.Products == nil || len(data.Products) > 100 || len(data.Products) != v.ProductCount {
			return nil, unavailable()
		}
		for _, ref := range data.Products {
			if !Identifier.MatchString(ref.ProductID) {
				return nil, unavailable()
			}
		}
		return struct {
			Collection
			Products []CollectionProductRef `json:"products"`
		}{v, data.Products}, nil
	}
	if id != "" {
		data, err := decode(envelope.Data, true)
		if err != nil {
			return nil, err
		}
		return struct {
			Data any `json:"data"`
		}{data}, nil
	}
	var rows []json.RawMessage
	var meta PageMeta
	var m map[string]json.RawMessage
	if json.Unmarshal(envelope.Data, &rows) != nil || rows == nil || len(rows) > limit || json.Unmarshal(envelope.Meta, &m) != nil || len(m["nextCursor"]) == 0 {
		return nil, unavailable()
	}
	if json.Unmarshal(envelope.Meta, &meta) != nil || (meta.NextCursor != nil && (*meta.NextCursor == "" || len(*meta.NextCursor) > 1024)) {
		return nil, unavailable()
	}
	values := make([]any, 0, len(rows))
	for _, row := range rows {
		v, err := decode(row, false)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return struct {
		Data []any    `json:"data"`
		Meta PageMeta `json:"meta"`
	}{values, meta}, nil
}
