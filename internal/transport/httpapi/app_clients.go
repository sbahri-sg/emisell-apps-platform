package httpapi

import (
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
	"net/http"
)

const clientCheckPath = "/api/v1/app/client-check"

func (s Server) appClientRoutes(r chi.Router, surface string) {
	r.Get("/app-clients", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.AppClients.List(r.Context(), portalPrincipal(r))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"clients": v, "limit": 200})
	})
	r.Get("/app-clients/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, h, err := s.AppClients.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"view": v, "history": h})
	})
	if surface == "developer" {
		r.Post("/app-clients", func(w http.ResponseWriter, r *http.Request) {
			var in appclient.Input
			if err := decode(w, r, &in); err != nil {
				s.fail(w, r, err)
				return
			}
			v, err := s.AppClients.Create(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), in)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"view": v})
		})
	}
	r.Post("/app-clients/{id}/actions", func(w http.ResponseWriter, r *http.Request) {
		var in appclient.Action
		if err := decode(w, r, &in); err != nil {
			s.fail(w, r, err)
			return
		}
		v, secret, err := s.AppClients.Act(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), in)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"view": v, "secret": secret, "secretAvailable": secret != ""})
	})
}
func (s Server) appClientCheck(r chi.Router) {
	if s.AppClients.Repo == nil {
		return
	}
	r.Post(clientCheckPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" || r.URL.RawQuery != "" {
			s.fail(w, r, fault.Forbidden)
			return
		}
		id, secret, ok := r.BasicAuth()
		if !ok {
			s.fail(w, r, fault.Unauthenticated)
			return
		}
		var in struct{}
		if err := decode(w, r, &in); err != nil {
			s.fail(w, r, err)
			return
		}
		b, err := s.AppClients.Authenticate(r.Context(), id, secret)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"clientId": id, "appId": b.AppID, "releaseId": b.ReleaseID, "redirectUri": b.RedirectURI, "oauthEnabled": false, "installable": false, "resourceGatewayAllowed": false})
	})
}
