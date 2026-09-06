package httpapi

import (
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/recovery"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func decodeRecovery(w http.ResponseWriter, r *http.Request) (recovery.Request, error) {
	var body struct {
		Reason           string `json:"reason"`
		ExpectedRevision *int64 `json:"expectedRevision"`
	}
	if err := decode(w, r, &body); err != nil {
		return recovery.Request{}, err
	}
	if body.ExpectedRevision == nil {
		return recovery.Request{}, fault.Invalid
	}
	return recovery.Request{Reason: body.Reason, ExpectedRevision: *body.ExpectedRevision}, nil
}

func (s Server) operationsRoutes(router chi.Router) {
	router.Get("/api/v1/workspaces/{tenant}/webhooks", func(w http.ResponseWriter, r *http.Request) {
		page, err := s.Webhooks.List(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), r.URL.Query().Get("status"), r.URL.Query().Get("cursor"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, page)
	})
	router.Get("/api/v1/workspaces/{tenant}/webhooks/{delivery}", func(w http.ResponseWriter, r *http.Request) {
		detail, err := s.Webhooks.Get(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), chi.URLParam(r, "delivery"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, detail)
	})
	router.Post("/api/v1/workspaces/{tenant}/webhooks/{delivery}/retry", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeRecovery(w, r)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		result, err := s.Webhooks.Retry(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), chi.URLParam(r, "delivery"), r.Header.Get("Idempotency-Key"), body)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 202, result)
	})
	router.Get("/api/v1/workspaces/{tenant}/connections", func(w http.ResponseWriter, r *http.Request) {
		items, err := s.Connections.List(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"items": items})
	})
	router.Post("/api/v1/workspaces/{tenant}/installations/{installation}/cleanup/retry", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeRecovery(w, r)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		result, err := s.Connections.RetryCleanup(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), chi.URLParam(r, "installation"), r.Header.Get("Idempotency-Key"), body)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 202, result)
	})
}
