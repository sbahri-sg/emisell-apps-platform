package service

import (
	"context"
	"crypto/ed25519"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/embedded"
)

// The source supplied through ManagedReleases must hold current release,
// assignment, client and approved launch locks for the complete callback.
// No default composition provides this source yet. Never use catalog DTOs here.
func (s Lifecycle) reviewedUIEligible(ctx context.Context, r domain.IntentRelease) error {
	current, ok := ctx.Value(managedReleaseKey{}).(domain.IntentRelease)
	if !ok || RequestHash(current) != RequestHash(r) || r.ExecutionProfile != domain.ReviewedUIPolicy ||
		r.UIBinding == nil || r.ManagedSource != nil || r.ShippingProvider != nil ||
		len(r.Scopes) != 0 || len(r.Capabilities) != 0 {
		return fault.Forbidden
	}
	b := r.UIBinding
	if !opaqueID.MatchString(b.ReleaseID) || !sha256Hex.MatchString(r.ManifestDigest) ||
		b.Launch.AppID != r.AppID || b.Launch.ReleaseDigest != r.ManifestDigest ||
		embedded.VerifyLaunch(b.Launch, b.Signature, ed25519.PublicKey(s.ReviewedUIKey), false) != nil {
		return fault.Forbidden
	}
	return nil
}

// WithReviewedUIAccess reads current source state before taking the installation
// lock, in the same order as activation. Uninstall never requires source access.
// A token issuer/redirect handler must finish within fn; do not cache its result.
func (s Lifecycle) WithReviewedUIAccess(ctx context.Context, p identity.ServicePrincipal, actor, id string, fn func(domain.UIBinding) error) error {
	o, err := s.owner(ctx, p, actor)
	if err != nil {
		return err
	}
	if !opaqueID.MatchString(id) || fn == nil {
		return fault.Invalid
	}
	repo, ok := s.Repo.(interface {
		WithCoreAccess(context.Context, domain.IntentOwner, string, func(domain.Access) error) error
	})
	if !ok {
		return fault.Unavailable
	}
	current, err := s.Repo.GetAccess(ctx, o, id)
	if err != nil {
		return err
	}
	if current.Release.InstallPolicy != domain.ReviewedUIPolicy {
		return fault.Forbidden
	}
	return s.Intents.withManaged(ctx, o.TenantID, current.Release.AppID, current.Release.Version, func(ctx context.Context) error {
		return repo.WithCoreAccess(ctx, o, id, func(a domain.Access) error {
			if a.Installation.Status != "active" || a.GrantState != "active" || len(a.GrantedScopes) != 0 {
				return fault.Forbidden
			}
			if err := s.reviewedUIEligible(ctx, a.Release); err != nil {
				return err
			}
			return fn(*a.Release.UIBinding)
		})
	})
}
