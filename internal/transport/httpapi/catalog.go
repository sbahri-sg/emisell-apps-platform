package httpapi

import (
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func (s Server) catalogPortalRoutes(r chi.Router, surface string) {
	r.Get("/catalog", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Catalog.List(r.Context(), portalPrincipal(r))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"releases": v, "limit": 200})
	})
	r.Get("/catalog/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, h, err := s.Catalog.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		var public []byte
		if s.Catalog.Signer != nil {
			public = s.Catalog.Signer.PublicKey()
		}
		write(w, 200, map[string]any{"release": v, "history": h, "trustedPublicKey": public})
	})
	if surface == "developer" {
		r.Get("/apps/{id}/tooling", func(w http.ResponseWriter, r *http.Request) {
			v, err := s.Drafts.Tooling(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, v)
		})
		return
	}
	r.Post("/submissions/{id}/catalog", func(w http.ResponseWriter, r *http.Request) {
		var body struct{}
		if err := decode(w, r, &body); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.Catalog.Sign(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v})
	})
	r.Post("/catalog/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var b service.CatalogAction
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.Catalog.Transition(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"release": v})
	})
}
func (s Server) publicCatalogRoutes(r chi.Router) {
	if s.Catalog.Repo == nil {
		return
	}
	r.Get("/api/v1/store/apps", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := 1
		if q.Get("page") != "" {
			v, err := strconv.Atoi(q.Get("page"))
			if err != nil {
				s.fail(w, r, fault.Invalid)
				return
			}
			page = v
		}
		v, total, err := s.Catalog.Public(r.Context(), service.CatalogQuery{Search: q.Get("search"), Capability: q.Get("capability"), Page: page})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"apps": v, "total": total, "page": page, "pageSize": 20})
	})
	r.Get("/api/v1/store/apps/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Catalog.PublicDetail(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"app": v})
	})
}
