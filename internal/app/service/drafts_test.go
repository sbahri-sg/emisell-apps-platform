package service

import (
	"emisell.app/platform/pkg/accessscope"
	"encoding/json"
	"testing"
)

func TestLegacyDraftHashAndResourceScopeSeparation(t *testing.T) {
	raw := json.RawMessage(`{"name":"Shipping","summary":"Shipping integration","description":"Details","version":"1.0.0","capability":"shipping/v1","scopes":["orders.read","shipping.read","shipping.write"],"endpoint":"https://example.invalid/app"}`)
	var d AppDocument
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Validate(true) != nil || RequestHash(d) != RequestHash(raw) {
		t.Fatal("legacy draft/hash changed")
	}
	d.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}
	if d.Validate(true) != nil || RequestHash(d) == RequestHash(raw) {
		t.Fatal("resource declaration not bound to request")
	}
	d.Scopes = []string{"read_products"}
	if d.Validate(false) == nil {
		t.Fatal("resource scopes accepted as fixture permissions")
	}
}

func TestAuthoringDocumentValidation(t *testing.T) {
	valid := AppDocument{Name: "Shipping", Summary: "Shipping integration", Description: "Details", Version: "1.0.0", Capability: "shipping/v1", Scopes: []string{"orders.read", "shipping.read", "shipping.write"}, Endpoint: "https://example.invalid/app"}
	if err := valid.Validate(true); err != nil {
		t.Fatal(err)
	}
	cases := []func(*AppDocument){func(d *AppDocument) { d.Endpoint = "http://example.invalid" }, func(d *AppDocument) { d.Endpoint = "https://user:secret@example.invalid" }, func(d *AppDocument) { d.Version = "01.0.0" }, func(d *AppDocument) { d.Capability = "provider/v1" }, func(d *AppDocument) { d.Scopes = []string{"payments.write"} }, func(d *AppDocument) { d.Scopes = []string{"shipping.read", "shipping.read"} }, func(d *AppDocument) { d.Summary = "" }}
	for i, mutate := range cases {
		d := valid
		mutate(&d)
		if d.Validate(true) == nil {
			t.Errorf("accepted case %d", i)
		}
	}
	d := valid
	d.Endpoint = ""
	d.Summary = ""
	d.Description = ""
	if d.Validate(false) != nil || d.Validate(true) == nil {
		t.Fatal("draft completeness policy")
	}
	for _, key := range []string{"", "short", "bad key spaces", "../request-key"} {
		if ValidRequestKey(key) {
			t.Fatal("invalid key", key)
		}
	}
}
