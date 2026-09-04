package application

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

func TestExtensionCatalogSeparatesMetadataFromExecution(t *testing.T) {
	catalog := OfficialExtensionCatalog()
	if len(catalog.Categories) != 6 || len(catalog.Families) != 3 || len(catalog.Capabilities) != 7 {
		t.Fatal("unexpected catalog shape")
	}
	for _, category := range catalog.Categories {
		if !slices.Contains([]domain.CatalogCategory{domain.CatalogCategoryPayment, domain.CatalogCategoryShipping, domain.CatalogCategoryERP, domain.CatalogCategoryMarketing, domain.CatalogCategoryOperations, domain.CatalogCategoryCustom}, category.ID) {
			t.Fatal("invented listing category")
		}
	}
	for _, family := range catalog.Families {
		if !validExtensionType(family.Type) {
			t.Fatalf("registry family not accepted: %s", family.Type)
		}
	}
	seen := map[string]bool{}
	for _, capability := range catalog.Capabilities {
		if seen[capability.ID] {
			t.Fatal("duplicate capability")
		}
		seen[capability.ID] = true
		if validExtensionType(domain.ExtensionType(capability.ID)) {
			t.Fatal("capability ID accepted as family")
		}
		if _, ok := LookupOfficialScope(capability.ID); ok {
			t.Fatal("capability ID became an OAuth scope")
		}
		if capability.ID == ShippingRatesCalculate {
			if capability.Availability != "pilot" || capability.ExecutionEnabled || len(capability.Endpoints) != 0 || len(capability.RequiredScopes) != 0 {
				t.Fatalf("shipping rate pilot capability is unsafe: %+v", capability)
			}
		} else if capability.Type == domain.ExtensionTypePayment || capability.Type == domain.ExtensionTypeShipping {
			if capability.Availability != "planned" || capability.ExecutionEnabled || len(capability.Endpoints) != 0 || len(capability.RequiredScopes) != 0 {
				t.Fatal("unimplemented execution advertised as active")
			}
		}
		if capability.Availability != "available" && capability.ExecutionEnabled {
			t.Fatal("pilot/planned enabled globally")
		}
		for _, name := range capability.RequiredScopes {
			scope, ok := LookupOfficialScope(name)
			if !ok || (capability.Availability == "available" && scope.Availability != domain.ScopeAvailabilityAvailable) {
				t.Fatalf("scope mismatch: %s", name)
			}
		}
	}
	for _, surface := range catalog.Surfaces {
		if surface.ID != "server_only" && surface.Availability != "planned" {
			t.Fatal("unimplemented UI runtime advertised as active")
		}
	}
	runtime := catalog.RuntimeContract
	if runtime.Version != "v1-draft" || runtime.Status != "draft" || runtime.ExecutionEnabled || runtime.TokenTTLSeconds != 60 {
		t.Fatal("draft runtime contract advertised with unsafe state")
	}
	expectedClaims := []string{"merchant_id", "installation_id", "extension_id", "version_id", "environment", "operation"}
	if !slices.Equal(runtime.IdentityClaims, expectedClaims) {
		t.Fatal("runtime identity binding drift")
	}
	capabilities := map[string]domain.AppCapabilityDefinition{}
	for _, capability := range catalog.Capabilities {
		capabilities[capability.ID] = capability
	}
	operationIDs := map[string]bool{}
	for _, operation := range runtime.Operations {
		if operationIDs[operation.ID] {
			t.Fatalf("duplicate runtime operation: %s", operation.ID)
		}
		operationIDs[operation.ID] = true
		capability, ok := capabilities[operation.CapabilityID]
		if !ok || capability.Invocation != "platform_to_provider" || capability.ExecutionEnabled ||
			(operation.CapabilityID == ShippingRatesCalculate && capability.Availability != "pilot") ||
			(operation.CapabilityID != ShippingRatesCalculate && capability.Availability != "planned") {
			t.Fatalf("runtime operation has no disabled planned capability: %s", operation.ID)
		}
		if operation.Mutation && operation.Idempotency != "required" {
			t.Fatalf("mutation without required idempotency: %s", operation.ID)
		}
		if operation.TimeoutMS <= 0 || operation.TimeoutMS > 20000 || operation.MaximumAttempts != 1 {
			t.Fatalf("unsafe runtime policy: %s", operation.ID)
		}
	}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"runtimeUrl", "clientSecret", "providerKey"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("runtime discovery leaked %s", forbidden)
		}
	}
}

func TestExtensionCatalogReturnsIndependentCopies(t *testing.T) {
	catalog := OfficialExtensionCatalog()
	catalog.Categories[0].Examples[0] = "changed"
	catalog.Families[0].ConfigurationSupported = false
	catalog.Capabilities[0].RequiredScopes[0] = "read_everything"
	catalog.Capabilities[0].Endpoints[0].Path = "/fake"
	catalog.RuntimeContract.IdentityClaims[0] = "caller_selected_merchant"
	catalog.RuntimeContract.Operations[0].MaximumAttempts = 99
	fresh := OfficialExtensionCatalog()
	if fresh.Categories[0].Examples[0] == "changed" || !fresh.Families[0].ConfigurationSupported || fresh.Capabilities[0].RequiredScopes[0] != "read_merchant" || fresh.Capabilities[0].Endpoints[0].Path != "/v1/merchant/profile" || fresh.RuntimeContract.IdentityClaims[0] != "merchant_id" || fresh.RuntimeContract.Operations[0].MaximumAttempts != 1 {
		t.Fatal("caller mutated registry")
	}
}
