package identity

import (
	"context"
	"crypto/rand"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"strings"
	"time"
)

// Resolve login from validated credentials, never from a client-supplied role.
func (s Portals) LoginUnified(ctx context.Context, email, password, old string) (PortalPrincipal, string, error) {
	if len(email) > 254 || len(password) > 256 {
		return PortalPrincipal{}, "", fault.Unauthenticated
	}
	var selected PortalPrincipal
	var selectedHash string
	matches := 0
	for _, surface := range []string{"admin", "developer"} {
		p, hash, err := s.Repo.FindPortal(ctx, surface, strings.ToLower(strings.TrimSpace(email)))
		if err != nil && !errors.Is(err, fault.NotFound) {
			return PortalPrincipal{}, "", err
		}
		if err != nil {
			hash = "pbkdf2-sha256$600000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}
		valid := VerifyPassword(password, hash)
		if valid && err == nil {
			selected = p
			selectedHash = hash
			matches++
		}
	}
	if matches != 1 {
		return PortalPrincipal{}, "", fault.Unauthenticated
	}
	// Switching accounts revokes the previous unified browser session, including another surface.
	if len(old) == 52 {
		for _, surface := range []string{"admin", "developer"} {
			if err := s.Logout(ctx, surface, old); err != nil {
				return PortalPrincipal{}, "", err
			}
		}
	}
	token := rand.Text() + rand.Text()
	if err := s.Repo.ReplacePortalSession(ctx, selected.ID, selected.Surface, tokenHash(token), tokenHash(old), selectedHash, time.Now().Add(8*time.Hour)); err != nil {
		return PortalPrincipal{}, "", err
	}
	return selected, token, nil
}
func (s Portals) UnifiedSession(ctx context.Context, token string) (PortalPrincipal, error) {
	for _, surface := range []string{"admin", "developer"} {
		p, err := s.Authenticate(ctx, surface, token)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, fault.Unauthenticated) {
			return PortalPrincipal{}, err
		}
	}
	return PortalPrincipal{}, fault.Unauthenticated
}
