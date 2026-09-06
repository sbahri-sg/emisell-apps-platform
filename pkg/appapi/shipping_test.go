package appapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOriginZonePreservesLegacyRequestBytes(t *testing.T) {
	r := Request{Operation: "get_rates", WeightGrams: 1200, DestinationZone: "ID-JKT"}
	raw, err := json.Marshal(r)
	if err != nil || string(raw) != `{"operation":"get_rates","weightGrams":1200,"destinationZone":"ID-JKT"}` {
		t.Fatal("legacy canonical request/idempotency input changed", string(raw), err)
	}
	r.OriginZone = "ID-ORIGIN"
	raw, err = json.Marshal(r)
	if err != nil || !strings.Contains(string(raw), `"originZone":"ID-ORIGIN"`) {
		t.Fatal("origin omitted", err)
	}
}
