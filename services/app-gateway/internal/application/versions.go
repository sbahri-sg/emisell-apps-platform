package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type SnapshotBuilder interface {
	Build(ctx context.Context, app domain.App) (domain.VersionSnapshot, error)
}

type VersionService struct {
	repository      ports.Repository
	snapshotBuilder SnapshotBuilder
	id              ids.Generator
	now             func() time.Time
}

type CreateVersionCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	AppID          string
	Version        string
	ReleaseNote    *string
}

func NewVersionService(repository ports.Repository, snapshotBuilder SnapshotBuilder, id ids.Generator, now func() time.Time) *VersionService {
	return &VersionService{repository: repository, snapshotBuilder: snapshotBuilder, id: id, now: now}
}

func (s *VersionService) List(ctx context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.AppVersion, ports.PageMeta, error) {
	return s.repository.ListVersions(ctx, organizationID, appID, filter)
}

func (s *VersionService) Get(ctx context.Context, organizationID, appID, versionID string) (domain.AppVersion, error) {
	return s.repository.GetVersion(ctx, organizationID, appID, versionID)
}

var semanticVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func (s *VersionService) Create(ctx context.Context, command CreateVersionCommand) (domain.AppVersion, error) {
	if !semanticVersion.MatchString(command.Version) {
		return domain.AppVersion{}, fmt.Errorf("%w: version must use semantic version format", domain.ErrValidation)
	}
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return domain.AppVersion{}, err
	}
	if app.Status == domain.AppStatusArchived {
		return domain.AppVersion{}, fmt.Errorf("%w: archived apps cannot create versions", domain.ErrConflict)
	}
	snapshot, err := s.snapshotBuilder.Build(ctx, app)
	if err != nil {
		return domain.AppVersion{}, fmt.Errorf("build version snapshot: %w", err)
	}
	snapshot.ConfigurationHash, err = configurationHash(snapshot)
	if err != nil {
		return domain.AppVersion{}, err
	}
	id, err := s.id()
	if err != nil {
		return domain.AppVersion{}, fmt.Errorf("generate version id: %w", err)
	}
	version := domain.AppVersion{
		ID:          id,
		AppID:       command.AppID,
		Version:     command.Version,
		Status:      domain.VersionStatusDraft,
		ReleaseNote: trimOptional(command.ReleaseNote),
		Snapshot:    snapshot,
		CreatedBy:   command.ActorID,
		CreatedAt:   s.now().UTC(),
	}
	return s.repository.CreateVersion(ctx, command.OrganizationID, version, ports.MutationMeta{ActorID: command.ActorID, Action: "version.created", IdempotencyKey: command.IdempotencyKey})
}

func (s *VersionService) Release(ctx context.Context, organizationID, actorID, appID, versionID, idempotencyKey string, expectedActiveVersionID *string) (domain.AppVersion, error) {
	return s.repository.ActivateVersion(ctx, organizationID, appID, versionID, domain.VersionStatusDraft, expectedActiveVersionID, ports.MutationMeta{ActorID: actorID, Action: "version.released", IdempotencyKey: idempotencyKey})
}

func (s *VersionService) Rollback(ctx context.Context, organizationID, actorID, appID, versionID, idempotencyKey string) (domain.AppVersion, error) {
	return s.repository.ActivateVersion(ctx, organizationID, appID, versionID, domain.VersionStatusReleased, nil, ports.MutationMeta{ActorID: actorID, Action: "version.rolled_back", IdempotencyKey: idempotencyKey})
}

func configurationHash(snapshot domain.VersionSnapshot) (string, error) {
	snapshot.ConfigurationHash = ""
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode version snapshot: %w", err)
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

type CurrentConfigurationSnapshotBuilder struct {
	Repository ports.Repository
}

func (builder CurrentConfigurationSnapshotBuilder) Build(ctx context.Context, app domain.App) (domain.VersionSnapshot, error) {
	extensions, err := builder.Repository.ListExtensions(ctx, app.OrganizationID, app.ID)
	if err != nil {
		return domain.VersionSnapshot{}, err
	}
	scopes, err := builder.Repository.ListScopes(ctx, app.OrganizationID, app.ID)
	if err != nil {
		return domain.VersionSnapshot{}, err
	}
	webhooks := make([]domain.WebhookSubscription, 0)
	cursor := ""
	for {
		items, meta, err := builder.Repository.ListWebhooks(ctx, app.OrganizationID, app.ID, ports.AppFilter{Cursor: cursor, Limit: 100})
		if err != nil {
			return domain.VersionSnapshot{}, err
		}
		webhooks = append(webhooks, items...)
		if meta.NextCursor == nil {
			break
		}
		cursor = *meta.NextCursor
	}
	snapshotExtensions := make([]domain.SnapshotExtension, 0, len(extensions))
	for _, extension := range extensions {
		if extension.Status == domain.ExtensionStatusDisabled {
			continue
		}
		snapshotExtensions = append(snapshotExtensions, domain.SnapshotExtension{
			ExtensionID:   extension.ID,
			Name:          extension.Name,
			Type:          string(extension.Type),
			RuntimeURL:    extension.RuntimeURL,
			Configuration: extension.Configuration,
		})
	}
	snapshotScopes := make([]domain.SnapshotScope, 0, len(scopes))
	for _, scope := range scopes {
		snapshotScopes = append(snapshotScopes, domain.SnapshotScope{Scope: scope.Scope, Access: string(scope.Access)})
	}
	redirectURLs := make([]string, 0, 1)
	if app.AppURL != nil && strings.TrimSpace(*app.AppURL) != "" {
		redirectURLs = append(redirectURLs, *app.AppURL)
	}
	snapshotWebhooks := make([]domain.SnapshotWebhook, 0, len(webhooks))
	for _, subscription := range webhooks {
		if subscription.Status != domain.WebhookStatusActive {
			continue
		}
		snapshotWebhooks = append(snapshotWebhooks, domain.SnapshotWebhook{
			SubscriptionID: subscription.ID, Event: subscription.Event, EndpointURL: subscription.EndpointURL,
		})
	}
	return domain.VersionSnapshot{
		Extensions:           snapshotExtensions,
		Scopes:               snapshotScopes,
		WebhookSubscriptions: snapshotWebhooks,
		RedirectURLs:         redirectURLs,
	}, nil
}

var _ SnapshotBuilder = CurrentConfigurationSnapshotBuilder{}
