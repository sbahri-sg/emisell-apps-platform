package identity

import (
	"context"
	"crypto/rand"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Managed keys are first-party Core service accounts, never app OAuth tokens.
type ManagedKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	TenantID  string     `json:"tenantId"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt"`
	Status    string     `json:"status"`
}
type CreateManagedKey struct {
	Name      string   `json:"name"`
	TenantID  string   `json:"tenantId"`
	Scopes    []string `json:"scopes"`
	ValidDays int      `json:"validDays"`
}
type ManagedKeyRepository interface {
	ListManagedKeys(context.Context) ([]ManagedKey, error)
	CreateManagedKey(context.Context, string, string, string, ManagedKey, string) (ManagedKey, bool, error)
	RevokeManagedKey(context.Context, string, string) (ManagedKey, error)
}
type ManagedKeys struct{ Repo ManagedKeyRepository }

func CoreScopes() []string {
	return []string{"payments.read", "payments.write", "shipping.read", "shipping.write", ScopeInstallIntentsRead, ScopeInstallIntentsWrite, ScopeInstallIntentsConsent}
}
func keyAdministrator(p PortalPrincipal) bool {
	return p.ID != "" && p.Surface == "admin" && p.Role == "administrator"
}

var keyIdentifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var keyRequest = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

func (s ManagedKeys) List(ctx context.Context, p PortalPrincipal) ([]ManagedKey, error) {
	if !keyAdministrator(p) {
		return nil, fault.Forbidden
	}
	return s.Repo.ListManagedKeys(ctx)
}

// A replay returns metadata only. Never persist or re-deliver a recoverable secret.
// A lost first response requires revocation and a newly generated key.
func (s ManagedKeys) Create(ctx context.Context, p PortalPrincipal, request string, in CreateManagedKey) (ManagedKey, string, error) {
	if !keyAdministrator(p) {
		return ManagedKey{}, "", fault.Forbidden
	}
	in.Name = strings.TrimSpace(in.Name)
	if !keyRequest.MatchString(request) || !keyIdentifier.MatchString(in.TenantID) || in.Name == "" || len(in.Name) > 80 || strings.ContainsFunc(in.Name, unicode.IsControl) || in.ValidDays < 1 || in.ValidDays > 30 || len(in.Scopes) == 0 || len(in.Scopes) > len(CoreScopes()) {
		return ManagedKey{}, "", fault.Invalid
	}
	in.Scopes = slices.Clone(in.Scopes)
	slices.Sort(in.Scopes)
	for i, scope := range in.Scopes {
		if !slices.Contains(CoreScopes(), scope) || (i > 0 && in.Scopes[i-1] == scope) {
			return ManagedKey{}, "", fault.Invalid
		}
	}
	body, err := json.Marshal(in)
	if err != nil {
		return ManagedKey{}, "", err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return ManagedKey{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	v := ManagedKey{ID: ids.New("corekey"), Name: in.Name, TenantID: in.TenantID, Scopes: in.Scopes, ExpiresAt: time.Now().Add(time.Duration(in.ValidDays) * 24 * time.Hour)}
	out, created, err := s.Repo.CreateManagedKey(ctx, p.ID, request, tokenHash(string(body)), v, tokenHash(token))
	if err != nil {
		return ManagedKey{}, "", err
	}
	if !created {
		token = ""
	}
	return out, token, nil
}
func (s ManagedKeys) Revoke(ctx context.Context, p PortalPrincipal, id string) (ManagedKey, error) {
	if !keyAdministrator(p) {
		return ManagedKey{}, fault.Forbidden
	}
	if !keyIdentifier.MatchString(id) {
		return ManagedKey{}, fault.Invalid
	}
	return s.Repo.RevokeManagedKey(ctx, p.ID, id)
}
