package application

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type InstallationService struct {
	repository ports.Repository
	id         ids.Generator
	now        func() time.Time
}

type CreateInstallationCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	AppID          string
	MerchantID     string
	MerchantName   string
	MerchantDomain *string
	Environment    domain.Environment
	GrantedScopes  []string
}

type UpdateInstallationCommand struct {
	OrganizationID string
	ActorID        string
	AppID          string
	InstallationID string
	Status         domain.InstallationStatus
	Revision       int64
}

type UpgradeInstallationCommand struct {
	OrganizationID             string
	ActorID                    string
	IdempotencyKey             string
	AppID                      string
	InstallationID             string
	TargetVersionID            string
	ExpectedInstalledVersionID string
	Revision                   int64
}

func NewInstallationService(repository ports.Repository, id ids.Generator, now func() time.Time) *InstallationService {
	return &InstallationService{repository: repository, id: id, now: now}
}

func (s *InstallationService) List(ctx context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.AppInstallation, ports.PageMeta, error) {
	return s.repository.ListInstallations(ctx, organizationID, appID, filter)
}

func (s *InstallationService) Get(ctx context.Context, organizationID, appID, installationID string) (domain.AppInstallation, error) {
	return s.repository.GetInstallation(ctx, organizationID, appID, installationID)
}

var uuidValue = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
var externalIdentityValue = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var merchantDomain = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)

func (s *InstallationService) Create(ctx context.Context, command CreateInstallationCommand) (domain.AppInstallation, error) {
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if app.Status != domain.AppStatusActive || app.ActiveVersionID == nil {
		return domain.AppInstallation{}, fmt.Errorf("%w: app must have an active version before installation", domain.ErrConflict)
	}
	merchantID := strings.TrimSpace(command.MerchantID)
	if !externalIdentityValue.MatchString(merchantID) {
		return domain.AppInstallation{}, fmt.Errorf("%w: merchantId must be a valid Emisell identifier", domain.ErrValidation)
	}
	name := strings.TrimSpace(command.MerchantName)
	if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		return domain.AppInstallation{}, fmt.Errorf("%w: merchantName must contain 2 to 120 characters", domain.ErrValidation)
	}
	var domainName *string
	if command.MerchantDomain != nil && strings.TrimSpace(*command.MerchantDomain) != "" {
		value := strings.ToLower(strings.TrimSpace(*command.MerchantDomain))
		if !merchantDomain.MatchString(value) {
			return domain.AppInstallation{}, fmt.Errorf("%w: merchantDomain must be a hostname without protocol", domain.ErrValidation)
		}
		domainName = &value
	}
	if command.Environment != domain.EnvironmentSandbox && command.Environment != domain.EnvironmentProduction {
		return domain.AppInstallation{}, fmt.Errorf("%w: environment must be sandbox or production", domain.ErrValidation)
	}
	if err := requireEnvironmentAccess(ctx, s.repository, command.OrganizationID, command.Environment); err != nil {
		return domain.AppInstallation{}, err
	}
	activeVersion, err := s.repository.GetVersion(ctx, command.OrganizationID, command.AppID, *app.ActiveVersionID)
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("read active version: %w", err)
	}
	grantedScopes, err := validateInstallationScopes(activeVersion.Snapshot.Scopes, command.GrantedScopes)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	id, err := s.id()
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("generate installation id: %w", err)
	}
	now := s.now().UTC()
	installation := domain.AppInstallation{
		ID: id, AppID: command.AppID, MerchantID: merchantID, MerchantName: name,
		MerchantDomain: domainName, Environment: command.Environment, Status: domain.InstallationStatusActive,
		InstalledVersionID: activeVersion.ID, GrantedScopes: grantedScopes, InstalledBy: command.ActorID,
		InstalledAt: now, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	return s.repository.CreateInstallation(ctx, command.OrganizationID, installation, activeVersion.ID, ports.MutationMeta{
		ActorID: command.ActorID, Action: "installation.created", IdempotencyKey: command.IdempotencyKey,
	})
}

func (s *InstallationService) Update(ctx context.Context, command UpdateInstallationCommand) (domain.AppInstallation, error) {
	if command.Revision <= 0 {
		return domain.AppInstallation{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	if command.Status != domain.InstallationStatusActive && command.Status != domain.InstallationStatusSuspended {
		return domain.AppInstallation{}, fmt.Errorf("%w: status must be active or suspended", domain.ErrValidation)
	}
	installation, err := s.repository.GetInstallation(ctx, command.OrganizationID, command.AppID, command.InstallationID)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if installation.Status == domain.InstallationStatusUninstalled {
		return domain.AppInstallation{}, fmt.Errorf("%w: uninstalled records cannot be reactivated", domain.ErrConflict)
	}
	installation.Status = command.Status
	return s.repository.UpdateInstallation(ctx, command.OrganizationID, installation, command.Revision, ports.MutationMeta{
		ActorID: command.ActorID, Action: "installation.updated",
	})
}

func (s *InstallationService) Upgrade(ctx context.Context, command UpgradeInstallationCommand) (domain.AppInstallation, error) {
	if command.Revision <= 0 {
		return domain.AppInstallation{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	if !uuidValue.MatchString(command.TargetVersionID) || !uuidValue.MatchString(command.ExpectedInstalledVersionID) {
		return domain.AppInstallation{}, fmt.Errorf("%w: targetVersionId and expectedInstalledVersionId must be UUIDs", domain.ErrValidation)
	}
	installation, err := s.repository.GetInstallation(ctx, command.OrganizationID, command.AppID, command.InstallationID)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if installation.Status == domain.InstallationStatusUninstalled {
		return domain.AppInstallation{}, fmt.Errorf("%w: uninstalled records cannot be upgraded", domain.ErrConflict)
	}
	if err := requireEnvironmentAccess(ctx, s.repository, command.OrganizationID, installation.Environment); err != nil {
		return domain.AppInstallation{}, err
	}
	targetVersion, err := s.repository.GetVersion(ctx, command.OrganizationID, command.AppID, command.TargetVersionID)
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("read target version: %w", err)
	}

	// Required scopes are approved as part of the manual upgrade. Existing optional
	// scopes are preserved only while they still exist; new optional scopes require
	// a separate future consent flow and are deliberately not auto-granted.
	targetScopes := make(map[string]domain.ScopeAccess, len(targetVersion.Snapshot.Scopes))
	requested := make([]string, 0, len(targetVersion.Snapshot.Scopes))
	for _, scope := range targetVersion.Snapshot.Scopes {
		access := domain.ScopeAccess(scope.Access)
		targetScopes[scope.Scope] = access
		if access == domain.ScopeAccessRequired {
			requested = append(requested, scope.Scope)
		}
	}
	for _, scope := range installation.GrantedScopes {
		if targetScopes[scope] == domain.ScopeAccessOptional {
			requested = append(requested, scope)
		}
	}
	grantedScopes, err := validateInstallationScopes(targetVersion.Snapshot.Scopes, requested)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	return s.repository.UpgradeInstallation(
		ctx, command.OrganizationID, command.AppID, command.InstallationID,
		command.TargetVersionID, command.ExpectedInstalledVersionID, grantedScopes,
		command.Revision, ports.MutationMeta{
			ActorID: command.ActorID, Action: "installation.upgraded", IdempotencyKey: command.IdempotencyKey,
		},
	)
}

func (s *InstallationService) Uninstall(ctx context.Context, organizationID, actorID, appID, installationID, idempotencyKey string) error {
	return s.repository.UninstallInstallation(ctx, organizationID, appID, installationID, ports.MutationMeta{
		ActorID: actorID, Action: "installation.uninstalled", IdempotencyKey: idempotencyKey,
	})
}

func validateInstallationScopes(snapshot []domain.SnapshotScope, requested []string) ([]string, error) {
	defined := make(map[string]domain.ScopeAccess, len(snapshot))
	for _, scope := range snapshot {
		defined[scope.Scope] = domain.ScopeAccess(scope.Access)
	}
	if requested == nil {
		requested = make([]string, 0, len(snapshot))
		for _, scope := range snapshot {
			requested = append(requested, scope.Scope)
		}
	}
	seen := make(map[string]struct{}, len(requested))
	result := make([]string, 0, len(requested))
	for _, raw := range requested {
		scope := strings.TrimSpace(raw)
		if _, ok := defined[scope]; !ok {
			return nil, fmt.Errorf("%w: scope %q is not part of the active version", domain.ErrValidation, scope)
		}
		if _, duplicate := seen[scope]; duplicate {
			return nil, fmt.Errorf("%w: duplicate granted scope %q", domain.ErrValidation, scope)
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	for scope, access := range defined {
		if access == domain.ScopeAccessRequired {
			if _, ok := seen[scope]; !ok {
				return nil, fmt.Errorf("%w: required scope %q must be granted", domain.ErrValidation, scope)
			}
		}
	}
	sort.Strings(result)
	return result, nil
}
