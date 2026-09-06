package httpapi

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/gatewaycontract"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type portalKey struct{}

func portalSurface(path string) string {
	for _, surface := range []string{"admin", "developer"} {
		if strings.HasPrefix(path, "/api/v1/"+surface+"/") {
			return surface
		}
	}
	return ""
}
func portalOrigin(surface string) string {
	if surface == "developer" {
		return "http://localhost:4319"
	}
	return "http://localhost:4317"
}
func portalCookie(surface string) string { return "emisell_" + surface + "_session" }
func portalPrincipal(r *http.Request) identity.PortalPrincipal {
	return r.Context().Value(portalKey{}).(identity.PortalPrincipal)
}
func setPortalCookie(w http.ResponseWriter, surface, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: portalCookie(surface), Value: token, Path: "/api/v1/" + surface, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func portalToken(r *http.Request, surface string) string {
	c, e := r.Cookie(portalCookie(surface))
	if e != nil {
		return ""
	}
	return c.Value
}
func (s Server) portalRoutes(router chi.Router) {
	if s.Portals.Repo == nil {
		return
	}
	for _, surface := range []string{"admin", "developer"} {
		router.Route("/api/v1/"+surface, func(r chi.Router) {
			// Browsers share cookies across localhost ports. Check the requesting origin
			// on reads too, not only the CSRF checks on mutations. No CORS wildcard.
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					expected := portalOrigin(surface)
					source := r.Header.Get("Origin")
					if source == "" {
						if ref, e := url.Parse(r.Referer()); e == nil && ref.Host != "" {
							source = ref.Scheme + "://" + ref.Host
						}
					}
					if source != expected {
						s.fail(w, r, fault.Forbidden)
						return
					}
					next.ServeHTTP(w, r)
				})
			})
			var mu sync.Mutex
			var window time.Time
			attempts := 0
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
				var b struct {
					Email    string `json:"email"`
					Password string `json:"password"`
				}
				if err := decode(w, r, &b); err != nil {
					s.fail(w, r, err)
					return
				}
				p, token, err := s.Portals.Login(r.Context(), surface, b.Email, b.Password, portalToken(r, surface))
				if err != nil {
					s.fail(w, r, err)
					return
				}
				setPortalCookie(w, surface, token, 8*3600)
				write(w, 200, map[string]any{"user": p})
			})
			r.Group(func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						p, err := s.Portals.Authenticate(r.Context(), surface, portalToken(r, surface))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), portalKey{}, p)))
					})
				})
				if surface == "admin" {
					r.Get("/staff", func(w http.ResponseWriter, r *http.Request) {
						rows, err := s.Portals.ListStaff(r.Context(), portalPrincipal(r), r.URL.Query().Get("afterId"))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						next := ""
						if len(rows) > 50 {
							rows = rows[:50]
							next = rows[49].ID
						}
						write(w, 200, map[string]any{"accounts": rows, "nextAfterId": next})
					})
					r.Get("/developers", func(w http.ResponseWriter, r *http.Request) {
						rows, err := s.Developers.ListOrganizations(r.Context(), portalPrincipal(r), r.URL.Query().Get("afterId"))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						next := ""
						if len(rows) > 50 {
							rows = rows[:50]
							next = rows[49].ID
						}
						write(w, 200, map[string]any{"organizations": rows, "nextAfterId": next})
					})
					r.Get("/developers/{id}", func(w http.ResponseWriter, r *http.Request) {
						org, err := s.Developers.GetOrganization(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						write(w, 200, map[string]any{"organization": org})
					})
				}
				r.Get("/access-scopes", func(w http.ResponseWriter, r *http.Request) {
					if surface == "developer" {
						if _, err := s.Developers.Organization(r.Context(), portalPrincipal(r)); err != nil {
							s.fail(w, r, err)
							return
						}
					}
					write(w, 200, accessscope.Reference())
				})
				r.Get("/access-scopes/verification", func(w http.ResponseWriter, r *http.Request) {
					if surface == "developer" {
						if _, err := s.Developers.Organization(r.Context(), portalPrincipal(r)); err != nil {
							s.fail(w, r, err)
							return
						}
					}
					write(w, 200, gatewaycontract.VerifyReadiness(time.Now()))
				})
				r.Get("/session", func(w http.ResponseWriter, r *http.Request) {
					result := map[string]any{"user": portalPrincipal(r)}
					if surface == "developer" {
						org, err := s.Developers.Organization(r.Context(), portalPrincipal(r))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						result["organization"] = org
					}
					write(w, 200, result)
				})
				if s.Catalog.Repo != nil {
					s.catalogPortalRoutes(r, surface)
				}
				if s.Integrations.Repo != nil {
					s.integrationRoutes(r, surface)
				}
				if s.ManagedShipping.Repo != nil {
					s.managedShippingRoutes(r, surface)
				}
				if s.Testing.Repo != nil {
					s.testingRoutes(r, surface)
				}
				if s.AppClients.Repo != nil {
					s.appClientRoutes(r, surface)
				}
				if s.EmbeddedLaunches.Repo.Pool != nil {
					s.embeddedLaunchRoutes(r, surface)
				}
				if s.UIReleases.Repo != nil {
					s.uiReleaseRoutes(r, surface)
				}
				r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
					var b struct{}
					if err := decode(w, r, &b); err != nil {
						s.fail(w, r, err)
						return
					}
					if err := s.Portals.Logout(r.Context(), surface, portalToken(r, surface)); err != nil {
						s.fail(w, r, err)
						return
					}
					setPortalCookie(w, surface, "", -1)
					write(w, 200, map[string]bool{"ok": true})
				})
				r.Get("/submissions", func(w http.ResponseWriter, r *http.Request) {
					v, err := s.Reviews.List(r.Context(), portalPrincipal(r))
					if err != nil {
						s.fail(w, r, err)
						return
					}
					write(w, 200, map[string]any{"submissions": v, "limit": 200})
				})
				r.Get("/submissions/{id}", func(w http.ResponseWriter, r *http.Request) {
					v, err := s.Reviews.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
					if err != nil {
						s.fail(w, r, err)
						return
					}
					h, err := s.Reviews.History(r.Context(), portalPrincipal(r), v.ID)
					if err != nil {
						s.fail(w, r, err)
						return
					}
					write(w, 200, map[string]any{"submission": v, "history": h})
				})
				if surface == "admin" {
					s.managedKeyRoutes(r)
					s.platformKeyRoutes(r)
					r.Post("/submissions/{id}/decision", func(w http.ResponseWriter, r *http.Request) {
						var b review.Decision
						if err := decode(w, r, &b); err != nil {
							s.fail(w, r, err)
							return
						}
						v, err := s.Reviews.Decide(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
						if err != nil {
							s.fail(w, r, err)
							return
						}
						write(w, 200, map[string]any{"submission": v})
					})
				} else {
					r.Get("/apps", func(w http.ResponseWriter, r *http.Request) {
						v, err := s.Drafts.List(r.Context(), portalPrincipal(r))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						write(w, 200, map[string]any{"apps": v, "limit": 200})
					})
					r.Get("/apps/{id}", func(w http.ResponseWriter, r *http.Request) {
						v, err := s.Drafts.Get(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						write(w, 200, map[string]any{"app": v})
					})
					save := func(w http.ResponseWriter, r *http.Request) {
						var b service.SaveDraft
						if err := decode(w, r, &b); err != nil {
							s.fail(w, r, err)
							return
						}
						v, err := s.Drafts.Save(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"), b)
						if err != nil {
							s.fail(w, r, err)
							return
						}
						write(w, 200, map[string]any{"app": v})
					}
					r.Post("/apps", save)
					r.Put("/apps/{id}", save)
					r.Post("/apps/{id}/submissions", func(w http.ResponseWriter, r *http.Request) {
						var b struct {
							Revision int `json:"revision"`
						}
						if err := decode(w, r, &b); err != nil {
							s.fail(w, r, err)
							return
						}
						v, err := s.Reviews.Submit(r.Context(), portalPrincipal(r), chi.URLParam(r, "id"), b.Revision, r.Header.Get("Idempotency-Key"))
						if err != nil {
							s.fail(w, r, err)
							return
						}
						write(w, 200, map[string]any{"submission": v})
					})
				}
			})
		})
	}
}
