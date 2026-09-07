package bootstrap

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
	"errors"
)

// Cross-module adapter: OAuth never reads app tables, app never imports OAuth.
type clientReleases struct {
	Integrations service.Integrations
	UI           service.UIReleases
	Resources    service.UIResourceReleases
}

func (s clientReleases) WithBinding(ctx context.Context, org, id string, fn func(appclient.Binding) error) error {
	if fn == nil {
		return fault.Invalid
	}
	if s.Resources.Repo != nil {
		_, err := s.Resources.Repo.UIResourceGet(ctx, org, id)
		if err == nil {
			return s.Resources.WithSigned(ctx, org, id, func(v service.UIResourceRelease) error {
				m := v.Manifest.UI
				// Bind the outer digest including required permissions, never UI metadata alone.
				return fn(appclient.Binding{ReleaseID: v.ID, OrganizationID: m.DeveloperID, AppID: m.AppID, Version: m.Version, Name: m.Name, Digest: v.Package.SHA256, Endpoint: m.URL})
			})
		}
		if !errors.Is(err, fault.NotFound) {
			return err
		}
	}
	if s.UI.Repo != nil {
		_, err := s.UI.Repo.UIGet(ctx, org, id)
		if err == nil {
			return s.UI.WithSigned(ctx, org, id, func(v service.UIRelease) error {
				m := v.Manifest
				return fn(appclient.Binding{ReleaseID: v.ID, OrganizationID: m.DeveloperID, AppID: m.AppID, Version: m.Version, Name: m.Name, Digest: v.Package.SHA256, Endpoint: m.URL})
			})
		}
		if !errors.Is(err, fault.NotFound) {
			return err
		}
	}
	return s.Integrations.WithSigned(ctx, org, id, func(v service.IntegrationRelease) error {
		m := v.Manifest
		return fn(appclient.Binding{ReleaseID: v.ID, OrganizationID: v.OrganizationID, AppID: m.Metadata.AppID, Version: m.Metadata.Version, Name: m.Metadata.Name, Digest: v.SHA256, Endpoint: m.Config.Endpoint, RedirectURI: m.Config.CallbackURL})
	})
}
