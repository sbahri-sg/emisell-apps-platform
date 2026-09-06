package httpapi

import (
	"emisell.app/platform/internal/identity"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) managedKeyRoutes(r chi.Router) {
	if s.ManagedKeys.Repo == nil {
		return
	}
	r.Get("/api-keys", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.ManagedKeys.List(r.Context(), portalPrincipal(r))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"keys": v, "allowedScopes": identity.CoreScopes(), "limit": 200, "maxValidDays": 30})
	})
	r.Post("/api-keys", func(w http.ResponseWriter, r *http.Request) {
		var b identity.CreateManagedKey
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, secret, err := s.ManagedKeys.Create(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"key": v, "secret": secret, "secretAvailable": secret != ""})
	})
	r.Post("/api-keys/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		var b struct{}
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.ManagedKeys.Revoke(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"key": v})
	})
}
