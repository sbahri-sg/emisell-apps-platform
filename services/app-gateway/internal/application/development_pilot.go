package application

import "emisell-app-platform/services/app-gateway/internal/domain"

// DevelopmentResourcePilot only changes test-install eligibility, never catalog publication.
// The process configuration may populate it only in development with the resource adapter enabled.
type DevelopmentResourcePilot struct {
	ReadProductsMerchants map[string]bool
}

func (p DevelopmentResourcePilot) unavailable(scopes []domain.SnapshotScope, merchant domain.MerchantIdentity) string {
	for _, scope := range scopes {
		definition, known := LookupOfficialScope(scope.Scope)
		if known && definition.Availability == domain.ScopeAvailabilityAvailable {
			continue
		}
		if scope.Scope == "read_products" && merchant.Environment == domain.EnvironmentSandbox && p.ReadProductsMerchants[merchant.MerchantID] {
			continue
		}
		return scope.Scope
	}
	return ""
}
