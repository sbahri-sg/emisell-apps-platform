package identity

import (
	"context"
	"crypto/rand"
	"emisell.app/platform/internal/platform/fault"
	"strings"
	"time"
)

// PortalPrincipal is deliberately unrelated to merchant membership and Core tokens.
type PortalPrincipal struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Surface string `json:"surface"`
	Role    string `json:"role"`
}

func (p PortalPrincipal) CanReview() bool {
	return p.Surface == "admin" && (p.Role == "administrator" || p.Role == "reviewer")
}

type PortalRepository interface {
	FindPortal(context.Context, string, string) (PortalPrincipal, string, error)
	ReplacePortalSession(context.Context, string, string, string, string, string, time.Time) error
	PortalSession(context.Context, string, string) (PortalPrincipal, error)
	DeletePortalSession(context.Context, string, string) error
}
type Portals struct{ Repo PortalRepository }

// StaffAccount excludes credentials and session material by construction.
type StaffAccount struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Enabled bool   `json:"enabled"`
}
type StaffDirectory interface {
	ListStaff(context.Context, string) ([]StaffAccount, error)
}

func (s Portals) ListStaff(ctx context.Context, actor PortalPrincipal, after string) ([]StaffAccount, error) {
	if actor.ID == "" || actor.Surface != "admin" || actor.Role != "administrator" {
		return nil, fault.Forbidden
	}
	if len(after) > 200 {
		return nil, fault.Invalid
	}
	repo, ok := s.Repo.(StaffDirectory)
	if !ok {
		return nil, fault.Forbidden
	}
	return repo.ListStaff(ctx, after)
}

func (s Portals) Login(ctx context.Context, surface, email, password, oldToken string) (PortalPrincipal, string, error) {
	if surface != "admin" || len(email) > 254 || len(password) > 256 {
		return PortalPrincipal{}, "", fault.Unauthenticated
	}
	p, hash, err := s.Repo.FindPortal(ctx, surface, strings.ToLower(strings.TrimSpace(email)))
	if err != nil && err != fault.NotFound {
		return p, "", err
	}
	if err != nil {
		hash = "pbkdf2-sha256$600000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	valid := VerifyPassword(password, hash)
	if !valid || err != nil {
		return PortalPrincipal{}, "", fault.Unauthenticated
	}
	token := rand.Text() + rand.Text()
	if err = s.Repo.ReplacePortalSession(ctx, p.ID, surface, tokenHash(token), tokenHash(oldToken), hash, time.Now().Add(8*time.Hour)); err != nil {
		return PortalPrincipal{}, "", err
	}
	return p, token, nil
}
func (s Portals) Authenticate(ctx context.Context, surface, token string) (PortalPrincipal, error) {
	if len(token) != 52 {
		return PortalPrincipal{}, fault.Unauthenticated
	}
	return s.Repo.PortalSession(ctx, surface, tokenHash(token))
}
func (s Portals) Logout(ctx context.Context, surface, token string) error {
	return s.Repo.DeletePortalSession(ctx, surface, tokenHash(token))
}
