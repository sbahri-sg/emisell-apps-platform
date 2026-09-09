package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/accessscope"
	"testing"
)

func TestPrivateProductScopeAndSignature(t *testing.T) {
	document := AppDocument{Name: "Products", Version: "1.0.0", Capability: PrivateProducts, Scopes: []string{}, AccessScopes: &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}}
	if document.Validate(false) != nil || document.Validate(true) == nil || PublicDistributionAllowed(PrivateProducts) {
		t.Fatal("private app entered public review")
	}
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	v := PrivateProductVersion{ID: "privateversion_1", AppID: "app_1", OrganizationID: "org_1", OwnerAccountID: "owner_1", ClientID: "eai_1", Document: document}
	signature, err := v.Sign(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = v.Verify(pub, signature); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"write_products", "read_orders", "read_users", "read_customers", "write_metaobjects"} {
		bad := document
		bad.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{scope}, Optional: []string{}}
		if bad.Validate(false) == nil {
			t.Fatal("unrequested access", scope)
		}
	}
	for _, mutate := range []func(*PrivateProductVersion){
		func(v *PrivateProductVersion) { v.AppID = "other" }, func(v *PrivateProductVersion) { v.OwnerAccountID = "other" },
		func(v *PrivateProductVersion) { v.ClientID = "other" }, func(v *PrivateProductVersion) { v.Document.Name = "other" },
		func(v *PrivateProductVersion) { v.Document.Endpoint = "https://example.com" },
	} {
		bad := v
		mutate(&bad)
		if _, err = bad.Verify(pub, signature); err == nil {
			t.Fatal("signature accepted modified binding")
		}
	}
}
