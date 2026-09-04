package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"emisell-app-platform/services/app-gateway/internal/security"
)

var ErrAdminLoginLimited = errors.New("admin login rate limited")

const AdminSessionTTL = 8 * time.Hour
const AdminSessionIdleTTL = 30 * time.Minute

type AdminLoginService struct {
	repository ports.AdminLoginRepository
	id         ids.Generator
	now        func() time.Time
	dummyHash  string
	hashing    chan struct{}
}

func NewAdminLoginService(repository ports.AdminLoginRepository, id ids.Generator, now func() time.Time) (*AdminLoginService, error) {
	dummy, err := randomIdentityToken()
	if err != nil {
		return nil, err
	}
	hash, err := security.HashPassword(dummy)
	if err != nil {
		return nil, err
	}
	return &AdminLoginService{repository: repository, id: id, now: now, dummyHash: hash, hashing: make(chan struct{}, 4)}, nil
}

func NormalizeAdminEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", fmt.Errorf("%w: a valid email is required", domain.ErrValidation)
	}
	return email, nil
}

func (s *AdminLoginService) Login(ctx context.Context, email, password, remoteIP string) (CreatedIdentitySession, error) {
	// Bound attacker-controlled work and use durable counters shared by replicas.
	select {
	case s.hashing <- struct{}{}:
		defer func() { <-s.hashing }()
	default:
		return CreatedIdentitySession{}, ErrAdminLoginLimited
	}
	now := s.now().UTC()
	email = strings.ToLower(strings.TrimSpace(email))
	for _, bucket := range []struct {
		key   string
		limit int
	}{{"ip:" + remoteIP, 30}, {"email:" + email, 5}} {
		allowed, err := s.repository.ConsumeAdminLoginAttempt(ctx, IdentityTokenDigest(bucket.key), bucket.limit, now)
		if err != nil {
			return CreatedIdentitySession{}, err
		}
		if !allowed {
			return CreatedIdentitySession{}, ErrAdminLoginLimited
		}
	}
	account, err := s.repository.FindAdminAccount(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return CreatedIdentitySession{}, err
	}
	hash := s.dummyHash
	if err == nil {
		hash = account.PasswordHash
	}
	valid := security.CheckPassword(hash, password)
	if !valid || err != nil || account.DisabledAt != nil {
		return CreatedIdentitySession{}, domain.ErrUnauthorized
	}
	token, err := randomIdentityToken()
	if err != nil {
		return CreatedIdentitySession{}, err
	}
	csrf, err := randomIdentityToken()
	if err != nil {
		return CreatedIdentitySession{}, err
	}
	id, err := s.id()
	if err != nil {
		return CreatedIdentitySession{}, err
	}
	session := domain.IdentitySession{ID: id, UserID: account.UserID, Email: account.Email, DisplayName: account.DisplayName,
		ActiveOrgID:      &account.OrganizationID,
		PlatformOperator: true, TokenHash: IdentityTokenDigest(token), CSRFTokenHash: IdentityTokenDigest(csrf),
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(AdminSessionTTL), IdleExpiresAt: now.Add(AdminSessionIdleTTL)}
	// The repository rechecks the password hash and enabled status under an
	// account lock, preventing a concurrent reset/disable from minting a session.
	if err := s.repository.CreateAdminSession(ctx, session, hash); err != nil {
		return CreatedIdentitySession{}, err
	}
	return CreatedIdentitySession{Session: session, SessionToken: token, CSRFToken: csrf}, nil
}

func (s *AdminLoginService) Authenticate(ctx context.Context, token string) (domain.IdentitySession, error) {
	if len(token) != 43 {
		return domain.IdentitySession{}, domain.ErrUnauthorized
	}
	return s.repository.GetAdminSession(ctx, IdentityTokenDigest(token), s.now().UTC(), AdminSessionIdleTTL)
}

func (s *AdminLoginService) Logout(ctx context.Context, token string) error {
	return s.repository.RevokeAdminSession(ctx, IdentityTokenDigest(token), s.now().UTC())
}
