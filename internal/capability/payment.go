package capability

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/appmanifest"
	"emisell.app/platform/pkg/sdk/events"
	"slices"
	"time"
)

type PaymentRecord struct {
	Resource
	InstallationID string     `json:"installationId"`
	ObservedAt     *time.Time `json:"observedAt"`
	StatusRevision int64      `json:"statusRevision"`
}
type PaymentHistory struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Revision   int64     `json:"revision"`
	Source     string    `json:"source"`
	OccurredAt time.Time `json:"occurredAt"`
}
type PaymentDetail struct {
	PaymentRecord
	History []PaymentHistory `json:"history"`
}
type PaymentPage struct {
	Items      []PaymentRecord `json:"items"`
	NextCursor string          `json:"nextCursor"`
}
type PaymentRepository interface {
	Payments(context.Context, string, string, string) (PaymentPage, error)
	Payment(context.Context, string, string) (PaymentDetail, error)
	ApplyCallback(context.Context, string, string, string, string, string, appapi.PaymentUpdate) (string, error)
}
type CallbackGate interface {
	WithInstallation(context.Context, string, string, func(domain.Installation) error) error
}
type CallbackConnections interface {
	VerifyCallback(context.Context, string, string, string, string, string, []byte) error
}
type CallbackApps interface {
	Get(context.Context, string) (appmanifest.Manifest, error)
}
type Payments struct {
	Repo        PaymentRepository
	Auth        identity.Authorizer
	Gate        CallbackGate
	Connections CallbackConnections
	Apps        CallbackApps
}

func (s Payments) List(ctx context.Context, user, tenant, status, cursor string) (PaymentPage, error) {
	if err := s.Auth.Authorize(ctx, user, tenant); err != nil {
		return PaymentPage{}, err
	}
	if (status != "" && events.PaymentRank(status) == 0) || (cursor != "" && !events.Token.MatchString(cursor)) {
		return PaymentPage{}, fault.Invalid
	}
	return s.Repo.Payments(ctx, tenant, status, cursor)
}
func (s Payments) Get(ctx context.Context, user, tenant, id string) (PaymentDetail, error) {
	if err := s.Auth.Authorize(ctx, user, tenant); err != nil {
		return PaymentDetail{}, err
	}
	if !events.Token.MatchString(id) {
		return PaymentDetail{}, fault.Invalid
	}
	return s.Repo.Payment(ctx, tenant, id)
}

// Receive authenticates under the same lifecycle gate as invocation/uninstall.
// No cookie, user-supplied provider URL, token refresh or external network call.
func (s Payments) Receive(ctx context.Context, tenant, ins, delivery, timestamp, signature, correlation string, raw []byte, update appapi.PaymentUpdate) (string, error) {
	if !events.Token.MatchString(tenant) || !events.Token.MatchString(ins) || !events.Token.MatchString(delivery) {
		return "", fault.Invalid
	}
	if s.Connections == nil {
		return "", fault.Unavailable
	}
	outcome := ""
	err := s.Gate.WithInstallation(ctx, tenant, ins, func(i domain.Installation) error {
		if i.Status != "active" {
			return fault.Conflict
		}
		app, err := s.Apps.Get(ctx, i.AppID)
		if err != nil {
			return err
		}
		if app.ExecutionProfile != "local-remote" || app.Version != i.Version || !slices.Equal(app.Scopes, i.Scopes) || !slices.Contains(i.Capabilities, "payment/v1") || !slices.Contains(i.Scopes, "payments.write") {
			return fault.Forbidden
		}
		if err = s.Connections.VerifyCallback(ctx, tenant, ins, delivery, timestamp, signature, raw); err != nil {
			return err
		}
		r := update.Resource
		p := events.PaymentStatus{ResourceID: r.ID, InstallationID: ins, Reference: r.Reference, Status: r.Status, AmountMinor: r.AmountMinor, Currency: r.Currency, Revision: r.Revision, Simulation: true}
		if update.Type != appapi.PaymentUpdateType || update.OccurredAt.IsZero() || update.OccurredAt.After(time.Now().Add(30*time.Second)) || p.Validate() != nil {
			return fault.Invalid
		}
		outcome, err = s.Repo.ApplyCallback(ctx, tenant, ins, delivery, string(raw), correlation, update)
		return err
	})
	return outcome, err
}
