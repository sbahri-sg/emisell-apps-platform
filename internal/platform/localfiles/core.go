package localfiles

import (
	"emisell.app/platform/internal/event/natsbus"
	"time"
)

type CoreCredential struct {
	ServiceID string              `json:"serviceId"`
	TenantID  string              `json:"tenantId"`
	Address   string              `json:"address"`
	Token     string              `json:"token"`
	ExpiresAt time.Time           `json:"expiresAt"`
	Broker    natsbus.Credentials `json:"broker"`
}
