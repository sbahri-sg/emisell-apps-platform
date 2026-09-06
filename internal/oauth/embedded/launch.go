package embedded

import (
	"context"
	"crypto/ed25519"
	identity "emisell.app/platform/pkg/embedded"
)

// Binding must be resolved from current, reviewed installation/release state.
// Resolve is responsible for matching the current installation release digest.
type Binding struct {
	Launch    identity.Launch
	Signature string
}
type ResolveLaunch func(context.Context, identity.Identity) (Binding, error)

type Launcher struct {
	Identity  Service
	Resolve   ResolveLaunch
	LaunchKey ed25519.PublicKey
	LocalTest bool
}

type Session struct {
	Launch    identity.Launch `json:"launch"`
	Token     string          `json:"identityToken"`
	ExpiresIn int             `json:"expiresIn"`
}

// Open always resolves and verifies the current release; no cached launch grant.
func (l Launcher) Open(ctx context.Context, id identity.Identity, parentOrigin string) (Session, error) {
	if l.Resolve == nil {
		return Session{}, ErrUnavailable
	}
	if !id.Valid() {
		return Session{}, identity.ErrInvalid
	}
	b, err := l.Resolve(ctx, id)
	if err != nil {
		return Session{}, err
	}
	if identity.VerifyLaunch(b.Launch, b.Signature, l.LaunchKey, l.LocalTest) != nil || b.Launch.AppID != id.AppID || b.Launch.ParentOrigin != parentOrigin {
		return Session{}, identity.ErrInvalid
	}
	if b.Launch.DisplayMode() != "embedded" {
		return Session{}, ErrDenied
	}
	token, err := l.Identity.Issue(ctx, id, b.Launch.ClientID)
	if err != nil {
		return Session{}, err
	}
	return Session{b.Launch, token, 60}, nil
}

// External resolves the reviewed destination without issuing an embedded token
// or asserting SSO. Authorization remains mandatory for an installed app link.
func (l Launcher) External(ctx context.Context, id identity.Identity, parentOrigin string) (identity.Launch, error) {
	if l.Resolve == nil || l.Identity.Authorize == nil {
		return identity.Launch{}, ErrUnavailable
	}
	if !id.Valid() {
		return identity.Launch{}, identity.ErrInvalid
	}
	b, err := l.Resolve(ctx, id)
	if err != nil {
		return identity.Launch{}, err
	}
	if identity.VerifyLaunch(b.Launch, b.Signature, l.LaunchKey, l.LocalTest) != nil || b.Launch.AppID != id.AppID || b.Launch.ParentOrigin != parentOrigin || b.Launch.DisplayMode() != "external" {
		return identity.Launch{}, ErrDenied
	}
	if err := l.Identity.Authorize(ctx, id, b.Launch.ClientID); err != nil {
		return identity.Launch{}, err
	}
	return b.Launch, nil
}
