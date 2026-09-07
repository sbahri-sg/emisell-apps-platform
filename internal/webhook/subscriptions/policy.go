package subscriptions

import "emisell.app/platform/pkg/webhookconfig"

// Compatibility for the earlier pending-request API. New app-specific
// configuration belongs to the versioned app document, not a separate review.
type Topic = webhookconfig.Topic

func Topics() []Topic                            { return webhookconfig.Topics() }
func requiredScope(topic string) (string, error) { return webhookconfig.RequiredScope(topic) }
func ValidEndpoint(raw string) bool              { return webhookconfig.ValidEndpoint(raw) }
