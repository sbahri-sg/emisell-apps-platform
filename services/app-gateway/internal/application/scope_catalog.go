package application

import "emisell-app-platform/services/app-gateway/internal/domain"

var officialScopes = []domain.ScopeDefinition{
	{
		Scope: "read_merchant", Name: "View merchant profile",
		Description: "Read the connected store identity, display name, primary domain, environment, and installation identifier.",
		Access:      "read", Risk: domain.ScopeRiskLow, DataClassification: "store_metadata", Approval: "merchant_consent",
		Availability: domain.ScopeAvailabilityAvailable, Resources: []string{"merchant"},
		Endpoints: []domain.ScopeEndpoint{{Method: "GET", Path: "/v1/merchant/profile"}}, RequiredForWebhooks: []string{},
	},
	{
		Scope: "read_products", Name: "View products",
		Description: "Read base product fields through the operator-enabled pilot. Disabled by default; general availability, variants, and resource webhooks are pending.",
		Access:      "read", Risk: domain.ScopeRiskStandard, DataClassification: "merchant_catalog", Approval: "merchant_consent",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"products"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{"products/created", "products/updated", "products/deleted"},
	},
	{
		Scope: "write_products", Name: "Manage products",
		Description: "Create, update, publish, archive, or delete products and variants.",
		Access:      "write", Risk: domain.ScopeRiskHigh, DataClassification: "merchant_catalog", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"products", "variants"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{},
	},
	{
		Scope: "read_inventory", Name: "View inventory",
		Description: "Read inventory quantities and availability by product variant and store location.",
		Access:      "read", Risk: domain.ScopeRiskStandard, DataClassification: "merchant_operations", Approval: "merchant_consent",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"inventory", "locations"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{"inventory/updated"},
	},
	{
		Scope: "write_inventory", Name: "Manage inventory",
		Description: "Adjust inventory quantities for product variants at authorized store locations.",
		Access:      "write", Risk: domain.ScopeRiskHigh, DataClassification: "merchant_operations", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"inventory", "locations"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{},
	},
	{
		Scope: "read_orders", Name: "View orders",
		Description: "Read orders, line items, totals, payment state, fulfillment state, and the minimum customer data needed for the order.",
		Access:      "read", Risk: domain.ScopeRiskSensitive, DataClassification: "orders_and_limited_pii", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"orders", "order_items"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{"orders/created", "orders/updated", "orders/cancelled"},
	},
	{
		Scope: "write_orders", Name: "Manage orders",
		Description: "Create or update supported order fields. Payment capture and refunds are excluded.",
		Access:      "write", Risk: domain.ScopeRiskHigh, DataClassification: "orders_and_limited_pii", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"orders", "order_items"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{},
	},
	{
		Scope: "read_customers", Name: "View customers",
		Description: "Read customer profiles and contact data required by an approved app use case.",
		Access:      "read", Risk: domain.ScopeRiskSensitive, DataClassification: "personal_data", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"customers"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{"customers/created", "customers/updated", "customers/deleted"},
	},
	{
		Scope: "write_customers", Name: "Manage customers",
		Description: "Create or update customer profiles and approved contact fields.",
		Access:      "write", Risk: domain.ScopeRiskHigh, DataClassification: "personal_data", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"customers"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{},
	},
	{
		Scope: "read_fulfillments", Name: "View fulfillments",
		Description: "Read fulfillment, shipment, tracking, and delivery status for merchant orders.",
		Access:      "read", Risk: domain.ScopeRiskSensitive, DataClassification: "merchant_operations_and_delivery_data", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"fulfillments", "shipments", "tracking"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{"fulfillments/updated"},
	},
	{
		Scope: "write_fulfillments", Name: "Manage fulfillments",
		Description: "Create fulfillment updates, attach tracking, or perform supported fulfillment actions.",
		Access:      "write", Risk: domain.ScopeRiskHigh, DataClassification: "merchant_operations_and_delivery_data", Approval: "merchant_consent_and_emisell_review",
		Availability: domain.ScopeAvailabilityPlanned, Resources: []string{"fulfillments", "shipments", "tracking"}, Endpoints: []domain.ScopeEndpoint{}, RequiredForWebhooks: []string{},
	},
}

func OfficialScopeCatalog() []domain.ScopeDefinition {
	result := make([]domain.ScopeDefinition, len(officialScopes))
	copy(result, officialScopes)
	return result
}

func LookupOfficialScope(scope string) (domain.ScopeDefinition, bool) {
	for _, definition := range officialScopes {
		if definition.Scope == scope {
			return definition, true
		}
	}
	return domain.ScopeDefinition{}, false
}
