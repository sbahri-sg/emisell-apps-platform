package httpapi

import (
	"emisell.app/platform/internal/app/service"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) uiResourceReleaseRoutes(router chi.Router, surface string) {
	router.Get("/ui-resource-releases", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.UIResourceReleases.List(r.Context(), portalPrincipal(r), r.URL.Query().Get("afterId"))
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
	router.Get("/ui-resource-releases/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.UIResourceReleases.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v, "installable": false})
	})
	if surface == "developer" {
		router.Post("/ui-resource-releases", func(w http.ResponseWriter, r *http.Request) {
			var b service.UIResourceReleaseInput
			if err := decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			v, err := s.UIResourceReleases.Submit(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"release": v, "installable": false})
		})
		return
	}
	router.Post("/ui-resource-releases/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var b service.CatalogAction
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.UIResourceReleases.Decide(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v, "installable": false})
	})
}
