package identity

import (
	"context"
	"crypto/rand"
	"emisell.app/platform/internal/platform/fault"
	"encoding/base64"
	"slices"
	"strings"
	"time"
)

// ServicePrincipal is distinct from a browser user. Legacy keys bind one tenant;
// platform keys bind a tenant per request, after authenticating first-party Core.
type ServicePrincipal struct {
	ID           string
	TenantID     string
	Scopes       []string
	ExpiresAt    time.Time
	PlatformFull bool
}
type PlatformServiceRepository interface {
	FindPlatformService(context.Context, string) (ServicePrincipal, error)
	PlatformServiceAllowed(context.Context, string, string) (bool, error)
}

func (p ServicePrincipal) AllowsServiceScope(scope string) bool {
	return p.PlatformFull || slices.Contains(p.Scopes, scope)
}

// Legacy intents may omit the tenant; they never gain a different tenant.
// Platform keys require an explicit, trusted Core tenant assertion on every call.
func (p ServicePrincipal) BindTenant(tenant string) (ServicePrincipal, error) {
	if p.PlatformFull {
		if !keyIdentifier.MatchString(tenant) {
			return ServicePrincipal{}, fault.Invalid
		}
		p.TenantID = tenant
	} else if tenant != "" && tenant != p.TenantID {
		return ServicePrincipal{}, fault.NotFound
	}
	return p, nil
}

type ServiceAccountRepository interface {
	FindService(context.Context, string) (ServicePrincipal, error)
	ServiceAllowed(context.Context, string, string) (bool, error)
	PutService(context.Context, ServicePrincipal, string, bool, string) error
	RevokeService(context.Context, string, string) error
}
type ServiceAccounts struct{ Repo ServiceAccountRepository }

// First-party Core only. Never provision these scopes for third-party apps.
const (
	ScopeInstallIntentsRead    = "apps.install_intents.read"
	ScopeInstallIntentsWrite   = "apps.install_intents.write"
	ScopeInstallIntentsConsent = "apps.install_intents.consent"
)

func (s ServiceAccounts) Authenticate(ctx context.Context, token string) (ServicePrincipal, error) {
	if len(token) == 47 && strings.HasPrefix(token, "epk_") {
		if repo, ok := s.Repo.(PlatformServiceRepository); ok {
			return repo.FindPlatformService(ctx, tokenHash(token))
		}
		return ServicePrincipal{}, fault.Unauthenticated
	}
	if len(token) != 43 {
		return ServicePrincipal{}, fault.Unauthenticated
	}
	return s.Repo.FindService(ctx, tokenHash(token))
}
func (s ServiceAccounts) Authorize(ctx context.Context, id, tenant string) error {
	var ok bool
	var err error
	if strings.HasPrefix(id, "platformkey_") {
		repo, available := s.Repo.(PlatformServiceRepository)
		if !available {
			return fault.Unauthenticated
		}
		ok, err = repo.PlatformServiceAllowed(ctx, id, tenant)
	} else {
		ok, err = s.Repo.ServiceAllowed(ctx, id, tenant)
	}
	if err != nil {
		return err
	}
	if !ok {
		return fault.NotFound
	}
	return nil
}
func (s ServiceAccounts) Issue(ctx context.Context, principal ServicePrincipal, rotate bool, actor string) (string, error) {
	if principal.PlatformFull || strings.HasPrefix(principal.ID, "platformkey_") || principal.ID == "" || principal.TenantID == "" || actor == "" || len(principal.Scopes) == 0 || !principal.ExpiresAt.After(time.Now()) || principal.ExpiresAt.After(time.Now().Add(31*24*time.Hour)) {
		return "", fault.Invalid
	}
	for _, scope := range principal.Scopes {
		if !slices.Contains(CoreScopes(), scope) {
			return "", fault.Invalid
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.Repo.PutService(ctx, principal, tokenHash(token), rotate, actor); err != nil {
		return "", err
	}
	return token, nil
}
func (s ServiceAccounts) Revoke(ctx context.Context, id, actor string) error {
	return s.Repo.RevokeService(ctx, id, actor)
}
