package memory

import (
	"context"
	"encoding/json"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func cloneConnection(value domain.ExtensionConnection) domain.ExtensionConnection {
	value.Scopes = append([]string{}, value.Scopes...)
	value.Ciphertext = append([]byte(nil), value.Ciphertext...)
	return value
}

func (r *Repository) WithExtensionConnection(_ context.Context, sel ports.ExtensionConnectionSelector, use func(*ports.ExtensionConnectionState) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sel.TokenHash != "" {
		found := false
		for _, value := range r.extensionConnections {
			if value.TokenHash == sel.TokenHash {
				sel.AppID, sel.InstallationID, sel.ExtensionID = value.AppID, value.InstallationID, value.ExtensionID
				sel.OrganizationID = r.apps[value.AppID].OrganizationID
				found = true
				break
			}
		}
		if !found {
			return domain.ErrUnauthorized
		}
	}
	app, ok := r.apps[sel.AppID]
	if !ok || app.OrganizationID != sel.OrganizationID {
		return domain.ErrNotFound
	}
	extension, ok := r.extensions[sel.AppID][sel.ExtensionID]
	if !ok {
		return domain.ErrNotFound
	}
	installation, ok := r.installations[sel.AppID][sel.InstallationID]
	if !ok {
		return domain.ErrNotFound
	}
	version, ok := r.versions[sel.AppID][installation.InstalledVersionID]
	if !ok {
		return domain.ErrNotFound
	}
	entitlement := r.organizationEntitlements[sel.OrganizationID]
	active := true
	if org, ok := r.developerOrganizations[sel.OrganizationID]; ok {
		active = org.Status == domain.OrganizationStatusActive
	}
	state := ports.ExtensionConnectionState{App: cloneApp(app), Extension: extension, Installation: installation, Version: version, OrganizationActive: active,
		Entitled: (installation.Environment == domain.EnvironmentSandbox && entitlement.SandboxAccess) || (installation.Environment == domain.EnvironmentProduction && entitlement.ProductionAccess)}
	key := sel.InstallationID + ":" + sel.ExtensionID
	if value, ok := r.extensionConnections[key]; ok {
		copy := cloneConnection(value)
		state.Connection = &copy
	}
	if err := use(&state); err != nil {
		return err
	}
	if state.Write != nil {
		id, err := r.id()
		if err != nil {
			return err
		}
		r.extensionConnections[key] = cloneConnection(*state.Write)
		r.appendAuditLocked(id, sel.OrganizationID, state.Audit, "extension_connection", state.Write.ID, map[string]any{"installationId": sel.InstallationID, "extensionId": sel.ExtensionID, "revision": state.Write.Revision, "status": state.Write.Status})
	}
	if state.AccessScope != "" {
		entry, _ := json.Marshal(map[string]any{"connectionId": state.Connection.ID, "revision": state.Connection.Revision, "scope": state.AccessScope, "requestId": state.RequestID})
		r.extensionCredentialAccesses = append(r.extensionCredentialAccesses, string(entry))
	}
	return nil
}
