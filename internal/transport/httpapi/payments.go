package httpapi

import (
	"bytes"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appapi"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
)

const paymentCallbackPath = "/api/v1/app-callbacks/payment/v1"

func (s Server) paymentCallback(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" {
		s.fail(w, r, fault.Forbidden)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
	if err != nil {
		s.fail(w, r, fault.Invalid)
		return
	}
	var u appapi.PaymentUpdate
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&u) != nil || d.Decode(&struct{}{}) != io.EOF {
		s.fail(w, r, fault.Invalid)
		return
	}
	outcome, err := s.Payments.Receive(r.Context(), r.Header.Get("X-Emisell-Tenant"), r.Header.Get("X-Emisell-Installation"), r.Header.Get("X-Emisell-Delivery"), r.Header.Get("X-Emisell-Timestamp"), r.Header.Get("X-Emisell-Signature"), correlation(r), raw, u)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, 200, map[string]string{"outcome": outcome})
}
func (s Server) paymentRoutes(r chi.Router) {
	r.Get("/api/v1/workspaces/{tenant}/payments", func(w http.ResponseWriter, r *http.Request) {
		page, err := s.Payments.List(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), r.URL.Query().Get("status"), r.URL.Query().Get("cursor"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, page)
	})
	r.Get("/api/v1/workspaces/{tenant}/payments/{payment}", func(w http.ResponseWriter, r *http.Request) {
		d, err := s.Payments.Get(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), chi.URLParam(r, "payment"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, d)
	})
}
