// Package embedded is the application boundary for embedded identity.
package embedded

import (
	"context"
	"crypto/ed25519"
	identity "emisell.app/platform/pkg/embedded"
	"errors"
	"time"
)

var ErrUnavailable = errors.New("embedded authorization unavailable")

// Authorize must check current installation, grant, staff membership and the
// registered app-client/launch binding. A token or request body is not evidence.
type Authorize func(context.Context, identity.Identity, string) error

type Service struct {
	PrivateKey    ed25519.PrivateKey
	Keys          map[string]ed25519.PublicKey
	KeyID, Issuer string
	Authorize     Authorize
	Now           func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Service) Issue(ctx context.Context, id identity.Identity, audience string) (string, error) {
	if s.Authorize == nil {
		return "", ErrUnavailable
	}
	if !id.Valid() {
		return "", identity.ErrInvalid
	}
	if err := s.Authorize(ctx, id, audience); err != nil {
		return "", err
	}
	return identity.Issue(s.PrivateKey, s.KeyID, s.Issuer, audience, id, s.now())
}

func (s Service) Authenticate(ctx context.Context, token, audience string, expected identity.Identity) (identity.Claims, error) {
	c, err := identity.Verify(token, s.Keys, s.Issuer, audience, expected, s.now())
	if err != nil {
		return identity.Claims{}, err
	}
	if s.Authorize == nil {
		return identity.Claims{}, ErrUnavailable
	}
	if err := s.Authorize(ctx, c.Identity, audience); err != nil {
		return identity.Claims{}, err
	}
	return c, nil
}
