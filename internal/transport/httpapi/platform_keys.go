package httpapi

import (
	"emisell.app/platform/internal/identity"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) platformKeyRoutes(r chi.Router) {
	if s.PlatformKeys.Repo == nil {
		return
	}
	r.Get("/platform-keys", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.PlatformKeys.List(r.Context(), portalPrincipal(r))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"keys": v, "limit": 200})
	})
	r.Post("/platform-keys", func(w http.ResponseWriter, r *http.Request) {
		var b identity.CreatePlatformKey
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, secret, err := s.PlatformKeys.Create(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"key": v, "secret": secret, "secretAvailable": secret != ""})
	})
	r.Post("/platform-keys/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		var b struct{}
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.PlatformKeys.Revoke(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"key": v})
	})
}
