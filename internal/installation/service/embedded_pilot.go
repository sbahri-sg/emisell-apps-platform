package service

import (
	"context"
	"errors"
	"slices"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
)

const EmbeddedPilotApp = domain.EmbeddedPilotApp
const EmbeddedPilotPolicy = domain.EmbeddedPilotPolicy

// LocalEmbeddedPilot is an explicit development enrollment, never catalog input.
// It grants no commerce capability or resource scope. Only the named merchant
// may create new consent/installation records for this fixed demo release.
type LocalEmbeddedPilot struct{ MerchantID string }

func NewLocalEmbeddedPilot(environment, merchant string) (*LocalEmbeddedPilot, error) {
	if environment != "development" || !opaqueID.MatchString(merchant) {
		return nil, errors.New("embedded pilot requires explicit development environment and merchant")
	}
	return &LocalEmbeddedPilot{MerchantID: merchant}, nil
}

func (p *LocalEmbeddedPilot) release(merchant string) (domain.IntentRelease, error) {
	if p == nil || p.MerchantID == "" || merchant != p.MerchantID {
		return domain.IntentRelease{}, fault.Forbidden
	}
	r := domain.IntentRelease{InstallPolicy: EmbeddedPilotPolicy, AppID: EmbeddedPilotApp, Name: "Embedded Demo · Local test", DeveloperID: "emisell-local-demo", Version: "1.0.0", Scopes: []string{}, Capabilities: []string{}, ExecutionProfile: EmbeddedPilotPolicy}
	// The enrollment is part of consent provenance, without inventing a second
	// tenant identity. Changing enrollment never transfers an existing grant.
	r.ManifestDigest = RequestHash([]any{r, merchant, "http://127.0.0.1:4321/seller", "embedded-demo-client"})
	return r, nil
}

// WithEmbeddedPilotAccess performs a fresh Core owner, release and grant check
// under the same installation lock as uninstall. This is not a resource token.
func (s Lifecycle) WithEmbeddedPilotAccess(ctx context.Context, p identity.ServicePrincipal, actor, id string, fn func(domain.Access) error) error {
	o, err := s.owner(ctx, p, actor)
	if err != nil {
		return err
	}
	if !opaqueID.MatchString(id) || fn == nil {
		return fault.Invalid
	}
	release, err := s.Intents.EmbeddedPilot.release(o.TenantID)
	if err != nil {
		return err
	}
	repo, ok := s.Repo.(interface {
		WithCoreAccess(context.Context, domain.IntentOwner, string, func(domain.Access) error) error
	})
	if !ok {
		return fault.Unavailable
	}
	return repo.WithCoreAccess(ctx, o, id, func(a domain.Access) error {
		if RequestHash(a.Release) != RequestHash(release) || a.Installation.Status != "active" || a.GrantState != "active" || !slices.Equal(a.GrantedScopes, release.Scopes) {
			return fault.Forbidden
		}
		return fn(a)
	})
}
