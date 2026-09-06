package httpapi

import (
	"emisell.app/platform/internal/oauth/embedded"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) embeddedLaunchRoutes(r chi.Router, surface string) {
	r.Get("/app-clients/{id}/launch", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.EmbeddedLaunches.ForClient(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"launch": v, "launchable": false})
	})
	r.Get("/embedded-launches/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.EmbeddedLaunches.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"launch": v, "launchable": false})
	})
	if surface == "developer" {
		r.Post("/embedded-launches", func(w http.ResponseWriter, r *http.Request) {
			var b embedded.LaunchInput
			if err := decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			v, err := s.EmbeddedLaunches.Submit(r.Context(), portalPrincipal(r), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"launch": v, "launchable": false})
		})
		return
	}
	r.Post("/embedded-launches/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Revision int    `json:"revision"`
			Status   string `json:"status"`
			Reason   string `json:"reason"`
		}
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.EmbeddedLaunches.Decide(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), b.Revision, b.Status, b.Reason)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"launch": v, "launchable": false})
	})
}
