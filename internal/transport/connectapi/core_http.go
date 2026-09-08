package connectapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const CoreHTTPPrefix = "/api/v1/core/rpc"

type coreHTTPKey struct{}

// First-party Apps control plane, not an arbitrary RPC proxy. Engine grants,
// payment/shipping execution, token issuance and metrics remain private. Each
// included operation retains its existing authorization and seller consent.
var coreProcedures = map[string]bool{
	"/emisell.integration.v1.ConnectionService/Check":                true,
	"/emisell.integration.v1.ConnectionService/EnsureMerchant":       true,
	"/emisell.testing.v1.TestDistributionService/ListAssignments":    true,
	"/emisell.testing.v1.TestDistributionService/StopAssignment":     true,
	"/emisell.installation.v1.InstallIntentService/Prepare":          true,
	"/emisell.installation.v1.InstallIntentService/Get":              true,
	"/emisell.installation.v1.InstallIntentService/Decide":           true,
	"/emisell.installation.v1.InstallationService/ListInstallations": true,
	"/emisell.installation.v1.InstallationService/GetInstallation":   true,
	"/emisell.installation.v1.InstallationService/Consume":           true,
	"/emisell.installation.v1.InstallationService/Activate":          true,
	"/emisell.installation.v1.InstallationService/Uninstall":         true,
}

// ExposeCore shares existing use cases. HTTP behind a TLS-terminating proxy is
// supported; the API listener must remain private and the proxy must overwrite
// X-Forwarded-Proto. No caller IP enrollment or tunnel is required.
func ExposeCore(portal, internal http.Handler, origin, flag string) (http.Handler, error) {
	if flag == "" || flag == "false" {
		return portal, nil
	}
	u, err := url.Parse(origin)
	if flag != "true" || err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return nil, errors.New("invalid Core HTTP ingress configuration")
	}
	slots := make(chan struct{}, 32)
	var mu sync.Mutex
	var window time.Time
	attempts := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/core/") {
			portal.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fail := func(status int, code string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
		}
		path := strings.TrimPrefix(r.URL.Path, CoreHTTPPrefix)
		if !strings.HasPrefix(r.URL.Path, CoreHTTPPrefix+"/") || !coreProcedures[path] || r.URL.EscapedPath() != r.URL.Path || r.URL.RawQuery != "" || r.URL.ForceQuery {
			fail(404, "not_found")
			return
		}
		if r.Method != "POST" {
			fail(405, "invalid_argument")
			return
		}
		if r.Host != u.Host || (u.Scheme == "https" && r.TLS == nil && (len(r.Header.Values("X-Forwarded-Proto")) != 1 || r.Header.Get("X-Forwarded-Proto") != "https")) {
			fail(403, "permission_denied")
			return
		}
		// Node fetch also emits Sec-Fetch-Mode: cors. It is not by itself a
		// browser signal; browsers additionally send Origin and/or Sec-Fetch-Site.
		for _, name := range []string{"Origin", "Cookie", "Content-Encoding", "Sec-Fetch-Site"} {
			if _, present := r.Header[http.CanonicalHeaderKey(name)]; present {
				fail(403, "permission_denied")
				return
			}
		}
		if len(r.Header.Values("Authorization")) != 1 || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer epk_") || len(r.Header.Get("Authorization")) != 54 {
			fail(401, "unauthenticated")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" || len(r.Header.Values("Content-Type")) != 1 || len(r.Header.Values("Connect-Protocol-Version")) != 1 || r.Header.Get("Connect-Protocol-Version") != "1" {
			fail(415, "invalid_argument")
			return
		}
		mu.Lock()
		if time.Since(window) >= time.Minute {
			window = time.Now()
			attempts = 0
		}
		attempts++
		limited := attempts > 1200
		mu.Unlock()
		if limited {
			w.Header().Set("Retry-After", "60")
			fail(429, "resource_exhausted")
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			fail(429, "resource_exhausted")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
		if err != nil {
			fail(413, "invalid_argument")
			return
		}
		if !json.Valid(body) {
			fail(400, "invalid_argument")
			return
		}
		ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), coreHTTPKey{}, true), 10*time.Second)
		defer cancel()
		forward := r.Clone(ctx)
		forward.Host = "127.0.0.1"
		forward.URL.Path, forward.URL.RawPath = path, ""
		forward.RequestURI = path
		forward.Body = io.NopCloser(strings.NewReader(string(body)))
		forward.Header = http.Header{"Authorization": {r.Header.Get("Authorization")}, "Content-Type": {"application/json"}, "Connect-Protocol-Version": {"1"}}
		internal.ServeHTTP(w, forward)
	}), nil
}
