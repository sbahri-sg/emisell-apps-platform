package simulator

import (
	"emisell.app/platform/internal/capability"
	"testing"
)

func TestInvalidRequestsFailClosed(t *testing.T) {
	rt := Runtime{}
	for _, request := range []capability.Request{
		{Operation: "create", AmountMinor: -1, Currency: "IDR", Reference: "order"},
		{Operation: "create", AmountMinor: 100, Currency: "USD", Reference: "order"},
		{Operation: "create", AmountMinor: 100, Currency: "IDR", Reference: "customer name with spaces"},
		{Operation: "status", ResourceID: "id", AmountMinor: 100},
		{Operation: "refund", ResourceID: "id"},
	} {
		if _, _, err := rt.Execute("payment/v1", request, &capability.Resource{ID: "id", Status: "authorized"}); err == nil {
			t.Fatalf("invalid payment accepted: %+v", request)
		}
	}
	for _, request := range []capability.Request{
		{Operation: "get_rates", WeightGrams: 0, DestinationZone: "ID-JKT"},
		{Operation: "get_rates", WeightGrams: 40000, DestinationZone: "ID-JKT"},
		{Operation: "get_rates", WeightGrams: 100, DestinationZone: "UNSUPPORTED"},
		{Operation: "track", ResourceID: "missing"},
	} {
		if _, _, err := rt.Execute("shipping/v1", request, nil); err == nil {
			t.Fatalf("invalid shipping accepted: %+v", request)
		}
	}
}
