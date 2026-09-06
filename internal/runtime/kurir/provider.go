package kurir

import (
	"context"
	"net/http"

	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
)

// ReadyProvider only checks the named provider. "Active" belongs to checkout
// selection and is intentionally not an installation requirement. Installed=true
// is not proof of live rates/fulfillment readiness; no execution is enabled here.
func (c *Client) ReadyProvider(ctx context.Context, merchant, installation string, binding appmanifest.ShippingProviderBinding) error {
	if !identity.MatchString(merchant) || !identity.MatchString(installation) || binding.Engine != "api-kurir" {
		return fault.Invalid
	}
	if _, err := appmanifest.ShippingProviderFixture(binding.ProviderCode); err != nil {
		return fault.Invalid
	}
	var result struct {
		Data *struct {
			Code      string `json:"code"`
			Installed *bool  `json:"installed"`
			Available *bool  `json:"available"`
			BuiltIn   *bool  `json:"built_in"`
		} `json:"data"`
	}
	if err := c.call(ctx, merchant, http.MethodGet, "/api/v1/integrations/providers/"+binding.ProviderCode, nil, &result); err != nil {
		return err
	}
	// Only the engine's canonical Emisell provider may use built-in readiness.
	// This never authorizes a third-party app to borrow platform credentials.
	wantBuiltIn := binding.ProviderCode == "emisell"
	if result.Data == nil || result.Data.Code != binding.ProviderCode || result.Data.Installed == nil || result.Data.Available == nil || result.Data.BuiltIn == nil || *result.Data.BuiltIn != wantBuiltIn {
		return fault.Unavailable
	}
	if !*result.Data.Installed || !*result.Data.Available {
		return fault.Conflict
	}
	return nil
}
