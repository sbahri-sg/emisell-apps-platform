package memory

import (
	"context"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

// developmentWebhookEventDefinitions mirrors migration 000015 for the in-memory
// adapter used by tests. Production always reads the persisted catalog.
func developmentWebhookEventDefinitions() map[string]domain.WebhookEventDefinition {
	definitions := []domain.WebhookEventDefinition{
		{Event: "app/uninstalled", Name: "App uninstalled", Description: "Sent after a merchant disconnects this app and its installation access is revoked.", Source: domain.WebhookEventSourceAppPlatform, Availability: domain.WebhookEventAvailabilityAvailable, APIVersion: "2026-09-01"},
		plannedWebhookDefinition("products/created", "Product created", "Sent when a product is created in the connected Emisell merchant account.", "read_products"),
		plannedWebhookDefinition("products/updated", "Product updated", "Sent when a product changes in the connected Emisell merchant account.", "read_products"),
		plannedWebhookDefinition("products/deleted", "Product deleted", "Sent when a product is deleted from the connected Emisell merchant account.", "read_products"),
		plannedWebhookDefinition("inventory/updated", "Inventory updated", "Sent when inventory availability changes for a connected merchant.", "read_inventory"),
		plannedWebhookDefinition("orders/created", "Order created", "Sent when an order is created for the connected Emisell merchant account.", "read_orders"),
		plannedWebhookDefinition("orders/updated", "Order updated", "Sent when an order changes for the connected Emisell merchant account.", "read_orders"),
		plannedWebhookDefinition("orders/cancelled", "Order cancelled", "Sent when an order is cancelled for the connected Emisell merchant account.", "read_orders"),
		plannedWebhookDefinition("customers/created", "Customer created", "Sent when a customer is created in the connected Emisell merchant account.", "read_customers"),
		plannedWebhookDefinition("customers/updated", "Customer updated", "Sent when a customer profile changes in the connected Emisell merchant account.", "read_customers"),
		plannedWebhookDefinition("customers/deleted", "Customer deleted", "Sent when a customer is deleted from the connected Emisell merchant account.", "read_customers"),
		plannedWebhookDefinition("fulfillments/updated", "Fulfillment updated", "Sent when fulfillment, shipment, tracking, or delivery state changes.", "read_fulfillments"),
	}
	items := make(map[string]domain.WebhookEventDefinition, len(definitions))
	for _, definition := range definitions {
		items[definition.Event] = definition
	}
	return items
}

func plannedWebhookDefinition(event, name, description, scope string) domain.WebhookEventDefinition {
	return domain.WebhookEventDefinition{
		Event: event, Name: name, Description: description,
		Source: domain.WebhookEventSourceEmisellBackend, Availability: domain.WebhookEventAvailabilityPlanned,
		RequiredScope: &scope, APIVersion: "2026-09-01",
	}
}

func (r *Repository) ListWebhookEventDefinitions(_ context.Context) ([]domain.WebhookEventDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.WebhookEventDefinition, 0, len(r.webhookEventDefinitions))
	for _, definition := range r.webhookEventDefinitions {
		items = append(items, definition)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Availability != items[j].Availability {
			return items[i].Availability == domain.WebhookEventAvailabilityAvailable
		}
		return items[i].Event < items[j].Event
	})
	return items, nil
}

func (r *Repository) GetWebhookEventDefinition(_ context.Context, event string) (domain.WebhookEventDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definition, ok := r.webhookEventDefinitions[event]
	if !ok {
		return domain.WebhookEventDefinition{}, domain.ErrNotFound
	}
	return definition, nil
}
