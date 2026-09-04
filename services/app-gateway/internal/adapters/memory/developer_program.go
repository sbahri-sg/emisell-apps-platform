package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListDeveloperApplications(_ context.Context, platformOrgID string, filter ports.DeveloperApplicationFilter) ([]domain.DeveloperApplication, ports.PageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	items := make([]domain.DeveloperApplication, 0)
	for _, application := range r.developerApplications {
		if application.PlatformOrgID != platformOrgID || (filter.Status != "" && application.Status != filter.Status) {
			continue
		}
		haystack := strings.ToLower(application.CompanyName + " " + application.CompanyDomain + " " + application.ContactName + " " + application.ContactEmail + " " + application.RequestedAppName)
		if search != "" && !strings.Contains(haystack, search) {
			continue
		}
		items = append(items, r.cloneDeveloperApplicationLocked(application))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return paginate(items, ports.AppFilter{Cursor: filter.Cursor, Limit: filter.Limit})
}

func (r *Repository) CreateDeveloperApplication(_ context.Context, application domain.DeveloperApplication, meta ports.MutationMeta) (domain.DeveloperApplication, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DeveloperApplication{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.developerApplications[application.ID] = cloneDeveloperApplication(application)
	r.appendAuditLocked(auditID, application.PlatformOrgID, meta, "developer_application", application.ID, map[string]any{"contact_email": application.ContactEmail})
	return cloneDeveloperApplication(application), nil
}

func (r *Repository) GetDeveloperApplication(_ context.Context, platformOrgID, applicationID string) (domain.DeveloperApplication, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	application, ok := r.developerApplications[applicationID]
	if !ok || application.PlatformOrgID != platformOrgID {
		return domain.DeveloperApplication{}, domain.ErrNotFound
	}
	return r.cloneDeveloperApplicationLocked(application), nil
}

func (r *Repository) StartDeveloperApplicationReview(_ context.Context, platformOrgID, applicationID, actorID string, expectedRevision int64, notes *string) (domain.DeveloperApplication, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	application, ok := r.developerApplications[applicationID]
	if !ok || application.PlatformOrgID != platformOrgID {
		return domain.DeveloperApplication{}, domain.ErrNotFound
	}
	if application.Revision != expectedRevision || application.Status != domain.DeveloperApplicationStatusSubmitted {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: application can no longer enter review", domain.ErrConflict)
	}
	now := r.now().UTC()
	application.Status = domain.DeveloperApplicationStatusUnderReview
	application.ReviewNotes = cloneString(notes)
	application.ReviewedBy = &actorID
	application.ReviewedAt = &now
	application.UpdatedAt = now
	application.Revision++
	r.developerApplications[applicationID] = application
	r.appendAuditLocked(auditID, platformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_application.review_started"}, "developer_application", applicationID, nil)
	return r.cloneDeveloperApplicationLocked(application), nil
}

func (r *Repository) ApproveDeveloperApplication(_ context.Context, platformOrgID string, application domain.DeveloperApplication, organizationID, organizationName, organizationSlug string, entitlement domain.OrganizationEntitlement, invitation domain.DeveloperInvitation, expectedRevision int64, meta ports.MutationMeta) (domain.DeveloperApplication, domain.DeveloperInvitation, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.developerApplications[application.ID]
	if !ok || current.PlatformOrgID != platformOrgID {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, domain.ErrNotFound
	}
	if current.Revision != expectedRevision || (current.Status != domain.DeveloperApplicationStatusSubmitted && current.Status != domain.DeveloperApplicationStatusUnderReview) {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, fmt.Errorf("%w: application can no longer be approved", domain.ErrConflict)
	}
	now := r.now().UTC()
	current.Status = domain.DeveloperApplicationStatusInvited
	current.OrganizationID = &organizationID
	current.ReviewedBy = &meta.ActorID
	current.ReviewedAt = &now
	current.UpdatedAt = now
	current.Revision++
	r.developerApplications[current.ID] = current
	r.organizationEntitlements[organizationID] = entitlement
	r.developerOrganizations[organizationID] = domain.DeveloperOrganization{
		ID: organizationID, Name: organizationName, Slug: organizationSlug,
		Status: domain.OrganizationStatusActive, Entitlement: entitlement,
		CreatedAt: now, UpdatedAt: now,
	}
	r.developerInvitations[invitation.ID] = invitation
	r.appendAuditLocked(auditID, platformOrgID, meta, "developer_application", current.ID, map[string]any{"organization_id": organizationID, "invitation_id": invitation.ID})
	result := r.cloneDeveloperApplicationLocked(current)
	return result, cloneDeveloperInvitation(invitation), nil
}

func (r *Repository) RejectDeveloperApplication(_ context.Context, platformOrgID, applicationID, actorID string, expectedRevision int64, notes string) (domain.DeveloperApplication, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	application, ok := r.developerApplications[applicationID]
	if !ok || application.PlatformOrgID != platformOrgID {
		return domain.DeveloperApplication{}, domain.ErrNotFound
	}
	if application.Revision != expectedRevision || (application.Status != domain.DeveloperApplicationStatusSubmitted && application.Status != domain.DeveloperApplicationStatusUnderReview) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: application can no longer be rejected", domain.ErrConflict)
	}
	now := r.now().UTC()
	application.Status = domain.DeveloperApplicationStatusRejected
	application.ReviewNotes = &notes
	application.ReviewedBy = &actorID
	application.ReviewedAt = &now
	application.UpdatedAt = now
	application.Revision++
	r.developerApplications[applicationID] = application
	r.appendAuditLocked(auditID, platformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_application.rejected"}, "developer_application", applicationID, nil)
	return r.cloneDeveloperApplicationLocked(application), nil
}

func (r *Repository) RotateDeveloperInvitation(_ context.Context, platformOrgID, applicationID string, invitation domain.DeveloperInvitation, expectedRevision int64, meta ports.MutationMeta) (domain.DeveloperApplication, domain.DeveloperInvitation, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	application, ok := r.developerApplications[applicationID]
	if !ok || application.PlatformOrgID != platformOrgID {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, domain.ErrNotFound
	}
	if application.Revision != expectedRevision || (application.Status != domain.DeveloperApplicationStatusInvited && application.Status != domain.DeveloperApplicationStatusApproved) {
		return domain.DeveloperApplication{}, domain.DeveloperInvitation{}, fmt.Errorf("%w: invitation can no longer be rotated", domain.ErrConflict)
	}
	now := r.now().UTC()
	for id, current := range r.developerInvitations {
		if current.ApplicationID == applicationID && current.Status == domain.DeveloperInvitationStatusPending {
			current.Status = domain.DeveloperInvitationStatusRevoked
			current.RevokedAt = &now
			r.developerInvitations[id] = current
		}
	}
	r.developerInvitations[invitation.ID] = invitation
	application.Status = domain.DeveloperApplicationStatusInvited
	application.UpdatedAt = now
	application.Revision++
	r.developerApplications[applicationID] = application
	r.appendAuditLocked(auditID, platformOrgID, meta, "developer_invitation", invitation.ID, map[string]any{"application_id": applicationID})
	return r.cloneDeveloperApplicationLocked(application), cloneDeveloperInvitation(invitation), nil
}

func (r *Repository) RevokeDeveloperInvitation(_ context.Context, platformOrgID, invitationID, actorID string) (domain.DeveloperInvitation, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DeveloperInvitation{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	invitation, ok := r.developerInvitations[invitationID]
	if !ok {
		return domain.DeveloperInvitation{}, domain.ErrNotFound
	}
	application, ok := r.developerApplications[invitation.ApplicationID]
	if !ok || application.PlatformOrgID != platformOrgID {
		return domain.DeveloperInvitation{}, domain.ErrNotFound
	}
	if invitation.Status != domain.DeveloperInvitationStatusPending {
		return domain.DeveloperInvitation{}, fmt.Errorf("%w: invitation is not pending", domain.ErrConflict)
	}
	now := r.now().UTC()
	invitation.Status = domain.DeveloperInvitationStatusRevoked
	invitation.RevokedAt = &now
	r.developerInvitations[invitationID] = invitation
	application.Status = domain.DeveloperApplicationStatusApproved
	application.UpdatedAt = now
	application.Revision++
	r.developerApplications[application.ID] = application
	r.appendAuditLocked(auditID, platformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_invitation.revoked"}, "developer_invitation", invitationID, nil)
	return cloneDeveloperInvitation(invitation), nil
}

func (r *Repository) AcceptDeveloperInvitation(_ context.Context, tokenHash, actorID, actorEmail, displayName string, acceptedAt time.Time) (domain.DeveloperApplication, domain.OrganizationEntitlement, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var invitation domain.DeveloperInvitation
	for _, candidate := range r.developerInvitations {
		if candidate.TokenHash == tokenHash {
			invitation = candidate
			break
		}
	}
	if invitation.ID == "" || invitation.Status != domain.DeveloperInvitationStatusPending || !strings.EqualFold(invitation.Email, actorEmail) {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
	}
	if !acceptedAt.Before(invitation.ExpiresAt) {
		invitation.Status = domain.DeveloperInvitationStatusExpired
		r.developerInvitations[invitation.ID] = invitation
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
	}
	application := r.developerApplications[invitation.ApplicationID]
	if application.Status != domain.DeveloperApplicationStatusInvited {
		return domain.DeveloperApplication{}, domain.OrganizationEntitlement{}, domain.ErrInvalidGrant
	}
	invitation.Status = domain.DeveloperInvitationStatusAccepted
	invitation.AcceptedBy = &actorID
	invitation.AcceptedAt = &acceptedAt
	r.developerInvitations[invitation.ID] = invitation
	application.Status = domain.DeveloperApplicationStatusActive
	application.UpdatedAt = acceptedAt
	application.Revision++
	r.developerApplications[application.ID] = application
	if r.organizationMembers[invitation.OrganizationID] == nil {
		r.organizationMembers[invitation.OrganizationID] = make(map[string]domain.DeveloperOrganizationMember)
	}
	r.organizationMembers[invitation.OrganizationID][actorID] = domain.DeveloperOrganizationMember{
		UserID: actorID, Email: actorEmail, DisplayName: displayName, Role: domain.RoleOwner, CreatedAt: acceptedAt,
	}
	r.identityMemberships[actorID] = append(r.identityMemberships[actorID], domain.OrganizationMembership{
		OrganizationID: invitation.OrganizationID, Name: r.developerOrganizations[invitation.OrganizationID].Name,
		Slug: r.developerOrganizations[invitation.OrganizationID].Slug, Status: string(domain.OrganizationStatusActive), Role: domain.RoleOwner,
	})
	r.appendAuditLocked(auditID, application.PlatformOrgID, ports.MutationMeta{ActorID: actorID, Action: "developer_invitation.accepted"}, "developer_invitation", invitation.ID, map[string]any{"organization_id": invitation.OrganizationID})
	return r.cloneDeveloperApplicationLocked(application), r.organizationEntitlements[invitation.OrganizationID], nil
}

func (r *Repository) cloneDeveloperApplicationLocked(application domain.DeveloperApplication) domain.DeveloperApplication {
	result := cloneDeveloperApplication(application)
	var latest *domain.DeveloperInvitation
	for _, invitation := range r.developerInvitations {
		if invitation.ApplicationID != application.ID {
			continue
		}
		candidate := cloneDeveloperInvitation(invitation)
		if candidate.Status == domain.DeveloperInvitationStatusPending && !r.now().UTC().Before(candidate.ExpiresAt) {
			candidate.Status = domain.DeveloperInvitationStatusExpired
		}
		if latest == nil || candidate.CreatedAt.After(latest.CreatedAt) {
			latest = &candidate
		}
	}
	result.CurrentInvitation = latest
	return result
}

func cloneDeveloperApplication(application domain.DeveloperApplication) domain.DeveloperApplication {
	application.RequestedScopes = append([]string(nil), application.RequestedScopes...)
	application.ReviewNotes = cloneString(application.ReviewNotes)
	application.OrganizationID = cloneString(application.OrganizationID)
	application.ReviewedBy = cloneString(application.ReviewedBy)
	if application.ReviewedAt != nil {
		value := *application.ReviewedAt
		application.ReviewedAt = &value
	}
	application.CurrentInvitation = nil
	return application
}

func cloneDeveloperInvitation(invitation domain.DeveloperInvitation) domain.DeveloperInvitation {
	invitation.AcceptedBy = cloneString(invitation.AcceptedBy)
	if invitation.AcceptedAt != nil {
		value := *invitation.AcceptedAt
		invitation.AcceptedAt = &value
	}
	if invitation.RevokedAt != nil {
		value := *invitation.RevokedAt
		invitation.RevokedAt = &value
	}
	return invitation
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

var _ ports.DeveloperProgramRepository = (*Repository)(nil)
