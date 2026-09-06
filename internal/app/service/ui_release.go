package service

import (
	"context"
	"crypto/ed25519"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/uirelease"
	"strings"
)

type UIRelease struct {
	ID       string             `json:"id"`
	Manifest uirelease.Manifest `json:"manifest"`
	Package  *uirelease.Package `json:"package,omitempty"`
	Status   string             `json:"status"`
	Revision int                `json:"revision"`
}
type UIReleaseInput struct {
	AppID   string `json:"appId,omitempty"`
	Version string `json:"version"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Mode    string `json:"mode"`
	URL     string `json:"url"`
	Reason  string `json:"reason"`
}
type UIReleaseRepository interface {
	UIWith(context.Context, string, string, func(UIRelease) error) error
	UIList(context.Context, string, string) ([]UIRelease, error)
	UIGet(context.Context, string, string) (UIRelease, error)
	UIChange(context.Context, string, string, string, string, string, *UIRelease, string, func(UIRelease) (UIRelease, error)) (UIRelease, error)
}
type UIReleases struct {
	Repo       UIReleaseRepository
	Developers developer.Service
	Key        ed25519.PrivateKey
}

func (s UIReleases) WithSigned(ctx context.Context, org, id string, fn func(UIRelease) error) error {
	if s.Repo == nil || len(s.Key) != ed25519.PrivateKeySize {
		return fault.Unavailable
	}
	if org == "" {
		return fault.Forbidden
	}
	return s.Repo.UIWith(ctx, org, id, func(v UIRelease) error {
		if v.Status != "signed" || v.Package == nil || v.Manifest != v.Package.Manifest || uirelease.Verify(*v.Package, s.Key.Public().(ed25519.PublicKey)) != nil {
			return fault.Conflict
		}
		return fn(v)
	})
}

func (s UIReleases) List(ctx context.Context, p identity.PortalPrincipal, after string) ([]UIRelease, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return nil, err
	}
	if after != "" && !testIdentifier.MatchString(after) {
		return nil, fault.Invalid
	}
	return s.Repo.UIList(ctx, org, after)
}

func (s UIReleases) Get(ctx context.Context, p identity.PortalPrincipal, id string) (UIRelease, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return UIRelease{}, err
	}
	return s.Repo.UIGet(ctx, org, id)
}
func (s UIReleases) scope(ctx context.Context, p identity.PortalPrincipal) (string, error) {
	if p.ID != "" && p.Surface == "admin" {
		return "", nil
	}
	o, err := s.Developers.Organization(ctx, p)
	return o.ID, err
}
func (s UIReleases) Submit(ctx context.Context, p identity.PortalPrincipal, key string, b UIReleaseInput) (UIRelease, error) {
	if p.Surface != "developer" {
		return UIRelease{}, fault.Forbidden
	}
	org, err := s.scope(ctx, p)
	if err != nil {
		return UIRelease{}, err
	}
	if !ValidRequestKey(key) || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return UIRelease{}, fault.Invalid
	}
	app := b.AppID
	if app == "" {
		app = ids.New("app")
	}
	m := uirelease.Manifest{Schema: uirelease.Schema, Policy: uirelease.Policy, AppID: app, DeveloperID: org, Version: b.Version, Name: b.Name, Summary: b.Summary, Mode: b.Mode, URL: b.URL, Pricing: "free"}
	if m.Validate() != nil {
		return UIRelease{}, fault.Invalid
	}
	v := UIRelease{ID: ids.New("uirelease"), Manifest: m, Status: "submitted", Revision: 1}
	// A supplied app ID must already belong to this organization. A new identity
	// is always generated server-side; request replay resolves the original ID.
	target := b.AppID
	return s.Repo.UIChange(ctx, org, p.ID, key, RequestHash(b), target, &v, b.Reason, nil)
}
func (s UIReleases) Decide(ctx context.Context, p identity.PortalPrincipal, id, key string, b CatalogAction) (UIRelease, error) {
	if p.ID == "" || p.Surface != "admin" || (p.Role != "administrator" && p.Role != "reviewer") {
		return UIRelease{}, fault.Forbidden
	}
	if (b.Status == "signed" || b.Status == "suspended") && p.Role != "administrator" {
		return UIRelease{}, fault.Forbidden
	}
	if !ValidRequestKey(key) || b.Revision < 1 || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return UIRelease{}, fault.Invalid
	}
	return s.Repo.UIChange(ctx, "", p.ID, key, RequestHash(struct {
		ID     string
		Action CatalogAction
	}{id, b}), id, nil, b.Reason, func(v UIRelease) (UIRelease, error) {
		if v.Revision != b.Revision {
			return v, fault.Conflict
		}
		if !((v.Status == "submitted" && (b.Status == "approved" || b.Status == "rejected")) ||
			(v.Status == "approved" && (b.Status == "signed" || b.Status == "suspended")) || (v.Status == "signed" && b.Status == "suspended")) {
			return v, fault.Conflict
		}
		if b.Status == "signed" {
			pkg, err := uirelease.Sign(v.Manifest, s.Key)
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
