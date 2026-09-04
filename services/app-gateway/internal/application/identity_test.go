package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

func TestIdentitySessionLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	sequence := 0
	id := func() (string, error) {
		sequence++
		return "01995f72-0000-7000-8000-" + map[int]string{1: "000000000101", 2: "000000000102"}[sequence], nil
	}
	repository := memory.NewRepository(id, func() time.Time { return now })
	repository.SeedIdentityMemberships("01995f72-0000-7000-8000-000000000002",
		domain.OrganizationMembership{OrganizationID: "01995f72-0000-7000-8000-000000000001", Name: "Emisell", Slug: "emisell", Status: "active", Role: domain.RoleOwner},
		domain.OrganizationMembership{OrganizationID: "01995f72-0000-7000-8000-000000000003", Name: "Partner", Slug: "partner", Status: "active", Role: domain.RoleDeveloper},
	)
	service := NewIdentityService(repository, id, func() time.Time { return now }, 12*time.Hour, 2*time.Hour)
	created, err := service.CreateSession(context.Background(), "01995f72-0000-7000-8000-000000000002", "User@Example.com", "Test User", true, "01995f72-0000-7000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if created.SessionToken == "" || created.CSRFToken == "" || created.Session.TokenHash == created.SessionToken {
		t.Fatal("expected opaque raw tokens and persisted digests")
	}
	session, membership, err := service.Authenticate(context.Background(), created.SessionToken)
	if err != nil || membership == nil || membership.OrganizationID != "01995f72-0000-7000-8000-000000000001" {
		t.Fatalf("authenticate session: membership=%+v err=%v", membership, err)
	}
	if err := service.ValidateCSRF(session, created.CSRFToken); err != nil {
		t.Fatalf("validate CSRF: %v", err)
	}
	if !errors.Is(service.ValidateCSRF(session, "wrong"), domain.ErrForbidden) {
		t.Fatal("wrong CSRF token must be forbidden")
	}
	_, switched, err := service.SwitchOrganization(context.Background(), session.ID, session.UserID, "01995f72-0000-7000-8000-000000000003")
	if err != nil || switched.Role != domain.RoleDeveloper {
		t.Fatalf("switch organization: membership=%+v err=%v", switched, err)
	}
	if _, _, err := service.SwitchOrganization(context.Background(), session.ID, session.UserID, "01995f72-0000-7000-8000-000000000099"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("non-member switch must be forbidden, got %v", err)
	}
	if err := service.Revoke(context.Background(), created.SessionToken); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Authenticate(context.Background(), created.SessionToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("revoked session must be unauthorized, got %v", err)
	}
}
