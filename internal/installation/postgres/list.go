package postgres

import (
	"context"
	"emisell.app/platform/pkg/uiresource"
	"slices"

	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
)

// One statement gives a consistent current installation/grant snapshot. Joining
// the current ID excludes retired consumption receipts after uninstall/reinstall.
// Core has already authorized the current merchant app manager; list metadata is
// shared across installers and Core keys, unlike the owner-bound mutation paths.
func (p Repository) ListInstalled(ctx context.Context, merchant, after string, limit int) ([]domain.InstalledApp, error) {
	rows, err := p.Pool.Query(ctx, `SELECT i.id,i.app_id,i.version,i.status,i.installed_at,
 c.release,g.state,g.scopes,i.scopes,i.capabilities
 FROM platform_installation.installations i
 JOIN platform_installation.intent_consumptions c ON c.tenant_id=i.tenant_id AND c.installation_id=i.id AND c.intent_id=i.intent_id
 JOIN platform_installation.access_grants g ON g.tenant_id=i.tenant_id AND g.installation_id=i.id
 WHERE i.tenant_id=$1 AND i.status!='uninstalled' AND i.id>$2
 ORDER BY i.id LIMIT $3`, merchant, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.InstalledApp{}
	for rows.Next() {
		var v domain.InstalledApp
		var release domain.IntentRelease
		var granted, scopes, caps []string
		if err := rows.Scan(&v.ID, &v.AppID, &v.Version, &v.Status, &v.InstalledAt, &release, &v.GrantState, &granted, &scopes, &caps); err != nil {
			return nil, err
		}
		pilot := release.InstallPolicy == domain.EmbeddedPilotPolicy && release.AppID == domain.EmbeddedPilotApp && release.Version == "1.0.0" && release.ExecutionProfile == domain.EmbeddedPilotPolicy && len(release.Scopes) == 0 && len(release.Capabilities) == 0
		ui := release.InstallPolicy == domain.ReviewedUIPolicy && release.ExecutionProfile == domain.ReviewedUIPolicy && release.UIBinding != nil && len(release.Scopes) == 0 && len(release.Capabilities) == 0
		// Listing is metadata, not fresh launch or resource authorization. Retain
		// the immutable resource identity and scope checks without requiring a
		// currently available endpoint (details/uninstall must remain reachable).
		resource := release.InstallPolicy == domain.ResourceAppPolicy && release.ExecutionProfile == domain.ResourceAppPolicy && release.ResourceBinding != nil &&
			release.ResourceBinding.ReleaseID != "" && release.ResourceBinding.ClientID != "" &&
			uiresource.ValidScopes(release.Scopes) && slices.Equal(release.ResourceBinding.AccessScopes.Required, release.Scopes) &&
			len(release.ResourceBinding.AccessScopes.Optional) == 0 && len(release.Capabilities) == 0
		if v.AppID != release.AppID || v.Version != release.Version || (!resource && !ui && !pilot && release.InstallPolicy != domain.InstallPolicy && release.InstallPolicy != domain.ManagedShippingPolicy && release.InstallPolicy != domain.ProviderAppPolicy) {
			return nil, fault.Forbidden
		}
		switch v.Status {
		case "active", "pending":
			if !slices.Equal(scopes, release.Scopes) || !slices.Equal(caps, release.RoutedCapabilities()) || v.GrantState != v.Status {
				return nil, fault.Forbidden
			}
			if v.Status == "active" && !slices.Equal(granted, release.Scopes) || v.Status == "pending" && len(granted) != 0 {
				return nil, fault.Forbidden
			}
		case "disabling":
			if v.GrantState != "revoked" || len(granted) != 0 {
				return nil, fault.Forbidden
			}
		default:
			return nil, fault.Forbidden
		}
		v.Name, v.DeveloperID, v.ExecutionProfile = release.Name, release.DeveloperID, release.ExecutionProfile
		items = append(items, v)
	}
	return items, rows.Err()
}
