package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type ExtensionService struct {
	repository ports.Repository
	id         ids.Generator
	now        func() time.Time
}

type CreateExtensionCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	AppID          string
	Name           string
	Type           domain.ExtensionType
	RuntimeURL     *string
	Configuration  map[string]interface{}
}

type UpdateExtensionCommand struct {
	OrganizationID string
	ActorID        string
	AppID          string
	ExtensionID    string
	Name           *string
	RuntimeURL     **string
	Configuration  *map[string]interface{}
	Revision       int64
}

func NewExtensionService(repository ports.Repository, id ids.Generator, now func() time.Time) *ExtensionService {
	return &ExtensionService{repository: repository, id: id, now: now}
}

func (s *ExtensionService) List(ctx context.Context, organizationID, appID string) ([]domain.AppExtension, error) {
	return s.repository.ListExtensions(ctx, organizationID, appID)
}

func (s *ExtensionService) Create(ctx context.Context, command CreateExtensionCommand) (domain.AppExtension, error) {
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return domain.AppExtension{}, err
	}
	if app.Status == domain.AppStatusArchived {
		return domain.AppExtension{}, fmt.Errorf("%w: archived apps cannot add extensions", domain.ErrConflict)
	}
	name, err := validateExtensionName(command.Name)
	if err != nil {
		return domain.AppExtension{}, err
	}
	if !validExtensionType(command.Type) {
		return domain.AppExtension{}, fmt.Errorf("%w: invalid extension type", domain.ErrValidation)
	}
	if err := validateExtensionURL(command.RuntimeURL); err != nil {
		return domain.AppExtension{}, err
	}
	id, err := s.id()
	if err != nil {
		return domain.AppExtension{}, fmt.Errorf("generate extension id: %w", err)
	}
	configuration := command.Configuration
	if configuration == nil {
		configuration = map[string]interface{}{}
	}
	now := s.now().UTC()
	extension := domain.AppExtension{
		ID: id, AppID: command.AppID, Name: name, Type: command.Type,
		Status: domain.ExtensionStatusDraft, RuntimeURL: trimOptional(command.RuntimeURL),
		Configuration: configuration, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	return s.repository.CreateExtension(ctx, command.OrganizationID, extension, ports.MutationMeta{
		ActorID: command.ActorID, Action: "extension.created", IdempotencyKey: command.IdempotencyKey,
	})
}

func (s *ExtensionService) Update(ctx context.Context, command UpdateExtensionCommand) (domain.AppExtension, error) {
	if command.Revision <= 0 {
		return domain.AppExtension{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return domain.AppExtension{}, err
	}
	if app.Status == domain.AppStatusArchived {
		return domain.AppExtension{}, fmt.Errorf("%w: archived apps cannot update extensions", domain.ErrConflict)
	}
	extension, err := s.repository.GetExtension(ctx, command.OrganizationID, command.AppID, command.ExtensionID)
	if err != nil {
		return domain.AppExtension{}, err
	}
	if extension.Status == domain.ExtensionStatusDisabled {
		return domain.AppExtension{}, fmt.Errorf("%w: disabled extensions cannot be updated", domain.ErrConflict)
	}
	if command.Name != nil {
		extension.Name, err = validateExtensionName(*command.Name)
		if err != nil {
			return domain.AppExtension{}, err
		}
	}
	if command.RuntimeURL != nil {
		if err := validateExtensionURL(*command.RuntimeURL); err != nil {
			return domain.AppExtension{}, err
		}
		extension.RuntimeURL = trimOptional(*command.RuntimeURL)
	}
	if command.Configuration != nil {
		if *command.Configuration == nil {
			return domain.AppExtension{}, fmt.Errorf("%w: configuration must be an object", domain.ErrValidation)
		}
		extension.Configuration = *command.Configuration
	}
	extension.Status = domain.ExtensionStatusDraft
	return s.repository.UpdateExtension(ctx, command.OrganizationID, extension, command.Revision, ports.MutationMeta{
		ActorID: command.ActorID, Action: "extension.updated",
	})
}

func (s *ExtensionService) Disable(ctx context.Context, organizationID, actorID, appID, extensionID string) error {
	app, err := s.repository.GetApp(ctx, organizationID, appID)
	if err != nil {
		return err
	}
	if app.Status == domain.AppStatusArchived {
		return fmt.Errorf("%w: archived apps cannot disable extensions", domain.ErrConflict)
	}
	return s.repository.DisableExtension(ctx, organizationID, appID, extensionID, ports.MutationMeta{
		ActorID: actorID, Action: "extension.disabled",
	})
}

func validateExtensionName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if len([]rune(name)) < 3 || len([]rune(name)) > 80 {
		return "", fmt.Errorf("%w: extension name must contain 3 to 80 characters", domain.ErrValidation)
	}
	return name, nil
}

func validExtensionType(value domain.ExtensionType) bool {
	for _, family := range OfficialExtensionCatalog().Families {
		if family.Type == value && family.ConfigurationSupported {
			return true
		}
	}
	return false
}

func validateExtensionURL(value *string) error {
	if value == nil {
		return nil
	}
	parsed, err := url.ParseRequestURI(strings.TrimSpace(*value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%w: runtime URL must use HTTPS", domain.ErrValidation)
	}
	return nil
}
