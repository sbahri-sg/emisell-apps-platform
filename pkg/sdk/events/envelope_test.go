package events_test

import (
	"emisell.app/platform/pkg/sdk/events"
	"encoding/json"
	"testing"
	"time"
)

func TestWireEnvelopeValidation(t *testing.T) {
	good := events.Envelope{ID: "evt_one", Type: "emisell.capability.invoked.v1", Version: 1, OccurredAt: time.Now().UTC(), TenantID: "tenant-one", Subject: "ins_one", ActorID: "svc_one", CorrelationID: "rpc_one", CausationID: "rpc_one", Producer: "emisell-app-platform", Payload: map[string]string{"capability": "payment/v1", "operation": "create"}}
	raw, _ := json.Marshal(good)
	decoded, err := events.Decode(raw)
	if err != nil || events.Subject(decoded) != "emisell.events.tenant-one.emisell.capability.invoked.v1" {
		t.Fatal(err)
	}
	for _, mutate := range []func(*events.Envelope){func(e *events.Envelope) { e.TenantID = "tenant.>" }, func(e *events.Envelope) { e.Type = "emisell.capability.invoked.v2" }, func(e *events.Envelope) { e.Version = 2 }, func(e *events.Envelope) { e.ID = "" }, func(e *events.Envelope) { e.Payload = nil }, func(e *events.Envelope) { e.Producer = "unknown" }} {
		bad := good
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		if _, err = events.Decode(raw); err == nil {
			t.Fatal("invalid envelope accepted")
		}
	}
	if _, err = events.Decode(make([]byte, 33000)); err == nil {
		t.Fatal("oversized payload accepted")
	}
}
