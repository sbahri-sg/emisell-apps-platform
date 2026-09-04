package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func TestInviteOnlyDeveloperLifecycle(t *testing.T) {
	var sequence int
	id := func() (string, error) {
		sequence++
		return fmt.Sprintf("01995f72-0000-7000-8000-%012d", sequence), nil
	}
	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	repository := memory.NewRepository(id, func() time.Time { return now })
	service := application.NewDeveloperProgramService(repository, id, func() time.Time { return now }, 48*time.Hour)
	ctx := context.Background()
	platformOrgID := "01995f72-0000-7000-8000-000000000100"
	operatorID := "01995f72-0000-7000-8000-000000000101"
	actorID := "01995f72-0000-7000-8000-000000000102"

	created, err := service.Create(ctx, application.CreateDeveloperApplicationCommand{
		PlatformOrgID: platformOrgID, ActorID: operatorID, CompanyName: "Nusantara Labs",
		CompanyDomain: "nusantara.example.com", ContactName: "Nadia Putri", ContactEmail: "nadia@example.com",
		RequestedAppName: "Nusantara Connect", AppType: domain.DeveloperAppTypeCustom,
		UseCase:         "Connect selected merchant orders with the partner fulfillment workflow.",
		RequestedScopes: []string{"read_orders", "write_fulfillments", "read_orders"},
	})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if created.Status != domain.DeveloperApplicationStatusSubmitted || len(created.RequestedScopes) != 2 {
		t.Fatalf("unexpected created application: %#v", created)
	}

	reviewed, err := service.StartReview(ctx, application.ReviewDeveloperApplicationCommand{
		PlatformOrgID: platformOrgID, ActorID: operatorID, ApplicationID: created.ID, Revision: created.Revision,
	})
	if err != nil {
		t.Fatalf("start review: %v", err)
	}
	if reviewed.Status != domain.DeveloperApplicationStatusUnderReview {
		t.Fatalf("review status = %s", reviewed.Status)
	}

	approved, err := service.Approve(ctx, application.ApproveDeveloperApplicationCommand{
		PlatformOrgID: platformOrgID, ActorID: operatorID, ApplicationID: created.ID, Revision: reviewed.Revision,
	})
	if err != nil {
		t.Fatalf("approve application: %v", err)
	}
	if approved.Application.Status != domain.DeveloperApplicationStatusInvited || !strings.HasPrefix(approved.InvitationToken, "emi_inv_") {
		t.Fatalf("unexpected approval: %#v", approved)
	}
	encoded, err := json.Marshal(approved.Application)
	if err != nil {
		t.Fatalf("marshal application: %v", err)
	}
	if strings.Contains(string(encoded), approved.InvitationToken) || strings.Contains(string(encoded), approved.Invitation.TokenHash) {
		t.Fatalf("application response leaked invitation material: %s", encoded)
	}

	if _, err := service.AcceptInvitation(ctx, approved.InvitationToken, actorID, "someone-else@example.com", "Wrong User"); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Fatalf("wrong email error = %v, want invalid grant", err)
	}
	accepted, err := service.AcceptInvitation(ctx, approved.InvitationToken, actorID, "nadia@example.com", "Nadia Putri")
	if err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	if accepted.Application.Status != domain.DeveloperApplicationStatusActive || !accepted.Entitlement.SandboxAccess || accepted.Entitlement.ProductionAccess {
		t.Fatalf("unexpected acceptance: %#v", accepted)
	}
	organizations := application.NewDeveloperOrganizationService(repository)
	listed, meta, err := organizations.List(ctx, ports.DeveloperOrganizationFilter{Search: "nusantara", Status: domain.OrganizationStatusActive})
	if err != nil || meta.HasMore || len(listed) != 1 || listed[0].Name != "Nusantara Labs" || listed[0].MembershipCount != 1 {
		t.Fatalf("unexpected organization list: items=%#v meta=%#v err=%v", listed, meta, err)
	}
	detail, err := organizations.Get(ctx, listed[0].ID)
	if err != nil || len(detail.Memberships) != 1 || detail.Memberships[0].Email != "nadia@example.com" || detail.Memberships[0].Role != domain.RoleOwner {
		t.Fatalf("unexpected organization detail: detail=%#v err=%v", detail, err)
	}
	if _, err := service.AcceptInvitation(ctx, approved.InvitationToken, actorID, "nadia@example.com", "Nadia Putri"); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Fatalf("replayed invite error = %v, want invalid grant", err)
	}
}
