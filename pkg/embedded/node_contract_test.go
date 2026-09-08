package embedded_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"emisell.app/platform/pkg/embedded"
)

func TestNodeStarterIdentityContract(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for cross-language contract")
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := embedded.Identity{MerchantID: "test-merchant", ActorID: "test-actor", AppID: "test-app", InstallationID: "test-installation"}
	token, err := embedded.Issue(key, "test-key", "test-platform", "test-client", id, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"token": token, "key": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), "identity": id})
	module, err := filepath.Abs("../../packages/developer-cli/templates/react-router/server/identity.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `
import {pathToFileURL} from 'node:url';
import assert from 'node:assert/strict';
const {createIdentityVerifier} = await import(pathToFileURL(process.argv[1]));
let raw = ''; for await (const chunk of process.stdin) raw += chunk;
const f = JSON.parse(raw); let active = true;
const verify = createIdentityVerifier({keys:{'test-key':f.key},issuer:'test-platform',audience:'test-client',resolveExpected:async()=>f.identity,authorizeCurrent:async()=>active});
const result = await verify(f.token);
assert.equal(result.merchantId, f.identity.merchantId);
active = false;
await assert.rejects(verify(f.token), /access_denied/);
`
	cmd := exec.Command(node, "--input-type=module", "-e", script, module)
	cmd.Stdin = bytes.NewReader(input)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Node verifier contract: %v\n%s", err, out)
	}
}
