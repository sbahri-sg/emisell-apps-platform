package application

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type WebhookService struct {
	repository ports.Repository
	cipher     SecretCipher
	keyVersion int
	id         ids.Generator
	now        func() time.Time
}

type CreateWebhookCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	AppID          string
	Event          string
	EndpointURL    string
}

type UpdateWebhookCommand struct {
	OrganizationID string
	ActorID        string
	AppID          string
	WebhookID      string
	EndpointURL    *string
	Status         *domain.WebhookStatus
	Revision       int64
}

type WebhookSecret struct {
	Subscription  domain.WebhookSubscription `json:"subscription"`
	SigningSecret string                     `json:"signingSecret"`
}

func NewWebhookService(repository ports.Repository, cipher SecretCipher, keyVersion int, id ids.Generator, now func() time.Time) *WebhookService {
	return &WebhookService{repository: repository, cipher: cipher, keyVersion: keyVersion, id: id, now: now}
}

func (s *WebhookService) List(ctx context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.WebhookSubscription, ports.PageMeta, error) {
	return s.repository.ListWebhooks(ctx, organizationID, appID, filter)
}

func (s *WebhookService) ListEventCatalog(ctx context.Context) ([]domain.WebhookEventDefinition, error) {
	return s.repository.ListWebhookEventDefinitions(ctx)
}

var webhookEvent = regexp.MustCompile(`^[a-z]+/[a-z_]+$`)

func (s *WebhookService) Create(ctx context.Context, command CreateWebhookCommand) (WebhookSecret, error) {
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return WebhookSecret{}, err
	}
	if app.Status == domain.AppStatusArchived {
		return WebhookSecret{}, fmt.Errorf("%w: archived apps cannot create webhooks", domain.ErrConflict)
	}
	event, err := validateWebhookEvent(command.Event)
	if err != nil {
		return WebhookSecret{}, err
	}
	definition, err := s.availableEventDefinition(ctx, event)
	if err != nil {
		return WebhookSecret{}, err
	}
	if definition.RequiredScope != nil {
		scopes, err := s.repository.ListScopes(ctx, command.OrganizationID, command.AppID)
		if err != nil {
			return WebhookSecret{}, err
		}
		hasScope := false
		for _, configured := range scopes {
			if configured.Scope == *definition.RequiredScope {
				hasScope = true
				break
			}
		}
		if !hasScope {
			return WebhookSecret{}, fmt.Errorf("%w: %s requires app scope %s", domain.ErrConflict, event, *definition.RequiredScope)
		}
	}
	endpoint, err := validateWebhookEndpoint(command.EndpointURL)
	if err != nil {
		return WebhookSecret{}, err
	}
	id, err := s.id()
	if err != nil {
		return WebhookSecret{}, fmt.Errorf("generate webhook id: %w", err)
	}
	secret, err := randomToken("whsec_", 32)
	if err != nil {
		return WebhookSecret{}, err
	}
	ciphertext, err := s.cipher.Encrypt([]byte(secret))
	if err != nil {
		return WebhookSecret{}, fmt.Errorf("encrypt webhook signing secret: %w", err)
	}
	now := s.now().UTC()
	created, err := s.repository.CreateWebhook(ctx, command.OrganizationID, domain.WebhookSubscription{
		ID: id, AppID: command.AppID, Event: event, EndpointURL: endpoint, Status: domain.WebhookStatusActive,
		SigningSecretFingerprint: secretFingerprint(secret), SigningSecretCiphertext: ciphertext,
		EncryptionKeyVersion: s.keyVersion, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}, ports.MutationMeta{ActorID: command.ActorID, Action: "webhook.created", IdempotencyKey: command.IdempotencyKey})
	if err != nil {
		return WebhookSecret{}, err
	}
	plaintext, err := s.cipher.Decrypt(created.SigningSecretCiphertext)
	if err != nil {
		return WebhookSecret{}, err
	}
	return WebhookSecret{Subscription: created, SigningSecret: string(plaintext)}, nil
}

func (s *WebhookService) availableEventDefinition(ctx context.Context, event string) (domain.WebhookEventDefinition, error) {
	definition, err := s.repository.GetWebhookEventDefinition(ctx, event)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.WebhookEventDefinition{}, fmt.Errorf("%w: event is not in the webhook event catalog", domain.ErrValidation)
	}
	if err != nil {
		return domain.WebhookEventDefinition{}, err
	}
	if definition.Availability != domain.WebhookEventAvailabilityAvailable {
		return domain.WebhookEventDefinition{}, fmt.Errorf("%w: %s is planned and cannot be subscribed yet", domain.ErrConflict, event)
	}
	return definition, nil
}

func (s *WebhookService) Update(ctx context.Context, command UpdateWebhookCommand) (domain.WebhookSubscription, error) {
	if command.Revision <= 0 {
		return domain.WebhookSubscription{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	if command.EndpointURL == nil && command.Status == nil {
		return domain.WebhookSubscription{}, fmt.Errorf("%w: endpointUrl or status is required", domain.ErrValidation)
	}
	subscription, err := s.repository.GetWebhook(ctx, command.OrganizationID, command.AppID, command.WebhookID)
	if err != nil {
		return domain.WebhookSubscription{}, err
	}
	if command.EndpointURL != nil {
		subscription.EndpointURL, err = validateWebhookEndpoint(*command.EndpointURL)
		if err != nil {
			return domain.WebhookSubscription{}, err
		}
	}
	if command.Status != nil {
		if *command.Status != domain.WebhookStatusActive && *command.Status != domain.WebhookStatusPaused {
			return domain.WebhookSubscription{}, fmt.Errorf("%w: status must be active or paused", domain.ErrValidation)
		}
		subscription.Status = *command.Status
	}
	return s.repository.UpdateWebhook(ctx, command.OrganizationID, subscription, command.Revision, ports.MutationMeta{ActorID: command.ActorID, Action: "webhook.updated"})
}

func (s *WebhookService) Disable(ctx context.Context, organizationID, actorID, appID, webhookID string) error {
	return s.repository.DisableWebhook(ctx, organizationID, appID, webhookID, ports.MutationMeta{ActorID: actorID, Action: "webhook.disabled"})
}

func validateWebhookEvent(value string) (string, error) {
	event := strings.TrimSpace(value)
	if !webhookEvent.MatchString(event) || len(event) > 120 {
		return "", fmt.Errorf("%w: event must use resource/action format", domain.ErrValidation)
	}
	return event, nil
}

func validateWebhookEndpoint(value string) (string, error) {
	endpoint := strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || len(endpoint) > 2048 {
		return "", fmt.Errorf("%w: endpointUrl must be a valid HTTPS URL without embedded credentials", domain.ErrValidation)
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && unsafeWebhookIP(ip) {
		return "", fmt.Errorf("%w: endpointUrl cannot target a private or local network", domain.ErrValidation)
	}
	return endpoint, nil
}
