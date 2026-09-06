package service

import (
	"context"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/catalogmanifest"
	"slices"
	"strings"
	"time"
)

type CatalogCandidate struct {
	SubmissionID, AppID, OrganizationID string
	Revision                            int
	Document                            AppDocument
}
type CatalogSource interface {
	CatalogCandidate(context.Context, identity.PortalPrincipal, string) (CatalogCandidate, error)
}
type CatalogSigner interface {
	Sign(catalogmanifest.Manifest) (catalogmanifest.Package, error)
	Verify(catalogmanifest.Package) error
	PublicKey() []byte
}
type CatalogRelease struct {
	ID           string                  `json:"id"`
	SubmissionID string                  `json:"submissionId"`
	Package      catalogmanifest.Package `json:"package"`
	Status       string                  `json:"status"`
	Revision     int                     `json:"revision"`
	CreatedAt    time.Time               `json:"createdAt"`
	UpdatedAt    time.Time               `json:"updatedAt"`
}
type CatalogAction struct {
	Status   string `json:"status"`
	Revision int    `json:"revision"`
	Reason   string `json:"reason"`
}
type CatalogAudit struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actorId"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurredAt"`
}
type CatalogQuery struct {
	Search, Capability string
	Page               int
}
type CatalogRepository interface {
	CatalogList(context.Context, string) ([]CatalogRelease, error)
	CatalogGet(context.Context, string, string) (CatalogRelease, error)
	CatalogSign(context.Context, string, string, string, CatalogCandidate, catalogmanifest.Package) (CatalogRelease, error)
	CatalogTransition(context.Context, string, string, string, string, CatalogAction) (CatalogRelease, error)
	CatalogHistory(context.Context, string) ([]CatalogAudit, error)
	CatalogPublic(context.Context, CatalogQuery) ([]CatalogRelease, int, error)
}
type Catalog struct {
	Repo       CatalogRepository
	Source     CatalogSource
	Signer     CatalogSigner
	Developers developer.Service
}

func catalogManifest(c CatalogCandidate) catalogmanifest.Manifest {
	scopes := slices.Clone(c.Document.Scopes)
	slices.Sort(scopes)
	m := catalogmanifest.Manifest{Schema: catalogmanifest.Schema, Policy: catalogmanifest.Policy, AppID: c.AppID, DeveloperID: c.OrganizationID, Version: c.Document.Version, Name: c.Document.Name, Summary: c.Document.Summary, Description: c.Document.Description, Capability: c.Document.Capability, Scopes: scopes, Runtime: "remote", Pricing: "free", Installable: false, SourceSHA256: RequestHash(c)}
	if c.Document.AccessScopes != nil {
		d := c.Document.AccessScopes.Canonical()
		m.Schema, m.Policy, m.AccessScopes = catalogmanifest.SchemaAccessScopes, catalogmanifest.PolicyAccessScopes, &d
	}
	return m
}
func (s Catalog) scope(ctx context.Context, p identity.PortalPrincipal) (string, error) {
	if p.ID != "" && p.Surface == "admin" {
		return "", nil
	}
	org, err := s.Developers.Organization(ctx, p)
	return org.ID, err
}
func (s Catalog) List(ctx context.Context, p identity.PortalPrincipal) ([]CatalogRelease, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return nil, err
	}
	return s.Repo.CatalogList(ctx, org)
}
func (s Catalog) Get(ctx context.Context, p identity.PortalPrincipal, id string) (CatalogRelease, []CatalogAudit, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return CatalogRelease{}, nil, err
	}
	v, err := s.Repo.CatalogGet(ctx, org, id)
	if err != nil {
		return v, nil, err
	}
	history, err := s.Repo.CatalogHistory(ctx, id)
	return v, history, err
}
func (s Catalog) Sign(ctx context.Context, p identity.PortalPrincipal, submission, key string) (CatalogRelease, error) {
	if p.Surface != "admin" || p.Role != "administrator" || p.ID == "" {
		return CatalogRelease{}, fault.Forbidden
	}
	if !ValidRequestKey(key) {
		return CatalogRelease{}, fault.Invalid
	}
	if s.Signer == nil {
		return CatalogRelease{}, fault.Unavailable
	}
	candidate, err := s.Source.CatalogCandidate(ctx, p, submission)
	if err != nil {
		return CatalogRelease{}, err
	}
	if err = candidate.Document.Validate(true); err != nil {
		return CatalogRelease{}, err
	}
	if err = ValidatePublicDistribution(candidate.Document.Capability); err != nil {
		return CatalogRelease{}, err
	}
	m := catalogManifest(candidate)
	if err = m.Validate(); err != nil {
		return CatalogRelease{}, fault.Invalid
	}
	pack, err := s.Signer.Sign(m)
	if err != nil {
		return CatalogRelease{}, fault.Unavailable
	}
	return s.Repo.CatalogSign(ctx, p.ID, key, RequestHash([]string{"sign", submission}), candidate, pack)
}
func (s Catalog) Transition(ctx context.Context, p identity.PortalPrincipal, id, key string, b CatalogAction) (CatalogRelease, error) {
	if p.Surface != "admin" || p.Role != "administrator" || p.ID == "" {
		return CatalogRelease{}, fault.Forbidden
	}
	if !ValidRequestKey(key) || (b.Status != "published" && b.Status != "suspended") || b.Revision < 1 || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return CatalogRelease{}, fault.Invalid
	}
	// Suspensions must remain possible even when a signing key is missing/corrupt.
	if b.Status == "published" {
		if s.Signer == nil {
			return CatalogRelease{}, fault.Unavailable
		}
		v, err := s.Repo.CatalogGet(ctx, "", id)
		if err != nil {
			return CatalogRelease{}, err
		}
		if err = s.Signer.Verify(v.Package); err != nil {
			return CatalogRelease{}, fault.Invalid
		}
		if err = ValidatePublicDistribution(v.Package.Manifest.Capability); err != nil {
			return CatalogRelease{}, err
		}
	}
	return s.Repo.CatalogTransition(ctx, p.ID, id, key, RequestHash(struct {
		ID   string
		Body CatalogAction
	}{id, b}), b)
}

// CatalogEntry is an allowlisted public DTO. No endpoint, credential, review
// notes, actor identity, submission ID, private manifest or tenant data.
type CatalogEntry struct {
	ID           string                   `json:"id"`
	AppID        string                   `json:"appId"`
	DeveloperID  string                   `json:"developerId"`
	Name         string                   `json:"name"`
	Summary      string                   `json:"summary"`
	Description  string                   `json:"description"`
	Version      string                   `json:"version"`
	Capability   string                   `json:"capability"`
	Scopes       []string                 `json:"scopes"`
	Pricing      string                   `json:"pricing"`
	Installable  bool                     `json:"installable"`
	AccessScopes *accessscope.Declaration `json:"accessScopes,omitempty"`
}

func catalogEntry(v CatalogRelease) CatalogEntry {
	m := v.Package.Manifest
	return CatalogEntry{ID: v.ID, AppID: m.AppID, DeveloperID: m.DeveloperID, Name: m.Name, Summary: m.Summary, Description: m.Description, Version: m.Version, Capability: m.Capability, Scopes: m.Scopes, Pricing: "free", Installable: false, AccessScopes: m.AccessScopes}
}
func (s Catalog) Public(ctx context.Context, q CatalogQuery) ([]CatalogEntry, int, error) {
	if len(q.Search) > 120 || q.Page < 1 || q.Page > 500 || (q.Capability != "" && q.Capability != "payment/v1" && q.Capability != "shipping/v1") {
		return nil, 0, fault.Invalid
	}
	// Preserve the old filter shape, but never expose reserved payment listings.
	if q.Capability == "payment/v1" {
		return []CatalogEntry{}, 0, nil
	}
	q.Capability = PublicCapability
	values, total, err := s.Repo.CatalogPublic(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	out := []CatalogEntry{}
	for _, v := range values {
		if s.Signer == nil {
			return nil, 0, fault.Unavailable
		}
		if err = s.Signer.Verify(v.Package); err != nil {
			return nil, 0, fault.Unavailable
		}
		out = append(out, catalogEntry(v))
	}
	return out, total, nil
}
func (s Catalog) PublicDetail(ctx context.Context, id string) (CatalogEntry, error) {
	v, err := s.Repo.CatalogGet(ctx, "", id)
	if err != nil {
		return CatalogEntry{}, err
	}
	if v.Status != "published" || !PublicDistributionAllowed(v.Package.Manifest.Capability) {
		return CatalogEntry{}, fault.NotFound
	}
	if s.Signer == nil || s.Signer.Verify(v.Package) != nil {
		return CatalogEntry{}, fault.Unavailable
	}
	return catalogEntry(v), nil
}

type Tooling struct {
	Manifest catalogmanifest.Manifest `json:"manifest"`
	Valid    bool                     `json:"valid"`
	Checks   []string                 `json:"checks"`
}

func (s Drafts) Tooling(ctx context.Context, p identity.PortalPrincipal, id string) (Tooling, error) {
	d, err := s.Get(ctx, p, id)
	if err != nil {
		return Tooling{}, err
	}
	m := catalogManifest(CatalogCandidate{AppID: d.ID, OrganizationID: d.OrganizationID, Revision: d.Revision, Document: d.Document})
	checks := []string{"Data katalog saja; bukan executable manifest atau izin install.", "Endpoint integrasi tidak dimasukkan ke katalog publik.", "Signature katalog tidak menyatakan app-code sudah dipindai."}
	if d.Document.AccessScopes != nil {
		checks = append(checks, "Scope resource adalah rencana kebutuhan, belum grantable. Review metadata bukan persetujuan akses data sensitif atau consent merchant.")
	}
	valid := d.Document.Validate(true) == nil && m.Validate() == nil
	if !PublicDistributionAllowed(d.Document.Capability) {
		valid = false
		checks = append(checks, PaymentBoundaryMessage)
	}
	if !valid {
		checks = append(checks, "Lengkapi nama, ringkasan, deskripsi, versi, endpoint HTTPS, dan scope capability sebelum diajukan.")
	}
	return Tooling{Manifest: m, Valid: valid, Checks: checks}, nil
}
