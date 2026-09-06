package gatewaycontract

import (
	"emisell.app/platform/pkg/accessscope"
	product "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strings"
	"testing"
	"time"
)

func testAccess() *product.AccessContext {
	return &product.AccessContext{TenantId: "tenant_a", InstallationId: "install_a", AppId: "app_a", ScopeProfile: accessscope.Profile, GrantRevision: 1, RequestId: "contract-request-0001"}
}

func TestMerchantOnlyAccessDoesNotRequireOrInventSecondID(t *testing.T) {
	a := testAccess()
	a.MerchantId, a.TenantId = "merchant_a", ""
	if ValidateAccess(a) != nil || MerchantID(a) != "merchant_a" || a.TenantId != "" {
		t.Fatal("canonical identity requires legacy alias or mutates input")
	}
	a.TenantId = "merchant_b"
	if ValidateAccess(a) == nil || MerchantID(a) != "" {
		t.Fatal("ambiguous merchant identity accepted")
	}
	a.TenantId = "merchant_a"
	if ValidateAccess(a) != nil {
		t.Fatal("equal compatibility alias rejected")
	}
	a.MerchantId, a.TenantId = "", ""
	if ValidateAccess(a) == nil {
		t.Fatal("missing merchant accepted")
	}
}
func testProduct() *product.Product {
	return &product.Product{Id: "product_a", Title: "Produk A", Handle: "produk-a", Status: product.ProductStatus_PRODUCT_STATUS_ACTIVE, UpdatedAt: timestamppb.New(time.Unix(1700000000, 0))}
}
func TestContractCoverage(t *testing.T) {
	h := Reference()
	if h.Live || len(h.Coverage) != 108 || len(h.Operations) != 2 {
		t.Fatal("handoff metadata drift")
	}
	methods := product.File_emisell_resource_product_v1_product_proto.Services().Get(0).Methods()
	for _, op := range h.Operations {
		if op.Implementation != "planned" || op.RequiredScope != "read_products" || methods.ByName(protoreflect.Name(op.Procedure[strings.LastIndex(op.Procedure, "/")+1:])) == nil {
			t.Fatal("operation drift")
		}
	}
	mapped := 0
	for _, c := range h.Coverage {
		if c.Grantable || c.Implementation != "planned" {
			t.Fatal("reference advertised active")
		}
		if len(c.Operations) > 0 {
			mapped++
			if c.ContractStatus != "partial" || (c.Scope != "read_products" && c.Scope != "write_products") {
				t.Fatal("scope overclaim")
			}
		}
	}
	if mapped != 2 {
		t.Fatal("mapping missing")
	}
}
func TestProductValidation(t *testing.T) {
	r := &product.ListRequest{Access: testAccess()}
	if ValidateList(r) != nil || PageSize(r) != 25 {
		t.Fatal("default page")
	}
	for _, mutate := range []func(*product.ListRequest){func(r *product.ListRequest) { r.Access = nil }, func(r *product.ListRequest) { r.PageSize = 101 }, func(r *product.ListRequest) { r.PageSize = -1 }, func(r *product.ListRequest) { r.Filter = &product.ProductFilter{Status: 99} }, func(r *product.ListRequest) { r.Filter = &product.ProductFilter{TitlePrefix: "bad\n"} }, func(r *product.ListRequest) { r.Cursor = strings.Repeat("x", 2049) }, func(r *product.ListRequest) { r.Access.ScopeProfile = "unknown" }, func(r *product.ListRequest) { r.Access.RequestId = "short" }} {
		bad := proto.Clone(r).(*product.ListRequest)
		mutate(bad)
		if ValidateList(bad) == nil {
			t.Fatal("invalid request accepted")
		}
	}
	p := testProduct()
	out := &product.ListResponse{Products: []*product.Product{p}}
	if ValidateListResponse(r, out) != nil || ValidateGetResponse(&product.GetRequest{Access: testAccess(), ProductId: p.Id}, &product.GetResponse{Product: p}) != nil {
		t.Fatal("valid projection rejected")
	}
	if ValidateListResponse(r, &product.ListResponse{Products: []*product.Product{p, p}}) == nil {
		t.Fatal("duplicate IDs")
	}
	if ValidateListResponse(r, &product.ListResponse{NextCursor: "loop"}) == nil {
		t.Fatal("empty cursor page")
	}
	if ValidateGetResponse(&product.GetRequest{Access: testAccess(), ProductId: "different"}, &product.GetResponse{Product: p}) == nil {
		t.Fatal("wrong resource ID")
	}
	for _, mutate := range []func(*product.Product){func(p *product.Product) { p.UpdatedAt = nil }, func(p *product.Product) { p.Status = 0 }, func(p *product.Product) { p.Title = "" }, func(p *product.Product) { p.Handle = "BAD HANDLE" }} {
		bad := proto.Clone(p).(*product.Product)
		mutate(bad)
		if ValidateProduct(bad) == nil {
			t.Fatal("invalid product accepted")
		}
	}
}
