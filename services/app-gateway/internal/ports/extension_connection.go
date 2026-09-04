package ports

import (
	"context"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type ExtensionConnectionSelector struct {
	OrganizationID, AppID, InstallationID, ExtensionID string
	// Machine requests supply only a token digest; storage derives tenant selectors.
	TokenHash string
}

type ExtensionConnectionState struct {
	App                          domain.App
	Extension                    domain.AppExtension
	Installation                 domain.AppInstallation
	Version                      domain.AppVersion
	OrganizationActive, Entitled bool
	Connection                   *domain.ExtensionConnection
	Write                        *domain.ExtensionConnection
	Audit                        MutationMeta
	AccessScope, RequestID       string
}

type ExtensionConnectionRepository interface {
	// Holds lifecycle locks through callback, persistence and audit commit. Callback
	// must not call other repository methods or perform external network requests.
	WithExtensionConnection(context.Context, ExtensionConnectionSelector, func(*ExtensionConnectionState) error) error
}
