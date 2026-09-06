package review

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
)

func (s Service) CatalogCandidate(ctx context.Context, p identity.PortalPrincipal, id string) (service.CatalogCandidate, error) {
	v, err := s.Get(ctx, p, id)
	if err != nil {
		return service.CatalogCandidate{}, err
	}
	if v.Status != "approved" {
		return service.CatalogCandidate{}, fault.Conflict
	}
	return service.CatalogCandidate{SubmissionID: v.ID, AppID: v.AppID, OrganizationID: v.OrganizationID, Revision: v.DraftRevision, Document: v.Snapshot}, nil
}
