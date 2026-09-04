package postgres

import (
	"context"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

const webhookEventDefinitionColumns = `
	event, name, description, source, availability, required_scope, api_version`

func scanWebhookEventDefinition(row rowScanner) (domain.WebhookEventDefinition, error) {
	var definition domain.WebhookEventDefinition
	var source string
	var availability string
	if err := row.Scan(
		&definition.Event, &definition.Name, &definition.Description, &source,
		&availability, &definition.RequiredScope, &definition.APIVersion,
	); err != nil {
		return domain.WebhookEventDefinition{}, mapError(err)
	}
	definition.Source = domain.WebhookEventSource(source)
	definition.Availability = domain.WebhookEventAvailability(availability)
	return definition, nil
}

func (r *Repository) ListWebhookEventDefinitions(ctx context.Context) ([]domain.WebhookEventDefinition, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+webhookEventDefinitionColumns+`
		FROM webhook_event_definitions
		ORDER BY CASE availability WHEN 'available' THEN 0 ELSE 1 END, event`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.WebhookEventDefinition, 0)
	for rows.Next() {
		item, err := scanWebhookEventDefinition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *Repository) GetWebhookEventDefinition(ctx context.Context, event string) (domain.WebhookEventDefinition, error) {
	return scanWebhookEventDefinition(r.pool.QueryRow(ctx, `
		SELECT `+webhookEventDefinitionColumns+`
		FROM webhook_event_definitions WHERE event = $1`, event))
}
