package httpapi

import (
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const unifiedCookie = "emisell_portal_session"

func unifiedToken(r *http.Request) string {
	c, e := r.Cookie(unifiedCookie)
	if e != nil {
		return ""
	}
	return c.Value
}
func (s Server) unifiedCookie(w http.ResponseWriter, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: unifiedCookie, Value: token, Path: "/api/v1", HttpOnly: true, Secure: s.publicOrigins.Secure, SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (s Server) unifiedRoutes(router chi.Router) {
	if s.Portals.Repo == nil {
		return
	}
	var mu sync.Mutex
	var window time.Time
	attempts := 0
	router.Route("/api/v1/portal", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				source := r.Header.Get("Origin")
				if source == "" {
					if u, e := url.Parse(r.Referer()); e == nil {
						source = u.Scheme + "://" + u.Host
					}
				}
				expected, _ := url.Parse(s.publicOrigins.Admin)
				if source != s.publicOrigins.Admin || (s.publicOrigins.Secure && r.Host != expected.Host) {
					s.fail(w, r, fault.Forbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			if time.Since(window) > time.Minute {
				window = time.Now()
				attempts = 0
			}
			attempts++
			limited := attempts > 30
			mu.Unlock()
			if limited {
				w.Header().Set("Retry-After", "60")
				write(w, 429, map[string]string{"error": "too_many_attempts"})
				return
			}
			var body struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if err := decode(w, r, &body); err != nil {
				s.fail(w, r, err)
				return
			}
			p, token, err := s.Portals.LoginUnified(r.Context(), body.Email, body.Password, unifiedToken(r))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			s.setPortalCookie(w, "admin", "", -1)
			s.setPortalCookie(w, "developer", "", -1)
			s.unifiedCookie(w, token, 8*3600)
			write(w, 200, map[string]any{"user": p})
		})
		r.Get("/session", func(w http.ResponseWriter, r *http.Request) {
			p, err := s.Portals.UnifiedSession(r.Context(), unifiedToken(r))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"user": p})
		})
	})
}
