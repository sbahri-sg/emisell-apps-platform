package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type AppService struct {
	repository        ports.Repository
	id                ids.Generator
	now               func() time.Time
	allowLoopbackHTTP bool
}

type CreateAppCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	Name           string
	Description    *string
	Distribution   domain.Distribution
	AppURL         *string
	ContactEmail   *string
}

type UpdateAppCommand struct {
	OrganizationID string
	ActorID        string
	AppID          string
	Name           *string
	Description    **string
	Distribution   *domain.Distribution
	AppURL         **string
	ContactEmail   **string
	Revision       int64
}

func NewAppService(repository ports.Repository, id ids.Generator, now func() time.Time, localHTTP ...bool) *AppService {
	return &AppService{repository: repository, id: id, now: now, allowLoopbackHTTP: len(localHTTP) > 0 && localHTTP[0]}
}

func (s *AppService) List(ctx context.Context, organizationID string, filter ports.AppFilter) ([]domain.App, ports.PageMeta, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: organization is required", domain.ErrValidation)
	}
	if filter.Status != "" && filter.Status != domain.AppStatusDraft && filter.Status != domain.AppStatusActive && filter.Status != domain.AppStatusArchived {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: invalid app status", domain.ErrValidation)
	}
	items, meta, err := s.repository.ListApps(ctx, organizationID, filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	for index := range items {
		items[index], err = s.withReleaseStatus(ctx, items[index])
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
	}
	return items, meta, nil
}

func (s *AppService) Create(ctx context.Context, command CreateAppCommand) (domain.App, error) {
	name := strings.TrimSpace(command.Name)
	if len([]rune(name)) < 3 || len([]rune(name)) > 80 {
		return domain.App{}, fmt.Errorf("%w: app name must contain 3 to 80 characters", domain.ErrValidation)
	}
	if command.Distribution != domain.DistributionCustom {
		return domain.App{}, fmt.Errorf("%w: only custom distribution is available during invite-only access", domain.ErrValidation)
	}
	if err := validateAppURL(command.AppURL, s.allowLoopbackHTTP); err != nil {
		return domain.App{}, err
	}
	if err := validateOptionalEmail(command.ContactEmail); err != nil {
		return domain.App{}, err
	}
	id, err := s.id()
	if err != nil {
		return domain.App{}, fmt.Errorf("generate app id: %w", err)
	}
	now := s.now().UTC()
	app := domain.App{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           name,
		Slug:           slugify(name),
		Description:    trimOptional(command.Description),
		Distribution:   command.Distribution,
		Status:         domain.AppStatusDraft,
		AppURL:         command.AppURL,
		ContactEmail:   command.ContactEmail,
		CreatedBy:      command.ActorID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	created, err := s.repository.CreateApp(ctx, app, ports.MutationMeta{ActorID: command.ActorID, Action: "app.created", IdempotencyKey: command.IdempotencyKey})
	if err != nil {
		return domain.App{}, err
	}
	return s.withReleaseStatus(ctx, created)
}

func (s *AppService) Get(ctx context.Context, organizationID, appID string) (domain.App, error) {
	app, err := s.repository.GetApp(ctx, organizationID, appID)
	if err != nil {
		return domain.App{}, err
	}
	return s.withReleaseStatus(ctx, app)
}

func (s *AppService) Update(ctx context.Context, command UpdateAppCommand) (domain.App, error) {
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return domain.App{}, err
	}
	if command.Revision <= 0 {
		return domain.App{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	if command.Name != nil {
		name := strings.TrimSpace(*command.Name)
		if len([]rune(name)) < 3 || len([]rune(name)) > 80 {
			return domain.App{}, fmt.Errorf("%w: app name must contain 3 to 80 characters", domain.ErrValidation)
		}
		app.Name = name
	}
	if command.Description != nil {
		app.Description = trimOptional(*command.Description)
	}
	if command.Distribution != nil {
		if *command.Distribution != domain.DistributionCustom {
			return domain.App{}, fmt.Errorf("%w: only custom distribution is available during invite-only access", domain.ErrValidation)
		}
		app.Distribution = *command.Distribution
	}
	if command.AppURL != nil {
		if err := validateAppURL(*command.AppURL, s.allowLoopbackHTTP); err != nil {
			return domain.App{}, err
		}
		app.AppURL = *command.AppURL
	}
	if command.ContactEmail != nil {
		if err := validateOptionalEmail(*command.ContactEmail); err != nil {
			return domain.App{}, err
		}
		app.ContactEmail = *command.ContactEmail
	}
	updated, err := s.repository.UpdateApp(ctx, app, command.Revision, ports.MutationMeta{ActorID: command.ActorID, Action: "app.updated"})
	if err != nil {
		return domain.App{}, err
	}
	return s.withReleaseStatus(ctx, updated)
}

func (s *AppService) Archive(ctx context.Context, organizationID, actorID, appID, idempotencyKey string) error {
	return s.repository.ArchiveApp(ctx, organizationID, appID, ports.MutationMeta{ActorID: actorID, Action: "app.archived", IdempotencyKey: idempotencyKey})
}

func (s *AppService) withReleaseStatus(ctx context.Context, app domain.App) (domain.App, error) {
	status, err := resolveAppReleaseStatus(ctx, s.repository, app.OrganizationID, app.ID)
	if err != nil {
		return domain.App{}, err
	}
	app.ReleaseStatus = status
	return app, nil
}

func resolveAppReleaseStatus(ctx context.Context, repository ports.Repository, organizationID, appID string) (domain.AppReleaseStatus, error) {
	catalog, available := repository.(ports.CatalogRepository)
	if !available {
		return domain.AppReleaseStatusDevelopment, nil
	}
	listing, err := catalog.GetCatalogListing(ctx, organizationID, appID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.AppReleaseStatusDevelopment, nil
	}
	if err != nil {
		return "", err
	}
	if listing.Status == domain.CatalogListingStatusPublished {
		return domain.AppReleaseStatusReleased, nil
	}
	return domain.AppReleaseStatusDevelopment, nil
}

func validateOptionalHTTPSURL(value *string) error {
	return validateAppURL(value, false)
}

func validateAppURL(value *string, allowLoopbackHTTP bool) error {
	if value == nil {
		return nil
	}
	parsed, err := url.Parse(*value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("%w: invalid app URL", domain.ErrValidation)
	}
	local := parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1")
	if parsed.Scheme != "https" && !(allowLoopbackHTTP && local) {
		return fmt.Errorf("%w: URL must use HTTPS", domain.ErrValidation)
	}
	return nil
}

func validateOptionalEmail(value *string) error {
	if value == nil {
		return nil
	}
	if _, err := mail.ParseAddress(*value); err != nil {
		return fmt.Errorf("%w: invalid contact email", domain.ErrValidation)
	}
	return nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

var slugSeparator = regexp.MustCompile(`-+`)

func slugify(value string) string {
	var builder strings.Builder
	for _, char := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(char), unicode.IsDigit(char):
			builder.WriteRune(char)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(slugSeparator.ReplaceAllString(builder.String(), "-"), "-")
}
