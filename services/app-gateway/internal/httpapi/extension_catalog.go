package httpapi

import (
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/application"
)

func (h *handlers) listExtensionCatalog(writer http.ResponseWriter, _ *http.Request) {
	// Public, non-tenant metadata, like the scope/event catalogs. No connection,
	// credential, merchant data, runtime URL or operational token is exposed.
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusOK, envelope[any]{Data: application.OfficialExtensionCatalog()})
}
