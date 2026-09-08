package resourceclient

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestExistingResourceAllowlist(t *testing.T) {
	for _, path := range []string{"/v1/orders", "/v1/orders/order-a", "/v1/settings/shipping", "/v1/settings/shipping/profile/profile-a"} {
		if err := ValidateResourceQuery(path, nil); err != nil {
			t.Fatal(path, err)
		}
		if err := ValidateResourceQuery(path, url.Values{"merchantId": {"other"}}); err == nil {
			t.Fatal("identity override accepted", path)
		}
	}
	for _, path := range []string{"/v1/orders/export", "/v1/orders/draft", "/v1/orders/order-a/items", "/v1/settings/shipping/credentials", "https://other.example/v1/orders"} {
		if err := ValidateResourceQuery(path, nil); err == nil {
			t.Fatal("unknown operation accepted", path)
		}
	}
	if ValidateResourceQuery("/v1/settings/shipping", url.Values{"status": {"PAID"}}) == nil {
		t.Fatal("order filter accepted by shipping")
	}
	if ValidateResourceQuery("/v1/orders/order-a", url.Values{"limit": {"5"}}) == nil {
		t.Fatal("detail query accepted")
	}
}

func TestShippingProjectionRejectsMissingFields(t *testing.T) {
	const body = `{"data":{"id":"shipping-a","splitShipping":false,"estimatedShow":"SHOW","fulfillmentTime":"ONE_DAY","customTime":null,"unit":"DAY","shopPromise":false,"profiles":[{"id":"profile-a","name":"General","isPrimary":false,"apiKey":"never-return"}]},"meta":{"nextCursor":null}}`
	value, err := decodeExisting([]byte(body), "read_shipping", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	projected, _ := json.Marshal(value)
	if strings.Contains(string(projected), "never-return") {
		t.Fatal("private field exposed")
	}
	for _, broken := range []string{strings.Replace(body, `"isPrimary":false,`, "", 1), strings.Replace(body, `"splitShipping":false`, `"splitShipping":null`, 1)} {
		if _, err := decodeExisting([]byte(broken), "read_shipping", "", 5); err == nil {
			t.Fatal("malformed upstream accepted")
		}
	}
}
