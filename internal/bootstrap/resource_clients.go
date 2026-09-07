package bootstrap

import (
	"context"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
)

// ResourceClientSource composes a trusted resource release/assignment source
// with current signed-release and confidential-client state. It is intentionally
// not mounted until the executable resource release source is configured.
// Lock order: resource assignment -> signed client release -> app-client -> installation.
type ResourceClientSource struct {
	Source  service.ManagedReleases
	Clients appclient.Service
}

func (s ResourceClientSource) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if s.Source == nil || s.Clients.Repo == nil || s.Clients.Releases == nil {
		return fault.Unavailable
	}
	if fn == nil {
		return fault.Invalid
	}
	return s.Source.WithRelease(ctx, merchant, app, version, func(r domain.IntentRelease) error {
		if r.AppID != app || r.Version != version || r.InstallPolicy != domain.ResourceAppPolicy || r.ExecutionProfile != domain.ResourceAppPolicy || r.ResourceBinding == nil || r.DeveloperID == "" {
			return fault.Forbidden
		}
		b := r.ResourceBinding
		return s.Clients.Releases.WithBinding(ctx, r.DeveloperID, b.ReleaseID, func(binding appclient.Binding) error {
			if binding.ReleaseID != b.ReleaseID || binding.OrganizationID != r.DeveloperID || binding.AppID != r.AppID || binding.Version != r.Version || binding.Digest != r.ManifestDigest {
				return fault.Forbidden
			}
			return s.Clients.WithBoundReady(ctx, b.ClientID, binding, func(client appclient.Client) error { return fn(r) })
		})
	})
}
