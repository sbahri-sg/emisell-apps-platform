package bootstrap_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDeveloperCLIContract(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	_, _, coreKey, _ := lifecycleClient(t, f)
	// Test-only equivalent of the local portal proxy: the public Host is fixed,
	// while the ephemeral listener avoids reserving the real portal port.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = "localhost:4317"
		f.server.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(proxy.Close)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node.js >=22 is required for the CLI contract test")
	}
	script, err := filepath.Abs("../../packages/developer-cli/test/backend-contract.mjs")
	if err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]string{"email": dev.email, "coreKey": coreKey, "merchant": f.tenant})
	cmd := exec.Command(node, script, proxy.URL, t.TempDir())
	cmd.Stdin = bytes.NewReader(input)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("CLI contract failed: %v\n%s", err, output)
	}
}
