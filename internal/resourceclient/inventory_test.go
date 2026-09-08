package resourceclient

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestInventoryLocationsQueriesAndProjection(t *testing.T) {
	for _, tc := range []struct{ path, query string }{
		{"/v1/products", "view=inventory&limit=5"}, {"/v1/products/p", "view=inventory"},
		{"/v1/settings/location", "q=Warehouse&limit=5"}, {"/v1/settings/location/l", ""},
	} {
		if err := ValidateResourceQuery(tc.path, mustQuery(tc.query)); err != nil {
			t.Fatal(tc, err)
		}
	}
	for _, tc := range []struct{ path, query string }{
		{"/v1/products", "view=other"}, {"/v1/products", "view=inventory&view=inventory"}, {"/v1/products", "view=inventory&q=Blue"},
		{"/v1/products/p", "view=inventory&limit=1"}, {"/v1/products/stock", "view=inventory"}, {"/v1/products/FIRST", "view=inventory"},
		{"/v1/settings/location", "view=inventory"}, {"/v1/settings/location", "address=true"}, {"/v1/settings/location/countries", ""},
	} {
		if ValidateResourceQuery(tc.path, mustQuery(tc.query)) == nil {
			t.Fatal("unsupported query accepted", tc)
		}
	}
	for _, tc := range []struct{ scope, id, row string }{
		{"read_inventory", "p", `{"id":"p","trackInventory":true,"continueSellingWhenOutOfStock":false,"stock":-2,"items":[{"id":"p","type":"PRODUCT","stock":-2,"levels":[{"locationId":"l","available":-2,"private":"hidden"}]}],"price":"hidden"}`},
		{"read_locations", "l", `{"id":"l","name":"Warehouse","isActive":false,"isPrimary":true,"isPhysicalStorefront":false,"isFulfillment":true,"shippingIsActive":true,"deliveryIsActive":false,"pickUpIsActive":false,"address":"hidden","phone":"hidden"}`},
	} {
		detail := []byte(`{"data":` + tc.row + `}`)
		result, err := decodeExisting(detail, tc.scope, tc.id, 5)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(result)
		if strings.Contains(string(raw), "hidden") {
			t.Fatal("private field exposed")
		}
		if _, err := decodeExisting(detail, tc.scope, "other", 5); err == nil {
			t.Fatal("wrong detail identity accepted")
		}
		if _, err := decodeExisting([]byte(`{"data":[`+tc.row+`],"meta":{"nextCursor":null}}`), tc.scope, "", 5); err != nil {
			t.Fatal(err)
		}
		for _, broken := range []string{strings.Replace(string(detail), `"stock":-2`, `"stock":0`, 1), strings.Replace(string(detail), `"isActive":false`, `"isActive":null`, 1), strings.Replace(string(detail), `"trackInventory":true`, `"trackInventory":null`, 1)} {
			if broken != string(detail) {
				if _, err := decodeExisting([]byte(broken), tc.scope, tc.id, 5); err == nil {
					t.Fatal("invalid projection accepted", broken)
				}
			}
		}
	}
}
func mustQuery(query string) url.Values {
	v, err := url.ParseQuery(query)
	if err != nil {
		panic(err)
	}
	return v
}
