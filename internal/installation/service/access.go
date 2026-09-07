package service

import (
	"context"
	"slices"
	"time"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/oauth/apptoken"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/appmanifest"
)

type AccessRepository interface {
	ConsumeIntent(context.Context, domain.IntentOwner, string, string, string, func(domain.InstallIntent, time.Time) error) (domain.AccessResult, error)
	GetAccess(context.Context, domain.IntentOwner, string) (domain.Access, error)
	ListInstalled(context.Context, string, string, int) ([]domain.InstalledApp, error)
	ChangeAccess(context.Context, domain.IntentOwner, string, string, string, string, string, string, func(domain.Access, time.Time) (domain.Access, error)) (domain.AccessResult, error)
	WithAppAccess(context.Context, string, string, string, string, string, func(domain.Access) error) (domain.Access, error)
}

// List is merchant-wide metadata for a freshly authorized Core app manager.
// It does not relax owner binding on Get, consent, activation, tokens or uninstall.
func (s Lifecycle) List(ctx context.Context, p identity.ServicePrincipal, actor, after string, size int) ([]domain.InstalledApp, string, error) {
	o, err := s.owner(ctx, p, actor)
	if err != nil {
		return nil, "", err
	}
	if size < 0 || size > 20 || (after != "" && !opaqueID.MatchString(after)) {
		return nil, "", fault.Invalid
	}
	if size == 0 {
		size = 20
	}
	items, err := s.Repo.ListInstalled(ctx, o.TenantID, after, size+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > size {
		items = items[:size]
		next = items[len(items)-1].ID
	}
	return items, next, nil
}

type Lifecycle struct {
	Resources ResourceReadiness
	// Explicit composition only. A missing key denies reviewed UI releases.
	ReviewedUIKey     []byte
	Repo              AccessRepository
	Intents           Intents
	Connections       Connections
	Shipping          ShippingReadiness
	ShippingProviders ShippingProviderReadiness
}

// ShippingReadiness checks configuration without provisioning or changing a provider.
type ShippingReadiness interface {
	Ready(context.Context, string, string) error
}

// ShippingProviderReadiness checks a release-bound provider's setup. It never
// selects that provider for checkout or changes upstream credentials.
type ShippingProviderReadiness interface {
	ReadyProvider(context.Context, string, string, appmanifest.ShippingProviderBinding) error
}

func (s Lifecycle) owner(ctx context.Context, p identity.ServicePrincipal, actor string) (domain.IntentOwner, error) {
	// New execution authority is never silently added to legacy consent-only keys.
	if !p.PlatformFull {
		return domain.IntentOwner{}, fault.Forbidden
	}
	return s.Intents.owner(ctx, p, actor, identity.ScopeInstallIntentsConsent)
}

func (s Lifecycle) eligible(ctx context.Context, release domain.IntentRelease) error {
	if release.InstallPolicy == domain.ResourceAppPolicy {
		return s.resourceEligible(ctx, release)
	}
	if release.ResourceBinding != nil {
		return fault.Forbidden
	}
	if release.InstallPolicy == domain.ReviewedUIPolicy {
		return s.reviewedUIEligible(ctx, release)
	}
	if release.InstallPolicy == EmbeddedPilotPolicy {
		current, ok := ctx.Value(managedReleaseKey{}).(domain.IntentRelease)
		if !ok || s.Intents.EmbeddedPilot == nil || current.AppID != EmbeddedPilotApp || RequestHash(current) != RequestHash(release) || len(current.Scopes) != 0 || len(current.Capabilities) != 0 {
			return fault.Forbidden
		}
		return nil
	}
	if release.InstallPolicy != domain.InstallPolicy && release.InstallPolicy != domain.ManagedShippingPolicy && release.InstallPolicy != domain.ProviderAppPolicy {
		return fault.Conflict
	}
	current, err := s.Intents.release(ctx, release.AppID, release.Version)
	if err != nil {
		return err
	}
	if RequestHash(current) != RequestHash(release) {
		return fault.Conflict
	}
	// Closed allowlist of implemented fixture permissions; Shopify resource handles
	// and arbitrary scopes cannot turn into grants through this pipeline.
	want := []string{"orders.read", "payments.read", "payments.write"}
	if current.ExecutionProfile == domain.ProviderAppProfile {
		if current.InstallPolicy != domain.ProviderAppPolicy || current.ManagedSource == nil || current.ManagedSource.MerchantID == "" || current.ManagedSource.Environment == "" || current.ShippingProvider == nil || current.ShippingProvider.Engine != "api-kurir" || current.ShippingProvider.ProviderCode == "" || !slices.Equal(current.Capabilities, []string{"shipping/v1"}) {
			return fault.Forbidden
		}
		if !slices.Equal(current.Scopes, []string{"shipping.read"}) && !slices.Equal(current.Scopes, []string{"shipping.read", "shipping.write"}) {
			return fault.Forbidden
		}
		want = current.Scopes
	} else if current.ExecutionProfile == domain.ManagedShippingProfile {
		if current.InstallPolicy != domain.ManagedShippingPolicy || current.ManagedSource == nil || current.ManagedSource.Environment != "local-isolated" || current.ShippingProvider == nil || current.ShippingProvider.Engine != "api-kurir" || current.ShippingProvider.ProviderCode != "emisell" || !slices.Equal(current.Capabilities, []string{"shipping/v1"}) {
			return fault.Forbidden
		}
		want = []string{"shipping.read"}
	} else if current.ExecutionProfile == appmanifest.ShippingProviderFixtureProfile {
		if current.ShippingProvider == nil || !slices.Equal(current.Capabilities, []string{"shipping/v1"}) {
			return fault.Forbidden
		}
		want = []string{"shipping.read"}
	} else if current.ExecutionProfile == appmanifest.KurirFixtureProfile {
		if current.AppID != appmanifest.KurirFixtureID || !slices.Equal(current.Capabilities, []string{"shipping/v1"}) {
			return fault.Forbidden
		}
		want = []string{"shipping.read"}
	} else if slices.Equal(current.Capabilities, []string{"shipping/v1"}) {
		want = []string{"orders.read", "shipping.read", "shipping.write"}
	} else if !slices.Equal(current.Capabilities, []string{"payment/v1"}) {
		return fault.Forbidden
	}
	if !slices.Equal(current.Scopes, want) {
		return fault.Forbidden
	}
	return nil
}

func (s Lifecycle) Consume(ctx context.Context, p identity.ServicePrincipal, actor, key, intentID, digest string) (domain.AccessResult, error) {
	o, err := s.owner(ctx, p, actor)
	if err != nil {
		return domain.AccessResult{}, err
	}
	if !KeyPattern.MatchString(key) || !opaqueID.MatchString(intentID) || !sha256Hex.MatchString(digest) {
		return domain.AccessResult{}, fault.Invalid
	}
	hash := RequestHash([]string{"consume", intentID, digest})
	if repo, ok := s.Repo.(accessReplay); ok {
		if v, e := repo.ReplayAccess(ctx, o, key, hash); e != nil {
			return domain.AccessResult{}, e
		} else if v != nil {
			return *v, nil
		}
	}
	intent, err := s.Intents.Repo.GetIntent(ctx, o, intentID)
	if err != nil {
		return domain.AccessResult{}, err
	}
	var result domain.AccessResult
	err = s.Intents.withManaged(ctx, o.TenantID, intent.Release.AppID, intent.Release.Version, func(ctx context.Context) error {
		var e error
		result, e = s.Repo.ConsumeIntent(ctx, o, key, hash, intentID, func(v domain.InstallIntent, now time.Time) error {
			if v.Effective(now).State != "consented" || v.ConsentDigest != digest {
				return fault.Conflict
			}
			return s.eligible(ctx, v.Release)
		})
		return e
	})
	return result, err
}

func (s Lifecycle) Get(ctx context.Context, p identity.ServicePrincipal, actor, id string) (domain.Access, error) {
	o, err := s.owner(ctx, p, actor)
	if err != nil {
		return domain.Access{}, err
	}
	if !opaqueID.MatchString(id) {
		return domain.Access{}, fault.Invalid
	}
	a, err := s.Repo.GetAccess(ctx, o, id)
	if err == nil && a.Release.InstallPolicy == domain.ResourceAppPolicy {
		a.ReviewedUILaunch = nil
		_ = s.WithResourceAccess(ctx, p, actor, id, []string{"read_products"}, func(current domain.Access) error {
			if current.Release.UIBinding != nil {
				b := *current.Release.UIBinding
				a.ReviewedUILaunch = &b
			}
			return nil
		})
	}
	if err == nil && a.Release.InstallPolicy == domain.ReviewedUIPolicy {
		// Details and uninstall remain available on source failure, but no URL is
		// returned until the complete current source and installation are checked.
		a.ReviewedUILaunch = nil
		_ = s.WithReviewedUIAccess(ctx, p, actor, id, func(b domain.UIBinding) error { a.ReviewedUILaunch = &b; return nil })
	}
	if err == nil && a.Release.InstallPolicy == EmbeddedPilotPolicy {
		// Preserve receipt/uninstall access when enrollment is disabled, but do
		// not permit an embedded identity to outlive the active local policy.
		e := s.WithEmbeddedPilotAccess(ctx, p, actor, id, func(current domain.Access) error { a = current; a.LocalEmbeddedAccess = true; return nil })
		if e != nil {
			a.LocalEmbeddedAccess = false
		}
	}
	return a, err
}

func (s Lifecycle) Execute(ctx context.Context, p identity.ServicePrincipal, actor, key, id, action string) (domain.AccessResult, error) {
	o, err := s.owner(ctx, p, actor)
	if err != nil {
		return domain.AccessResult{}, err
	}
	if !KeyPattern.MatchString(key) || !opaqueID.MatchString(id) || !slices.Contains([]string{"activate", "issue_token", "uninstall"}, action) {
		return domain.AccessResult{}, fault.Invalid
	}
	var secret, tokenID, tokenHash string
	if repo, ok := s.Repo.(accessReplay); ok {
		if v, e := repo.ReplayAccess(ctx, o, key, RequestHash([]string{action, id})); e != nil {
			return domain.AccessResult{}, e
		} else if v != nil {
			return *v, nil
		}
	}
	if action == "issue_token" {
		secret, tokenID = apptoken.New(), ids.New("apptoken")
		tokenHash = apptoken.Hash(secret)
	}
	change := func(ctx context.Context) (domain.AccessResult, error) {
		return s.Repo.ChangeAccess(ctx, o, key, RequestHash([]string{action, id}), id, action, tokenID, tokenHash, func(a domain.Access, _ time.Time) (domain.Access, error) {
			if action == "uninstall" {
				// Revocation must remain possible when the release is missing or invalid.
				if a.Installation.Status == "uninstalled" || a.Installation.Status == "disabling" {
					return a, nil
				}
				a.Installation.Status = "uninstalled"
				if a.Release.ExecutionProfile == "local-remote" {
					a.Installation.Status = "disabling"
				}
				a.Installation.Scopes, a.Installation.Capabilities = []string{}, []string{}
				a.GrantState, a.GrantedScopes = "revoked", []string{}
				return a, nil
			}
			if err := s.eligible(ctx, a.Release); err != nil {
				return a, err
			}
			if a.Installation.Status != "pending" && a.Installation.Status != "active" {
				return a, fault.Conflict
			}
			if a.GrantState == "revoked" {
				return a, fault.Forbidden
			}
			if (a.Installation.Status == "active" && a.GrantState != "active") || (a.Installation.Status == "pending" && a.GrantState != "pending") {
				return a, fault.Forbidden
			}
			if action == "issue_token" {
				if a.Release.InstallPolicy == domain.ResourceAppPolicy {
					return a, fault.Forbidden
				}
				if a.Release.InstallPolicy == EmbeddedPilotPolicy || a.Release.InstallPolicy == domain.ReviewedUIPolicy {
					return a, fault.Forbidden
				}
				if a.Release.ExecutionProfile == appmanifest.ShippingProviderFixtureProfile || a.Release.ExecutionProfile == domain.ManagedShippingProfile || a.Release.ExecutionProfile == domain.ProviderAppProfile {
					return a, fault.Forbidden // Engine delegation/token audience is not implemented by this fixture.
				}
				if a.Installation.Status != "active" || a.GrantState != "active" || !slices.Equal(a.GrantedScopes, a.Release.Scopes) {
					return a, fault.Conflict
				}
				return a, nil
			}
			if a.Release.ExecutionProfile == "local-remote" {
				if s.Connections == nil {
					return a, fault.Unavailable
				}
				if err := s.Connections.Ready(ctx, o.TenantID, id); err != nil {
					return a, err
				}
			}
			if a.Release.ExecutionProfile == appmanifest.KurirFixtureProfile {
				if s.Shipping == nil {
					return a, fault.Unavailable
				}
				if err := s.Shipping.Ready(ctx, o.TenantID, id); err != nil {
					return a, err
				}
			}
			if a.Release.ExecutionProfile == appmanifest.ShippingProviderFixtureProfile || a.Release.ExecutionProfile == domain.ManagedShippingProfile || a.Release.ExecutionProfile == domain.ProviderAppProfile {
				if s.ShippingProviders == nil || a.Release.ShippingProvider == nil {
					return a, fault.Unavailable
				}
				if err := s.ShippingProviders.ReadyProvider(ctx, o.TenantID, id, *a.Release.ShippingProvider); err != nil {
					return a, err
				}
				// Installed app access is not the selected checkout route. The engine
				// owns provider selection; invocation needs a separate engine grant gate.
				a.Installation.Capabilities = []string{}
			}
			a.Installation.Status, a.GrantState = "active", "active"
			a.GrantedScopes = slices.Clone(a.Release.Scopes)
			return a, nil
		})
	}
	var v domain.AccessResult
	if action == "uninstall" {
		v, err = change(ctx)
	} else {
		current, e := s.Repo.GetAccess(ctx, o, id)
		if e != nil {
			return v, e
		}
		err = s.Intents.withManaged(ctx, o.TenantID, current.Release.AppID, current.Release.Version, func(ctx context.Context) error { var e error; v, e = change(ctx); return e })
	}
	if err == nil && action == "issue_token" && !v.Replayed {
		v.Token.Secret = secret
	}
	return v, err
}

// CheckAppToken is a server-to-server self-check only, not a resource gateway.
// The repository holds the lifecycle lock through the current-release check.
func (s Lifecycle) CheckAppToken(ctx context.Context, token, tenant, app, installation string) (domain.Access, error) {
	if !apptoken.Valid(token) {
		return domain.Access{}, fault.Unauthenticated
	}
	if !opaqueID.MatchString(tenant) || !opaqueID.MatchString(app) || !opaqueID.MatchString(installation) {
		return domain.Access{}, fault.Invalid
	}
	return s.Repo.WithAppAccess(ctx, apptoken.Hash(token), apptoken.Audience, tenant, app, installation, func(a domain.Access) error {
		return s.eligible(ctx, a.Release)
	})
}
