package httpapi_test

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/emisell"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
)

// Real Go HTTP/auth/OAuth -> Node/JWT -> Prisma/PostgreSQL. Gateway storage is in-memory.
// The backend runner provisions and owns the disposable database; never use a merchant database.
func TestProductsNodeContract(t *testing.T) {
	backend := os.Getenv("EMISELL_API_SERVICE_DIR")
	if backend == "" {
		t.Skip("set EMISELL_API_SERVICE_DIR for cross-repository HTTP contract test")
	}
	if os.Getenv("RESOURCE_TEST_DATABASE_URL") == "" || os.Getenv("RESOURCE_TEST_DATABASE_NAME") == "" {
		t.Fatal("run api-service/scripts/test-app-platform-resources.mjs with the App Platform checkout; a disposable database is required")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(backend, "tests/fixtures/app-platform-resource-server.mjs"))
	cmd.Env = append(os.Environ(), "RESOURCE_TEST_PUBLIC_KEY="+string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("Node fixture did not start")
	}
	client, err := emisell.NewProducts(emisell.Options{Origin: scanner.Text(), KeyID: "contract-test", AllowHTTP: true, PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})})
	if err != nil {
		t.Fatal(err)
	}
	handler, repository := testServerWithProducts(client, time.Now)
	app := postApp(t, handler, "resource-app-request-0001")
	credential := decodeData[application.CredentialSecret](t, request(t, handler, http.MethodPost, "/v1/apps/"+app.ID+"/credentials", `{"environment":"sandbox"}`, testOrganizationID, domain.RoleOwner, "resource-credential-0001"))
	scopes := request(t, handler, http.MethodPut, "/v1/apps/"+app.ID+"/scopes", `{"scopes":[{"scope":"read_products","access":"optional"},{"scope":"read_merchant","access":"optional"}]}`, testOrganizationID, domain.RoleDeveloper, "")
	if scopes.Code != 200 {
		t.Fatalf("scopes: %d", scopes.Code)
	}
	version := postVersion(t, handler, app.ID, "1.0.0", "resource-version-0001")
	releaseVersion(t, handler, app.ID, version.ID, "resource-release-0001", nil, 200, domain.RoleOwner)
	mint := func(merchant, scope string) application.OAuthTokenResponse {
		verifier := strings.Repeat("p", 43)
		digest := sha256.Sum256([]byte(verifier))
		body := fmt.Sprintf(`{"clientId":%q,"redirectUri":"https://apps.emisell.com/loyalty","state":"resource-state-0001","codeChallenge":%q,"merchantId":%q,"merchantName":"Contract Test","environment":"sandbox","grantedScopes":[%q]}`, credential.Credential.ClientID, base64.RawURLEncoding.EncodeToString(digest[:]), merchant, scope)
		authorized := request(t, handler, http.MethodPost, "/v1/oauth/authorizations", body, testOrganizationID, domain.RoleOwner, "")
		if authorized.Code != 201 {
			t.Fatalf("authorization: %d %s", authorized.Code, authorized.Body.String())
		}
		grant := decodeData[application.OAuthAuthorizationResponse](t, authorized)
		form := url.Values{"grant_type": {"authorization_code"}, "code": {grant.Code}, "redirect_uri": {"https://apps.emisell.com/loyalty"}, "code_verifier": {verifier}}
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(credential.Credential.ClientID, credential.ClientSecret)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("exchange: %d", res.Code)
		}
		var token application.OAuthTokenResponse
		if err := json.Unmarshal(res.Body.Bytes(), &token); err != nil {
			t.Fatal(err)
		}
		return token
	}
	get := func(path, token string, expected int) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		// Attempts to override the tenant must not cross the gateway boundary.
		req.Header.Set("X-Emisell-Merchant-ID", "merchant-b")
		req.Header.Set("X-Request-ID", "invalid correlation value")
		req.Header.Set("X-Merchant-ID", "merchant-b")
		req.Header.Set("Cookie", "emisell_session=not-forwarded")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != expected {
			t.Fatalf("%s: got %d want %d: %s", path, res.Code, expected, res.Body.String())
		}
		if strings.Contains(res.Body.String(), "MUST-NOT-LEAK") || strings.Contains(res.Body.String(), credential.ClientSecret) || res.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("unsafe product response")
		}
		return res
	}
	get("/v1/products", "", 401)
	get("/v1/products", testToken, 401)
	noScope := mint("merchant-no-scope", "read_merchant")
	get("/v1/products", noScope.AccessToken, 403)
	token := mint("merchant-a", "read_products")
	disabled := httpapi.NewServer(httpapi.Dependencies{InstallationAccess: application.NewInstallationAccessService(repository, time.Now), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	disabledRequest := httptest.NewRequest("GET", "/v1/products", nil)
	disabledRequest.Header.Set("Authorization", "Bearer "+token.AccessToken)
	disabledResponse := httptest.NewRecorder()
	disabled.ServeHTTP(disabledResponse, disabledRequest)
	if disabledResponse.Code != 503 || !strings.Contains(disabledResponse.Body.String(), "resource_disabled") {
		t.Fatal("default gate did not reject authenticated request")
	}
	first := get("/v1/products?limit=1", token.AccessToken, 200)
	var page emisell.Page
	if json.Unmarshal(first.Body.Bytes(), &page) != nil || len(page.Data) != 1 || page.Data[0].ID != "product-a2" || page.Meta.NextCursor == nil {
		t.Fatal("unexpected first page")
	}
	firstCursor := *page.Meta.NextCursor
	second := get("/v1/products?limit=1&cursor="+url.QueryEscape(*page.Meta.NextCursor), token.AccessToken, 200)
	if json.Unmarshal(second.Body.Bytes(), &page) != nil || len(page.Data) != 1 || page.Data[0].ID != "product-a1" || page.Meta.NextCursor != nil {
		t.Fatal("unexpected second page")
	}
	productResponse := get("/v1/products/product-a1", token.AccessToken, 200)
	var product emisell.Item
	if json.Unmarshal(productResponse.Body.Bytes(), &product) != nil || product.Data.Price != "60000.5" || product.Data.Stock == nil || *product.Data.Stock != 45 {
		t.Fatal("base price/stock must not be replaced by variant price, variant stock or stock minus soldCount")
	}
	get("/v1/products/product-b1", token.AccessToken, 404)
	merchantB := mint("merchant-b", "read_products")
	merchantBResponse := get("/v1/products", merchantB.AccessToken, 200)
	if json.Unmarshal(merchantBResponse.Body.Bytes(), &page) != nil || len(page.Data) != 1 || page.Data[0].ID != "product-b1" {
		t.Fatal("merchant B must receive only its own products")
	}
	get("/v1/products/product-a1", merchantB.AccessToken, 404)
	get("/v1/products?cursor="+url.QueryEscape(firstCursor), merchantB.AccessToken, 400)
	for _, merchant := range []string{"merchant-inactive", "merchant-suspended", "merchant-missing"} {
		denied := mint(merchant, "read_products")
		// Backend policy failures are intentionally redacted as dependency failures to the provider.
		get("/v1/products", denied.AccessToken, 503)
	}
	get("/v1/products?merchantId=merchant-b", token.AccessToken, 400)
	get("/v1/products?limit=1&limit=2", token.AccessToken, 400)
	get("/v1/products/product-a1?published=true", token.AccessToken, 400)
	uninstall := request(t, handler, http.MethodDelete, "/v1/apps/"+app.ID+"/installations/"+token.Installation.ID, "", testOrganizationID, domain.RoleOwner, "resource-uninstall-0001")
	if uninstall.Code != 204 {
		t.Fatalf("uninstall: %d", uninstall.Code)
	}
	get("/v1/products", token.AccessToken, 401)
	get("/v1/products/product-a1", token.AccessToken, 401)
	get("/v1/products", merchantB.AccessToken, 200)
}
