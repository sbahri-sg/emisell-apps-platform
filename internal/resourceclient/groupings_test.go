package resourceclient

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestGroupingOperationAndProjection(t *testing.T) {
	for _, kind := range []string{"catalogs", "collections"} {
		path := "/v1/" + kind
		if ValidateResourceQuery(path, url.Values{"q": {"Blue"}, "limit": {"5"}}) != nil {
			t.Fatal("valid search rejected")
		}
		for _, suffix := range []string{"/first", "/FIRST", "/bulk-delete", "/a/products"} {
			if ValidateResourceQuery(path+suffix, nil) == nil {
				t.Fatal("unsupported route accepted")
			}
		}
		if ValidateResourceQuery(path+"/a", url.Values{"q": {"Blue"}}) == nil {
			t.Fatal("detail filter accepted")
		}
	}
	for _, tc := range []struct{ scope, body string }{
		{"read_catalogs", `{"id":"a","title":"Blue","status":"ARCHIVED","currencyCode":null,"autoIncludeNewProducts":true,"createdAt":"2026-09-01T00:00:00Z","productCount":1,"products":[{"productId":"p","variantId":null,"price":"private-price"}],"overallAdjustmentValue":17}`},
		{"read_collections", `{"id":"a","name":"Blue","description":null,"createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-01T00:00:00Z","productCount":1,"products":[{"productId":"p","price":"private-price"}],"seo":"private-seo"}`},
	} {
		detail := []byte(`{"data":` + tc.body + `}`)
		result, err := decodeExisting(detail, tc.scope, "a", 5)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(result)
		for _, private := range []string{"private-", "overallAdjustmentValue", "seo"} {
			if strings.Contains(string(body), private) {
				t.Fatal("unexpected projection", string(body))
			}
		}
		if _, err := decodeExisting(detail, tc.scope, "b", 5); err == nil {
			t.Fatal("wrong detail identity accepted")
		}
		for _, broken := range []string{strings.Replace(string(detail), `"productCount":1`, `"productCount":2`, 1), strings.Replace(string(detail), `"productCount":1`, `"productCount":null`, 1)} {
			if _, err := decodeExisting([]byte(broken), tc.scope, "a", 5); err == nil {
				t.Fatal("malformed detail accepted")
			}
		}
		list := []byte(`{"data":[` + tc.body + `],"meta":{"nextCursor":null}}`)
		result, err = decodeExisting(list, tc.scope, "", 5)
		if err != nil {
			t.Fatal(err)
		}
		body, _ = json.Marshal(result)
		if strings.Contains(string(body), `"products"`) {
			t.Fatal("list expanded membership")
		}
	}
}
