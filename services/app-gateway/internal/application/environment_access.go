package application

import (
	"context"
	"errors"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func requireEnvironmentAccess(ctx context.Context, repository ports.Repository, organizationID string, environment domain.Environment) error {
	entitlement, err := repository.GetOrganizationEntitlement(ctx, organizationID)
	if errors.Is(err, domain.ErrNotFound) {
		if environment == domain.EnvironmentSandbox {
			// Keep the built-in development workspace usable before it joins the
			// invite-only developer program. Production is never implicit.
			return nil
		}
		return fmt.Errorf("%w: production access is not enabled for this organization", domain.ErrForbidden)
	}
	if err != nil {
		return err
	}
	if environment == domain.EnvironmentSandbox && !entitlement.SandboxAccess {
		return fmt.Errorf("%w: sandbox access is not enabled for this organization", domain.ErrForbidden)
	}
	if environment == domain.EnvironmentProduction && !entitlement.ProductionAccess {
		return fmt.Errorf("%w: production access is not enabled for this organization", domain.ErrForbidden)
	}
	return nil
}
