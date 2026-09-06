package webhook

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/recovery"
	"emisell.app/platform/pkg/appmanifest"
	"emisell.app/platform/pkg/sdk/events"
	"errors"
	"slices"
	"time"
)

type Summary struct {
	Pending   int64 `json:"pending"`
	Delivered int64 `json:"delivered"`
	Dead      int64 `json:"dead"`
	Cancelled int64 `json:"cancelled"`
}
type Record struct {
	ID             string     `json:"id"`
	InstallationID string     `json:"installationId"`
	EventID        string     `json:"eventId"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	Revision       int64      `json:"revision"`
	EnqueuedAt     time.Time  `json:"enqueuedAt"`
	NextAt         *time.Time `json:"nextAt"`
	LastAttemptAt  *time.Time `json:"lastAttemptAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	LastError      string     `json:"lastError"`
}
type Audit struct {
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	ActorID    string    `json:"actorId"`
	OccurredAt time.Time `json:"occurredAt"`
}
type Detail struct {
	Record
	EventType     string  `json:"eventType"`
	CorrelationID string  `json:"correlationId"`
	History       []Audit `json:"history"`
	CanRetry      bool    `json:"canRetry"`
}
type Page struct {
	Items      []Record `json:"items"`
	NextCursor string   `json:"nextCursor"`
	Summary    Summary  `json:"summary"`
}
type MonitorRepository interface {
	Page(context.Context, string, string, string) (Page, error)
	Detail(context.Context, string, string) (Detail, error)
	RequestReplay(context.Context, string, string, string, string, recovery.Request) (recovery.Result, error)
}
type LifecycleGate interface {
	WithInstallation(context.Context, string, string, func(domain.Installation) error) error
}
type Registry interface {
	Get(context.Context, string) (appmanifest.Manifest, error)
}
type Monitor struct {
	Repo MonitorRepository
	Auth identity.Authorizer
	Gate LifecycleGate
	Apps Registry
}

func (m Monitor) List(ctx context.Context, user, tenant, status, cursor string) (Page, error) {
	if err := m.Auth.Authorize(ctx, user, tenant); err != nil {
		return Page{}, err
	}
	if !slices.Contains([]string{"", "pending", "delivered", "dead", "cancelled"}, status) || (cursor != "" && !events.Token.MatchString(cursor)) {
		return Page{}, fault.Invalid
	}
	return m.Repo.Page(ctx, tenant, status, cursor)
}
func (m Monitor) active(ctx context.Context, ins domain.Installation) error {
	if ins.Status != "active" {
		return fault.Conflict
	}
	app, err := m.Apps.Get(ctx, ins.AppID)
	if err != nil {
		return err
	}
	if app.ExecutionProfile != "local-remote" || ins.Version != app.Version || !slices.Equal(ins.Scopes, app.Scopes) {
		return fault.Conflict
	}
	return nil
}
func (m Monitor) Get(ctx context.Context, user, tenant, id string) (Detail, error) {
	if err := m.Auth.Authorize(ctx, user, tenant); err != nil {
		return Detail{}, err
	}
	if !events.Token.MatchString(id) {
		return Detail{}, fault.Invalid
	}
	d, err := m.Repo.Detail(ctx, tenant, id)
	if err != nil {
		return d, err
	}
	if d.Status == "dead" {
		err = m.Gate.WithInstallation(ctx, tenant, d.InstallationID, func(ins domain.Installation) error { return m.active(ctx, ins) })
		if err == nil {
			d.CanRetry = true
		} else if !errors.Is(err, fault.NotFound) && !errors.Is(err, fault.Conflict) {
			return d, err
		}
	}
	return d, nil
}
func (m Monitor) Retry(ctx context.Context, user, tenant, id, key string, request recovery.Request) (recovery.Result, error) {
	if err := m.Auth.Authorize(ctx, user, tenant); err != nil {
		return recovery.Result{}, err
	}
	if err := request.Validate(key); err != nil {
		return recovery.Result{}, err
	}
	if !events.Token.MatchString(id) {
		return recovery.Result{}, fault.Invalid
	}
	d, err := m.Repo.Detail(ctx, tenant, id)
	if err != nil {
		return recovery.Result{}, err
	}
	var result recovery.Result
	err = m.Gate.WithInstallation(ctx, tenant, d.InstallationID, func(ins domain.Installation) error {
		if err := m.active(ctx, ins); err != nil {
			return err
		}
		var err error
		result, err = m.Repo.RequestReplay(ctx, user, tenant, id, key, request)
		return err
	})
	return result, err
}
