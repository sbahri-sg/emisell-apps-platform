// Package webhook owns delivery policy; storage and broker implement its ports.
package webhook

import (
	"bytes"
	"context"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/sdk/events"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type Delivery struct {
	ID           string    `json:"id"`
	Tenant       string    `json:"tenantId"`
	Installation string    `json:"installationId"`
	EventID      string    `json:"eventId"`
	Status       string    `json:"status"`
	Attempts     int       `json:"attempts"`
	NextAt       time.Time `json:"nextAt"`
	Body         []byte    `json:"-"`
	CreatedAt    time.Time `json:"-"`
}
type Outcome struct {
	Status, Reason string
	Attempts       int
	NextAt         time.Time
}
type Counts struct{ Pending, Dead int64 }
type Repository interface {
	Enqueue(context.Context, events.Envelope) error
	Candidate(context.Context) (Delivery, error)
	WithDelivery(context.Context, string, func(Delivery) (Outcome, error)) (string, error)
	Cancel(context.Context, Delivery) error
	List(context.Context, string) ([]Delivery, error)
	Replay(context.Context, string, string, string) error
	Counts(context.Context) (Counts, error)
}
type Service struct {
	Repo        Repository
	Connections *oauth.Service
}

func (s Service) Ingest(ctx context.Context, e events.Envelope) error {
	if e.Validate() != nil {
		return fault.Invalid
	}
	if e.Type != "emisell.capability.invoked.v1" {
		return nil
	}
	err := s.Connections.Gate.WithInstallation(ctx, e.TenantID, e.Subject, func(ins domain.Installation) error {
		if ins.Status != "active" {
			return nil
		}
		app, err := s.Connections.Apps.Get(ctx, ins.AppID)
		if err != nil {
			return err
		}
		if !allowsLocalWebhook(e.TenantID, e.Subject, e.Type, ins, app) {
			return nil
		}
		return s.Repo.Enqueue(ctx, e)
	})
	if errors.Is(err, fault.NotFound) {
		return nil
	}
	return err
}
func (s Service) DeliverOne(ctx context.Context) (string, error) {
	d, err := s.Repo.Candidate(ctx)
	if errors.Is(err, fault.NotFound) {
		return "idle", nil
	}
	if err != nil {
		return "", err
	}
	outcome := "idle"
	err = s.Connections.Gate.WithInstallation(ctx, d.Tenant, d.Installation, func(ins domain.Installation) error {
		var err error
		outcome, err = s.Repo.WithDelivery(ctx, d.ID, func(delivery Delivery) (Outcome, error) {
			if ins.Status != "active" {
				return Outcome{Status: "cancelled", Reason: "installation_inactive", Attempts: delivery.Attempts, NextAt: time.Now()}, nil
			}
			app, err := s.Connections.Apps.Get(ctx, ins.AppID)
			if err != nil {
				return Outcome{}, err
			}
			envelope, decodeErr := events.Decode(delivery.Body)
			if decodeErr != nil || envelope.ID != delivery.EventID || envelope.TenantID != delivery.Tenant || envelope.Subject != delivery.Installation || !allowsLocalWebhook(delivery.Tenant, delivery.Installation, envelope.Type, ins, app) {
				return Outcome{Status: "cancelled", Reason: "subscription_or_scope_revoked", Attempts: delivery.Attempts, NextAt: time.Now()}, nil
			}
			next := Outcome{Status: "pending", Attempts: delivery.Attempts + 1}
			next.NextAt = time.Now().Add(time.Duration(1<<min(next.Attempts, 8)) * time.Second)
			if time.Since(delivery.CreatedAt) > 24*time.Hour || next.Attempts > 12 {
				next.Attempts = delivery.Attempts // No receiver/connection attempt was made.
				next.Status = "dead"
				next.Reason = "retry_budget_exhausted"
				return next, nil
			}
			connection, err := s.Connections.Access(ctx, delivery.Tenant, delivery.Installation)
			if err != nil {
				next.Reason = "connection_unavailable"
			} else {
				next.Status, next.Reason = s.send(ctx, delivery, connection.WebhookSecret)
			}
			if next.Status != "delivered" && next.Attempts >= 12 {
				next.Status = "dead"
			}
			return next, nil
		})
		return err
	})
	if errors.Is(err, fault.NotFound) {
		return "cancelled", s.Repo.Cancel(ctx, d)
	}
	return outcome, err
}

// Fixed-origin local transport; it cannot follow redirects or use a proxy.
func (s Service) send(ctx context.Context, d Delivery, secret string) (string, string) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.Connections.Config.Origin+"/v1/webhooks", bytes.NewReader(d.Body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Emisell-Tenant", d.Tenant)
	request.Header.Set("X-Emisell-Installation", d.Installation)
	request.Header.Set("X-Emisell-Delivery", d.ID)
	request.Header.Set("X-Emisell-Timestamp", timestamp)
	request.Header.Set("X-Emisell-Signature", appapi.SignWebhook(secret, d.Tenant, d.Installation, d.ID, timestamp, d.Body))
	response, err := s.Connections.HTTP.Do(request)
	if err != nil {
		return "pending", "receiver_unavailable"
	}
	response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return "delivered", ""
	}
	return "pending", "receiver_rejected"
}
func (s Service) List(ctx context.Context, tenant string) ([]Delivery, error) {
	return s.Repo.List(ctx, tenant)
}
func (s Service) Replay(ctx context.Context, tenant, id, reason string) error {
	return s.Repo.Replay(ctx, tenant, id, reason)
}
