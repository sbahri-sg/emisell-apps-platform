package service

import (
	"context"
	"regexp"
	"slices"
	"time"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/appmanifest"
)

type IntentRepository interface {
	GetIntent(context.Context, domain.IntentOwner, string) (domain.InstallIntent, error)
	ChangeIntent(context.Context, domain.IntentOwner, string, string, string, func(*domain.InstallIntent, time.Time) (domain.InstallIntent, error)) (domain.InstallIntent, error)
}

type Intents struct {
	Repo          IntentRepository
	Apps          Registry
	Auth          identity.Authorizer
	Managed       ManagedReleases
	EmbeddedPilot *LocalEmbeddedPilot
}

type PrepareIntent struct {
	AppID   string
	Version string
}

type DecideIntent struct {
	ID       string
	Digest   string
	Decision string
}

var opaqueID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@-]{0,127}$`)
var sha256Hex = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s Intents) owner(ctx context.Context, p identity.ServicePrincipal, actor, scope string) (domain.IntentOwner, error) {
	if p.ID == "" || p.TenantID == "" || (!p.PlatformFull && !time.Now().Before(p.ExpiresAt)) {
		return domain.IntentOwner{}, fault.Unauthenticated
	}
	if !p.AllowsServiceScope(scope) {
		return domain.IntentOwner{}, fault.Forbidden
	}
	if err := s.Auth.Authorize(ctx, p.ID, p.TenantID); err != nil {
		return domain.IntentOwner{}, err
	}
	if !opaqueID.MatchString(actor) {
		return domain.IntentOwner{}, fault.Invalid
	}
	return domain.IntentOwner{TenantID: p.TenantID, ServiceID: p.ID, ActorID: actor}, nil
}

func (s Intents) release(ctx context.Context, app, version string) (domain.IntentRelease, error) {
	if r, ok := ctx.Value(managedReleaseKey{}).(domain.IntentRelease); ok && r.AppID == app && r.Version == version {
		return r, nil
	}
	m, err := s.Apps.Get(ctx, app)
	if err != nil {
		return domain.IntentRelease{}, err
	}
	if m.Version != version {
		return domain.IntentRelease{}, fault.Conflict
	}
	// The injected app registry verifies fixture signatures. No catalog fallback.
	if m.Schema != "emisell.app/v1" || (m.ExecutionProfile != "local-simulator" && m.ExecutionProfile != "local-remote" && m.ExecutionProfile != appmanifest.KurirFixtureProfile && m.ExecutionProfile != appmanifest.ShippingProviderFixtureProfile) {
		return domain.IntentRelease{}, fault.Unavailable
	}
	if m.ExecutionProfile == appmanifest.ShippingProviderFixtureProfile && !appmanifest.ValidShippingProviderFixture(m) {
		return domain.IntentRelease{}, fault.Unavailable
	}
	if m.ShippingProvider != nil {
		binding := *m.ShippingProvider
		m.ShippingProvider = &binding
	}
	m.Scopes = slices.Clone(m.Scopes)
	m.Capabilities = slices.Clone(m.Capabilities)
	m.Subscriptions = slices.Clone(m.Subscriptions)
	slices.Sort(m.Scopes)
	slices.Sort(m.Capabilities)
	slices.Sort(m.Subscriptions)
	return domain.IntentRelease{InstallPolicy: domain.InstallPolicy, AppID: m.ID, Name: m.Name, DeveloperID: m.DeveloperID, Version: m.Version, ManifestDigest: RequestHash(m), Scopes: m.Scopes, Capabilities: m.Capabilities, ExecutionProfile: m.ExecutionProfile, ShippingProvider: m.ShippingProvider}, nil
}

func (s Intents) Prepare(ctx context.Context, p identity.ServicePrincipal, actor, key string, request PrepareIntent) (domain.InstallIntent, error) {
	owner, err := s.owner(ctx, p, actor, identity.ScopeInstallIntentsWrite)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	if !KeyPattern.MatchString(key) || !opaqueID.MatchString(request.AppID) || len(request.Version) == 0 || len(request.Version) > 64 {
		return domain.InstallIntent{}, fault.Invalid
	}
	hash := RequestHash(struct {
		Operation string
		Request   PrepareIntent
	}{"prepare", request})
	if repo, ok := s.Repo.(intentReplay); ok {
		if v, e := repo.ReplayIntent(ctx, owner, key, hash); e != nil {
			return domain.InstallIntent{}, e
		} else if v != nil {
			return *v, nil
		}
	}
	var result domain.InstallIntent
	err = s.withManaged(ctx, owner.TenantID, request.AppID, request.Version, func(ctx context.Context) error {
		var err error
		result, err = s.Repo.ChangeIntent(ctx, owner, key, hash, "", func(_ *domain.InstallIntent, now time.Time) (domain.InstallIntent, error) {
			release, err := s.release(ctx, request.AppID, request.Version)
			if err != nil {
				return domain.InstallIntent{}, err
			}
			return domain.NewInstallIntent(ids.New("intent"), owner, release, now), nil
		})
		return err
	})
	return result, err
}

func (s Intents) Get(ctx context.Context, p identity.ServicePrincipal, actor, id string) (domain.InstallIntent, error) {
	owner, err := s.owner(ctx, p, actor, identity.ScopeInstallIntentsRead)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	if !opaqueID.MatchString(id) {
		return domain.InstallIntent{}, fault.Invalid
	}
	return s.Repo.GetIntent(ctx, owner, id)
}

func (s Intents) Decide(ctx context.Context, p identity.ServicePrincipal, actor, key string, request DecideIntent) (domain.InstallIntent, error) {
	owner, err := s.owner(ctx, p, actor, identity.ScopeInstallIntentsConsent)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	if !KeyPattern.MatchString(key) || !opaqueID.MatchString(request.ID) || !sha256Hex.MatchString(request.Digest) || (request.Decision != "consent" && request.Decision != "deny") {
		return domain.InstallIntent{}, fault.Invalid
	}
	hash := RequestHash(struct {
		Operation string
		Request   DecideIntent
	}{"decide", request})
	if repo, ok := s.Repo.(intentReplay); ok {
		if v, e := repo.ReplayIntent(ctx, owner, key, hash); e != nil {
			return domain.InstallIntent{}, e
		} else if v != nil {
			return *v, nil
		}
	}
	change := func(ctx context.Context) (domain.InstallIntent, error) {
		return s.Repo.ChangeIntent(ctx, owner, key, hash, request.ID, func(current *domain.InstallIntent, now time.Time) (domain.InstallIntent, error) {
			if current == nil {
				return domain.InstallIntent{}, fault.NotFound
			}
			next, err := current.Decide(request.Digest, request.Decision, now)
			if err != nil {
				return domain.InstallIntent{}, err
			}
			if request.Decision == "consent" {
				release, err := s.release(ctx, current.Release.AppID, current.Release.Version)
				if err != nil {
					return domain.InstallIntent{}, err
				}
				if RequestHash(release) != RequestHash(current.Release) {
					return domain.InstallIntent{}, fault.Conflict
				}
			}
			return next, nil
		})
	}
	if request.Decision == "deny" {
		return change(ctx)
	}
	current, err := s.Repo.GetIntent(ctx, owner, request.ID)
	if err != nil {
		return domain.InstallIntent{}, err
	}
	var result domain.InstallIntent
	err = s.withManaged(ctx, owner.TenantID, current.Release.AppID, current.Release.Version, func(ctx context.Context) error { var e error; result, e = change(ctx); return e })
	return result, err
}
