package gatewaycontract

import (
	"slices"
	"testing"
	"time"
)

func TestReadinessInventoryDoesNotPromotePartialContracts(t *testing.T) {
	now := time.Unix(1700000000, 0)
	r := VerifyReadiness(now)
	h := Reference()
	if r.Environment != "local" || len(r.ContractRevision) != 64 || r.ContractRevision != h.ContractRevision || Reference().ContractRevision != h.ContractRevision || len(r.Operations) != len(h.Operations) {
		t.Fatal("readiness/contract drift")
	}
	for i, o := range r.Operations {
		if o.Procedure != h.Operations[i].Procedure || o.Version != h.Operations[i].Version || o.Status != "planned" || len(o.Blockers) == 0 || !slices.Equal(o.AcceptedScopes, h.Operations[i].AcceptedScopes) {
			t.Fatal("operation readiness drift")
		}
		for _, handle := range o.AcceptedScopes {
			found := false
			for _, s := range r.Scopes {
				if s.Handle == handle && slices.Contains(s.Operations, o.Procedure) {
					found = true
				}
			}
			if !found {
				t.Fatal("operation without scope mapping")
			}
		}
	}
	if r.CoreChecked || r.Verification != "platform_build_inventory" || !r.CheckedAt.Equal(now) || len(r.Scopes) != len(Reference().Coverage) {
		t.Fatal("invalid inventory provenance")
	}
	seen := map[string]bool{}
	partial := 0
	for _, s := range r.Scopes {
		if seen[s.Handle] || s.Grantable || s.Status != "planned" || len(s.Blockers) == 0 {
			t.Fatal("scope incorrectly promoted", s.Handle)
		}
		seen[s.Handle] = true
		if s.ContractStatus == "partial" {
			partial++
			if len(s.Operations) != 2 {
				t.Fatal("partial mapping lost")
			}
		}
	}
	if partial != 2 {
		t.Fatal("unexpected contract coverage")
	}
}
