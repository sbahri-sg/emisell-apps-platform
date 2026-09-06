package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"
	"unicode"

	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
)

// PlatformKey authenticates first-party Core, not an app, merchant or browser.
// It has no tenant binding or automatic expiry. Only explicit revocation ends it.
type PlatformKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Access    string     `json:"access"`
	CreatedAt time.Time  `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt"`
	Status    string     `json:"status"`
}
type CreatePlatformKey struct {
	Name string `json:"name"`
}
type PlatformKeyRepository interface {
	ListPlatformKeys(context.Context) ([]PlatformKey, error)
	CreatePlatformKey(context.Context, string, string, string, PlatformKey, string) (PlatformKey, bool, error)
	RevokePlatformKey(context.Context, string, string) (PlatformKey, error)
}
type PlatformKeys struct{ Repo PlatformKeyRepository }

func (s PlatformKeys) List(ctx context.Context, p PortalPrincipal) ([]PlatformKey, error) {
	if !keyAdministrator(p) {
		return nil, fault.Forbidden
	}
	return s.Repo.ListPlatformKeys(ctx)
}
func (s PlatformKeys) Create(ctx context.Context, p PortalPrincipal, request string, in CreatePlatformKey) (PlatformKey, string, error) {
	if !keyAdministrator(p) {
		return PlatformKey{}, "", fault.Forbidden
	}
	in.Name = strings.TrimSpace(in.Name)
	if !keyRequest.MatchString(request) || in.Name == "" || len(in.Name) > 80 || strings.ContainsFunc(in.Name, unicode.IsControl) {
		return PlatformKey{}, "", fault.Invalid
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return PlatformKey{}, "", err
	}
	token := "epk_" + base64.RawURLEncoding.EncodeToString(raw)
	v := PlatformKey{ID: ids.New("platformkey"), Name: in.Name, Access: "platform_full"}
	out, created, err := s.Repo.CreatePlatformKey(ctx, p.ID, request, tokenHash(in.Name), v, tokenHash(token))
	if err != nil {
		return PlatformKey{}, "", err
	}
	// No recoverable secret is stored, including for idempotent retries.
	if !created {
		token = ""
	}
	return out, token, nil
}
func (s PlatformKeys) Revoke(ctx context.Context, p PortalPrincipal, id string) (PlatformKey, error) {
	if !keyAdministrator(p) {
		return PlatformKey{}, fault.Forbidden
	}
	if !keyIdentifier.MatchString(id) || !strings.HasPrefix(id, "platformkey_") {
		return PlatformKey{}, fault.Invalid
	}
	return s.Repo.RevokePlatformKey(ctx, p.ID, id)
}
