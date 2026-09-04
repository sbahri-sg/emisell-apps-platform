package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type DeveloperProgramService struct {
	repository    ports.DeveloperProgramRepository
	id            ids.Generator
	now           func() time.Time
	invitationTTL time.Duration
}

type CreateDeveloperApplicationCommand struct {
	PlatformOrgID    string
	ActorID          string
	CompanyName      string
	CompanyDomain    string
	ContactName      string
	ContactEmail     string
	RequestedAppName string
	AppType          domain.DeveloperAppType
	UseCase          string
	RequestedScopes  []string
}

type ReviewDeveloperApplicationCommand struct {
	PlatformOrgID string
	ActorID       string
	ApplicationID string
	Revision      int64
	Notes         *string
}

type ApproveDeveloperApplicationCommand struct {
	PlatformOrgID string
	ActorID       string
	ApplicationID string
	Revision      int64
	MaxApps       int
	MaxWebhooks   int
}

type RejectDeveloperApplicationCommand struct {
	PlatformOrgID string
	ActorID       string
	ApplicationID string
	Revision      int64
	Notes         string
}

type RotateDeveloperInvitationCommand struct {
	PlatformOrgID string
	ActorID       string
	ApplicationID string
	Revision      int64
}

type ApprovalResult struct {
	Application     domain.DeveloperApplication `json:"application"`
	Invitation      domain.DeveloperInvitation  `json:"invitation"`
	InvitationToken string                      `json:"invitationToken"`
}

type InvitationAcceptance struct {
	Application domain.DeveloperApplication    `json:"application"`
	Entitlement domain.OrganizationEntitlement `json:"entitlement"`
}

var (
	companyDomainPattern  = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}\.)+[a-z0-9][a-z0-9-]{0,62}$`)
	requestedScopePattern = regexp.MustCompile(`^(read|write)_[a-z0-9_]{2,60}$`)
)

func NewDeveloperProgramService(repository ports.DeveloperProgramRepository, id ids.Generator, now func() time.Time, invitationTTL time.Duration) *DeveloperProgramService {
	return &DeveloperProgramService{repository: repository, id: id, now: now, invitationTTL: invitationTTL}
}

func (s *DeveloperProgramService) List(ctx context.Context, platformOrgID string, filter ports.DeveloperApplicationFilter) ([]domain.DeveloperApplication, ports.PageMeta, error) {
	if strings.TrimSpace(platformOrgID) == "" {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: platform organization is required", domain.ErrValidation)
	}
	if filter.Status != "" && !validDeveloperApplicationStatus(filter.Status) {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: invalid developer application status", domain.ErrValidation)
	}
	return s.repository.ListDeveloperApplications(ctx, platformOrgID, filter)
}

func (s *DeveloperProgramService) Get(ctx context.Context, platformOrgID, applicationID string) (domain.DeveloperApplication, error) {
	return s.repository.GetDeveloperApplication(ctx, platformOrgID, applicationID)
}

func (s *DeveloperProgramService) Create(ctx context.Context, command CreateDeveloperApplicationCommand) (domain.DeveloperApplication, error) {
	companyName := strings.TrimSpace(command.CompanyName)
	contactName := strings.TrimSpace(command.ContactName)
	requestedAppName := strings.TrimSpace(command.RequestedAppName)
	useCase := strings.TrimSpace(command.UseCase)
	if runeLengthOutside(companyName, 2, 120) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: company name must contain 2 to 120 characters", domain.ErrValidation)
	}
	companyDomain := strings.ToLower(strings.TrimSpace(command.CompanyDomain))
	if len(companyDomain) > 253 || !companyDomainPattern.MatchString(companyDomain) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: company domain must be a valid hostname", domain.ErrValidation)
	}
	if runeLengthOutside(contactName, 2, 120) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: contact name must contain 2 to 120 characters", domain.ErrValidation)
	}
	contactEmail, err := normalizeEmail(command.ContactEmail)
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	if runeLengthOutside(requestedAppName, 3, 80) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: app name must contain 3 to 80 characters", domain.ErrValidation)
	}
	if !validDeveloperAppType(command.AppType) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: invalid app type", domain.ErrValidation)
	}
	if runeLengthOutside(useCase, 20, 2000) {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: use case must contain 20 to 2000 characters", domain.ErrValidation)
	}
	scopes, err := normalizeRequestedScopes(command.RequestedScopes)
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	applicationID, err := s.id()
	if err != nil {
		return domain.DeveloperApplication{}, fmt.Errorf("generate developer application id: %w", err)
	}
	now := s.now().UTC()
	application := domain.DeveloperApplication{
		ID: applicationID, PlatformOrgID: command.PlatformOrgID, CompanyName: companyName,
		CompanyDomain: companyDomain, ContactName: contactName, ContactEmail: contactEmail,
		RequestedAppName: requestedAppName, AppType: command.AppType, UseCase: useCase,
		RequestedScopes: scopes, Status: domain.DeveloperApplicationStatusSubmitted,
		SubmittedBy: command.ActorID, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	return s.repository.CreateDeveloperApplication(ctx, application, ports.MutationMeta{ActorID: command.ActorID, Action: "developer_application.created"})
}

func (s *DeveloperProgramService) StartReview(ctx context.Context, command ReviewDeveloperApplicationCommand) (domain.DeveloperApplication, error) {
	if command.Revision < 1 {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	notes, err := normalizeReviewNotes(command.Notes, false)
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	return s.repository.StartDeveloperApplicationReview(ctx, command.PlatformOrgID, command.ApplicationID, command.ActorID, command.Revision, notes)
}

func (s *DeveloperProgramService) Approve(ctx context.Context, command ApproveDeveloperApplicationCommand) (ApprovalResult, error) {
	if command.Revision < 1 {
		return ApprovalResult{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	if command.MaxApps == 0 {
		command.MaxApps = 3
	}
	if command.MaxWebhooks == 0 {
		command.MaxWebhooks = 20
	}
	if command.MaxApps < 1 || command.MaxApps > 100 || command.MaxWebhooks < 1 || command.MaxWebhooks > 1000 {
		return ApprovalResult{}, fmt.Errorf("%w: entitlement limits are outside the allowed range", domain.ErrValidation)
	}
	application, err := s.repository.GetDeveloperApplication(ctx, command.PlatformOrgID, command.ApplicationID)
	if err != nil {
		return ApprovalResult{}, err
	}
	organizationID, err := s.id()
	if err != nil {
		return ApprovalResult{}, fmt.Errorf("generate organization id: %w", err)
	}
	invitationID, err := s.id()
	if err != nil {
		return ApprovalResult{}, fmt.Errorf("generate invitation id: %w", err)
	}
	token, tokenHash, err := newInvitationToken()
	if err != nil {
		return ApprovalResult{}, err
	}
	now := s.now().UTC()
	entitlement := domain.OrganizationEntitlement{
		OrganizationID: organizationID, SandboxAccess: true, ProductionAccess: false,
		MaxApps: command.MaxApps, MaxWebhooks: command.MaxWebhooks, CreatedAt: now, UpdatedAt: now,
	}
	invitation := domain.DeveloperInvitation{
		ID: invitationID, ApplicationID: application.ID, OrganizationID: organizationID,
		Email: application.ContactEmail, Role: domain.RoleOwner, TokenHash: tokenHash,
		Status: domain.DeveloperInvitationStatusPending, CreatedBy: command.ActorID,
		CreatedAt: now, ExpiresAt: now.Add(s.invitationTTL),
	}
	organizationSlugBase := slugify(application.CompanyName)
	if organizationSlugBase == "" {
		organizationSlugBase = "developer"
	}
	organizationSlug := organizationSlugBase + "-" + strings.ReplaceAll(application.ID[:8], "-", "")
	updated, createdInvitation, err := s.repository.ApproveDeveloperApplication(
		ctx, command.PlatformOrgID, application, organizationID, application.CompanyName, organizationSlug,
		entitlement, invitation, command.Revision,
		ports.MutationMeta{ActorID: command.ActorID, Action: "developer_application.approved"},
	)
	if err != nil {
		return ApprovalResult{}, err
	}
	return ApprovalResult{Application: updated, Invitation: createdInvitation, InvitationToken: token}, nil
}

func (s *DeveloperProgramService) Reject(ctx context.Context, command RejectDeveloperApplicationCommand) (domain.DeveloperApplication, error) {
	if command.Revision < 1 {
		return domain.DeveloperApplication{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	notes, err := normalizeReviewNotes(&command.Notes, true)
	if err != nil {
		return domain.DeveloperApplication{}, err
	}
	return s.repository.RejectDeveloperApplication(ctx, command.PlatformOrgID, command.ApplicationID, command.ActorID, command.Revision, *notes)
}

func (s *DeveloperProgramService) RotateInvitation(ctx context.Context, command RotateDeveloperInvitationCommand) (ApprovalResult, error) {
	if command.Revision < 1 {
		return ApprovalResult{}, fmt.Errorf("%w: revision is required", domain.ErrValidation)
	}
	application, err := s.repository.GetDeveloperApplication(ctx, command.PlatformOrgID, command.ApplicationID)
	if err != nil {
		return ApprovalResult{}, err
	}
	if application.OrganizationID == nil {
		return ApprovalResult{}, fmt.Errorf("%w: application has no approved organization", domain.ErrConflict)
	}
	invitationID, err := s.id()
	if err != nil {
		return ApprovalResult{}, fmt.Errorf("generate invitation id: %w", err)
	}
	token, tokenHash, err := newInvitationToken()
	if err != nil {
		return ApprovalResult{}, err
	}
	now := s.now().UTC()
	invitation := domain.DeveloperInvitation{
		ID: invitationID, ApplicationID: application.ID, OrganizationID: *application.OrganizationID,
		Email: application.ContactEmail, Role: domain.RoleOwner, TokenHash: tokenHash,
		Status: domain.DeveloperInvitationStatusPending, CreatedBy: command.ActorID,
		CreatedAt: now, ExpiresAt: now.Add(s.invitationTTL),
	}
	updated, createdInvitation, err := s.repository.RotateDeveloperInvitation(
		ctx, command.PlatformOrgID, command.ApplicationID, invitation, command.Revision,
		ports.MutationMeta{ActorID: command.ActorID, Action: "developer_invitation.rotated"},
	)
	if err != nil {
		return ApprovalResult{}, err
	}
	return ApprovalResult{Application: updated, Invitation: createdInvitation, InvitationToken: token}, nil
}

func (s *DeveloperProgramService) RevokeInvitation(ctx context.Context, platformOrgID, invitationID, actorID string) (domain.DeveloperInvitation, error) {
	return s.repository.RevokeDeveloperInvitation(ctx, platformOrgID, invitationID, actorID)
}

func (s *DeveloperProgramService) AcceptInvitation(ctx context.Context, token, actorID, actorEmail, displayName string) (InvitationAcceptance, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 200 || !strings.HasPrefix(token, "emi_inv_") {
		return InvitationAcceptance{}, fmt.Errorf("%w: invitation token is invalid", domain.ErrValidation)
	}
	email, err := normalizeEmail(actorEmail)
	if err != nil {
		return InvitationAcceptance{}, fmt.Errorf("%w: authenticated identity must include a valid email", domain.ErrUnauthorized)
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}
	if len([]rune(displayName)) > 120 {
		displayName = string([]rune(displayName)[:120])
	}
	application, entitlement, err := s.repository.AcceptDeveloperInvitation(ctx, invitationTokenHash(token), actorID, email, displayName, s.now().UTC())
	if err != nil {
		return InvitationAcceptance{}, err
	}
	return InvitationAcceptance{Application: application, Entitlement: entitlement}, nil
}

func newInvitationToken() (string, string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", fmt.Errorf("generate invitation token: %w", err)
	}
	token := "emi_inv_" + base64.RawURLEncoding.EncodeToString(random)
	return token, invitationTokenHash(token), nil
}

func invitationTokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized || len(normalized) > 254 {
		return "", fmt.Errorf("%w: contact email is invalid", domain.ErrValidation)
	}
	return normalized, nil
}

func normalizeRequestedScopes(values []string) ([]string, error) {
	if len(values) > 50 {
		return nil, fmt.Errorf("%w: no more than 50 scopes may be requested", domain.ErrValidation)
	}
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		scope := strings.ToLower(strings.TrimSpace(value))
		if !requestedScopePattern.MatchString(scope) {
			return nil, fmt.Errorf("%w: scope %q must use read_ or write_ naming", domain.ErrValidation, value)
		}
		unique[scope] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for scope := range unique {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result, nil
}

func normalizeReviewNotes(value *string, required bool) (*string, error) {
	if value == nil {
		if required {
			return nil, fmt.Errorf("%w: review notes are required", domain.ErrValidation)
		}
		return nil, nil
	}
	notes := strings.TrimSpace(*value)
	if required && len([]rune(notes)) < 5 {
		return nil, fmt.Errorf("%w: review notes must contain at least 5 characters", domain.ErrValidation)
	}
	if len([]rune(notes)) > 2000 {
		return nil, fmt.Errorf("%w: review notes may contain at most 2000 characters", domain.ErrValidation)
	}
	if notes == "" {
		return nil, nil
	}
	return &notes, nil
}

func validDeveloperAppType(value domain.DeveloperAppType) bool {
	return value == domain.DeveloperAppTypePayment || value == domain.DeveloperAppTypeShipping || value == domain.DeveloperAppTypeERP || value == domain.DeveloperAppTypeMarketing || value == domain.DeveloperAppTypeCustom
}

func validDeveloperApplicationStatus(value domain.DeveloperApplicationStatus) bool {
	return value == domain.DeveloperApplicationStatusSubmitted || value == domain.DeveloperApplicationStatusUnderReview || value == domain.DeveloperApplicationStatusApproved || value == domain.DeveloperApplicationStatusInvited || value == domain.DeveloperApplicationStatusActive || value == domain.DeveloperApplicationStatusRejected
}

func runeLengthOutside(value string, minimum, maximum int) bool {
	length := len([]rune(value))
	return length < minimum || length > maximum
}
