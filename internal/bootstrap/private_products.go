package bootstrap

import (
	"context"
	"crypto/ed25519"
	appservice "emisell.app/platform/internal/app/service"
	devrepo "emisell.app/platform/internal/developer/postgres"
	"emisell.app/platform/internal/identity/merchantlogin"
	"emisell.app/platform/internal/installation/domain"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appidentity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/transport/connectapi"
	"emisell.app/platform/pkg/accessscope"
	"github.com/jackc/pgx/v5"
	"slices"
	"strings"
)

// Separate signed source for private, headless product readers. No marketplace
// review/assignment is implied and there is no fallback to fixture execution.
type PrivateProductInstallSource struct {
	UI       ReviewedUIInstallSource
	Existing installservice.ManagedReleases
}

func (s PrivateProductInstallSource) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	found, err := s.UI.Releases.HasPrivateProducts(ctx, app)
	if err != nil {
		return err
	}
	if !found {
		if s.Existing == nil {
			return fault.NotFound
		}
		return s.Existing.WithRelease(ctx, merchant, app, version, fn)
	}
	owner := installservice.ResourceOwner(ctx)
	if owner.TenantID != merchant || owner.ActorID == "" || owner.ServiceID == "" || s.UI.Products == nil || len(s.UI.ResourceKey) != ed25519.PrivateKeySize {
		return fault.Forbidden
	}
	return s.UI.Releases.WithPrivateProducts(ctx, app, version, s.UI.ResourceKey.Public().(ed25519.PublicKey), func(v appservice.PrivateProductVersion, digest string) error {
		identities := merchantlogin.Repository{Pool: s.UI.Clients.Pool}
		return identities.WithCoreOwner(ctx, v.OwnerAccountID, owner.ActorID, func(tx pgx.Tx) error {
			if err := (devrepo.Repository{}).OwnsOrganizationTx(ctx, tx, v.OwnerAccountID, v.OrganizationID); err != nil {
				return err
			}
			client, err := (appidentity.Repository{}).ClientIDTx(ctx, tx, v.OrganizationID, v.AppID)
			if err != nil || client != v.ClientID {
				return fault.Forbidden
			}
			return fn(domain.IntentRelease{InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy,
				AppID: v.AppID, Name: v.Document.Name, DeveloperID: v.OrganizationID, Version: v.Document.Version, ManifestDigest: digest,
				Scopes: []string{"read_products"}, Capabilities: []string{}, ResourceBinding: &domain.ResourceBinding{ReleaseID: v.ID, ClientID: v.ClientID,
					AccessScopes: accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}}})
		})
	})
}
func (s PrivateProductInstallSource) ReadyResource(ctx context.Context, r domain.IntentRelease) error {
	if r.ResourceBinding == nil {
		return fault.Forbidden
	}
	if !strings.HasPrefix(r.ResourceBinding.ReleaseID, "privateversion_") {
		return s.UI.ReadyResource(ctx, r)
	}
	if s.UI.Products == nil || installservice.ResourceOwner(ctx).ActorID == "" || r.UIBinding != nil || r.ManagedSource != nil || r.ShippingProvider != nil ||
		!slices.Equal(r.Scopes, []string{"read_products"}) || len(r.Capabilities) != 0 || r.ResourceBinding.Webhooks != nil ||
		!strings.HasPrefix(r.ResourceBinding.ClientID, "eai_") {
		return fault.Forbidden
	}
	return nil
}

func enablePrivateProducts(base *connectapi.Server, ui ReviewedUIInstallSource) {
	source := PrivateProductInstallSource{UI: ui, Existing: base.Intents.Managed}
	base.Intents.Managed = source
	base.Lifecycle.Intents = base.Intents
	base.Lifecycle.Resources = source
	box, _ := localfiles.ReadApplicationCredentialBox()
	identities := appidentity.Repository{Pool: ui.Clients.Pool, Box: box}
	base.PrivateResourceAuth = func(ctx context.Context, id, secret string) (string, error) {
		c, err := identities.Authenticate(ctx, id, secret)
		return c.AppID, err
	}
}
