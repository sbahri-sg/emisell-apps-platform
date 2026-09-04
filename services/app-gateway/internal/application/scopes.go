package application

import (
	"context"
	"fmt"
	"regexp"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type ScopeService struct {
	repository ports.Repository
}

type ReplaceScopesCommand struct {
	OrganizationID string
	ActorID        string
	AppID          string
	Scopes         []domain.AppScope
}

func NewScopeService(repository ports.Repository) *ScopeService {
	return &ScopeService{repository: repository}
}

func (s *ScopeService) List(ctx context.Context, organizationID, appID string) ([]domain.AppScope, error) {
	return s.repository.ListScopes(ctx, organizationID, appID)
}

func (s *ScopeService) Catalog() []domain.ScopeDefinition {
	return OfficialScopeCatalog()
}

var scopeName = regexp.MustCompile(`^(read|write)_[a-z_]+$`)

func (s *ScopeService) Replace(ctx context.Context, command ReplaceScopesCommand) ([]domain.AppScope, error) {
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return nil, err
	}
	if app.Status == domain.AppStatusArchived {
		return nil, fmt.Errorf("%w: archived apps cannot change scopes", domain.ErrConflict)
	}
	if len(command.Scopes) > 100 {
		return nil, fmt.Errorf("%w: an app can request at most 100 scopes", domain.ErrValidation)
	}
	seen := make(map[string]struct{}, len(command.Scopes))
	validated := make([]domain.AppScope, 0, len(command.Scopes))
	for _, item := range command.Scopes {
		if !scopeName.MatchString(item.Scope) || len(item.Scope) > 100 {
			return nil, fmt.Errorf("%w: invalid scope %q", domain.ErrValidation, item.Scope)
		}
		if _, registered := LookupOfficialScope(item.Scope); !registered {
			return nil, fmt.Errorf("%w: scope %q is not registered in the Emisell scope catalog", domain.ErrValidation, item.Scope)
		}
		if item.Access != domain.ScopeAccessRequired && item.Access != domain.ScopeAccessOptional {
			return nil, fmt.Errorf("%w: invalid access for scope %q", domain.ErrValidation, item.Scope)
		}
		if _, exists := seen[item.Scope]; exists {
			return nil, fmt.Errorf("%w: duplicate scope %q", domain.ErrValidation, item.Scope)
		}
		seen[item.Scope] = struct{}{}
		item.AppID = command.AppID
		validated = append(validated, item)
	}
	return s.repository.ReplaceScopes(ctx, command.OrganizationID, command.AppID, validated, ports.MutationMeta{
		ActorID: command.ActorID, Action: "scopes.replaced",
	})
}
