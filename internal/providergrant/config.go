package providergrant

import (
	"emisell.app/platform/internal/platform/localfiles"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
)

// Attach leaves the public dashboard and all existing engine protocols intact.
// The caller must use its internal loopback listener, never the public proxy.
func Attach(base http.Handler, path string, source Source) (http.Handler, error) {
	if path == "" {
		return base, nil
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("absolute provider grant configuration required")
	}
	var cfg struct {
		EngineKey string
		Apps      map[string]string
	}
	if err := localfiles.Read(path, &cfg); err != nil {
		return nil, err
	}
	h, err := Handler(Service{Source: source, Enrolled: cfg.Apps}, cfg.EngineKey)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != Path && r.URL.Path != TargetsPath {
			base.ServeHTTP(w, r)
			return
		}
		u, err := url.Parse("http://" + r.Host)
		if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
			w.WriteHeader(403)
			return
		}
		h.ServeHTTP(w, r)
	}), nil
}
