package bootstrap_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDeveloperCLIContract(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node.js >=22 is required for the CLI contract test")
	}
	script, err := filepath.Abs("../../packages/developer-cli/test/backend-contract.mjs")
	if err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]string{"email": dev.email, "password": dev.password})
	cmd := exec.Command(node, script, f.server.URL, t.TempDir())
	cmd.Stdin = bytes.NewReader(input)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("CLI contract failed: %v\n%s", err, output)
	}
}
