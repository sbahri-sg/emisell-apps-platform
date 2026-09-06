package httpapi

import (
	"emisell.app/platform/internal/app/service"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) managedShippingRoutes(r chi.Router, surface string) {
	r.Get("/managed-shipping-releases", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.ManagedShipping.List(r.Context(), portalPrincipal(r))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"releases": v, "limit": 200})
	})
	r.Get("/managed-shipping-releases/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, h, err := s.ManagedShipping.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		var public []byte
		if s.ManagedShipping.Signer != nil {
			public = s.ManagedShipping.Signer.PublicKey()
		}
		write(w, 200, map[string]any{"release": v, "history": h, "readiness": s.ManagedShipping.Readiness(v), "trustedPublicKey": public})
	})
	if surface == "developer" {
		r.Post("/managed-shipping-releases", func(w http.ResponseWriter, r *http.Request) {
			var b service.ManagedShippingInput
			if err := decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			v, err := s.ManagedShipping.Submit(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"release": v, "readiness": s.ManagedShipping.Readiness(v)})
		})
		return
	}
	r.Post("/managed-shipping-releases/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var b service.CatalogAction
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.ManagedShipping.Transition(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v, "readiness": s.ManagedShipping.Readiness(v)})
	})
}
