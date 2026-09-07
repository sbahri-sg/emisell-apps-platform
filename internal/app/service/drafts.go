package service

import (
	"context"
	"crypto/sha256"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/webhookconfig"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

// AppDocument is an authoring contract, not a signed executable manifest.
type AppDocument struct {
	Name         string                   `json:"name"`
	Summary      string                   `json:"summary"`
	Description  string                   `json:"description"`
	Version      string                   `json:"version"`
	Capability   string                   `json:"capability"`
	Scopes       []string                 `json:"scopes"`
	Endpoint     string                   `json:"endpoint"`
	AccessScopes *accessscope.Declaration `json:"accessScopes,omitempty"`
	Webhooks     *webhookconfig.Config    `json:"webhooks,omitempty"`
}
type Draft struct {
	ID             string      `json:"id"`
	OrganizationID string      `json:"organizationId"`
	Revision       int         `json:"revision"`
	Document       AppDocument `json:"document"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}
type SaveDraft struct {
	Revision int         `json:"revision"`
	Document AppDocument `json:"document"`
}
type DraftRepository interface {
	Drafts(context.Context, string) ([]Draft, error)
	Draft(context.Context, string, string) (Draft, error)
	SaveDraft(context.Context, string, string, string, string, string, SaveDraft) (Draft, error)
}
type Drafts struct {
	Repo       DraftRepository
	Developers developer.Service
}

var appVersion = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$`)
var requestKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

func ValidRequestKey(key string) bool { return requestKey.MatchString(key) }
func RequestHash(v any) string {
	raw, _ := json.Marshal(v)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
func (d AppDocument) Validate(submit bool) error {
	if d.Webhooks != nil && d.Webhooks.Validate(d.AccessScopes) != nil {
		return fault.Invalid
	}
	if d.AccessScopes != nil && d.AccessScopes.Validate() != nil {
		return fault.Invalid
	}
	if strings.TrimSpace(d.Name) == "" || len(d.Name) > 100 || len(d.Summary) > 180 || len(d.Description) > 5000 || !appVersion.MatchString(d.Version) || len(d.Endpoint) > 2048 {
		return fault.Invalid
	}
	allowed := []string{"orders.read", "payments.read", "payments.write"}
	if d.Capability == "shipping/v1" {
		allowed = []string{"orders.read", "shipping.read", "shipping.write"}
	} else if d.Capability != "payment/v1" {
		return fault.Invalid
	}
	if len(d.Scopes) > len(allowed) {
		return fault.Invalid
	}
	seen := map[string]bool{}
	for _, scope := range d.Scopes {
		if !slices.Contains(allowed, scope) || seen[scope] {
			return fault.Invalid
		}
		seen[scope] = true
	}
	if d.Endpoint != "" {
		u, err := url.Parse(d.Endpoint)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return fault.Invalid
		}
	}
	if submit && (strings.TrimSpace(d.Summary) == "" || strings.TrimSpace(d.Description) == "" || d.Endpoint == "" || len(d.Scopes) != len(allowed)) {
		return fault.Invalid
	}
	return nil
}
func (s Drafts) List(ctx context.Context, p identity.PortalPrincipal) ([]Draft, error) {
	org, err := s.Developers.Organization(ctx, p)
	if err != nil {
		return nil, err
	}
	return s.Repo.Drafts(ctx, org.ID)
}
func (s Drafts) Get(ctx context.Context, p identity.PortalPrincipal, id string) (Draft, error) {
	org, err := s.Developers.Organization(ctx, p)
	if err != nil {
		return Draft{}, err
	}
	return s.Repo.Draft(ctx, org.ID, id)
}
func (s Drafts) Save(ctx context.Context, p identity.PortalPrincipal, id, key string, b SaveDraft) (Draft, error) {
	org, err := s.Developers.Organization(ctx, p)
	if err != nil {
		return Draft{}, err
	}
	if !ValidRequestKey(key) || b.Revision < 0 || (id == "" && b.Revision != 0) || (id != "" && b.Revision == 0) {
		return Draft{}, fault.Invalid
	}
	if err = b.Document.Validate(false); err != nil {
		return Draft{}, err
	}
	if err = ValidatePublicDistribution(b.Document.Capability); err != nil {
		return Draft{}, err
	}
	if id != "" {
		previous, err := s.Repo.Draft(ctx, org.ID, id)
		if err != nil {
			return Draft{}, err
		}
		if err = ValidatePublicDistribution(previous.Document.Capability); err != nil {
			return Draft{}, err
		}
	}
	return s.Repo.SaveDraft(ctx, org.ID, p.ID, id, key, RequestHash(struct {
		ID   string
		Body SaveDraft
	}{id, b}), b)
}
