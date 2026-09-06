package main

import (
	"emisell.app/platform/pkg/gatewaycontract"
	"encoding/json"
	"os"
	"testing"
)

func TestGatewayExportNeedsNoDatabaseOrCredentials(t *testing.T) {
	t.Setenv("EMISELL_DATABASE_URL", "invalid-do-not-connect")
	oldArgs, oldOut := os.Args, os.Stdout
	f, err := os.CreateTemp(t.TempDir(), "gateway-export-*.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Args, os.Stdout = oldArgs, oldOut; f.Close() })
	os.Args = []string{"cli", "gateway-contract"}
	os.Stdout = f
	if err = run(); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var h gatewaycontract.Handoff
	if err = json.NewDecoder(f).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.Live || len(h.Coverage) != 108 || len(h.Operations) != 2 {
		t.Fatal("bad handoff export")
	}
}
