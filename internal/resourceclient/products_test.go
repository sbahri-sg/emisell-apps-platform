package resourceclient

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/service"
	"encoding/pem"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProductsNodeContract(t *testing.T) {
	backend := os.Getenv("EMISELL_API_SERVICE_DIR")
	if backend == "" {
		t.Skip("use api-service isolated resource test runner")
	}
	if os.Getenv("RESOURCE_TEST_DATABASE_NAME") == "" {
		t.Fatal("isolated database required")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(backend, "tests/fixtures/app-platform-resource-server.mjs"))
	cmd.Env = append(os.Environ(), "RESOURCE_TEST_PUBLIC_KEY="+string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub})))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("resource server did not start")
	}
	client, err := NewProducts(Options{Origin: scanner.Text(), Environment: "sandbox", KeyID: "contract-test", AllowHTTP: true, PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})})
	if err != nil {
		t.Fatal(err)
	}
	// Private transport test: no OAuth or production grant claim. Public entry
	// remains gated by Lifecycle.WithResourceAccess, tested separately below.
	access := delegation{MerchantID: "merchant-a", InstallationID: "installation-a", AppID: "app-test", Environment: "sandbox"}
	result, err := client.read(ctx, access, "", url.Values{"limit": {"1"}}, "resource-request-123456")
	if err != nil {
		t.Fatal(err)
	}
	page := result.(Page)
	if len(page.Data) != 1 || page.Meta.NextCursor == nil {
		t.Fatal("pagination missing")
	}
	_, err = client.read(ctx, access, "product-b1", nil, "resource-request-123456")
	if err == nil {
		t.Fatal("cross merchant product exposed")
	}
	access.MerchantID = "merchant-b"
	_, err = client.read(ctx, access, "", url.Values{"cursor": {*page.Meta.NextCursor}}, "resource-request-123456")
	if err == nil {
		t.Fatal("cross merchant cursor accepted")
	}
	access.MerchantID = "merchant-inactive"
	if _, err = client.read(ctx, access, "", nil, "resource-request-123456"); err == nil {
		t.Fatal("inactive merchant accepted")
	}
	if _, err = client.Read(ctx, service.Lifecycle{}, identity.ServicePrincipal{}, "", "installation-a", "", nil, "resource-request-123456"); err == nil {
		t.Fatal("missing lifecycle authority accepted")
	}
}

func TestRejectRemotePlainHTTP(t *testing.T) {
	if _, err := NewProducts(Options{Origin: "http://example.com", Environment: "sandbox", KeyID: "test", AllowHTTP: true}); err == nil {
		t.Fatal("remote plaintext accepted")
	}
}
