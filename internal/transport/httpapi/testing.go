package httpapi

import (
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func (s Server) testingRoutes(r chi.Router, surface string) {
	r.Get("/test-assignments", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for k, v := range q {
			if (k != "afterId" && k != "pageSize") || len(v) != 1 {
				s.fail(w, r, fault.Invalid)
				return
			}
		}
		size := 0
		var err error
		if q.Get("pageSize") != "" {
			size, err = strconv.Atoi(q.Get("pageSize"))
		}
		if err != nil {
			s.fail(w, r, fault.Invalid)
			return
		}
		rows, next, err := s.Testing.List(r.Context(), portalPrincipal(r), q.Get("afterId"), size)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"assignments": rows, "nextAfterId": next})
	})
	r.Get("/test-assignments/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, h, err := s.Testing.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"assignment": v, "history": h})
	})
	if surface == "developer" {
		r.Post("/test-assignments", func(w http.ResponseWriter, r *http.Request) {
			var b service.AssignmentInput
			if err := decode(w, r, &b); err != nil {
				s.fail(w, r, err)
				return
			}
			v, err := s.Testing.Request(r.Context(), portalPrincipal(r), r.Header.Get("Idempotency-Key"), b)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"assignment": v})
		})
		return
	}
	r.Post("/test-assignments/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var b service.AssignmentAction
		if err := decode(w, r, &b); err != nil {
			s.fail(w, r, err)
			return
		}
		v, err := s.Testing.Decide(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"assignment": v})
	})
}
