package memory

import (
	"context"
	"sort"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListDeveloperOrganizations(_ context.Context, filter ports.DeveloperOrganizationFilter) ([]domain.DeveloperOrganization, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	items := make([]domain.DeveloperOrganization, 0, len(r.developerOrganizations))
	for _, stored := range r.developerOrganizations {
		if filter.Status != "" && stored.Status != filter.Status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(stored.Name+" "+stored.Slug), search) {
			continue
		}
		organization := r.developerOrganizationLocked(stored.ID)
		items = append(items, organization)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return paginate(items, ports.AppFilter{Cursor: filter.Cursor, Limit: filter.Limit})
}

func (r *Repository) GetDeveloperOrganization(_ context.Context, organizationID string) (domain.DeveloperOrganizationDetail, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.developerOrganizations[organizationID]; !ok {
		return domain.DeveloperOrganizationDetail{}, domain.ErrNotFound
	}
	members := make([]domain.DeveloperOrganizationMember, 0, len(r.organizationMembers[organizationID]))
	for _, member := range r.organizationMembers[organizationID] {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].CreatedAt.Equal(members[j].CreatedAt) {
			return members[i].UserID < members[j].UserID
		}
		return members[i].CreatedAt.Before(members[j].CreatedAt)
	})
	return domain.DeveloperOrganizationDetail{DeveloperOrganization: r.developerOrganizationLocked(organizationID), Memberships: members}, nil
}

func (r *Repository) developerOrganizationLocked(organizationID string) domain.DeveloperOrganization {
	organization := r.developerOrganizations[organizationID]
	organization.Entitlement = r.organizationEntitlements[organizationID]
	organization.MembershipCount = len(r.organizationMembers[organizationID])
	for _, app := range r.apps {
		if app.OrganizationID == organizationID && app.Status != domain.AppStatusArchived {
			organization.AppCount++
		}
	}
	return organization
}

var _ ports.DeveloperOrganizationRepository = (*Repository)(nil)
