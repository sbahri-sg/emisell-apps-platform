package httpapi

import (
	"emisell.app/platform/internal/app/service"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) integrationRoutes(r chi.Router, surface string) {
	r.Get("/integration-releases", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Integrations.List(r.Context(), portalPrincipal(r))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"releases": v, "limit": 200})
	})
	r.Get("/integration-releases/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, h, err := s.Integrations.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		var public []byte
		if s.Integrations.Signer != nil {
			public = s.Integrations.Signer.PublicKey()
		}
		write(w, 200, map[string]any{"release": v, "history": h, "validation": s.Integrations.Readiness(v), "trustedPublicKey": public})
	})
	if surface == "developer" {
		r.Post("/integration-releases/validate", func(w http.ResponseWriter, r *http.Request) {
			var b service.IntegrationInput
			if err := decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			v, report, err := s.Integrations.Prepare(r.Context(), portalPrincipal(r), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"manifest": v.Manifest, "validation": report})
		})
		r.Post("/integration-releases", func(w http.ResponseWriter, r *http.Request) {
			var b service.IntegrationInput
			if err := decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			v, err := s.Integrations.Submit(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"release": v, "validation": s.Integrations.Readiness(v)})
		})
		return
	}
	r.Post("/integration-releases/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var b service.IntegrationAction
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.Integrations.Transition(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v, "validation": s.Integrations.Readiness(v)})
	})
}
