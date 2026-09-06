package review

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"strings"
	"time"
)

type Submission struct {
	ID             string              `json:"id"`
	AppID          string              `json:"appId"`
	OrganizationID string              `json:"organizationId"`
	SubmitterID    string              `json:"submitterId"`
	DraftRevision  int                 `json:"draftRevision"`
	Version        string              `json:"version"`
	Snapshot       service.AppDocument `json:"snapshot"`
	Status         string              `json:"status"`
	CreatedAt      time.Time           `json:"createdAt"`
	DecidedAt      *time.Time          `json:"decidedAt"`
	ReviewerID     string              `json:"reviewerId"`
	Feedback       string              `json:"feedback"`
}
type Decision struct {
	Status   string `json:"status"`
	Feedback string `json:"feedback"`
}
type Audit struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actorId"`
	Action     string    `json:"action"`
	Feedback   string    `json:"feedback"`
	OccurredAt time.Time `json:"occurredAt"`
}
type Repository interface {
	List(context.Context, string) ([]Submission, error)
	Get(context.Context, string) (Submission, error)
	History(context.Context, string) ([]Audit, error)
	Submit(context.Context, identity.PortalPrincipal, service.Draft, int, string, string) (Submission, error)
	Decide(context.Context, identity.PortalPrincipal, string, string, string, Decision) (Submission, error)
}
type DraftReader interface {
	Get(context.Context, identity.PortalPrincipal, string) (service.Draft, error)
}
type Service struct {
	Repo       Repository
	Drafts     DraftReader
	Developers developer.Service
}

func (s Service) List(ctx context.Context, p identity.PortalPrincipal) ([]Submission, error) {
	org := ""
	if p.Surface == "developer" {
		v, err := s.Developers.Organization(ctx, p)
		if err != nil {
			return nil, err
		}
		org = v.ID
	} else if p.Surface != "admin" || p.ID == "" {
		return nil, fault.Forbidden
	}
	return s.Repo.List(ctx, org)
}
func (s Service) Get(ctx context.Context, p identity.PortalPrincipal, id string) (Submission, error) {
	v, err := s.Repo.Get(ctx, id)
	if err != nil {
		return v, err
	}
	if p.Surface == "developer" {
		org, e := s.Developers.Organization(ctx, p)
		if e != nil {
			return Submission{}, e
		}
		if org.ID != v.OrganizationID {
			return Submission{}, fault.NotFound
		}
	} else if p.Surface != "admin" || p.ID == "" {
		return Submission{}, fault.Forbidden
	}
	return v, nil
}
func (s Service) History(ctx context.Context, p identity.PortalPrincipal, id string) ([]Audit, error) {
	if _, err := s.Get(ctx, p, id); err != nil {
		return nil, err
	}
	return s.Repo.History(ctx, id)
}
func (s Service) Submit(ctx context.Context, p identity.PortalPrincipal, app string, revision int, key string) (Submission, error) {
	if !service.ValidRequestKey(key) || revision < 1 {
		return Submission{}, fault.Invalid
	}
	d, err := s.Drafts.Get(ctx, p, app)
	if err != nil {
		return Submission{}, err
	}
	if err = service.ValidatePublicDistribution(d.Document.Capability); err != nil {
		return Submission{}, err
	}
	// Retry lookup happens inside the repository before revision validation. This
	// preserves the original response even when a later draft has been saved.
	hash := service.RequestHash(struct {
		App      string
		Revision int
	}{app, revision})
	return s.Repo.Submit(ctx, p, d, revision, key, hash)
}
func (s Service) Decide(ctx context.Context, p identity.PortalPrincipal, id, key string, b Decision) (Submission, error) {
	if !p.CanReview() {
		return Submission{}, fault.Forbidden
	}
	if !service.ValidRequestKey(key) || (b.Status != "approved" && b.Status != "changes_requested" && b.Status != "rejected") || strings.TrimSpace(b.Feedback) == "" || len(b.Feedback) > 4000 {
		return Submission{}, fault.Invalid
	}
	if b.Status == "approved" {
		v, err := s.Get(ctx, p, id)
		if err != nil {
			return Submission{}, err
		}
		if err = service.ValidatePublicDistribution(v.Snapshot.Capability); err != nil {
			return Submission{}, err
		}
	}
	return s.Repo.Decide(ctx, p, id, key, service.RequestHash(struct {
		ID   string
		Body Decision
	}{id, b}), b)
}
