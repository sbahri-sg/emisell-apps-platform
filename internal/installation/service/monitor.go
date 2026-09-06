package service

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/recovery"
	"emisell.app/platform/pkg/sdk/events"
	"time"
)

type CleanupAudit struct {
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	ActorID    string    `json:"actorId"`
	OccurredAt time.Time `json:"occurredAt"`
}
type ConnectionRecord struct {
	InstallationID   string         `json:"installationId"`
	AppID            string         `json:"appId"`
	Name             string         `json:"name"`
	Status           string         `json:"status"`
	ConnectionStatus string         `json:"connectionStatus"`
	CleanupAttempts  int            `json:"cleanupAttempts"`
	CleanupNextAt    *time.Time     `json:"cleanupNextAt"`
	Revision         int64          `json:"revision"`
	CanRetryCleanup  bool           `json:"canRetryCleanup"`
	History          []CleanupAudit `json:"history"`
}
type MonitorRepository interface {
	ConnectionRecords(context.Context, string) ([]ConnectionRecord, error)
	RequestCleanup(context.Context, string, string, string, string, recovery.Request) (recovery.Result, error)
}
type Monitor struct {
	Repo        MonitorRepository
	Auth        identity.Authorizer
	Apps        Registry
	Connections Connections
}

// List reports stored connection state only. Reads must not refresh tokens or contact providers.
func (m Monitor) List(ctx context.Context, user, tenant string) ([]ConnectionRecord, error) {
	if err := m.Auth.Authorize(ctx, user, tenant); err != nil {
		return nil, err
	}
	rows, err := m.Repo.ConnectionRecords(ctx, tenant)
	if err != nil {
		return nil, err
	}
	out := []ConnectionRecord{}
	for _, r := range rows {
		app, err := m.Apps.Get(ctx, r.AppID)
		if err != nil {
			return nil, err
		}
		if app.ExecutionProfile != "local-remote" {
			continue
		}
		r.Name = app.Name
		r.ConnectionStatus = "unavailable"
		if m.Connections != nil {
			r.ConnectionStatus, err = m.Connections.Status(ctx, tenant, r.InstallationID)
			if err != nil {
				return nil, err
			}
		}
		r.CanRetryCleanup = r.Status == "disabling" && r.CleanupAttempts >= 12
		out = append(out, r)
	}
	return out, nil
}
func (m Monitor) RetryCleanup(ctx context.Context, user, tenant, id, key string, request recovery.Request) (recovery.Result, error) {
	if err := m.Auth.Authorize(ctx, user, tenant); err != nil {
		return recovery.Result{}, err
	}
	if err := request.Validate(key); err != nil {
		return recovery.Result{}, err
	}
	if !events.Token.MatchString(id) {
		return recovery.Result{}, fault.Invalid
	}
	return m.Repo.RequestCleanup(ctx, user, tenant, id, key, request)
}
