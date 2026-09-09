package httpapi

import (
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/url"
)

func (s Server) applicationCredentialRoutes(r chi.Router) {
	r.Get("/apps/{id}/install-url", func(w http.ResponseWriter, r *http.Request) {
		d, err := s.Drafts.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		origin := s.sellerOrigin()
		if origin == "" || d.ActiveVersion == nil {
			s.fail(w, r, fault.Unavailable)
			return
		}
		query := url.Values{"app": {d.ID}, "version": {d.ActiveVersion.Document.Version}}
		w.Header().Set("Cache-Control", "no-store")
		write(w, 200, map[string]string{"url": origin + "/auth/stores?" + query.Encode()})
	})
	r.Get("/apps/{id}/credentials", func(w http.ResponseWriter, r *http.Request) {
		d, err := s.Drafts.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		c, err := s.AppIdentities.Get(r.Context(), d.OrganizationID, d.ID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		write(w, 200, map[string]any{"credential": c})
	})
	for _, action := range []string{"reveal", "rotate"} {
		r.Post("/apps/{id}/credentials/"+action, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if r.URL.RawQuery != "" {
				s.fail(w, r, fault.Invalid)
				return
			}
			p := portalPrincipal(r)
			d, err := s.Drafts.Get(r.Context(), p, chi.URLParam(r, "id"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			var in struct {
				Version int `json:"version"`
			}
			if err = decode(w, r, &in); err != nil {
				s.fail(w, r, err)
				return
			}
			if in.Version < 1 {
				s.fail(w, r, fault.Invalid)
				return
			}
			if action == "reveal" {
				c, secret, err := s.AppIdentities.Reveal(r.Context(), d.OrganizationID, d.ID, p.ID, in.Version)
				if err != nil {
					s.fail(w, r, err)
					return
				}
				write(w, 200, map[string]any{"credential": c, "secret": secret})
				return
			}
			c, err := s.AppIdentities.Rotate(r.Context(), d.OrganizationID, d.ID, p.ID, r.Header.Get("Idempotency-Key"), in.Version)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"credential": c})
		})
	}
}
