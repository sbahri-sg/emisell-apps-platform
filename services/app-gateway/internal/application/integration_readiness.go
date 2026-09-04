package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

// IntegrationReadiness is a read-only configuration inspection, not a network
// probe or a publication/production authorization. Errors never become passes.
func (s *CatalogService) IntegrationReadiness(ctx context.Context, organizationID, appID string) (domain.IntegrationReadiness, error) {
	organizationID, appID = strings.ToLower(strings.TrimSpace(organizationID)), strings.ToLower(strings.TrimSpace(appID))
	if !uuidValue.MatchString(organizationID) || !uuidValue.MatchString(appID) {
		return domain.IntegrationReadiness{}, fmt.Errorf("%w: organizationId and appId must be UUIDs", domain.ErrValidation)
	}
	app, err := s.apps.GetApp(ctx, organizationID, appID)
	if err != nil {
		return domain.IntegrationReadiness{}, err
	}
	report := domain.IntegrationReadiness{
		AppID: app.ID, OrganizationID: app.OrganizationID, AppName: app.Name,
		AppRevision: app.Revision, ActiveVersionID: app.ActiveVersionID,
		CheckedAt: s.now().UTC(), Checks: []domain.IntegrationCheck{}, Scopes: []domain.IntegrationScope{},
		ListingStatus: domain.CatalogListingStatusDraft,
	}
	add := func(code, title, status, detail, section string) {
		report.Checks = append(report.Checks, domain.IntegrationCheck{Code: code, Title: title, Status: status, Detail: detail, Section: section})
	}
	if organizations, ok := s.catalog.(ports.DeveloperOrganizationRepository); ok {
		organization, err := organizations.GetDeveloperOrganization(ctx, organizationID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return domain.IntegrationReadiness{}, err
		}
		switch {
		case errors.Is(err, domain.ErrNotFound):
			add("organization", "Developer admission", "attention", "No admitted developer organization record was found. A built-in development workspace is not production admission.", "home")
		case organization.Status != domain.OrganizationStatusActive:
			add("organization", "Developer admission", "blocked", "The developer organization is suspended. App configuration does not override organization access.", "home")
		default:
			add("organization", "Developer admission", "pass", "The admitted developer organization is active. Production access is checked separately.", "home")
		}
	}
	if app.Status == domain.AppStatusActive {
		add("app_status", "App status", "pass", "The app is active. This does not grant production access or publish a listing.", "settings")
	} else {
		add("app_status", "App status", "blocked", "The app must be active before installation.", "settings")
	}
	if catalogLaunchURLAllowed(app.AppURL) {
		add("launch_url", "Launch URL", "pass", "An HTTPS launch URL is configured. Reachability has not been tested.", "settings")
	} else if app.AppURL != nil && validateAppURL(app.AppURL, true) == nil {
		add("launch_url", "Launch URL", "attention", "The local development URL is not eligible for catalog publication.", "settings")
	} else {
		add("launch_url", "Launch URL", "blocked", "Configure a valid launch URL before testing an installation.", "settings")
	}
	var version *domain.AppVersion
	if app.ActiveVersionID != nil {
		loaded, err := s.apps.GetVersion(ctx, organizationID, appID, *app.ActiveVersionID)
		if err != nil {
			return domain.IntegrationReadiness{}, err
		}
		version = &loaded
		report.ActiveVersion = &loaded.Version
	}
	if version == nil || version.Status != domain.VersionStatusActive {
		add("active_version", "Active version", "blocked", "Create and activate an immutable version before testing.", "versions")
	} else {
		add("active_version", "Active version", "pass", "An active configuration snapshot exists. Activation is separate from catalog publication.", "versions")
		if len(version.Snapshot.RedirectURLs) == 0 {
			add("redirect_urls", "OAuth callback allowlist", "blocked", "The active version has no callback URL. Update configuration and activate a new version.", "versions")
		} else {
			add("redirect_urls", "OAuth callback allowlist", "pass", fmt.Sprintf("%d callback URL(s) in the active snapshot. The provider must use an exact allowed URL and PKCE.", len(version.Snapshot.RedirectURLs)), "versions")
		}
		current, err := (CurrentConfigurationSnapshotBuilder{Repository: s.apps}).Build(ctx, app)
		if err != nil {
			return domain.IntegrationReadiness{}, err
		}
		currentHash, err := readinessSnapshotHash(current)
		if err != nil {
			return domain.IntegrationReadiness{}, err
		}
		activeHash, err := readinessSnapshotHash(version.Snapshot)
		if err != nil {
			return domain.IntegrationReadiness{}, err
		}
		if currentHash == activeHash {
			add("draft_changes", "Unreleased configuration", "pass", "Current extensions, scopes, webhooks and callbacks match the active snapshot.", "versions")
		} else {
			add("draft_changes", "Unreleased configuration", "attention", "Configuration differs from the active snapshot. Review the version diff and activate a new version when intended.", "versions")
		}
		unavailable := 0
		for _, scope := range version.Snapshot.Scopes {
			definition, known := LookupOfficialScope(scope.Scope)
			item := domain.IntegrationScope{Scope: scope.Scope, Access: scope.Access, Availability: domain.ScopeAvailabilityPlanned, Endpoints: []domain.ScopeEndpoint{}}
			if known {
				item.Availability = definition.Availability
				item.Endpoints = append(item.Endpoints, definition.Endpoints...)
			}
			if item.Availability != domain.ScopeAvailabilityAvailable {
				unavailable++
			}
			report.Scopes = append(report.Scopes, item)
		}
		sort.Slice(report.Scopes, func(i, j int) bool { return report.Scopes[i].Scope < report.Scopes[j].Scope })
		if unavailable > 0 {
			add("scopes", "Provider resource scopes", "attention", fmt.Sprintf("%d scope(s) are not generally available and block catalog publication. An operator-enabled development pilot is not general availability.", unavailable), "api-access")
		} else {
			add("scopes", "Provider resource scopes", "pass", "The active snapshot requests only generally available scopes, or no resource scopes. Merchant consent is still required.", "api-access")
		}
		add("extensions", "Extensions", "info", fmt.Sprintf("%d extension(s) in the active snapshot. Configuration does not prove a runtime is deployed; apps without extensions are allowed.", len(version.Snapshot.Extensions)), "extensions")
		add("webhooks", "Webhook subscriptions", "info", fmt.Sprintf("%d subscription(s) in the active snapshot. Inspect delivery history and verify signatures in the provider; configuration alone is not delivery evidence.", len(version.Snapshot.WebhookSubscriptions)), "webhooks")
	}
	credentials, err := s.apps.ListCredentials(ctx, organizationID, appID)
	if err != nil {
		return domain.IntegrationReadiness{}, err
	}
	for _, environment := range []domain.Environment{domain.EnvironmentSandbox, domain.EnvironmentProduction} {
		accessErr := requireEnvironmentAccess(ctx, s.apps, organizationID, environment)
		if accessErr != nil && !errors.Is(accessErr, domain.ErrForbidden) {
			return domain.IntegrationReadiness{}, accessErr
		}
		usable := 0
		for _, credential := range credentials {
			if credential.Environment == environment && credential.Status == domain.CredentialStatusActive && credential.RevokedAt == nil && (credential.ExpiresAt == nil || credential.ExpiresAt.After(report.CheckedAt)) {
				usable++
			}
		}
		code, title := "development_credentials", "Development credentials"
		if environment == domain.EnvironmentProduction {
			code, title = "production_credentials", "Production access & credentials"
		}
		switch {
		case accessErr != nil:
			add(code, title, "attention", "Organization access is not enabled. Creating credentials or publishing an app does not grant access.", "api-access")
		case usable == 0:
			add(code, title, "attention", "No active, unexpired credential is available for this access context.", "api-access")
		default:
			add(code, title, "pass", fmt.Sprintf("%d active, unexpired credential(s). No secrets are included and token exchange has not been verified by this check.", usable), "api-access")
		}
	}
	installations, meta, err := s.apps.ListInstallations(ctx, organizationID, appID, ports.AppFilter{Limit: 100})
	if err != nil {
		return domain.IntegrationReadiness{}, err
	}
	report.Installations.Sampled, report.Installations.HasMore = len(installations), meta.HasMore
	for _, installation := range installations {
		if installation.Status != domain.InstallationStatusActive {
			report.Installations.Inactive++
		} else if app.ActiveVersionID != nil && installation.InstalledVersionID == *app.ActiveVersionID {
			report.Installations.ActiveCurrentVersion++
		} else {
			report.Installations.ActiveOtherVersion++
		}
	}
	if report.Installations.ActiveCurrentVersion > 0 {
		add("installation_evidence", "Installation evidence", "pass", "An active installation uses the current version. This does not prove successful resource calls, runtime behavior or uninstall handling.", "home")
	} else {
		add("installation_evidence", "Installation evidence", "attention", "No active current-version installation was found in the inspected records. A pending test invitation is not a completed installation.", "home")
	}
	if meta.HasMore {
		add("installation_sample", "Installation sample", "info", "Only the first 100 installations were inspected; counts are a sample, not totals.", "home")
	}
	listing, err := s.catalog.GetCatalogListing(ctx, organizationID, appID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.IntegrationReadiness{}, err
	}
	if err == nil {
		report.ListingStatus, report.ListingRevision = listing.Status, listing.Revision
	}
	add("end_to_end", "Emisell end-to-end verification", "attention", "Not verified by this inspection. Test merchant consent, PKCE exchange, permitted resource access, denied cross-merchant access and revoked access after uninstall in an isolated environment.", "home")
	// The report is assembled from several reads. Do not return a misleading
	// active-version identity if the app changed during inspection.
	latest, err := s.apps.GetApp(ctx, organizationID, appID)
	if err != nil {
		return domain.IntegrationReadiness{}, err
	}
	if latest.Revision != app.Revision || !sameOptionalID(latest.ActiveVersionID, app.ActiveVersionID) {
		return domain.IntegrationReadiness{}, fmt.Errorf("%w: app changed during inspection; refresh the report", domain.ErrConflict)
	}
	return report, nil
}

func readinessSnapshotHash(snapshot domain.VersionSnapshot) (string, error) {
	// Repository ordering is not part of the configuration's meaning. Copy the
	// slices before sorting, since memory adapters may share their backing data.
	snapshot.Extensions = append([]domain.SnapshotExtension{}, snapshot.Extensions...)
	snapshot.Scopes = append([]domain.SnapshotScope{}, snapshot.Scopes...)
	snapshot.WebhookSubscriptions = append([]domain.SnapshotWebhook{}, snapshot.WebhookSubscriptions...)
	snapshot.RedirectURLs = append([]string{}, snapshot.RedirectURLs...)
	sort.Slice(snapshot.Extensions, func(i, j int) bool { return snapshot.Extensions[i].ExtensionID < snapshot.Extensions[j].ExtensionID })
	sort.Slice(snapshot.Scopes, func(i, j int) bool { return snapshot.Scopes[i].Scope < snapshot.Scopes[j].Scope })
	sort.Slice(snapshot.WebhookSubscriptions, func(i, j int) bool {
		return snapshot.WebhookSubscriptions[i].SubscriptionID < snapshot.WebhookSubscriptions[j].SubscriptionID
	})
	sort.Strings(snapshot.RedirectURLs)
	return configurationHash(snapshot)
}

func sameOptionalID(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
