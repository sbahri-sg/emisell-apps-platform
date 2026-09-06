// Package appapi is the provider-neutral remote app HTTP wire contract.
package appapi

type Request struct {
	Operation       string `json:"operation"`
	ResourceID      string `json:"resourceId,omitempty"`
	Reference       string `json:"reference,omitempty"`
	AmountMinor     int64  `json:"amountMinor,omitempty"`
	Currency        string `json:"currency,omitempty"`
	WeightGrams     int    `json:"weightGrams,omitempty"`
	DestinationZone string `json:"destinationZone,omitempty"`
	OriginZone      string `json:"originZone,omitempty"`
}
type Resource struct {
	ID          string `json:"id"`
	Reference   string `json:"reference"`
	Status      string `json:"status"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	// Revision is monotonic per resource; absent on legacy resources.
	Revision int64 `json:"revision,omitempty"`
}
type Rate struct {
	Service     string `json:"service"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}
type Response struct {
	Capability     string    `json:"capability"`
	Operation      string    `json:"operation"`
	InstallationID string    `json:"installationId"`
	Simulation     bool      `json:"simulation"`
	Resource       *Resource `json:"resource,omitempty"`
	Rates          []Rate    `json:"rates,omitempty"`
}
type Invocation struct {
	TenantID       string  `json:"tenantId"`
	InstallationID string  `json:"installationId"`
	Capability     string  `json:"capability"`
	IdempotencyKey string  `json:"idempotencyKey"`
	Request        Request `json:"request"`
}
