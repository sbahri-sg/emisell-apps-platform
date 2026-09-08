package identity

import (
	"context"
	"strings"

	"emisell.app/platform/internal/platform/fault"
)

// Optional repository extension so legacy account implementations cannot gain
// merchant provisioning by default. Core validates the seller in its own DB.
type MerchantReferenceRepository interface {
	EnsureMerchantReference(ctx context.Context, serviceID, merchantID, actorID string) (bool, error)
}

func (s ServiceAccounts) EnsureMerchant(ctx context.Context, principal ServicePrincipal, merchantID, actorID string) (bool, error) {
	if !principal.PlatformFull || !strings.HasPrefix(principal.ID, "platformkey_") {
		return false, fault.Forbidden
	}
	if !keyIdentifier.MatchString(principal.ID) || !keyIdentifier.MatchString(merchantID) || !keyIdentifier.MatchString(actorID) {
		return false, fault.Invalid
	}
	repo, ok := s.Repo.(MerchantReferenceRepository)
	if !ok {
		return false, fault.Unavailable
	}
	return repo.EnsureMerchantReference(ctx, principal.ID, merchantID, actorID)
}
