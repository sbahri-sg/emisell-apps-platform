package httpapi

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"emisell.app/platform/internal/identity/merchantlogin"
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
)

const developerLoginPrefix = "/api/v1/developer-login"
const developerLoginCookie = "emisell_developer_login"

func merchantLoginURL(seller, request string) string {
	return seller + "/auth/login?" + url.Values{"returnTo": {"/api/app-platform/sso?request=" + url.QueryEscape(request)}}.Encode()
}

func (s Server) sellerOrigin() string {
	value := os.Getenv("EMISELL_SELLER_ORIGIN")
	if value == "" && os.Getenv("EMISELL_ENV") != "production" {
		value = "http://localhost:3000"
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") ||
		(u.Scheme != "https" && !(u.Scheme == "http" && os.Getenv("EMISELL_ENV") != "production")) {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (s Server) developerBrowserOrigin(r *http.Request) string {
	for _, origin := range []string{s.publicOrigins.Admin, s.publicOrigins.Developer} {
		u, err := url.Parse(origin)
		if err == nil && u.Host == r.Host {
			return origin
		}
	}
	return ""
}

func (s Server) startDeveloperLogin(w http.ResponseWriter, r *http.Request) {
	origin := s.developerBrowserOrigin(r)
	seller := s.sellerOrigin()
	if origin == "" || r.Header.Get("Origin") != origin {
		s.fail(w, r, fault.Forbidden)
		return
	}
	if seller == "" {
		s.fail(w, r, fault.Unavailable)
		return
	}
	var body struct{}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	id, proof, err := s.DeveloperLogin.Start(r.Context(), origin)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: developerLoginCookie, Value: id + ":" + proof, Path: developerLoginPrefix, HttpOnly: true, Secure: s.publicOrigins.Secure, SameSite: http.SameSiteLaxMode, MaxAge: 300})
	write(w, 200, map[string]string{"authorizeUrl": merchantLoginURL(seller, id)})
}

func (s Server) developerLoginRoutes(router chi.Router) {
	if s.DeveloperLogin.Pool == nil {
		return
	}
	var mu sync.Mutex
	var window time.Time
	attempts := 0
	router.Route(developerLoginPrefix, func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Referrer-Policy", "no-referrer")
				if r.Method == "POST" {
					mu.Lock()
					if time.Since(window) > time.Minute {
						window = time.Now()
						attempts = 0
					}
					attempts++
					limited := attempts > 60
					mu.Unlock()
					if limited {
						write(w, 429, map[string]string{"error": "too_many_attempts"})
						return
					}
				}
				next.ServeHTTP(w, r)
			})
		})
		s.cliLoginRoutes(r)
		r.Post("/start", func(w http.ResponseWriter, r *http.Request) { s.startDeveloperLogin(w, r) })
		r.Post("/approve", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" || r.URL.RawQuery != "" {
				s.fail(w, r, fault.Forbidden)
				return
			}
			header := r.Header.Get("Authorization")
			if len(r.Header.Values("Authorization")) != 1 || !strings.HasPrefix(header, "Bearer epk_") {
				s.fail(w, r, fault.Unauthenticated)
				return
			}
			p, err := s.CoreAccounts.Authenticate(r.Context(), strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			if !p.PlatformFull {
				s.fail(w, r, fault.Forbidden)
				return
			}
			var a merchantlogin.Assertion
			if err = decode(w, r, &a); err != nil {
				s.fail(w, r, err)
				return
			}
			origin, code, err := s.DeveloperLogin.Approve(r.Context(), a)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]string{"exchangeUrl": origin + developerLoginPrefix + "/finish?request=" + url.QueryEscape(a.Request) + "&code=" + url.QueryEscape(code)})
		})
		r.Get("/finish", func(w http.ResponseWriter, r *http.Request) {
			origin := s.developerBrowserOrigin(r)
			if origin == "" {
				s.fail(w, r, fault.Forbidden)
				return
			}
			if cli, err := s.DeveloperLogin.IsCLI(r.Context(), r.URL.Query().Get("request")); err == nil && cli {
				if _, err := s.DeveloperLogin.CLIConfirmation(r.Context(), r.URL.Query().Get("request"), r.URL.Query().Get("code"), origin, false); err != nil {
					s.fail(w, r, err)
					return
				}
				http.Redirect(w, r, origin+developerLoginPrefix+"/cli/confirm?"+url.Values{"request": {r.URL.Query().Get("request")}, "code": {r.URL.Query().Get("code")}}.Encode(), http.StatusSeeOther)
				return
			}
			cookie, err := r.Cookie(developerLoginCookie)
			if err != nil {
				s.fail(w, r, fault.Unauthenticated)
				return
			}
			parts := strings.Split(cookie.Value, ":")
			if len(parts) != 2 || parts[0] != r.URL.Query().Get("request") {
				s.fail(w, r, fault.Unauthenticated)
				return
			}
			token, err := s.DeveloperLogin.Finish(r.Context(), parts[0], parts[1], r.URL.Query().Get("code"), origin)
			http.SetCookie(w, &http.Cookie{Name: developerLoginCookie, Path: developerLoginPrefix, HttpOnly: true, Secure: s.publicOrigins.Secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
			if err != nil {
				http.Redirect(w, r, origin+"/development?login=failed", http.StatusSeeOther)
				return
			}
			if origin == s.publicOrigins.Admin {
				s.unifiedCookie(w, token, 3600)
			} else {
				s.setPortalCookie(w, "developer", token, 3600)
			}
			http.Redirect(w, r, origin+"/development", http.StatusSeeOther)
		})
	})
}
