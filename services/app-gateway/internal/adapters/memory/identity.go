package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) ListOrganizationMemberships(_ context.Context, userID string) ([]domain.OrganizationMembership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]domain.OrganizationMembership(nil), r.identityMemberships[userID]...), nil
}

func (r *Repository) CreateIdentitySession(_ context.Context, session domain.IdentitySession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.identitySessions[session.TokenHash]; exists {
		return fmt.Errorf("%w: session token already exists", domain.ErrConflict)
	}
	r.identitySessions[session.TokenHash] = session
	return nil
}

func (r *Repository) GetIdentitySessionByTokenHash(_ context.Context, tokenHash string, now time.Time, idleTTL time.Duration) (domain.IdentitySession, *domain.OrganizationMembership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, exists := r.identitySessions[tokenHash]
	if !exists || session.RevokedAt != nil || !now.Before(session.ExpiresAt) || !now.Before(session.IdleExpiresAt) {
		return domain.IdentitySession{}, nil, domain.ErrUnauthorized
	}
	var membership *domain.OrganizationMembership
	if session.ActiveOrgID != nil {
		for _, candidate := range r.identityMemberships[session.UserID] {
			if candidate.OrganizationID == *session.ActiveOrgID && candidate.Status == "active" {
				copy := candidate
				membership = &copy
				break
			}
		}
		if membership == nil {
			session.ActiveOrgID = nil
		}
	}
	session.LastSeenAt = now
	session.IdleExpiresAt = now.Add(idleTTL)
	if session.IdleExpiresAt.After(session.ExpiresAt) {
		session.IdleExpiresAt = session.ExpiresAt
	}
	r.identitySessions[tokenHash] = session
	return session, membership, nil
}

func (r *Repository) SwitchIdentitySessionOrganization(_ context.Context, sessionID, userID, organizationID string, now time.Time) (domain.IdentitySession, domain.OrganizationMembership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var membership domain.OrganizationMembership
	for _, candidate := range r.identityMemberships[userID] {
		if candidate.OrganizationID == organizationID && candidate.Status == "active" {
			membership = candidate
			break
		}
	}
	if membership.OrganizationID == "" {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, domain.ErrForbidden
	}
	for hash, session := range r.identitySessions {
		if session.ID != sessionID || session.UserID != userID || session.RevokedAt != nil || !now.Before(session.ExpiresAt) || !now.Before(session.IdleExpiresAt) {
			continue
		}
		session.ActiveOrgID = &organizationID
		session.LastSeenAt = now
		r.identitySessions[hash] = session
		return session, membership, nil
	}
	return domain.IdentitySession{}, domain.OrganizationMembership{}, domain.ErrUnauthorized
}

func (r *Repository) RevokeIdentitySession(_ context.Context, tokenHash string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, exists := r.identitySessions[tokenHash]; exists {
		session.RevokedAt = &now
		r.identitySessions[tokenHash] = session
	}
	return nil
}

func (r *Repository) CreateOIDCLoginState(_ context.Context, state domain.OIDCLoginState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.oidcLoginStates[state.StateHash] = state
	return nil
}

func (r *Repository) ConsumeOIDCLoginState(_ context.Context, stateHash string, now time.Time) (domain.OIDCLoginState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, exists := r.oidcLoginStates[stateHash]
	if !exists || state.ConsumedAt != nil || !now.Before(state.ExpiresAt) {
		return domain.OIDCLoginState{}, domain.ErrInvalidGrant
	}
	state.ConsumedAt = &now
	r.oidcLoginStates[stateHash] = state
	return state, nil
}

func (r *Repository) ProvisionOIDCUser(_ context.Context, provider, subject, email, _ string, userID string, _ time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := provider + ":" + subject
	if existing := r.oidcUsers[key]; existing != "" {
		return existing, nil
	}
	for _, session := range r.identitySessions {
		if strings.EqualFold(session.Email, email) {
			r.oidcUsers[key] = session.UserID
			return session.UserID, nil
		}
	}
	r.oidcUsers[key] = userID
	return userID, nil
}

var _ ports.IdentityRepository = (*Repository)(nil)

func (r *Repository) SeedIdentityMemberships(userID string, memberships ...domain.OrganizationMembership) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.identityMemberships[userID] = append([]domain.OrganizationMembership(nil), memberships...)
}
