package service

import (
	"context"
	"strings"
	"time"

	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/integrationmanifest"
)

type IntegrationRelease struct {
	ID             string                       `json:"id"`
	OrganizationID string                       `json:"organizationId"`
	Manifest       integrationmanifest.Manifest `json:"manifest"`
	SHA256         string                       `json:"sha256"`
	Package        *integrationmanifest.Package `json:"package,omitempty"`
	Status         string                       `json:"status"`
	Revision       int                          `json:"revision"`
	CreatedAt      time.Time                    `json:"createdAt"`
	UpdatedAt      time.Time                    `json:"updatedAt"`
}
type IntegrationInput struct {
	SubmissionID string                     `json:"submissionId"`
	Config       integrationmanifest.Config `json:"config"`
}
type IntegrationAction struct {
	Status   string `json:"status"`
	Revision int    `json:"revision"`
	Reason   string `json:"reason"`
}
type IntegrationRepository interface {
	IntegrationList(context.Context, string) ([]IntegrationRelease, error)
	IntegrationGet(context.Context, string, string) (IntegrationRelease, error)
	IntegrationWith(context.Context, string, string, func(IntegrationRelease) error) error
	IntegrationHistory(context.Context, string) ([]CatalogAudit, error)
	IntegrationMutate(context.Context, string, string, string, string, string, *IntegrationRelease, string, func(IntegrationRelease) (IntegrationRelease, error)) (IntegrationRelease, error)
}
type IntegrationSigner interface {
	Sign(integrationmanifest.Manifest) (integrationmanifest.Package, error)
	Verify(integrationmanifest.Package) error
	PublicKey() []byte
}
type Integrations struct {
	Repo       IntegrationRepository
	Source     CatalogSource
	Developers developer.Service
	Signer     IntegrationSigner
}

// WithSigned retains a shared release lock across another module's short commit.
// Callers must already resolve their organization; never hold this during I/O.
func (s Integrations) WithSigned(ctx context.Context, org, id string, fn func(IntegrationRelease) error) error {
	if org == "" {
		return fault.Forbidden
	}
	return s.Repo.IntegrationWith(ctx, org, id, func(v IntegrationRelease) error {
		if v.Status != "signed" || v.Package == nil || !s.Readiness(v).Valid {
			return fault.Conflict
		}
		return fn(v)
	})
}

func (s Integrations) scope(ctx context.Context, p identity.PortalPrincipal) (string, error) {
	if p.Surface == "admin" && p.ID != "" {
		return "", nil
	}
	o, err := s.Developers.Organization(ctx, p)
	return o.ID, err
}
func (s Integrations) List(ctx context.Context, p identity.PortalPrincipal) ([]IntegrationRelease, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return nil, err
	}
	return s.Repo.IntegrationList(ctx, org)
}
func (s Integrations) Get(ctx context.Context, p identity.PortalPrincipal, id string) (IntegrationRelease, []CatalogAudit, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return IntegrationRelease{}, nil, err
	}
	v, err := s.Repo.IntegrationGet(ctx, org, id)
	if err != nil {
		return v, nil, err
	}
	h, err := s.Repo.IntegrationHistory(ctx, id)
	return v, h, err
}
func (s Integrations) Prepare(ctx context.Context, p identity.PortalPrincipal, b IntegrationInput) (IntegrationRelease, integrationmanifest.Report, error) {
	o, err := s.Developers.Organization(ctx, p)
	if err != nil {
		return IntegrationRelease{}, integrationmanifest.Report{}, err
	}
	c, err := s.Source.CatalogCandidate(ctx, p, b.SubmissionID)
	if err != nil {
		return IntegrationRelease{}, integrationmanifest.Report{}, err
	}
	if c.OrganizationID != o.ID {
		return IntegrationRelease{}, integrationmanifest.Report{}, fault.NotFound
	}
	m := integrationmanifest.Manifest{Schema: integrationmanifest.Schema, Policy: integrationmanifest.Policy, SubmissionID: c.SubmissionID, Metadata: catalogManifest(c), Config: b.Config}
	report := integrationmanifest.Inspect(m)
	match := b.Config.Endpoint == c.Document.Endpoint
	report.Checks = append(report.Checks, integrationmanifest.Check{Code: "approved_endpoint", Passed: match, Message: "Endpoint harus sama persis dengan snapshot metadata yang telah disetujui."})
	report.Valid = report.Valid && match
	report = distributionReport(report, m.Metadata.Capability)
	v := IntegrationRelease{OrganizationID: o.ID, Manifest: m, Status: "submitted", Revision: 1}
	if report.Valid {
		raw, _ := integrationmanifest.Canonical(m)
		v.SHA256 = integrationmanifest.Digest(raw)
	}
	return v, report, nil
}
func (s Integrations) Submit(ctx context.Context, p identity.PortalPrincipal, key string, b IntegrationInput) (IntegrationRelease, error) {
	if !ValidRequestKey(key) {
		return IntegrationRelease{}, fault.Invalid
	}
	v, report, err := s.Prepare(ctx, p, b)
	if err != nil {
		return v, err
	}
	if !report.Valid {
		return v, fault.Invalid
	}
	return s.Repo.IntegrationMutate(ctx, v.OrganizationID, p.ID, key, RequestHash([]any{"submit-integration", b}), "", &v, "Konfigurasi diajukan oleh developer.", nil)
}
func (s Integrations) Readiness(v IntegrationRelease) integrationmanifest.Report {
	r := integrationmanifest.Inspect(v.Manifest)
	raw, err := integrationmanifest.Canonical(v.Manifest)
	ok := err == nil && integrationmanifest.Digest(raw) == v.SHA256
	r.Checks = append(r.Checks, integrationmanifest.Check{Code: "integrity", Passed: ok, Message: "Checksum snapshot konfigurasi tidak berubah."})
	r.Valid = r.Valid && ok
	if v.Package != nil || v.Status == "signed" {
		ok = v.Package != nil && s.Signer != nil && s.Signer.Verify(*v.Package) == nil && v.Package.SHA256 == v.SHA256
		r.Checks = append(r.Checks, integrationmanifest.Check{Code: "signature", Passed: ok, Message: "Verifikasi signature dengan trusted key integrasi yang aktif."})
		r.Valid = r.Valid && ok
	}
	if v.Status != "signed" {
		r.Blockers = append(r.Blockers, "Release belum berstatus signed (atau sudah ditangguhkan).")
	}
	return distributionReport(r, v.Manifest.Metadata.Capability)
}

// Keep distribution policy outside canonicalization/signature verification.
func distributionReport(r integrationmanifest.Report, capability string) integrationmanifest.Report {
	ok := PublicDistributionAllowed(capability)
	r.Checks = append(r.Checks, integrationmanifest.Check{Code: "public_distribution", Passed: ok, Message: "Distribusi aplikasi umum saat ini mendukung shipping/v1; payment gateway merupakan integrasi internal Emisell."})
	r.Valid = r.Valid && ok
	if !ok {
		r.Blockers = append(r.Blockers, PaymentBoundaryMessage)
	}
	return r
}
func (s Integrations) Transition(ctx context.Context, p identity.PortalPrincipal, id, key string, b IntegrationAction) (IntegrationRelease, error) {
	if p.ID == "" || !p.CanReview() {
		return IntegrationRelease{}, fault.Forbidden
	}
	if (b.Status == "signed" || b.Status == "suspended") && p.Role != "administrator" {
		return IntegrationRelease{}, fault.Forbidden
	}
	if !ValidRequestKey(key) || b.Revision < 1 || strings.TrimSpace(b.Reason) == "" || len(b.Reason) > 2000 {
		return IntegrationRelease{}, fault.Invalid
	}
	if b.Status != "approved" && b.Status != "rejected" && b.Status != "signed" && b.Status != "suspended" {
		return IntegrationRelease{}, fault.Invalid
	}
	return s.Repo.IntegrationMutate(ctx, "", p.ID, key, RequestHash([]any{"integration-status", id, b}), id, nil, b.Reason, func(v IntegrationRelease) (IntegrationRelease, error) {
		if v.Revision != b.Revision {
			return v, fault.Conflict
		}
		allowed := (v.Status == "submitted" && (b.Status == "approved" || b.Status == "rejected")) || (v.Status == "approved" && b.Status == "signed") || ((v.Status == "approved" || v.Status == "signed") && b.Status == "suspended")
		if !allowed {
			return v, fault.Conflict
		}
		// Rejection/suspension must remain possible during key or policy outages.
		if b.Status == "approved" || b.Status == "signed" {
			if !s.Readiness(v).Valid {
				return v, fault.Invalid
			}
		}
		if b.Status == "signed" {
			if s.Signer == nil {
				return v, fault.Unavailable
			}
			pkg, err := s.Signer.Sign(v.Manifest)
			if err != nil {
				return v, fault.Unavailable
			}
			if s.Signer.Verify(pkg) != nil || pkg.SHA256 != v.SHA256 {
				return v, fault.Unavailable
			}
			v.Package = &pkg
		}
		v.Status = b.Status
		v.Revision++
		return v, nil
	})
}
