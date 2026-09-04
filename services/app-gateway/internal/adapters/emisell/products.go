// Package emisell implements the private App Gateway -> Emisell resource boundary.
package emisell

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

const ReadProducts = "read_products"

var Identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var correlationID = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
var decimal = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)
var utcTimestamp = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,3})?Z$`)

type Options struct {
	Origin, KeyID string
	PrivateKeyPEM []byte
	AllowHTTP     bool
	Timeout       time.Duration
}

type Products struct {
	origin, keyID string
	key           *rsa.PrivateKey
	client        *http.Client
}

type Product struct {
	ID                            string    `json:"id"`
	Name                          string    `json:"name"`
	Slug                          string    `json:"slug"`
	SKU                           *string   `json:"sku"`
	Description                   *string   `json:"description"`
	Price                         string    `json:"price"`
	CompareAtPrice                *string   `json:"compareAtPrice"`
	Stock                         *int64    `json:"stock"`
	IsPhysical                    bool      `json:"isPhysical"`
	IsPublished                   bool      `json:"isPublished"`
	Status                        string    `json:"status"`
	Type                          *string   `json:"type"`
	TrackInventory                bool      `json:"trackInventory"`
	ContinueSellingWhenOutOfStock bool      `json:"continueSellingWhenOutOfStock"`
	CreatedAt                     time.Time `json:"createdAt"`
	UpdatedAt                     time.Time `json:"updatedAt"`
}

// Reject partial or null required upstream fields; never turn a malformed response into plausible data.
func (p *Product) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range []string{"id", "name", "slug", "price", "isPhysical", "isPublished", "status", "trackInventory", "continueSellingWhenOutOfStock", "createdAt", "updatedAt"} {
		if len(fields[name]) == 0 || string(fields[name]) == "null" {
			return errors.New("incomplete product response")
		}
	}
	for _, name := range []string{"sku", "description", "compareAtPrice", "stock", "type"} {
		if len(fields[name]) == 0 {
			return errors.New("incomplete product response")
		}
	}
	type decoded Product
	return json.Unmarshal(data, (*decoded)(p))
}

type PageMeta struct {
	NextCursor *string `json:"nextCursor"`
}
type Page struct {
	Data []Product `json:"data"`
	Meta PageMeta  `json:"meta"`
}
type Item struct {
	Data Product `json:"data"`
}
type Error struct {
	Status     int
	Code       string
	RetryAfter int
}

func (e *Error) Error() string { return e.Code }
func unavailable() error       { return &Error{Status: 503, Code: "resource_unavailable"} }

func NewProducts(options Options) (*Products, error) {
	u, err := url.Parse(options.Origin)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		(u.Scheme != "https" && !(options.AllowHTTP && u.Scheme == "http")) || !Identifier.MatchString(options.KeyID) {
		return nil, errors.New("invalid Emisell resource origin or signing key ID")
	}
	block, rest := pem.Decode(options.PrivateKeyPEM)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("invalid resource private key PEM")
	}
	var key *rsa.PrivateKey
	if block.Type == "RSA PRIVATE KEY" {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else {
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		key, _ = parsed.(*rsa.PrivateKey)
	}
	if err != nil || key == nil || key.N.BitLen() < 2048 || key.Validate() != nil {
		return nil, errors.New("resource key must be RSA >= 2048 bits")
	}
	if options.Timeout == 0 {
		options.Timeout = 5 * time.Second
	}
	if options.Timeout < time.Millisecond || options.Timeout > 10*time.Second {
		return nil, errors.New("invalid resource timeout")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = options.Timeout
	return &Products{origin: options.Origin, keyID: options.KeyID, key: key, client: &http.Client{
		Transport: transport, Timeout: options.Timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Each call mints a fresh, narrowly scoped assertion after validating installation context.
func (p *Products) assertion(access domain.InstallationAccessContext) (string, error) {
	if access.InstallationStatus != domain.InstallationStatusActive || !Identifier.MatchString(access.MerchantID) ||
		!Identifier.MatchString(access.InstallationID) || !Identifier.MatchString(access.AppID) ||
		(access.Environment != domain.EnvironmentSandbox && access.Environment != domain.EnvironmentProduction) {
		return "", domain.ErrForbidden
	}
	if err := application.RequireInstallationScopes(access, ReadProducts); err != nil {
		return "", err
	}
	now := time.Now().UTC().Unix()
	expiry := now + 45
	if access.TokenExpiresAt.Unix() < expiry {
		expiry = access.TokenExpiresAt.Unix()
	}
	if expiry <= now {
		return "", domain.ErrUnauthorized
	}
	jti := make([]byte, 24)
	if _, err := rand.Read(jti); err != nil {
		return "", unavailable()
	}
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": p.keyID})
	payload, _ := json.Marshal(map[string]any{"iss": "emisell-app-platform", "aud": "emisell-api-service", "sub": "app-gateway", "iat": now, "nbf": now, "exp": expiry,
		"jti": base64.RawURLEncoding.EncodeToString(jti), "merchant_id": access.MerchantID, "installation_id": access.InstallationID, "app_id": access.AppID, "environment": access.Environment, "scope": []string{ReadProducts}})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", unavailable()
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func validProduct(product Product) bool {
	return Identifier.MatchString(product.ID) && product.Name != "" && decimal.MatchString(product.Price) &&
		(product.CompareAtPrice == nil || decimal.MatchString(*product.CompareAtPrice)) && !product.CreatedAt.IsZero() && !product.UpdatedAt.IsZero()
}

func (p *Products) Read(ctx context.Context, access domain.InstallationAccessContext, id string, query url.Values, requestID string) (any, error) {
	if err := ValidateQuery(query, id != ""); err != nil {
		return nil, err
	}
	if !correlationID.MatchString(requestID) {
		return nil, unavailable()
	}
	if id != "" && !Identifier.MatchString(id) {
		return nil, &Error{Status: 400, Code: "invalid_request"}
	}
	assertion, err := p.assertion(access)
	if err != nil {
		return nil, err
	}
	path := "/internal/app-platform/v1/products"
	if id != "" {
		path += "/" + id
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.origin+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, unavailable()
	}
	request.Header.Set("Authorization", "Bearer "+assertion)
	request.Header.Set("X-Emisell-Merchant-ID", access.MerchantID)
	request.Header.Set("X-Emisell-Installation-ID", access.InstallationID)
	request.Header.Set("X-Request-ID", requestID)
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, unavailable()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		switch response.StatusCode {
		case 400:
			return nil, &Error{Status: 400, Code: "invalid_request"}
		case 404:
			if id != "" {
				return nil, &Error{Status: 404, Code: "not_found"}
			}
		case 429:
			retry, _ := strconv.Atoi(response.Header.Get("Retry-After"))
			if retry < 1 || retry > 60 {
				retry = 60
			}
			return nil, &Error{Status: 429, Code: "rate_limited", RetryAfter: retry}
		}
		// Internal assertion failures are a service fault, not invalid provider credentials.
		return nil, unavailable()
	}
	const maxBody = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return nil, unavailable()
	}
	if id != "" {
		var item Item
		if json.Unmarshal(body, &item) != nil || !validProduct(item.Data) || item.Data.ID != id {
			return nil, unavailable()
		}
		return item, nil
	}
	var page Page
	var shape struct {
		Meta map[string]json.RawMessage `json:"meta"`
	}
	if json.Unmarshal(body, &shape) != nil || len(shape.Meta["nextCursor"]) == 0 {
		return nil, unavailable()
	}
	if json.Unmarshal(body, &page) != nil || page.Data == nil || len(page.Data) > 100 || (page.Meta.NextCursor != nil && len(*page.Meta.NextCursor) > 1024) {
		return nil, unavailable()
	}
	for _, product := range page.Data {
		if !validProduct(product) {
			return nil, unavailable()
		}
	}
	limit := 50
	if query.Get("limit") != "" {
		limit, _ = strconv.Atoi(query.Get("limit"))
	}
	if len(page.Data) > limit || (page.Meta.NextCursor != nil && *page.Meta.NextCursor == "") {
		return nil, unavailable()
	}
	return page, nil
}

func ValidateQuery(query url.Values, detail bool) error {
	invalid := &Error{Status: 400, Code: "invalid_request"}
	if detail && len(query) != 0 {
		return invalid
	}
	for key, values := range query {
		if len(values) != 1 || values[0] == "" {
			return invalid
		}
		switch key {
		case "limit":
			n, err := strconv.Atoi(values[0])
			if err != nil || n < 1 || n > 100 || fmt.Sprint(n) != values[0] {
				return invalid
			}
		case "published":
			if values[0] != "true" && values[0] != "false" {
				return invalid
			}
		case "updatedAfter":
			if _, err := time.Parse(time.RFC3339Nano, values[0]); err != nil || !utcTimestamp.MatchString(values[0]) {
				return invalid
			}
		case "cursor":
			if len(values[0]) > 1024 {
				return invalid
			}
		default:
			return invalid
		}
	}
	return nil
}
