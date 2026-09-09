package httpapi

import (
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) applicationContactRoutes(r chi.Router) {
	r.Get("/apps/{id}/contact", func(w http.ResponseWriter, r *http.Request) {
		d, err := s.Drafts.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		c, err := s.AppContacts.Get(r.Context(), d.OrganizationID, d.ID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"contact": c})
	})
	r.Put("/apps/{id}/contact", func(w http.ResponseWriter, r *http.Request) {
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
			Email    string `json:"email"`
			Revision int    `json:"revision"`
		}
		if err = decode(w, r, &in); err != nil {
			s.fail(w, r, err)
			return
		}
		c, err := s.AppContacts.Save(r.Context(), d.OrganizationID, d.ID, p.ID, in.Email, in.Revision)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"contact": c})
	})
}
