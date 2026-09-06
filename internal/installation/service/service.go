package service

import (
	"context"
	"crypto/sha256"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"slices"
)

type Registry interface {
	Get(context.Context, string) (appmanifest.Manifest, error)
	List(context.Context) ([]appmanifest.Manifest, error)
}
type Repository interface {
	List(context.Context, string) ([]domain.Installation, error)
	Events(context.Context, string) ([]event.Envelope, error)
	Change(context.Context, string, string, string, string, string, func(*domain.Installation) (*domain.Installation, *event.Envelope, error)) (*domain.Installation, error)
	WithActive(context.Context, string, string, func(domain.Installation) error) error
}
type Service struct {
	Repo        Repository
	Apps        Registry
	Auth        identity.Authorizer
	Connections Connections
}
type Connections interface {
	Ready(context.Context, string, string) error
	Status(context.Context, string, string) (string, error)
	Revoke(context.Context, string, string) error
}

var KeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

func RequestHash(v any) string {
	data, _ := json.Marshal(v)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (s Service) List(ctx context.Context, user, tenant string) ([]domain.Installation, error) {
	if err := s.Auth.Authorize(ctx, user, tenant); err != nil {
		return nil, err
	}
	items, err := s.Repo.List(ctx, tenant)
	if err != nil {
		return nil, err
	}
	for i := range items {
		app, err := s.Apps.Get(ctx, items[i].AppID)
		if err != nil {
			return nil, err
		}
		items[i].ExecutionProfile = app.ExecutionProfile
		if app.ExecutionProfile == "local-remote" && s.Connections != nil {
			items[i].ConnectionStatus, err = s.Connections.Status(ctx, tenant, items[i].ID)
			if err != nil {
				return nil, err
			}
		}
	}
	return items, nil
}
func (s Service) Events(ctx context.Context, user, tenant string) ([]event.Envelope, error) {
	if err := s.Auth.Authorize(ctx, user, tenant); err != nil {
		return nil, err
	}
	return s.Repo.Events(ctx, tenant)
}
func (s Service) Execute(ctx context.Context, user, tenant, key, correlation string, action domain.Action) (*domain.Installation, error) {
	if err := s.Auth.Authorize(ctx, user, tenant); err != nil {
		return nil, err
	}
	if !KeyPattern.MatchString(key) {
		return nil, fault.Invalid
	}
	if action.Type != "install" && action.Type != "activate" && action.Type != "uninstall" {
		return nil, fault.Invalid
	}
	if action.Type != "install" && (action.Version != "" || len(action.Grants) != 0) {
		return nil, fault.Invalid
	}
	app, err := s.Apps.Get(ctx, action.AppID)
	if err != nil {
		return nil, err
	}
	if app.ExecutionProfile == appmanifest.KurirFixtureProfile || app.ExecutionProfile == appmanifest.ShippingProviderFixtureProfile {
		return nil, fault.Forbidden // Only the consent-bound Core lifecycle may install this reference.
	}
	action.Grants = slices.Clone(action.Grants)
	slices.Sort(action.Grants)
	return s.Repo.Change(ctx, tenant, user, key, RequestHash(action), app.ID, func(current *domain.Installation) (*domain.Installation, *event.Envelope, error) {
		// Intent-managed installations must use the owner-bound Core lifecycle.
		if current != nil && current.IntentID != "" {
			return nil, nil, fault.Forbidden
		}
		if action.Type == "activate" && app.ExecutionProfile == "local-remote" {
			if current == nil || (current.Status != "pending" && current.Status != "active") {
				return nil, nil, fault.Conflict
			}
			if s.Connections == nil {
				return nil, nil, fault.Unavailable
			}
			if err := s.Connections.Ready(ctx, tenant, current.ID); err != nil {
				return nil, nil, err
			}
		}
		next, changed, err := domain.Transition(current, app, action)
		if err != nil || !changed {
			return next, nil, err
		}
		kind := map[string]string{"install": "installed", "activate": "activated", "uninstall": "uninstalled"}[action.Type]
		if next.Status == "disabling" {
			kind = "disabling"
		}
		envelope := event.New("emisell.app."+kind+".v1", tenant, user, next.ID, correlation, map[string]any{"appId": app.ID, "name": app.Name, "status": next.Status, "version": next.Version, "scopes": next.Scopes})
		return next, &envelope, nil
	})
}

// Gate holds the same tenant lifecycle lock as uninstall until invocation exits.
// A routing change cannot race a newly authorized capability operation.
func (s Service) WithActive(ctx context.Context, user, tenant, capability, scope string, fn func(domain.Installation) error) error {
	if err := s.Auth.Authorize(ctx, user, tenant); err != nil {
		return err
	}
	apps, err := s.Apps.List(ctx)
	if err != nil {
		return err
	}
	return s.Repo.WithActive(ctx, tenant, capability, func(ins domain.Installation) error {
		if !slices.Contains(ins.Scopes, scope) {
			return fault.Forbidden
		}
		for _, app := range apps {
			if app.ID == ins.AppID && app.Version == ins.Version && slices.Contains(app.Capabilities, capability) {
				if !slices.Equal(ins.Scopes, app.Scopes) {
					return fault.Forbidden
				}
				ins.ExecutionProfile = app.ExecutionProfile
				return fn(ins)
			}
		}
		return fault.Unavailable
	})
}
