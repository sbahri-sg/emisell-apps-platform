// Package conformance provides opt-in product gateway tests for Core implementers.
// Call Run from an isolated backend integration test, never against production.
package conformance

import (
	"connectrpc.com/connect"
	"context"
	"emisell.app/platform/pkg/gatewaycontract"
	product "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1"
	productconnect "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1/productv1connect"
	"google.golang.org/protobuf/proto"
	"slices"
	"strings"
	"testing"
	"time"
)

type Identity string

const (
	ReadA      Identity = "read_a"
	ReadB      Identity = "read_b"
	WriteA     Identity = "write_a"
	NoScopeA   Identity = "no_scope_a"
	Invalid    Identity = "invalid"
	Absent     Identity = "absent"
	RevokedA   Identity = "revoked_a"
	StaleA     Identity = "stale_a"
	SuspendedA Identity = "suspended_a"
)

// Client must issue actual test credentials/state for every Identity; it must
// not fake success/error responses. A/B are separate tenants + installations.
// ProductsA is the entire stable A dataset (>=3 rows, >=2 statuses), sorted by ID.
type Suite struct {
	Client                func(Identity) productconnect.ProductServiceClient
	AccessA, AccessB      *product.AccessContext
	ProductsA             []*product.Product
	ProductBID, MissingID string
}

func Run(t *testing.T, s Suite) {
	t.Helper()
	if s.Client == nil || gatewaycontract.ValidateAccess(s.AccessA) != nil || gatewaycontract.ValidateAccess(s.AccessB) != nil || gatewaycontract.MerchantID(s.AccessA) == gatewaycontract.MerchantID(s.AccessB) || s.AccessA.InstallationId == s.AccessB.InstallationId || len(s.ProductsA) < 3 || s.ProductBID == s.MissingID || gatewaycontract.ValidateGet(&product.GetRequest{Access: s.AccessB, ProductId: s.ProductBID}) != nil || gatewaycontract.ValidateGet(&product.GetRequest{Access: s.AccessA, ProductId: s.MissingID}) != nil {
		t.Fatal("invalid isolated conformance fixtures")
	}
	statuses := map[product.ProductStatus]bool{}
	for i, p := range s.ProductsA {
		if gatewaycontract.ValidateProduct(p) != nil || (i > 0 && p.Id <= s.ProductsA[i-1].Id) || p.Id == s.ProductBID || p.Id == s.MissingID {
			t.Fatal("invalid product fixtures")
		}
		statuses[p.Status] = true
	}
	if len(statuses) < 2 {
		t.Fatal("fixture must exercise status filtering")
	}
	access := func(a *product.AccessContext) *product.AccessContext {
		out := proto.Clone(a).(*product.AccessContext)
		out.MerchantId, out.TenantId = gatewaycontract.MerchantID(a), ""
		return out
	}
	ctx := func(t *testing.T) context.Context {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		return c
	}
	expectCode := func(t *testing.T, err error, want connect.Code) {
		t.Helper()
		if err == nil || connect.CodeOf(err) != want {
			t.Fatalf("error code got %v, want %v", connect.CodeOf(err), want)
		}
	}
	get := func(a *product.AccessContext, id string) *product.GetRequest {
		return &product.GetRequest{Access: access(a), ProductId: id}
	}
	for _, identity := range []Identity{ReadA, WriteA} {
		t.Run(string(identity)+"/get", func(t *testing.T) {
			r := get(s.AccessA, s.ProductsA[0].Id)
			out, err := s.Client(identity).Get(ctx(t), connect.NewRequest(r))
			if err != nil || out == nil || gatewaycontract.ValidateGetResponse(r, out.Msg) != nil || !proto.Equal(out.Msg.Product, s.ProductsA[0]) {
				t.Fatal("read/implied-write product mismatch", err)
			}
		})
	}
	for _, tc := range []struct {
		identity Identity
		code     connect.Code
	}{{Absent, connect.CodeUnauthenticated}, {Invalid, connect.CodeUnauthenticated}, {NoScopeA, connect.CodePermissionDenied}, {RevokedA, connect.CodePermissionDenied}, {StaleA, connect.CodePermissionDenied}, {SuspendedA, connect.CodePermissionDenied}} {
		t.Run("deny/"+string(tc.identity), func(t *testing.T) {
			_, err := s.Client(tc.identity).Get(ctx(t), connect.NewRequest(get(s.AccessA, s.ProductsA[0].Id)))
			expectCode(t, err, tc.code)
			_, err = s.Client(tc.identity).List(ctx(t), connect.NewRequest(&product.ListRequest{Access: access(s.AccessA)}))
			expectCode(t, err, tc.code)
		})
	}
	for _, header := range []string{"Cookie", "Origin"} {
		t.Run("reject-browser/"+header, func(t *testing.T) {
			r := connect.NewRequest(get(s.AccessA, s.ProductsA[0].Id))
			r.Header().Set(header, "test-untrusted-browser")
			_, err := s.Client(ReadA).Get(ctx(t), r)
			expectCode(t, err, connect.CodePermissionDenied)
		})
	}
	for _, field := range []string{"merchant", "installation", "app", "revision"} {
		t.Run("context-binding/"+field, func(t *testing.T) {
			r := get(s.AccessA, s.ProductsA[0].Id)
			switch field {
			case "merchant":
				r.Access.MerchantId = gatewaycontract.MerchantID(s.AccessB)
			case "installation":
				r.Access.InstallationId = s.AccessB.InstallationId
			case "app":
				r.Access.AppId = "different_app"
				if r.Access.AppId == s.AccessA.AppId {
					r.Access.AppId = "another_app"
				}
			case "revision":
				r.Access.GrantRevision++
			}
			_, err := s.Client(ReadA).Get(ctx(t), connect.NewRequest(r))
			expectCode(t, err, connect.CodePermissionDenied)
		})
	}
	for _, id := range []string{s.ProductBID, s.MissingID} {
		t.Run("not-found/"+id, func(t *testing.T) {
			_, err := s.Client(ReadA).Get(ctx(t), connect.NewRequest(get(s.AccessA, id)))
			expectCode(t, err, connect.CodeNotFound)
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*product.ListRequest)
	}{
		{"negative-page", func(r *product.ListRequest) { r.PageSize = -1 }},
		{"large-page", func(r *product.ListRequest) { r.PageSize = 101 }},
		{"large-cursor", func(r *product.ListRequest) { r.Cursor = strings.Repeat("x", 2049) }},
		{"bad-status", func(r *product.ListRequest) { r.Filter = &product.ProductFilter{Status: 999} }},
		{"long-prefix", func(r *product.ListRequest) { r.Filter = &product.ProductFilter{TitlePrefix: strings.Repeat("x", 101)} }},
		{"missing-access", func(r *product.ListRequest) { r.Access = nil }},
		{"unknown-profile", func(r *product.ListRequest) { r.Access.ScopeProfile = "unknown" }},
	} {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			r := &product.ListRequest{Access: access(s.AccessA)}
			tc.mutate(r)
			_, err := s.Client(ReadA).List(ctx(t), connect.NewRequest(r))
			expectCode(t, err, connect.CodeInvalidArgument)
		})
	}
	t.Run("pagination-and-filter", func(t *testing.T) {
		// Product titles can exceed the filter's 100-byte limit. A single rune
		// remains valid UTF-8 and bounded even for maximum-length fixture titles.
		prefix := string([]rune(s.ProductsA[0].Title)[:1])
		for _, filter := range []*product.ProductFilter{nil, {Status: s.ProductsA[0].Status}, {TitlePrefix: prefix}, {TitlePrefix: "conformance-no-match-prefix"}} {
			r := &product.ListRequest{Access: access(s.AccessA), PageSize: 1, Filter: filter}
			want := []string{}
			for _, p := range s.ProductsA {
				if strings.HasPrefix(p.Title, filter.GetTitlePrefix()) && (filter.GetStatus() == 0 || p.Status == filter.Status) {
					want = append(want, p.Id)
				}
			}
			got := []string{}
			seen := map[string]bool{}
			for page := 0; page <= len(s.ProductsA); page++ {
				out, err := s.Client(ReadA).List(ctx(t), connect.NewRequest(r))
				if err != nil || out == nil || gatewaycontract.ValidateListResponse(r, out.Msg) != nil {
					t.Fatal("invalid list response", err)
				}
				for _, p := range out.Msg.Products {
					got = append(got, p.Id)
				}
				if out.Msg.NextCursor == "" {
					break
				}
				if seen[out.Msg.NextCursor] {
					t.Fatal("cursor cycle")
				}
				seen[out.Msg.NextCursor] = true
				r.Cursor = out.Msg.NextCursor
			}
			if !slices.Equal(got, want) {
				t.Fatalf("tenant/filter/page mismatch: got %v want %v", got, want)
			}
		}
	})
	t.Run("cursor-binding", func(t *testing.T) {
		r := &product.ListRequest{Access: access(s.AccessA), PageSize: 1}
		out, err := s.Client(ReadA).List(ctx(t), connect.NewRequest(r))
		if err != nil || out == nil || out.Msg.NextCursor == "" {
			t.Fatal("fixture must paginate", err)
		}
		for _, mode := range []string{"tamper", "filter", "page-size", "tenant"} {
			t.Run(mode, func(t *testing.T) {
				n := proto.Clone(r).(*product.ListRequest)
				n.Cursor = out.Msg.NextCursor
				who := ReadA
				switch mode {
				case "tamper":
					n.Cursor += "x"
				case "filter":
					n.Filter = &product.ProductFilter{TitlePrefix: "changed"}
				case "page-size":
					n.PageSize = 2
				case "tenant":
					n.Access = access(s.AccessB)
					who = ReadB
				}
				_, err := s.Client(who).List(ctx(t), connect.NewRequest(n))
				expectCode(t, err, connect.CodeInvalidArgument)
			})
		}
	})
}
