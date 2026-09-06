package service

import (
	"context"
	"strings"
	"time"

	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/managedshipping"
)

type ManagedShippingRelease struct {
	ID             string                   `json:"id"`
	OrganizationID string                   `json:"organizationId"`
	Manifest       managedshipping.Manifest `json:"manifest"`
	SHA256         string                   `json:"sha256"`
	Package        *managedshipping.Package `json:"package,omitempty"`
	Status         string                   `json:"status"`
	Revision       int                      `json:"revision"`
	CreatedAt      time.Time                `json:"createdAt"`
	UpdatedAt      time.Time                `json:"updatedAt"`
}
type ManagedShippingInput struct {
	AppID         string                  `json:"appId"`
	DraftRevision int                     `json:"draftRevision"`
	Binding       managedshipping.Binding `json:"binding"`
	Reason        string                  `json:"reason"`
}
type ManagedShippingSigner interface {
	Sign(managedshipping.Manifest) (managedshipping.Package, error)
	Verify(managedshipping.Package) error
	PublicKey() []byte
}
type ManagedShippingRepository interface {
	ManagedShippingWith(context.Context, string, string, func(ManagedShippingRelease) error) error
	ManagedShippingReplay(context.Context, string, string, string, string) (*ManagedShippingRelease, error)
	ManagedShippingList(context.Context, string) ([]ManagedShippingRelease, error)
	ManagedShippingGet(context.Context, string, string) (ManagedShippingRelease, error)
	ManagedShippingHistory(context.Context, string) ([]CatalogAudit, error)
	ManagedShippingMutate(context.Context, string, string, string, string, string, *ManagedShippingRelease, string, func(ManagedShippingRelease) (ManagedShippingRelease, error)) (ManagedShippingRelease, error)
}

// WithSigned retains a shared release lock through a short distribution commit.
// Use a separate gate pool to avoid starving the writer's pool with waiters.
func (s ManagedShipping) WithSigned(ctx context.Context, org, id string, fn func(ManagedShippingRelease) error) error {
	if s.Repo == nil || s.Signer == nil {
		return fault.Unavailable
	}
	return s.Repo.ManagedShippingWith(ctx, org, id, func(v ManagedShippingRelease) error {
		if !s.Readiness(v).ConfigurationReady {
			return fault.Conflict
		}
		return fn(v)
	})
}

type ManagedShipping struct {
	Repo       ManagedShippingRepository
	Drafts     DraftRepository
	Developers developer.Service
	Signer     ManagedShippingSigner
}

func (s ManagedShipping) scope(ctx context.Context, p identity.PortalPrincipal) (string, error) {
	if p.ID != "" && p.Surface == "admin" {
		return "", nil
	}
	org, err := s.Developers.Organization(ctx, p)
	return org.ID, err
}
func (s ManagedShipping) List(ctx context.Context, p identity.PortalPrincipal) ([]ManagedShippingRelease, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return nil, err
	}
	return s.Repo.ManagedShippingList(ctx, org)
}
func (s ManagedShipping) Get(ctx context.Context, p identity.PortalPrincipal, id string) (ManagedShippingRelease, []CatalogAudit, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return ManagedShippingRelease{}, nil, err
	}
	v, err := s.Repo.ManagedShippingGet(ctx, org, id)
	if err != nil {
		return v, nil, err
	}
	h, err := s.Repo.ManagedShippingHistory(ctx, id)
	return v, h, err
}
func (s ManagedShipping) Submit(ctx context.Context, p identity.PortalPrincipal, key string, b ManagedShippingInput) (ManagedShippingRelease, error) {
	if p.Surface != "developer" || p.ID == "" {
		return ManagedShippingRelease{}, fault.Forbidden
	}
	org, err := s.scope(ctx, p)
	if err != nil {
		return ManagedShippingRelease{}, err
	}
	if !ValidRequestKey(key) || !testIdentifier.MatchString(b.AppID) || b.DraftRevision < 1 || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return ManagedShippingRelease{}, fault.Invalid
	}
	if old, err := s.Repo.ManagedShippingReplay(ctx, org, p.ID, key, RequestHash(b)); err != nil {
		return ManagedShippingRelease{}, err
	} else if old != nil {
		return *old, nil
	}
	d, err := s.Drafts.Draft(ctx, org, b.AppID)
	if err != nil {
		return ManagedShippingRelease{}, err
	}
	if d.Revision != b.DraftRevision {
		return ManagedShippingRelease{}, fault.Conflict
	}
	if d.OrganizationID != org || d.ID != b.AppID || d.Document.Endpoint != "" || d.Document.AccessScopes != nil || d.Document.Validate(false) != nil {
		return ManagedShippingRelease{}, fault.Invalid
	}
	m := managedshipping.Manifest{Schema: managedshipping.Schema, Policy: managedshipping.Policy, AppID: d.ID, DeveloperID: org,
		Version: d.Document.Version, Name: d.Document.Name, Summary: d.Document.Summary, Description: d.Document.Description,
		DraftRevision: d.Revision, SourceSHA256: RequestHash(d), Capability: d.Document.Capability, Scopes: d.Document.Scopes, Binding: b.Binding, Pricing: "free"}
	raw, err := managedshipping.Canonical(m)
	if err != nil {
		return ManagedShippingRelease{}, fault.Invalid
	}
	v := ManagedShippingRelease{OrganizationID: org, Manifest: m, SHA256: managedshipping.Digest(raw)}
	return s.Repo.ManagedShippingMutate(ctx, org, p.ID, key, RequestHash(b), "", &v, b.Reason, nil)
}
func (s ManagedShipping) Transition(ctx context.Context, p identity.PortalPrincipal, id, key string, b CatalogAction) (ManagedShippingRelease, error) {
	if p.ID == "" || p.Surface != "admin" || (p.Role != "administrator" && p.Role != "reviewer") {
		return ManagedShippingRelease{}, fault.Forbidden
	}
	if (b.Status == "signed" || b.Status == "suspended") && p.Role != "administrator" {
		return ManagedShippingRelease{}, fault.Forbidden
	}
	if !ValidRequestKey(key) || !testIdentifier.MatchString(id) || b.Revision < 1 || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return ManagedShippingRelease{}, fault.Invalid
	}
	if b.Status != "approved" && b.Status != "rejected" && b.Status != "signed" && b.Status != "suspended" {
		return ManagedShippingRelease{}, fault.Invalid
	}
	return s.Repo.ManagedShippingMutate(ctx, "", p.ID, key, RequestHash(struct {
		ID     string
		Action CatalogAction
	}{id, b}), id, nil, b.Reason, func(v ManagedShippingRelease) (ManagedShippingRelease, error) {
		if v.Revision != b.Revision {
			return v, fault.Conflict
		}
		valid := (v.Status == "submitted" && (b.Status == "approved" || b.Status == "rejected")) ||
			(v.Status == "approved" && (b.Status == "signed" || b.Status == "suspended")) || (v.Status == "signed" && b.Status == "suspended")
		if !valid {
			return v, fault.Conflict
		}
		if b.Status == "approved" || b.Status == "signed" {
			raw, err := managedshipping.Canonical(v.Manifest)
			if err != nil || managedshipping.Digest(raw) != v.SHA256 || v.Manifest.DeveloperID != v.OrganizationID {
				return v, fault.Invalid
			}
		}
		if b.Status == "signed" {
			if s.Signer == nil {
				return v, fault.Unavailable
			}
			pack, err := s.Signer.Sign(v.Manifest)
			if err != nil || s.Signer.Verify(pack) != nil || pack.SHA256 != v.SHA256 {
				return v, fault.Unavailable
			}
			v.Package = &pack
		}
		v.Status = b.Status
		v.Revision++
		return v, nil
	})
}

// Publishing identity/configuration is not installation eligibility. The engine
// grant gate and merchant-specific distribution must be wired before enabling it.
func (s ManagedShipping) Readiness(v ManagedShippingRelease) TestReadiness {
	valid := false
	if v.Status == "signed" && v.Package != nil && s.Signer != nil {
		raw, err := managedshipping.Canonical(v.Manifest)
		valid = err == nil && managedshipping.Digest(raw) == v.SHA256 && v.Package.SHA256 == v.SHA256 && v.Manifest.DeveloperID == v.OrganizationID && s.Signer.Verify(*v.Package) == nil
	}
	return TestReadiness{ConfigurationReady: valid, RequiredScopesReady: v.Manifest.Validate() == nil, Installable: false,
		Blockers: []string{"managed_installation_not_available", "engine_grant_enforcement_not_available"}}
}
