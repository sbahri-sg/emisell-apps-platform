package application

import (
	"context"
	"fmt"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type CredentialService struct {
	repository ports.Repository
	cipher     SecretCipher
	keyVersion int
	id         ids.Generator
	now        func() time.Time
}

type CreateCredentialCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	AppID          string
	Environment    domain.Environment
	ExpiresAt      *time.Time
}

type CredentialSecret struct {
	Credential   domain.AppCredential `json:"credential"`
	ClientSecret string               `json:"clientSecret"`
}

func NewCredentialService(repository ports.Repository, cipher SecretCipher, keyVersion int, id ids.Generator, now func() time.Time) *CredentialService {
	return &CredentialService{repository: repository, cipher: cipher, keyVersion: keyVersion, id: id, now: now}
}

func (s *CredentialService) List(ctx context.Context, organizationID, appID string) ([]domain.AppCredential, error) {
	credentials, err := s.repository.ListCredentials(ctx, organizationID, appID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	for index := range credentials {
		if credentials[index].Status == domain.CredentialStatusActive && credentials[index].ExpiresAt != nil && !credentials[index].ExpiresAt.After(now) {
			credentials[index].Status = domain.CredentialStatusExpired
		}
	}
	return credentials, nil
}

func (s *CredentialService) Create(ctx context.Context, command CreateCredentialCommand) (CredentialSecret, error) {
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return CredentialSecret{}, err
	}
	if app.Status == domain.AppStatusArchived {
		return CredentialSecret{}, fmt.Errorf("%w: archived apps cannot create credentials", domain.ErrConflict)
	}
	if command.Environment != domain.EnvironmentSandbox && command.Environment != domain.EnvironmentProduction {
		return CredentialSecret{}, fmt.Errorf("%w: environment must be sandbox or production", domain.ErrValidation)
	}
	if err := requireEnvironmentAccess(ctx, s.repository, command.OrganizationID, command.Environment); err != nil {
		return CredentialSecret{}, err
	}
	now := s.now().UTC()
	if command.ExpiresAt != nil {
		expiresAt := command.ExpiresAt.UTC()
		if !expiresAt.After(now) {
			return CredentialSecret{}, fmt.Errorf("%w: expiresAt must be in the future", domain.ErrValidation)
		}
		command.ExpiresAt = &expiresAt
	}
	id, err := s.id()
	if err != nil {
		return CredentialSecret{}, fmt.Errorf("generate credential id: %w", err)
	}
	prefix := "test"
	if command.Environment == domain.EnvironmentProduction {
		prefix = "live"
	}
	clientID, err := randomToken("em_"+prefix+"_", 12)
	if err != nil {
		return CredentialSecret{}, err
	}
	secret, err := randomToken("es_"+prefix+"_", 32)
	if err != nil {
		return CredentialSecret{}, err
	}
	ciphertext, err := s.cipher.Encrypt([]byte(secret))
	if err != nil {
		return CredentialSecret{}, fmt.Errorf("encrypt credential secret: %w", err)
	}
	created, err := s.repository.CreateCredential(ctx, command.OrganizationID, domain.AppCredential{
		ID: id, AppID: command.AppID, Environment: command.Environment, ClientID: clientID,
		SecretFingerprint: secretFingerprint(secret), Status: domain.CredentialStatusActive,
		ExpiresAt: command.ExpiresAt, CreatedBy: command.ActorID, CreatedAt: now,
		SecretCiphertext: ciphertext, EncryptionKeyVersion: s.keyVersion,
	}, ports.MutationMeta{ActorID: command.ActorID, Action: "credential.created", IdempotencyKey: command.IdempotencyKey})
	if err != nil {
		return CredentialSecret{}, err
	}
	return s.reveal(created)
}

func (s *CredentialService) Rotate(ctx context.Context, organizationID, actorID, appID, credentialID, idempotencyKey string) (CredentialSecret, error) {
	app, err := s.repository.GetApp(ctx, organizationID, appID)
	if err != nil {
		return CredentialSecret{}, err
	}
	if app.Status == domain.AppStatusArchived {
		return CredentialSecret{}, fmt.Errorf("%w: archived apps cannot rotate credentials", domain.ErrConflict)
	}
	credentials, err := s.repository.ListCredentials(ctx, organizationID, appID)
	if err != nil {
		return CredentialSecret{}, err
	}
	var target *domain.AppCredential
	for index := range credentials {
		if credentials[index].ID == credentialID {
			target = &credentials[index]
			break
		}
	}
	if target == nil {
		return CredentialSecret{}, domain.ErrNotFound
	}
	if err := requireEnvironmentAccess(ctx, s.repository, organizationID, target.Environment); err != nil {
		return CredentialSecret{}, err
	}
	secret, err := randomToken("es_rotate_", 32)
	if err != nil {
		return CredentialSecret{}, err
	}
	ciphertext, err := s.cipher.Encrypt([]byte(secret))
	if err != nil {
		return CredentialSecret{}, fmt.Errorf("encrypt credential secret: %w", err)
	}
	credential, err := s.repository.RotateCredential(ctx, organizationID, appID, credentialID, ciphertext, secretFingerprint(secret), s.keyVersion, ports.MutationMeta{
		ActorID: actorID, Action: "credential.rotated", IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return CredentialSecret{}, err
	}
	return s.reveal(credential)
}

func (s *CredentialService) Revoke(ctx context.Context, organizationID, actorID, appID, credentialID, idempotencyKey string) error {
	return s.repository.RevokeCredential(ctx, organizationID, appID, credentialID, ports.MutationMeta{
		ActorID: actorID, Action: "credential.revoked", IdempotencyKey: idempotencyKey,
	})
}

func (s *CredentialService) reveal(credential domain.AppCredential) (CredentialSecret, error) {
	plaintext, err := s.cipher.Decrypt(credential.SecretCiphertext)
	if err != nil {
		return CredentialSecret{}, err
	}
	return CredentialSecret{Credential: credential, ClientSecret: string(plaintext)}, nil
}
