package conformance_test

import (
	"connectrpc.com/connect"
	"context"
	"crypto/rand"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/gatewaycontract"
	"emisell.app/platform/pkg/gatewaycontract/conformance"
	product "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1"
	productconnect "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1/productv1connect"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test-only implementation exercises the reusable suite over real Connect HTTP.
// It is not exported, registered by bootstrap, or used as a production gateway.
type reference struct {
	a, b     *product.AccessContext
	products map[string][]*product.Product
	mu       sync.Mutex
	cursors  map[string]cursor
}
type cursor struct {
	key, last string
	expires   time.Time
}

func failure(c connect.Code) error {
	return connect.NewError(c, errors.New("contract reference rejected"))
}
func (s *reference) auth(h http.Header) (*product.AccessContext, error) {
	if h.Get("Cookie") != "" || h.Get("Origin") != "" {
		return nil, failure(connect.CodePermissionDenied)
	}
	switch conformance.Identity(strings.TrimPrefix(h.Get("Authorization"), "Bearer ")) {
	case conformance.ReadA, conformance.WriteA:
		return s.a, nil
	case conformance.ReadB:
		return s.b, nil
	case conformance.NoScopeA, conformance.RevokedA, conformance.StaleA, conformance.SuspendedA:
		return nil, failure(connect.CodePermissionDenied)
	default:
		return nil, failure(connect.CodeUnauthenticated)
	}
}
func bind(a, b *product.AccessContext) error {
	if gatewaycontract.MerchantID(a) != gatewaycontract.MerchantID(b) || a.InstallationId != b.InstallationId || a.AppId != b.AppId || a.ScopeProfile != b.ScopeProfile || a.GrantRevision != b.GrantRevision {
		return failure(connect.CodePermissionDenied)
	}
	return nil
}
func (s *reference) Get(_ context.Context, r *connect.Request[product.GetRequest]) (*connect.Response[product.GetResponse], error) {
	p, err := s.auth(r.Header())
	if err != nil {
		return nil, err
	}
	if gatewaycontract.ValidateGet(r.Msg) != nil {
		return nil, failure(connect.CodeInvalidArgument)
	}
	if err = bind(p, r.Msg.Access); err != nil {
		return nil, err
	}
	for _, v := range s.products[gatewaycontract.MerchantID(p)] {
		if v.Id == r.Msg.ProductId {
			return connect.NewResponse(&product.GetResponse{Product: proto.Clone(v).(*product.Product)}), nil
		}
	}
	return nil, failure(connect.CodeNotFound)
}
func (s *reference) List(_ context.Context, r *connect.Request[product.ListRequest]) (*connect.Response[product.ListResponse], error) {
	p, err := s.auth(r.Header())
	if err != nil {
		return nil, err
	}
	if gatewaycontract.ValidateList(r.Msg) != nil {
		return nil, failure(connect.CodeInvalidArgument)
	}
	if err = bind(p, r.Msg.Access); err != nil {
		return nil, err
	}
	bound := proto.Clone(r.Msg).(*product.ListRequest)
	bound.Cursor = ""
	bound.Access.RequestId = ""
	bound.PageSize = int32(gatewaycontract.PageSize(bound))
	key, _ := json.Marshal(bound)
	after := ""
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Msg.Cursor != "" {
		c, ok := s.cursors[r.Msg.Cursor]
		if !ok || c.key != string(key) || time.Now().After(c.expires) {
			return nil, failure(connect.CodeInvalidArgument)
		}
		after = c.last
	}
	matched := []*product.Product{}
	for _, v := range s.products[gatewaycontract.MerchantID(p)] {
		if v.Id > after && strings.HasPrefix(v.Title, r.Msg.GetFilter().GetTitlePrefix()) && (r.Msg.GetFilter().GetStatus() == 0 || v.Status == r.Msg.Filter.Status) {
			matched = append(matched, proto.Clone(v).(*product.Product))
		}
	}
	out := &product.ListResponse{Products: matched}
	size := gatewaycontract.PageSize(r.Msg)
	if len(matched) > size {
		out.Products = matched[:size]
		out.NextCursor = rand.Text()
		s.cursors[out.NextCursor] = cursor{key: string(key), last: out.Products[size-1].Id, expires: time.Now().Add(15 * time.Minute)}
	}
	return connect.NewResponse(out), nil
}
func TestSuiteAgainstIsolatedHTTPReference(t *testing.T) {
	a := &product.AccessContext{TenantId: "tenant_a", InstallationId: "installation_a", AppId: "different_app", ScopeProfile: accessscope.Profile, GrantRevision: 1, RequestId: "reference-request-0001"}
	b := &product.AccessContext{TenantId: "tenant_b", InstallationId: "installation_b", AppId: "app_b", ScopeProfile: accessscope.Profile, GrantRevision: 1, RequestId: "reference-request-0002"}
	products := []*product.Product{}
	for i, id := range []string{"product_a", "product_b", "product_c"} {
		products = append(products, &product.Product{Id: id, Title: []string{"Alpha", "Beta", "Gamma"}[i], Handle: []string{"alpha", "beta", "gamma"}[i], Status: []product.ProductStatus{product.ProductStatus_PRODUCT_STATUS_ACTIVE, product.ProductStatus_PRODUCT_STATUS_DRAFT, product.ProductStatus_PRODUCT_STATUS_ARCHIVED}[i], UpdatedAt: timestamppb.New(time.Unix(1700000000, 0))})
	}
	products[0].Title = "商品" + strings.Repeat("A", 200)
	s := &reference{a: a, b: b, products: map[string][]*product.Product{a.TenantId: products, b.TenantId: {{Id: "product_foreign", Title: "Foreign", Handle: "foreign", Status: product.ProductStatus_PRODUCT_STATUS_ACTIVE, UpdatedAt: timestamppb.New(time.Unix(1700000000, 0))}}}, cursors: map[string]cursor{}}
	path, handler := productconnect.NewProductServiceHandler(s)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	conformance.Run(t, conformance.Suite{AccessA: a, AccessB: b, ProductsA: products, ProductBID: "product_foreign", MissingID: strings.Repeat("m", 100), Client: func(who conformance.Identity) productconnect.ProductServiceClient {
		return productconnect.NewProductServiceClient(server.Client(), server.URL, connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if who != conformance.Absent {
					req.Header().Set("Authorization", "Bearer "+string(who))
				}
				return next(ctx, req)
			}
		})))
	}})
}
