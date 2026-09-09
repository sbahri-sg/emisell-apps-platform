package service

import (
	"context"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"strings"
)

type ManagedReleases interface {
	WithRelease(context.Context, string, string, string, func(domain.IntentRelease) error) error
}

type managedReleaseKey struct{}
type managedMerchantKey struct{}
type managedOwnerKey struct{}

// Available only after Core authentication and current merchant authorization.
func ResourceOwner(ctx context.Context) domain.IntentOwner {
	owner, _ := ctx.Value(managedOwnerKey{}).(domain.IntentOwner)
	return owner
}

// ResourceMerchant is authoritative only inside the managed release callback.
func ResourceMerchant(ctx context.Context) string {
	v, _ := ctx.Value(managedMerchantKey{}).(string)
	return v
}

type accessReplay interface {
	ReplayAccess(context.Context, domain.IntentOwner, string, string) (*domain.AccessResult, error)
}
type intentReplay interface {
	ReplayIntent(context.Context, domain.IntentOwner, string, string) (*domain.InstallIntent, error)
}

// Every managed mutation retains release/assignment locks through its commit.
// Legacy fixtures continue using their existing registry and receipt semantics.
func (s Intents) withManaged(ctx context.Context, owner domain.IntentOwner, app, version string, fn func(context.Context) error) error {
	merchant := owner.TenantID
	ctx = context.WithValue(ctx, managedOwnerKey{}, owner)
	if app == EmbeddedPilotApp {
		r, err := s.EmbeddedPilot.release(merchant)
		if err != nil {
			return err
		}
		if r.Version != version {
			return fault.Conflict
		}
		return fn(context.WithValue(ctx, managedReleaseKey{}, r))
	}
	if !strings.HasPrefix(app, "app_") {
		return fn(ctx)
	}
	if s.Managed == nil {
		return fault.NotFound
	}
	return s.Managed.WithRelease(ctx, merchant, app, version, func(r domain.IntentRelease) error {
		return fn(context.WithValue(context.WithValue(ctx, managedReleaseKey{}, r), managedMerchantKey{}, merchant))
	})
}
