package bootstrap

import (
	"context"
	"crypto/ed25519"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/installation/domain"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appclient"
	oauthui "emisell.app/platform/internal/oauth/embedded"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/embedded"
	"emisell.app/platform/pkg/uiresource"
	"reflect"
	"slices"
)

func (s ReviewedUIInstallSource) WithResourceRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if s.Products == nil || len(s.ResourceKey) != ed25519.PrivateKeySize {
		return fault.Unavailable
	}
	return s.Releases.WithResourceUIAssignment(ctx, merchant, app, version, func(a appservice.Assignment, v appservice.UIResourceRelease) error {
		if v.Status != "signed" || v.Package == nil || !reflect.DeepEqual(v.Manifest, v.Package.Manifest) || uiresource.Verify(*v.Package, s.ResourceKey.Public().(ed25519.PublicKey)) != nil || a.ReleaseSHA256 != v.Package.SHA256 {
			return fault.Forbidden
		}
		m := v.Manifest.UI
		c, err := s.Clients.ForRelease(ctx, m.DeveloperID, v.ID)
		if err != nil {
			return err
		}
		expected := appclient.Binding{ReleaseID: v.ID, OrganizationID: m.DeveloperID, AppID: m.AppID, Version: m.Version, Name: m.Name, Digest: v.Package.SHA256, Endpoint: m.URL}
		return s.Current.WithBinding(ctx, c.ID, expected, func(b oauthui.Binding) error {
			if b.Launch.URL != m.URL || b.Launch.DisplayMode() != m.Mode {
				return fault.Forbidden
			}
			return fn(domain.IntentRelease{AppID: m.AppID, Name: m.Name, DeveloperID: m.DeveloperID, Version: m.Version, ManifestDigest: v.Package.SHA256,
				InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy, Scopes: slices.Clone(v.Manifest.RequiredScopes), Capabilities: []string{},
				UIBinding:       &domain.UIBinding{AssignmentID: a.ID, ReleaseID: v.ID, Launch: b.Launch, Signature: b.Signature},
				ResourceBinding: &domain.ResourceBinding{ReleaseID: v.ID, ClientID: c.ID, AccessScopes: accessscope.Declaration{Profile: accessscope.ReviewedProfile(v.Manifest.RequiredScopes), Required: slices.Clone(v.Manifest.RequiredScopes), Optional: []string{}}}})
		})
	})
}

// Called only inside the current managed release callback. Its release,
// assignment, client and launch locks are still held; do not reacquire them.
func (s ReviewedUIInstallSource) ReadyResource(ctx context.Context, r domain.IntentRelease) error {
	if s.Products == nil || installservice.ResourceMerchant(ctx) == "" || len(s.ResourceKey) != ed25519.PrivateKeySize || r.UIBinding == nil || r.ResourceBinding == nil {
		return fault.Unavailable
	}
	b, u := r.ResourceBinding, r.UIBinding
	if !uiresource.ValidScopes(r.Scopes) || !slices.Equal(b.AccessScopes.Required, r.Scopes) || len(b.AccessScopes.Optional) != 0 || b.Webhooks != nil ||
		u.AssignmentID == "" || b.ReleaseID != u.ReleaseID || b.ClientID != u.Launch.ClientID || u.Launch.AppID != r.AppID || u.Launch.ReleaseDigest != r.ManifestDigest || u.Launch.ParentOrigin != s.Current.ParentOrigin ||
		embedded.VerifyLaunch(u.Launch, u.Signature, s.Current.Key, false) != nil {
		return fault.Forbidden
	}
	return nil
}
