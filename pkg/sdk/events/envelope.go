// Package events defines the versioned wire contract; it has no broker dependency.
package events

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"time"
)

type Envelope struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Version       int       `json:"version"`
	OccurredAt    time.Time `json:"occurredAt"`
	TenantID      string    `json:"tenantId"`
	Subject       string    `json:"subject"`
	ActorID       string    `json:"actorId"`
	CorrelationID string    `json:"correlationId"`
	CausationID   string    `json:"causationId"`
	Producer      string    `json:"producer"`
	Payload       any       `json:"payload"`
}

var Token = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (e Envelope) Validate() error {
	if !Token.MatchString(e.ID) || !Token.MatchString(e.TenantID) || !Token.MatchString(e.Subject) || !Token.MatchString(e.ActorID) || !Token.MatchString(e.CorrelationID) || !Token.MatchString(e.CausationID) || e.Version != 1 || e.OccurredAt.IsZero() || e.Producer != "emisell-app-platform" || e.Payload == nil {
		return errors.New("invalid event envelope")
	}
	if !slices.Contains([]string{"emisell.app.installed.v1", "emisell.app.activated.v1", "emisell.app.disabling.v1", "emisell.app.uninstalled.v1", "emisell.capability.invoked.v1", PaymentStatusType}, e.Type) {
		return errors.New("unsupported event version/type")
	}
	if e.Type == PaymentStatusType {
		_, err := DecodePayment(e)
		return err
	}
	return nil
}
func Subject(e Envelope) string { return "emisell.events." + e.TenantID + "." + e.Type }
func Decode(raw []byte) (Envelope, error) {
	var e Envelope
	if len(raw) > 32<<10 {
		return e, errors.New("event too large")
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return e, errors.New("invalid event JSON")
	}
	return e, e.Validate()
}
