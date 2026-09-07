package service

import (
	"context"
	"crypto/ed25519"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/uirelease"
	"emisell.app/platform/pkg/uiresource"
	"reflect"
	"strings"
)

type UIResourceRelease struct {
	ID       string              `json:"id"`
	Manifest uiresource.Manifest `json:"manifest"`
	Package  *uiresource.Package `json:"package,omitempty"`
	Status   string              `json:"status"`
	Revision int                 `json:"revision"`
}
type UIResourceReleaseInput struct {
	RequiredScopes []string `json:"requiredScopes"`
	AppID          string   `json:"appId,omitempty"`
	Version        string   `json:"version"`
	Name           string   `json:"name"`
	Summary        string   `json:"summary"`
	Mode           string   `json:"mode"`
	URL            string   `json:"url"`
	Reason         string   `json:"reason"`
}
type UIResourceReleaseRepository interface {
	UIResourceWith(context.Context, string, string, func(UIResourceRelease) error) error
	UIResourceList(context.Context, string, string) ([]UIResourceRelease, error)
	UIResourceGet(context.Context, string, string) (UIResourceRelease, error)
	UIResourceChange(context.Context, string, string, string, string, string, *UIResourceRelease, string, func(UIResourceRelease) (UIResourceRelease, error)) (UIResourceRelease, error)
}
type UIResourceReleases struct {
	Repo       UIResourceReleaseRepository
	Developers developer.Service
	Key        ed25519.PrivateKey
}

func (s UIResourceReleases) WithSigned(ctx context.Context, org, id string, fn func(UIResourceRelease) error) error {
	if s.Repo == nil || len(s.Key) != ed25519.PrivateKeySize {
		return fault.Unavailable
	}
	if org == "" {
		return fault.Forbidden
	}
	return s.Repo.UIResourceWith(ctx, org, id, func(v UIResourceRelease) error {
		if v.Status != "signed" || v.Package == nil || !reflect.DeepEqual(v.Manifest, v.Package.Manifest) || uiresource.Verify(*v.Package, s.Key.Public().(ed25519.PublicKey)) != nil {
			return fault.Conflict
		}
		return fn(v)
	})
}

func (s UIResourceReleases) List(ctx context.Context, p identity.PortalPrincipal, after string) ([]UIResourceRelease, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return nil, err
	}
	if after != "" && !testIdentifier.MatchString(after) {
		return nil, fault.Invalid
	}
	return s.Repo.UIResourceList(ctx, org, after)
}

func (s UIResourceReleases) Get(ctx context.Context, p identity.PortalPrincipal, id string) (UIResourceRelease, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return UIResourceRelease{}, err
	}
	return s.Repo.UIResourceGet(ctx, org, id)
}
func (s UIResourceReleases) scope(ctx context.Context, p identity.PortalPrincipal) (string, error) {
	if p.ID != "" && p.Surface == "admin" {
		return "", nil
	}
	o, err := s.Developers.Organization(ctx, p)
	return o.ID, err
}
func (s UIResourceReleases) Submit(ctx context.Context, p identity.PortalPrincipal, key string, b UIResourceReleaseInput) (UIResourceRelease, error) {
	if p.Surface != "developer" {
		return UIResourceRelease{}, fault.Forbidden
	}
	org, err := s.scope(ctx, p)
	if err != nil {
		return UIResourceRelease{}, err
	}
	if !ValidRequestKey(key) || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return UIResourceRelease{}, fault.Invalid
	}
	app := b.AppID
	if app == "" {
		app = ids.New("app")
	}
	ui := uirelease.Manifest{Schema: uirelease.Schema, Policy: uirelease.Policy, AppID: app, DeveloperID: org, Version: b.Version, Name: b.Name, Summary: b.Summary, Mode: b.Mode, URL: b.URL, Pricing: "free"}
	m := uiresource.Manifest{Schema: uiresource.Schema, Policy: uiresource.Policy, UI: ui, RequiredScopes: append([]string(nil), b.RequiredScopes...)}
	if m.Validate() != nil {
		return UIResourceRelease{}, fault.Invalid
	}
	v := UIResourceRelease{ID: ids.New("uirelease"), Manifest: m, Status: "submitted", Revision: 1}
	// A supplied app ID must already belong to this organization. A new identity
	// is always generated server-side; request replay resolves the original ID.
	target := b.AppID
	return s.Repo.UIResourceChange(ctx, org, p.ID, key, RequestHash(b), target, &v, b.Reason, nil)
}
func (s UIResourceReleases) Decide(ctx context.Context, p identity.PortalPrincipal, id, key string, b CatalogAction) (UIResourceRelease, error) {
	if p.ID == "" || p.Surface != "admin" || (p.Role != "administrator" && p.Role != "reviewer") {
		return UIResourceRelease{}, fault.Forbidden
	}
	if (b.Status == "signed" || b.Status == "suspended") && p.Role != "administrator" {
		return UIResourceRelease{}, fault.Forbidden
	}
	if !ValidRequestKey(key) || b.Revision < 1 || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return UIResourceRelease{}, fault.Invalid
	}
	return s.Repo.UIResourceChange(ctx, "", p.ID, key, RequestHash(struct {
		ID     string
		Action CatalogAction
	}{id, b}), id, nil, b.Reason, func(v UIResourceRelease) (UIResourceRelease, error) {
		if v.Revision != b.Revision {
			return v, fault.Conflict
		}
		if !((v.Status == "submitted" && (b.Status == "approved" || b.Status == "rejected")) ||
			(v.Status == "approved" && (b.Status == "signed" || b.Status == "suspended")) || (v.Status == "signed" && b.Status == "suspended")) {
			return v, fault.Conflict
		}
		if b.Status == "signed" {
			pkg, err := uiresource.Sign(v.Manifest, s.Key)
			if err != nil {
				return v, fault.Unavailable
			}
			v.Package = &pkg
		}
		v.Status = b.Status
		v.Revision++
		return v, nil
	})
}
