package httpapi

import (
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/webhook/subscriptions"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s Server) webhookSubscriptionRoutes(r chi.Router) {
	r.Get("/webhook-topics", func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.Developers.Organization(r.Context(), portalPrincipal(r)); err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"topics": subscriptions.Topics(), "deliveryEnabled": false})
	})
	r.Route("/apps/{id}/webhook-subscriptions", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			app, err := s.Drafts.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			after := r.URL.Query().Get("afterId")
			if len(after) > 128 {
				s.fail(w, r, fault.Invalid)
				return
			}
			rows, err := s.WebhookSubscriptions.List(r.Context(), app.OrganizationID, app.ID, after)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			next := ""
			if len(rows) > 50 {
				rows = rows[:50]
				next = rows[49].ID
			}
			write(w, 200, map[string]any{"subscriptions": rows, "nextAfterId": next})
		})
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			app, err := s.Drafts.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			var b subscriptions.Input
			if err = decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			record, err := s.WebhookSubscriptions.Create(r.Context(), app.OrganizationID, portalPrincipal(r).ID, app.ID, r.Header.Get("Idempotency-Key"), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"subscription": record})
		})
		r.Post("/{subscriptionId}/revoke", func(w http.ResponseWriter, r *http.Request) {
			app, err := s.Drafts.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			var b struct{}
			if err = decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			record, err := s.WebhookSubscriptions.Revoke(r.Context(), app.OrganizationID, portalPrincipal(r).ID, app.ID, chi.URLParam(r, "subscriptionId"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"subscription": record})
		})
	})
}
