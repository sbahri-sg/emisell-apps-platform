package developer

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"net/mail"
	"strings"
	"unicode/utf8"
)

type CreateAccount struct {
	OrganizationName string `json:"organizationName"`
	Email            string `json:"email"`
	Password         string `json:"password"`
}
type AccountCreator interface {
	CreateDeveloper(context.Context, string, string, string, string) (AdminOrganization, error)
}

func (s Service) CreateAccount(ctx context.Context, actor identity.PortalPrincipal, input CreateAccount) (AdminOrganization, error) {
	if !adminAllowed(actor) {
		return AdminOrganization{}, fault.Forbidden
	}
	repo, ok := s.Repo.(AccountCreator)
	if !ok {
		return AdminOrganization{}, fault.Forbidden
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.OrganizationName = strings.TrimSpace(input.OrganizationName)
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 || input.OrganizationName == "" || utf8.RuneCountInString(input.OrganizationName) > 120 {
		return AdminOrganization{}, fault.Invalid
	}
	hash, err := identity.HashPassword(input.Password)
	if err != nil {
		return AdminOrganization{}, err
	}
	return repo.CreateDeveloper(ctx, actor.ID, input.OrganizationName, input.Email, hash)
}
