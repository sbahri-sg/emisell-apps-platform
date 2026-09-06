package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
)

// Assignment is distribution authorization, never consent or a runtime grant.
type Assignment struct {
	ReleaseKind    string    `json:"releaseKind,omitempty"`
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	ReleaseID      string    `json:"releaseId"`
	ReleaseSHA256  string    `json:"releaseSha256"`
	MerchantID     string    `json:"merchantId"`
	Status         string    `json:"status"`
	Revision       int       `json:"revision"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type TestReadiness struct {
	ConfigurationReady  bool     `json:"configurationReady"`
	RequiredScopesReady bool     `json:"requiredScopesReady"`
	Installable         bool     `json:"installable"`
	Blockers            []string `json:"blockers"`
}

// TestApp is the explicit merchant-wide read-only metadata projection. No URLs,
// actor IDs, organization memberships, signatures, credentials or grant objects.
type TestApp struct {
	AssignmentID     string        `json:"assignmentId"`
	AppID            string        `json:"appId"`
	AppName          string        `json:"appName"`
	Version          string        `json:"version"`
	Capability       string        `json:"capability"`
	Readiness        TestReadiness `json:"readiness"`
	ExecutionProfile string        `json:"executionProfile,omitempty"`
}
type AssignmentView struct {
	Assignment
	App TestApp `json:"app"`
}
type AssignmentInput struct {
	ReleaseKind string `json:"releaseKind,omitempty"`
	ReleaseID   string `json:"releaseId"`
	MerchantID  string `json:"merchantId"`
	Reason      string `json:"reason"`
}
type AssignmentAction struct {
	Status   string `json:"status"`
	Revision int    `json:"revision"`
	Reason   string `json:"reason"`
}
type AssignmentRepository interface {
	AssignmentReplay(context.Context, string, string, string, string) (*Assignment, error)
	AssignmentList(context.Context, string, string, string, int) ([]Assignment, string, error)
	AssignmentGet(context.Context, string, string) (Assignment, error)
	AssignmentHistory(context.Context, string) ([]CatalogAudit, error)
	AssignmentMutate(context.Context, string, string, string, string, string, *Assignment, string, func(Assignment) (Assignment, error)) (Assignment, error)
}
type MerchantDirectory interface {
	KnownMerchant(context.Context, string) (bool, error)
}
type Testing struct {
	UIReady               func(context.Context, Assignment) error
	UI                    UIReleases
	ManagedInstallEnabled bool
	Repo                  AssignmentRepository
	Releases              Integrations
	Managed               ManagedShipping
	Merchants             MerchantDirectory
}

var testIdentifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func TestPage(after string, size int) (int, error) {
	if (after != "" && !testIdentifier.MatchString(after)) || size < 0 || size > 20 {
		return 0, fault.Invalid
	}
	if size == 0 {
		size = 20
	}
	return size, nil
}
func (s Testing) view(ctx context.Context, a Assignment) (AssignmentView, error) {
	if a.ReleaseKind == "ui" {
		if s.UI.Repo == nil {
			return AssignmentView{}, fault.Unavailable
		}
		v, err := s.UI.Repo.UIGet(ctx, a.OrganizationID, a.ReleaseID)
		if err != nil {
			return AssignmentView{}, err
		}
		ready := TestReadiness{RequiredScopesReady: true, Blockers: []string{"ui_installation_not_available"}}
		ready.ConfigurationReady = s.UI.WithSigned(ctx, a.OrganizationID, a.ReleaseID, func(current UIRelease) error {
			if current.Package.SHA256 != a.ReleaseSHA256 {
				return fault.Conflict
			}
			return nil
		}) == nil
		if !ready.ConfigurationReady {
			ready.Blockers = append(ready.Blockers, "release_not_ready")
		}
		if s.UIReady != nil && ready.ConfigurationReady && s.UIReady(ctx, a) == nil {
			ready.Installable = true
			ready.Blockers = []string{}
		}
		return AssignmentView{a, TestApp{a.ID, v.Manifest.AppID, v.Manifest.Name, v.Manifest.Version, "", ready, "reviewed-ui/v1"}}, nil
	}
	if a.ReleaseKind == "managed_shipping" {
		if s.Managed.Repo == nil {
			return AssignmentView{}, fault.Unavailable
		}
		v, err := s.Managed.Repo.ManagedShippingGet(ctx, a.OrganizationID, a.ReleaseID)
		if err != nil {
			return AssignmentView{}, err
		}
		ready := s.Managed.Readiness(v)
		ready.ConfigurationReady = ready.ConfigurationReady && v.SHA256 == a.ReleaseSHA256
		ready.Blockers = []string{"managed_installation_not_available", "engine_grant_enforcement_not_available"}
		if s.ManagedInstallEnabled {
			ready.Blockers = []string{}
			ready.Installable = ready.ConfigurationReady && ready.RequiredScopesReady && a.Status == "approved"
		}
		if !ready.ConfigurationReady {
			ready.Blockers = append(ready.Blockers, "release_not_ready")
		}
		m := v.Manifest
		return AssignmentView{a, TestApp{a.ID, m.AppID, m.Name, m.Version, m.Capability, ready, ""}}, nil
	}
	v, err := s.Releases.Repo.IntegrationGet(ctx, a.OrganizationID, a.ReleaseID)
	if err != nil {
		return AssignmentView{}, err
	}
	report := s.Releases.Readiness(v)
	ready := TestReadiness{ConfigurationReady: v.Status == "signed" && report.Valid && v.SHA256 == a.ReleaseSHA256,
		Blockers: []string{"runtime_not_available", "oauth_not_available", "endpoint_runtime_unverified"}}
	for _, c := range report.Checks {
		if c.Code == "required_scopes" {
			ready.RequiredScopesReady = c.Passed
		}
	}
	if !ready.ConfigurationReady {
		ready.Blockers = append(ready.Blockers, "release_not_ready")
	}
	if !ready.RequiredScopesReady {
		ready.Blockers = append(ready.Blockers, "required_scopes_not_ready")
	}
	// Deliberately no path to installable=true in this metadata-only milestone.
	m := v.Manifest.Metadata
	return AssignmentView{a, TestApp{a.ID, m.AppID, m.Name, m.Version, m.Capability, ready, ""}}, nil
}
func (s Testing) List(ctx context.Context, p identity.PortalPrincipal, after string, size int) ([]AssignmentView, string, error) {
	org, err := s.Releases.scope(ctx, p)
	if err != nil {
		return nil, "", err
	}
	size, err = TestPage(after, size)
	if err != nil {
		return nil, "", err
	}
	rows, next, err := s.Repo.AssignmentList(ctx, org, "", after, size)
	if err != nil {
		return nil, "", err
	}
	out := []AssignmentView{}
	for _, a := range rows {
		v, e := s.view(ctx, a)
		if e != nil {
			return nil, "", e
		}
		out = append(out, v)
	}
	return out, next, nil
}
func (s Testing) Get(ctx context.Context, p identity.PortalPrincipal, id string) (AssignmentView, []CatalogAudit, error) {
	org, err := s.Releases.scope(ctx, p)
	if err != nil {
		return AssignmentView{}, nil, err
	}
	a, err := s.Repo.AssignmentGet(ctx, org, id)
	if err != nil {
		return AssignmentView{}, nil, err
	}
	v, err := s.view(ctx, a)
	if err != nil {
		return v, nil, err
	}
	h, err := s.Repo.AssignmentHistory(ctx, a.ID)
	return v, h, err
}
func (s Testing) ForMerchant(ctx context.Context, merchant, after string, size int) ([]TestApp, string, error) {
	if !testIdentifier.MatchString(merchant) {
		return nil, "", fault.Invalid
	}
	size, err := TestPage(after, size)
	if err != nil {
		return nil, "", err
	}
	rows, next, err := s.Repo.AssignmentList(ctx, "", merchant, after, size)
	if s.UIReady != nil {
		if repo, ok := s.Repo.(interface {
			UIAssignmentList(context.Context, string, string, int) ([]Assignment, string, error)
		}); ok {
			rows, next, err = repo.UIAssignmentList(ctx, merchant, after, size)
		}
	}
	if err != nil {
		return nil, "", err
	}
	out := []TestApp{}
	for _, a := range rows {
		v, e := s.view(ctx, a)
		if e != nil {
			return nil, "", e
		}
		out = append(out, v.App)
	}
	return out, next, nil
}
func (s Testing) Request(ctx context.Context, p identity.PortalPrincipal, key string, b AssignmentInput) (Assignment, error) {
	// No merchant lookup here: developers cannot discover merchant accounts.
	org, err := s.Releases.Developers.Organization(ctx, p)
	if err != nil {
		return Assignment{}, err
	}
	b.Reason = strings.TrimSpace(b.Reason)
	if (b.ReleaseKind != "" && b.ReleaseKind != "managed_shipping" && b.ReleaseKind != "ui") || !ValidRequestKey(key) || !testIdentifier.MatchString(b.ReleaseID) || !testIdentifier.MatchString(b.MerchantID) || b.Reason == "" || len(b.Reason) > 2000 {
		return Assignment{}, fault.Invalid
	}
	var result Assignment
	if old, e := s.Repo.AssignmentReplay(ctx, org.ID, p.ID, key, RequestHash([]any{"test-request", b})); e != nil {
		return result, e
	} else if old != nil {
		return *old, nil
	}
	if b.ReleaseKind == "ui" {
		err = s.UI.WithSigned(ctx, org.ID, b.ReleaseID, func(v UIRelease) error {
			a := Assignment{ReleaseKind: "ui", OrganizationID: org.ID, ReleaseID: v.ID, ReleaseSHA256: v.Package.SHA256, MerchantID: b.MerchantID, Status: "requested", Revision: 1}
			var e error
			result, e = s.Repo.AssignmentMutate(ctx, org.ID, p.ID, key, RequestHash([]any{"test-request", b}), "", &a, b.Reason, nil)
			return e
		})
		return result, err
	}
	if b.ReleaseKind == "managed_shipping" {
		err = s.Managed.WithSigned(ctx, org.ID, b.ReleaseID, func(v ManagedShippingRelease) error {
			a := Assignment{ReleaseKind: b.ReleaseKind, OrganizationID: org.ID, ReleaseID: v.ID, ReleaseSHA256: v.SHA256, MerchantID: b.MerchantID, Status: "requested", Revision: 1}
			var e error
			result, e = s.Repo.AssignmentMutate(ctx, org.ID, p.ID, key, RequestHash([]any{"test-request", b}), "", &a, b.Reason, nil)
			return e
		})
		return result, err
	}
	// Separate release gate pool holds a shared lock across the short assignment commit.
	err = s.Releases.WithSigned(ctx, org.ID, b.ReleaseID, func(v IntegrationRelease) error {
		a := Assignment{OrganizationID: org.ID, ReleaseID: v.ID, ReleaseSHA256: v.SHA256, MerchantID: b.MerchantID, Status: "requested", Revision: 1}
		var e error
		result, e = s.Repo.AssignmentMutate(ctx, org.ID, p.ID, key, RequestHash([]any{"test-request", b}), "", &a, b.Reason, nil)
		return e
	})
	return result, err
}
func (s Testing) Decide(ctx context.Context, p identity.PortalPrincipal, id, key string, b AssignmentAction) (Assignment, error) {
	if p.ID == "" || p.Surface != "admin" || p.Role != "administrator" {
		return Assignment{}, fault.Forbidden
	}
	b.Reason = strings.TrimSpace(b.Reason)
	if !ValidRequestKey(key) || !testIdentifier.MatchString(id) || b.Revision < 1 || b.Reason == "" || len(b.Reason) > 2000 {
		return Assignment{}, fault.Invalid
	}
	if b.Status != "approved" && b.Status != "rejected" && b.Status != "revoked" {
		return Assignment{}, fault.Invalid
	}
	var result Assignment
	mutate := func() error {
		var e error
		result, e = s.Repo.AssignmentMutate(ctx, "", p.ID, key, RequestHash([]any{"test-decision", id, b}), id, nil, b.Reason, func(a Assignment) (Assignment, error) {
			if a.Revision != b.Revision || !(a.Status == "requested" && (b.Status == "approved" || b.Status == "rejected") || a.Status == "approved" && b.Status == "revoked") {
				return a, fault.Conflict
			}
			a.Status = b.Status
			a.Revision++
			return a, nil
		})
		return e
	}
	// Reject/revoke remains available during signer or runtime outages.
	if b.Status != "approved" {
		return resultAfter(mutate, &result)
	}
	a, err := s.Repo.AssignmentGet(ctx, "", id)
	if err != nil {
		return result, err
	}
	// An old approval retry returns the current terminal state, never revives it.
	if a.Status != "requested" {
		return resultAfter(mutate, &result)
	}
	known, err := s.Merchants.KnownMerchant(ctx, a.MerchantID)
	if err != nil {
		return result, err
	}
	if !known {
		return result, fault.NotFound
	}
	if a.ReleaseKind == "ui" {
		err = s.UI.WithSigned(ctx, a.OrganizationID, a.ReleaseID, func(v UIRelease) error {
			if v.Package.SHA256 != a.ReleaseSHA256 {
				return fault.Conflict
			}
			return mutate()
		})
		return result, err
	}
	if a.ReleaseKind == "managed_shipping" {
		err = s.Managed.WithSigned(ctx, a.OrganizationID, a.ReleaseID, func(v ManagedShippingRelease) error {
			if v.SHA256 != a.ReleaseSHA256 {
				return fault.Conflict
			}
			return mutate()
		})
		return result, err
	}
	err = s.Releases.WithSigned(ctx, a.OrganizationID, a.ReleaseID, func(v IntegrationRelease) error {
		if v.SHA256 != a.ReleaseSHA256 {
			return fault.Conflict
		}
		return mutate()
	})
	return result, err
}
func resultAfter(fn func() error, v *Assignment) (Assignment, error) { err := fn(); return *v, err }
