// Package simulator is the explicit local reference adapter, never a provider.
package simulator

import (
	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"regexp"
	"slices"
)

type Runtime struct{}

var referencePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (Runtime) Execute(cap string, r capability.Request, current *capability.Resource) (*capability.Resource, []capability.Rate, error) {
	if cap == "payment/v1" {
		if r.WeightGrams != 0 || r.DestinationZone != "" {
			return nil, nil, fault.Invalid
		}
		if r.Operation == "create" {
			if r.ResourceID != "" || !referencePattern.MatchString(r.Reference) || r.Currency != "IDR" || r.AmountMinor <= 0 || r.AmountMinor > 1_000_000_000_000 {
				return nil, nil, fault.Invalid
			}
			return &capability.Resource{ID: ids.New("pay"), Reference: r.Reference, Status: "authorized", AmountMinor: r.AmountMinor, Currency: r.Currency, Revision: 1}, nil, nil
		}
		if r.ResourceID == "" || r.Reference != "" || r.Currency != "" || r.AmountMinor != 0 {
			return nil, nil, fault.Invalid
		}
		if current == nil {
			return nil, nil, fault.NotFound
		}
		next := *current
		switch r.Operation {
		case "capture":
			if next.Status != "authorized" && next.Status != "captured" {
				return nil, nil, fault.Conflict
			}
			next.Status = "captured"
		case "refund":
			if next.Status != "captured" && next.Status != "refunded" {
				return nil, nil, fault.Conflict
			}
			next.Status = "refunded"
		case "status":
		default:
			return nil, nil, fault.Invalid
		}
		if next.Status != current.Status {
			next.Revision++
		}
		return &next, nil, nil
	}
	if cap == "shipping/v1" {
		if r.AmountMinor != 0 || r.Currency != "" {
			return nil, nil, fault.Invalid
		}
		if r.Operation == "track" {
			if r.ResourceID == "" || r.Reference != "" || r.WeightGrams != 0 || r.DestinationZone != "" {
				return nil, nil, fault.Invalid
			}
			if current == nil {
				return nil, nil, fault.NotFound
			}
			return current, nil, nil
		}
		if r.ResourceID != "" || r.WeightGrams <= 0 || r.WeightGrams > 30000 || !slices.Contains([]string{"ID-JKT", "ID-BDG", "ID-SBY"}, r.DestinationZone) {
			return nil, nil, fault.Invalid
		}
		amount := int64(5000 + ((r.WeightGrams+999)/1000)*1000)
		if r.Operation == "get_rates" && r.Reference == "" {
			return nil, []capability.Rate{{Service: "standard-simulation", AmountMinor: amount, Currency: "IDR"}}, nil
		}
		if r.Operation == "create" && referencePattern.MatchString(r.Reference) {
			return &capability.Resource{ID: ids.New("shp"), Reference: r.Reference, Status: "created", AmountMinor: amount, Currency: "IDR"}, nil, nil
		}
	}
	return nil, nil, fault.Invalid
}
