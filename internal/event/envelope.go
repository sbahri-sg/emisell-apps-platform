package event

import (
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/sdk/events"
	"time"
)

// Envelope is shared contract data, never a broker SDK type.
type Envelope = events.Envelope

func New(kind, tenant, actor, subject, correlation string, payload any) Envelope {
	return Envelope{ID: ids.New("evt"), Type: kind, Version: 1, OccurredAt: time.Now().UTC(), TenantID: tenant, Subject: subject, ActorID: actor, CorrelationID: correlation, CausationID: correlation, Producer: "emisell-app-platform", Payload: payload}
}
