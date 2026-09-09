package service

import (
	"context"
	"emisell.app/platform/internal/apppermission"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"slices"
	"strings"
)

// ResourceReadiness is a server-owned verifier, never a browser flag. The
// implementation must attest supported scope operations, authenticated app client,
// release signature and runtime availability. Default composition has none.
// ManagedReleases must retain the matching release/client locks through commit.
type ResourceReadiness interface {
	ReadyResource(context.Context, domain.IntentRelease) error
}

func (s Lifecycle) resourceEligible(ctx context.Context, r domain.IntentRelease) error {
	current, ok := ctx.Value(managedReleaseKey{}).(domain.IntentRelease)
	if !ok || s.Resources == nil || RequestHash(current) != RequestHash(r) || r.ExecutionProfile != domain.ResourceAppPolicy || r.ResourceBinding == nil || r.ManagedSource != nil || r.ShippingProvider != nil || len(r.Capabilities) != 0 {
		return fault.Forbidden
	}
	b := r.ResourceBinding
	if !strings.HasPrefix(r.AppID, "app_") || !opaqueID.MatchString(r.AppID) || !opaqueID.MatchString(b.ReleaseID) || !opaqueID.MatchString(b.ClientID) || !sha256Hex.MatchString(r.ManifestDigest) || b.AccessScopes.Validate() != nil {
		return fault.Forbidden
	}
	// Initial installation grants exactly required scopes. Optional scopes need a
	// separate future consent operation, never implicit union with required scopes.
	required := b.AccessScopes.Canonical().Required
	if len(required) == 0 || !slices.Equal(r.Scopes, required) {
		return fault.Forbidden
	}
	if b.Webhooks != nil && b.Webhooks.Validate(&b.AccessScopes) != nil {
		return fault.Forbidden
	}
	return s.Resources.ReadyResource(ctx, r)
}

// WithResourceAccess is for authenticated Core delegation. No installation token
// or developer portal session can call this as a substitute for Core authority.
// Required scopes must be supplied by the server's operation contract.
func (s Lifecycle) WithResourceAccess(ctx context.Context, p identity.ServicePrincipal, actor, id string, required []string, fn func(domain.Access) error) error {
	owner, err := s.owner(ctx, p, actor)
	if err != nil {
		return err
	}
	if !opaqueID.MatchString(id) || len(required) == 0 || fn == nil {
		return fault.Invalid
	}
	repo, ok := s.Repo.(interface {
		WithCoreAccess(context.Context, domain.IntentOwner, string, func(domain.Access) error) error
	})
	if !ok {
		return fault.Unavailable
	}
	a, err := s.Repo.GetAccess(ctx, owner, id)
	if err != nil {
		return err
	}
	if a.Release.InstallPolicy != domain.ResourceAppPolicy {
		return fault.Forbidden
	}
	return s.Intents.withManaged(ctx, owner, a.Release.AppID, a.Release.Version, func(ctx context.Context) error {
		return repo.WithCoreAccess(ctx, owner, id, func(current domain.Access) error {
			if err := s.resourceEligible(ctx, current.Release); err != nil {
				return err
			}
			expected := apppermission.Identity{MerchantID: owner.TenantID, AppID: current.Release.AppID, InstallationID: id}
			grant := apppermission.Grant{Identity: expected, Active: current.Installation.Status == "active" && current.GrantState == "active", Revoked: current.GrantState == "revoked", ConsentedScopes: current.Release.Scopes, GrantedScopes: current.GrantedScopes}
			if current.Installation.AppID != expected.AppID || current.Installation.ID != id || !apppermission.Allows(expected, grant, required) {
				return fault.Forbidden
			}
			return fn(current)
		})
	})
}
