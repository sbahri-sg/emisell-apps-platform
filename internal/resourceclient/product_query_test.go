package resourceclient

import (
	"net/url"
	"strings"
	"testing"
)

func TestProductSearchQuery(t *testing.T) {
	for _, q := range []string{"Blue", "Produk baru", "100%_sku", "baju biru"} {
		if err := ValidateQuery(url.Values{"q": {q}, "limit": {"5"}, "cursor": {"page.signature"}}, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{"", " Blue", "Blue ", "Blue\n", strings.Repeat("a", 101), strings.Repeat("é", 51)} {
		if ValidateQuery(url.Values{"q": {q}}, false) == nil {
			t.Fatalf("accepted invalid search %q", q)
		}
	}
	if ValidateQuery(url.Values{"q": {"a", "b"}}, false) == nil || ValidateQuery(url.Values{"q": {"Blue"}}, true) == nil {
		t.Fatal("duplicate/detail search accepted")
	}
}
