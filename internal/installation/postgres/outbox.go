package postgres

import (
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/platform/outboxpg"
)

// Outbox exports this module's delivery port, not its tables.
func (p Repository) Outbox() event.Outbox {
	return outboxpg.Store{Pool: p.Pool, Schema: "platform_installation"}
}
