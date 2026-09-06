package httpapi

import (
	"emisell.app/platform/internal/app/service"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) uiReleaseRoutes(router chi.Router, surface string) {
	router.Get("/ui-releases", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.UIReleases.List(r.Context(), portalPrincipal(r), r.URL.Query().Get("afterId"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		next := ""
		if len(v) == 20 {
			next = v[len(v)-1].ID
		}
		write(w, 200, map[string]any{"releases": v, "nextAfterId": next, "installable": false})
	})
	router.Get("/ui-releases/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.UIReleases.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v, "installable": false})
	})
	if surface == "developer" {
		router.Post("/ui-releases", func(w http.ResponseWriter, r *http.Request) {
			var b service.UIReleaseInput
			if err := decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			v, err := s.UIReleases.Submit(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"release": v, "installable": false})
		})
		return
	}
	router.Post("/ui-releases/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var b service.CatalogAction
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.UIReleases.Decide(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v, "installable": false})
	})
}
