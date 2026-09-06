package gatewaycontract

import (
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/merchantid"
	product "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid product contract")
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var requestPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
var handlePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var cursorPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]*$`)

// ValidateAccess validates shape only. It MUST NOT replace authentication,
// delegation binding, current-grant lookup, operation support or authorization.
func ValidateAccess(a *product.AccessContext) error {
	if a == nil {
		return ErrInvalid
	}
	id, err := merchantid.Resolve(a.MerchantId, a.TenantId)
	if err != nil || !idPattern.MatchString(id) || !idPattern.MatchString(a.InstallationId) || !idPattern.MatchString(a.AppId) || a.ScopeProfile != accessscope.Profile || a.GrantRevision < 1 || !requestPattern.MatchString(a.RequestId) {
		return ErrInvalid
	}
	return nil
}

// MerchantID resolves one identity without mutating the request. ValidateAccess
// must succeed before this value is used for delegation or data queries.
func MerchantID(a *product.AccessContext) string {
	if a == nil {
		return ""
	}
	id, _ := merchantid.Resolve(a.MerchantId, a.TenantId)
	return id
}
func validText(s string, max int) bool {
	return len(s) <= max && utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl)
}
func validCursor(s string) bool { return len(s) <= 2048 && cursorPattern.MatchString(s) }
func validStatus(s product.ProductStatus, optional bool) bool {
	return (optional && s == product.ProductStatus_PRODUCT_STATUS_UNSPECIFIED) || s == product.ProductStatus_PRODUCT_STATUS_DRAFT || s == product.ProductStatus_PRODUCT_STATUS_ACTIVE || s == product.ProductStatus_PRODUCT_STATUS_ARCHIVED
}
func PageSize(r *product.ListRequest) int {
	if r.GetPageSize() == 0 {
		return 25
	}
	return int(r.GetPageSize())
}
func ValidateList(r *product.ListRequest) error {
	if r == nil || ValidateAccess(r.Access) != nil || r.PageSize < 0 || r.PageSize > 100 || !validCursor(r.Cursor) || !validText(r.GetFilter().GetTitlePrefix(), 100) || !validStatus(r.GetFilter().GetStatus(), true) {
		return ErrInvalid
	}
	return nil
}
func ValidateGet(r *product.GetRequest) error {
	if r == nil || ValidateAccess(r.Access) != nil || !idPattern.MatchString(r.ProductId) {
		return ErrInvalid
	}
	return nil
}
func ValidateProduct(p *product.Product) error {
	if p == nil || !idPattern.MatchString(p.Id) || strings.TrimSpace(p.Title) == "" || !validText(p.Title, 255) || len(p.Handle) > 100 || !handlePattern.MatchString(p.Handle) || !validStatus(p.Status, false) || p.UpdatedAt == nil || p.UpdatedAt.CheckValid() != nil {
		return ErrInvalid
	}
	return nil
}
func ValidateListResponse(r *product.ListRequest, out *product.ListResponse) error {
	if ValidateList(r) != nil || out == nil || len(out.Products) > PageSize(r) || !validCursor(out.NextCursor) || (out.NextCursor != "" && (out.NextCursor == r.Cursor || len(out.Products) == 0)) {
		return ErrInvalid
	}
	previous := ""
	for _, p := range out.Products {
		if ValidateProduct(p) != nil || (previous != "" && p.Id <= previous) || !strings.HasPrefix(p.Title, r.GetFilter().GetTitlePrefix()) || (r.GetFilter().GetStatus() != product.ProductStatus_PRODUCT_STATUS_UNSPECIFIED && p.Status != r.Filter.Status) {
			return ErrInvalid
		}
		previous = p.Id
	}
	return nil
}
func ValidateGetResponse(r *product.GetRequest, out *product.GetResponse) error {
	if ValidateGet(r) != nil || out == nil || ValidateProduct(out.Product) != nil || out.Product.Id != r.ProductId {
		return ErrInvalid
	}
	return nil
}
