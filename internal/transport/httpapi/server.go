package httpapi

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	installation "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/oauth/embedded"
	"emisell.app/platform/internal/platform/config"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/internal/webhook"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	OverviewPool     *pgxpool.Pool
	UIReleases       service.UIReleases
	EmbeddedLaunches embedded.Reviews
	Testing          service.Testing
	AppClients       appclient.Service
	ManagedKeys      identity.ManagedKeys
	PlatformKeys     identity.PlatformKeys
	Portals          identity.Portals
	Developers       developer.Service
	Drafts           service.Drafts
	Reviews          review.Service
	Catalog          service.Catalog
	Integrations     service.Integrations
	ManagedShipping  service.ManagedShipping
	Identity         identity.Service
	Apps             service.Registry
	Installations    installation.Service
	AppAccess        installation.Lifecycle
	Capabilities     capability.Service
	OAuth            *oauth.Service
	Webhooks         webhook.Monitor
	Connections      installation.Monitor
	Payments         capability.Payments
	Ready            func(context.Context) error
	Origin           string
	publicOrigins    config.PublicOrigins
	Logger           *slog.Logger
}
type userKey struct{}
type requestKey struct{}

const cookieName = "emisell_local_session"

func (s Server) Handler() http.Handler {
	var err error
	s.publicOrigins, err = config.ReadPublicOrigins()
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			write(w, 503, map[string]string{"error": "invalid_public_origins"})
		})
	}
	router := chi.NewRouter()
	metrics := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "emisell_http_requests_total", Help: "Completed local API requests."}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "emisell_http_request_duration_seconds", Help: "Local API request duration."}, []string{"method", "route"})
	metrics.MustRegister(requests, duration)
	originURL, _ := url.Parse(s.Origin)
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := ids.New("req")
			w.Header().Set("X-Request-ID", requestID)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			// Reject DNS rebinding, foreign browser origins, and non-JSON unsafe requests.
			if os.Getenv("EMISELL_STORE_DISABLED") == "true" && strings.HasPrefix(r.URL.Path, "/api/v1/store/") {
				write(w, 404, map[string]string{"error": "not_found"})
				return
			}
			if os.Getenv("EMISELL_ENV") == "production" && portalSurface(r.URL.Path) == "" && !strings.HasPrefix(r.URL.Path, "/api/v1/portal/") && !strings.HasPrefix(r.URL.Path, "/api/v1/store/") && r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
				write(w, 404, map[string]string{"error": "not_found"})
				return
			}
			host := strings.Split(r.Host, ":")[0]
			if (s.publicOrigins.Secure && !s.publicOrigins.AllowsHost(r.Host)) || (!s.publicOrigins.Secure && host != "localhost" && host != "127.0.0.1") {
				write(w, 403, map[string]string{"error": "forbidden"})
				return
			}
			if r.Method != "GET" && r.Method != "HEAD" {
				callback := r.Method == "POST" && r.URL.Path == paymentCallbackPath && r.URL.RawQuery == "" && r.Header.Get("Origin") == "" && r.Header.Get("Cookie") == ""
				clientCheck := r.Method == "POST" && r.URL.Path == clientCheckPath && r.URL.RawQuery == "" && r.Header.Get("Origin") == "" && r.Header.Get("Cookie") == ""
				expectedOrigin := originURL.String()
				if surface := portalSurface(r.URL.Path); surface != "" {
					expectedOrigin = s.portalOrigin(surface)
					if unifiedToken(r) != "" {
						expectedOrigin = s.publicOrigins.Admin
					}
				}
				if strings.HasPrefix(r.URL.Path, "/api/v1/portal/") {
					expectedOrigin = s.publicOrigins.Admin
				}
				if !callback && !clientCheck && r.Header.Get("Origin") != expectedOrigin {
					write(w, 403, map[string]string{"error": "forbidden_origin"})
					return
				}
				media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if err != nil || media != "application/json" {
					write(w, 415, map[string]string{"error": "json_required"})
					return
				}
			}
			ctx, cancel := context.WithTimeout(otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header)), 10*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, requestKey{}, requestID)
			ctx, span := otel.Tracer("emisell/http").Start(ctx, "http.request")
			defer span.End()
			started := time.Now()
			rec := &recorder{ResponseWriter: w, status: 200}
			defer func() {
				if recovered := recover(); recovered != nil {
					write(rec, 500, map[string]string{"error": "internal_error"})
					s.Logger.Error("request panic", "request_id", requestID)
				}
				pattern := chi.RouteContext(ctx).RoutePattern()
				if pattern == "" {
					pattern = "unmatched"
				}
				span.SetName(r.Method + " " + pattern)
				span.SetAttributes(attribute.String("http.route", pattern), attribute.Int("http.response.status_code", rec.status))
				requests.WithLabelValues(r.Method, pattern, strconv.Itoa(rec.status)).Inc()
				duration.WithLabelValues(r.Method, pattern).Observe(time.Since(started).Seconds())
				s.Logger.Info("request", "request_id", requestID, "trace_id", span.SpanContext().TraceID().String(), "method", r.Method, "route", pattern, "status", rec.status, "duration_ms", time.Since(started).Milliseconds())
			}()
			next.ServeHTTP(rec, r.WithContext(ctx))
		})
	})
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"status": "ok", "mode": "local-simulator"})
	})
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Ready(r.Context()); err != nil {
			s.fail(w, r, fault.Unavailable)
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	router.Handle("/metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{}))
	router.Post(paymentCallbackPath, s.paymentCallback)
	router.Get("/api/v1/app/installation-access", s.appInstallationAccess)
	s.portalRoutes(router)
	s.unifiedRoutes(router)
	s.publicCatalogRoutes(router)
	s.appClientCheck(router)
	var throttle sync.Mutex
	var window time.Time
	var attempts int
	router.Post("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		throttle.Lock()
		if time.Since(window) > time.Minute {
			window = time.Now()
			attempts = 0
		}
		attempts++
		limited := attempts > 30
		throttle.Unlock()
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
		user, token, err := s.Identity.Login(r.Context(), body.Email, body.Password)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		// Replace any pre-existing authenticated session instead of accumulating it.
		if old, err := r.Cookie(cookieName); err == nil {
			if err = s.Identity.Logout(r.Context(), old.Value); err != nil {
				s.fail(w, r, err)
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/api/v1", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 8 * 3600})
		write(w, 200, map[string]any{"user": user})
	})
	router.Group(func(r chi.Router) {
		r.Use(s.authenticate)
		r.Get("/api/v1/session", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]any{"user": principal(r)}) })
		r.Post("/api/v1/logout", func(w http.ResponseWriter, r *http.Request) {
			c, _ := r.Cookie(cookieName)
			if err := s.Identity.Logout(r.Context(), c.Value); err != nil {
				s.fail(w, r, err)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/api/v1", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
			write(w, 200, map[string]bool{"ok": true})
		})
		r.Get("/api/v1/apps", func(w http.ResponseWriter, r *http.Request) {
			apps, err := s.Apps.List(r.Context())
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"apps": apps})
		})
		r.Get("/api/v1/dashboard", s.dashboard)
		s.operationsRoutes(r)
		s.paymentRoutes(r)
		if s.OAuth != nil {
			r.Post("/api/v1/workspaces/{tenant}/installations/{installation}/oauth", func(w http.ResponseWriter, r *http.Request) {
				var body struct{}
				if err := decode(w, r, &body); err != nil {
					s.fail(w, r, err)
					return
				}
				cookie, _ := r.Cookie(cookieName)
				authorize, err := s.OAuth.Begin(r.Context(), principal(r).ID, cookie.Value, chi.URLParam(r, "tenant"), chi.URLParam(r, "installation"), r.Header.Get("Idempotency-Key"))
				if err != nil {
					s.fail(w, r, err)
					return
				}
				write(w, 200, map[string]string{"authorizationUrl": authorize})
			})
			r.Get("/api/v1/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
				cookie, _ := r.Cookie(cookieName)
				tenant, err := s.OAuth.Callback(r.Context(), principal(r).ID, cookie.Value, r.URL.Query().Get("state"), r.URL.Query().Get("code"))
				q := url.Values{"view": {"apps"}}
				if tenant != "" {
					q.Set("workspace", tenant)
				}
				if err != nil {
					q.Set("oauth", "failed")
				} else {
					q.Set("oauth", "connected")
				}
				w.Header().Set("Referrer-Policy", "no-referrer")
				http.Redirect(w, r, s.Origin+"/?"+q.Encode(), http.StatusSeeOther)
			})
		}
		r.Get("/api/v1/workspaces/{tenant}/installations", func(w http.ResponseWriter, r *http.Request) {
			items, err := s.Installations.List(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"installations": items})
		})
		r.Get("/api/v1/workspaces/{tenant}/events", func(w http.ResponseWriter, r *http.Request) {
			items, err := s.Installations.Events(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"events": items})
		})
		r.Post("/api/v1/workspaces/{tenant}/installations", func(w http.ResponseWriter, r *http.Request) {
			var body domain.Action
			if err := decode(w, r, &body); err != nil {
				s.fail(w, r, err)
				return
			}
			result, err := s.Installations.Execute(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), r.Header.Get("Idempotency-Key"), correlation(r), body)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, map[string]any{"installation": result})
		})
		r.Post("/api/v1/workspaces/{tenant}/capabilities/{capability}/v1/invoke", func(w http.ResponseWriter, r *http.Request) {
			var body capability.Request
			if err := decode(w, r, &body); err != nil {
				s.fail(w, r, err)
				return
			}
			result, err := s.Capabilities.Invoke(r.Context(), principal(r).ID, chi.URLParam(r, "tenant"), chi.URLParam(r, "capability")+"/v1", r.Header.Get("Idempotency-Key"), correlation(r), body)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, result)
		})
	})
	return router
}
func (s Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			s.fail(w, r, fault.Unauthenticated)
			return
		}
		u, err := s.Identity.Authenticate(r.Context(), c.Value)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}
func principal(r *http.Request) identity.User { return r.Context().Value(userKey{}).(identity.User) }
func correlation(r *http.Request) string      { return r.Context().Value(requestKey{}).(string) }
func (s Server) dashboard(w http.ResponseWriter, r *http.Request) {
	user := principal(r)
	workspaces, err := s.Identity.Workspaces(r.Context(), user.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	apps, err := s.Apps.List(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := []map[string]any{}
	for _, ws := range workspaces {
		installations, err := s.Installations.List(r.Context(), user.ID, ws.ID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		events, err := s.Installations.Events(r.Context(), user.ID, ws.ID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		activity := []map[string]any{}
		for _, e := range events {
			payload, _ := e.Payload.(map[string]any)
			name, _ := payload["name"].(string)
			appID, _ := payload["appId"].(string)
			verb := map[string]string{"emisell.app.installed.v1": "dipasang", "emisell.app.activated.v1": "diaktifkan", "emisell.app.disabling.v1": "dinonaktifkan; pembersihan koneksi diproses", "emisell.app.uninstalled.v1": "dilepas"}[e.Type]
			activity = append(activity, map[string]any{"id": e.ID, "appId": appID, "title": name + " " + verb, "description": "Tercatat di PostgreSQL · " + e.Type, "occurredAt": e.OccurredAt})
		}
		out = append(out, map[string]any{"id": ws.ID, "name": ws.Name, "installations": installations, "activity": activity})
	}
	write(w, 200, map[string]any{"user": user, "catalog": apps, "workspaces": out, "mode": "local", "loading": false, "error": nil})
}
func decode(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return fault.Invalid
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fault.Invalid
	}
	return nil
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func (s Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := 500
	code := "internal_error"
	for _, item := range []struct {
		err    error
		status int
	}{{fault.Invalid, 400}, {fault.Unauthenticated, 401}, {fault.Forbidden, 403}, {fault.NotFound, 404}, {fault.Conflict, 409}, {fault.Unavailable, 503}} {
		if errors.Is(err, item.err) {
			status = item.status
			code = item.err.Error()
			break
		}
	}
	if status == 500 {
		s.Logger.Error("request failed", "request_id", correlation(r), "error_type", "internal")
	}
	write(w, status, map[string]string{"error": code, "requestId": correlation(r)})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (w *recorder) WriteHeader(status int) { w.status = status; w.ResponseWriter.WriteHeader(status) }
