package events

import (
	"encoding/json"
	"errors"
)

const PaymentStatusType = "emisell.payment.status_changed.v1"

// PaymentStatus is a complete provider-neutral snapshot. Revision is ordered
// per tenant/resource, never by event arrival time or broker sequence.
type PaymentStatus struct {
	ResourceID     string `json:"resourceId"`
	InstallationID string `json:"installationId"`
	Reference      string `json:"reference"`
	Status         string `json:"status"`
	AmountMinor    int64  `json:"amountMinor"`
	Currency       string `json:"currency"`
	Revision       int64  `json:"revision"`
	Simulation     bool   `json:"simulation"`
}

func PaymentRank(status string) int {
	switch status {
	case "authorized":
		return 1
	case "captured":
		return 2
	case "refunded":
		return 3
	}
	return 0
}
func (p PaymentStatus) Validate() error {
	if !Token.MatchString(p.ResourceID) || !Token.MatchString(p.InstallationID) || !Token.MatchString(p.Reference) || PaymentRank(p.Status) == 0 || p.AmountMinor <= 0 || p.AmountMinor > 1_000_000_000_000 || p.Currency != "IDR" || p.Revision < 1 || p.Revision > 1_000_000_000_000 || !p.Simulation {
		return errors.New("invalid local payment snapshot")
	}
	return nil
}
func DecodePayment(e Envelope) (PaymentStatus, error) {
	var p PaymentStatus
	raw, err := json.Marshal(e.Payload)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	if e.Type != PaymentStatusType || e.Subject != p.ResourceID {
		return p, errors.New("payment subject mismatch")
	}
	return p, p.Validate()
}
