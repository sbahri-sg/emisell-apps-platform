package developer

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
)

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}
type Repository interface {
	Organization(context.Context, string) (Organization, error)
}
type Service struct{ Repo Repository }

type AdminOrganization struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount"`
}
type DirectoryRepository interface {
	ListOrganizations(context.Context, string) ([]AdminOrganization, error)
	GetOrganization(context.Context, string) (AdminOrganization, error)
}

func adminAllowed(p identity.PortalPrincipal) bool {
	return p.ID != "" && p.Surface == "admin" && p.Role == "administrator"
}
func (s Service) ListOrganizations(ctx context.Context, p identity.PortalPrincipal, after string) ([]AdminOrganization, error) {
	if !adminAllowed(p) {
		return nil, fault.Forbidden
	}
	repo, ok := s.Repo.(DirectoryRepository)
	if !ok {
		return nil, fault.Forbidden
	}
	if len(after) > 200 {
		return nil, fault.Forbidden
	}
	return repo.ListOrganizations(ctx, after)
}
func (s Service) GetOrganization(ctx context.Context, p identity.PortalPrincipal, id string) (AdminOrganization, error) {
	if !adminAllowed(p) {
		return AdminOrganization{}, fault.Forbidden
	}
	repo, ok := s.Repo.(DirectoryRepository)
	if !ok {
		return AdminOrganization{}, fault.Forbidden
	}
	return repo.GetOrganization(ctx, id)
}

func (s Service) Organization(ctx context.Context, p identity.PortalPrincipal) (Organization, error) {
	if p.Surface != "developer" || p.Role != "developer" || p.ID == "" {
		return Organization{}, fault.Forbidden
	}
	return s.Repo.Organization(ctx, p.ID)
}
