package bootstrap

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/accessscope"
)

// ResourceAssignmentGate must retain an approved merchant assignment lock while
// calling fn. IDs come from persisted review state, never a browser assertion.
type ResourceAssignmentGate interface {
	WithApproved(context.Context, string, string, string, func(organization, release, client string) error) error
}

// UIResourceInstallSource joins authoring and app-client proof to consent input.
// Lock order: assignment -> release -> client -> installation. A signature or
// approved assignment alone never creates an active grant.
type UIResourceInstallSource struct {
	Assignments ResourceAssignmentGate
	Releases    service.UIResourceReleases
	Clients     appclient.Service
}

func (s UIResourceInstallSource) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if s.Assignments == nil || s.Releases.Repo == nil || s.Clients.Repo == nil {
		return fault.Unavailable
	}
	if merchant == "" || app == "" || version == "" || fn == nil {
		return fault.Invalid
	}
	return s.Assignments.WithApproved(ctx, merchant, app, version, func(org, release, client string) error {
		if org == "" || release == "" || client == "" {
			return fault.Forbidden
		}
		return s.Releases.WithSigned(ctx, org, release, func(v service.UIResourceRelease) error {
			m := v.Manifest.UI
			if v.ID != release || m.DeveloperID != org || m.AppID != app || m.Version != version {
				return fault.Forbidden
			}
			b := appclient.Binding{ReleaseID: release, OrganizationID: org, AppID: app, Version: version, Name: m.Name, Digest: v.Package.SHA256, Endpoint: m.URL}
			return s.Clients.WithBoundReady(ctx, client, b, func(appclient.Client) error {
				scopes := append([]string(nil), v.Manifest.RequiredScopes...)
				return fn(domain.IntentRelease{InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy, AppID: app, Name: m.Name, DeveloperID: org, Version: version, ManifestDigest: v.Package.SHA256, Scopes: scopes, Capabilities: []string{}, ResourceBinding: &domain.ResourceBinding{ReleaseID: release, ClientID: client, AccessScopes: accessscope.Declaration{Profile: accessscope.Profile, Required: append([]string(nil), scopes...), Optional: []string{}}}})
			})
		})
	})
}
