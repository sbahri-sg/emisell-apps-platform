package memory

import (
	"context"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) UpsertMerchantIdentity(_ context.Context, identity domain.MerchantIdentity) (domain.MerchantIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := merchantIdentityKey(identity.UserID, identity.MerchantID, identity.Environment)
	if current, exists := r.merchantIdentities[key]; exists {
		identity.CreatedAt = current.CreatedAt
	}
	r.merchantIdentities[key] = cloneMerchantIdentity(identity)
	return cloneMerchantIdentity(identity), nil
}

func (r *Repository) GetMerchantIdentity(_ context.Context, userID, merchantID string, environment domain.Environment) (domain.MerchantIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if identity, exists := r.merchantIdentities[merchantIdentityKey(userID, merchantID, environment)]; exists {
		return cloneMerchantIdentity(identity), nil
	}
	return domain.MerchantIdentity{}, domain.ErrNotFound
}

func (r *Repository) ResolveMerchantIdentity(_ context.Context, merchantID string) (domain.MerchantIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var resolved *domain.MerchantIdentity
	for _, identity := range r.merchantIdentities {
		if identity.MerchantID != merchantID {
			continue
		}
		if resolved == nil || identity.UpdatedAt.After(resolved.UpdatedAt) {
			copy := cloneMerchantIdentity(identity)
			resolved = &copy
		}
	}
	if resolved == nil {
		return domain.MerchantIdentity{}, domain.ErrNotFound
	}
	return *resolved, nil
}

func merchantIdentityKey(userID, merchantID string, environment domain.Environment) string {
	return userID + ":" + merchantID + ":" + string(environment)
}

func (r *Repository) ListMerchantInstalledApps(_ context.Context, merchantID string, environment domain.Environment) ([]domain.MerchantInstalledApp, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.MerchantInstalledApp, 0)
	for appID, installations := range r.installations {
		app := r.apps[appID]
		for _, installation := range installations {
			if installation.MerchantID != merchantID || installation.Environment != environment || installation.Status == domain.InstallationStatusUninstalled {
				continue
			}
			version := r.versions[appID][installation.InstalledVersionID]
			items = append(items, merchantInstalledApp(app, version, installation))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].InstalledAt.After(items[j].InstalledAt) })
	return items, nil
}

func (r *Repository) GetMerchantInstalledApp(_ context.Context, merchantID, installationID string, environment domain.Environment) (domain.MerchantInstalledApp, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for appID, installations := range r.installations {
		installation, exists := installations[installationID]
		if !exists || installation.MerchantID != merchantID || installation.Environment != environment {
			continue
		}
		return merchantInstalledApp(r.apps[appID], r.versions[appID][installation.InstalledVersionID], installation), nil
	}
	return domain.MerchantInstalledApp{}, domain.ErrNotFound
}

func merchantInstalledApp(app domain.App, version domain.AppVersion, installation domain.AppInstallation) domain.MerchantInstalledApp {
	return domain.MerchantInstalledApp{
		OrganizationID: app.OrganizationID, InstallationID: installation.ID,
		AppID: app.ID, AppName: app.Name, AppDescription: cloneString(app.Description), AppURL: cloneString(app.AppURL),
		Version: version.Version, Status: installation.Status, GrantedScopes: append([]string(nil), installation.GrantedScopes...),
		InstalledAt: installation.InstalledAt, UpdatedAt: installation.UpdatedAt,
	}
}

func cloneMerchantIdentity(identity domain.MerchantIdentity) domain.MerchantIdentity {
	identity.Domain = cloneString(identity.Domain)
	return identity
}

var _ ports.MerchantRepository = (*Repository)(nil)
