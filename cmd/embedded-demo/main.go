// embedded-demo is an isolated, loopback-only protocol exercise. It is not Core
// authentication and never loads merchant data, credentials or a database.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	app "emisell.app/platform/internal/oauth/embedded"
	"emisell.app/platform/pkg/appui"
	contract "emisell.app/platform/pkg/embedded"
)

//go:embed web/*
var assets embed.FS

const parentOrigin = "http://127.0.0.1:4320"
const appOrigin = "http://127.0.0.1:4321"

type demo struct {
	mu       sync.RWMutex
	revoked  bool
	csrf     string
	id       contract.Identity
	launch   app.Launcher
	verifier app.Service
}

func newDemo() (*demo, error) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	launchPub, launchPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	d := &demo{csrf: hex.EncodeToString(nonce), id: contract.Identity{MerchantID: "demo-store", ActorID: "demo-staff", AppID: "embedded-demo", InstallationID: "demo-installation"}}
	digest := sha256.Sum256([]byte("emisell.embedded-demo/v1:synthetic-installation-only"))
	metadata := contract.Launch{AppID: d.id.AppID, ClientID: "demo-client", ReleaseDigest: hex.EncodeToString(digest[:]), URL: appOrigin + "/", ParentOrigin: parentOrigin}
	signature, err := contract.SignLaunch(metadata, launchPrivate, true)
	if err != nil {
		return nil, err
	}
	authorize := func(_ context.Context, id contract.Identity, audience string) error {
		// Caller retains the demo state read lock through issuance/verification.
		if d.revoked || id != d.id || audience != metadata.ClientID {
			return app.ErrDenied
		}
		return nil
	}
	issuer := app.Service{PrivateKey: private, KeyID: "demo-ephemeral", Issuer: parentOrigin, Authorize: authorize}
	d.verifier = app.Service{Keys: map[string]ed25519.PublicKey{"demo-ephemeral": pub}, Issuer: parentOrigin, Authorize: authorize}
	d.launch = app.Launcher{Identity: issuer, LaunchKey: launchPub, LocalTest: true, Resolve: func(_ context.Context, id contract.Identity) (app.Binding, error) {
		if d.revoked || id != d.id {
			return app.Binding{}, app.ErrDenied
		}
		return app.Binding{Launch: metadata, Signature: signature}, nil
	}}
	return d, nil
}

func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (d *demo) handlers() (http.Handler, http.Handler) {
	host, frame := http.NewServeMux(), http.NewServeMux()
	launch := app.LaunchHandler(d.launch, parentOrigin, func(_ context.Context, r *http.Request, installation string) (contract.Identity, error) {
		// Synthetic identity is intentionally confined to this demo executable.
		if r.Header.Get("X-Demo-CSRF") != d.csrf || installation != d.id.InstallationID {
			return contract.Identity{}, app.ErrDenied
		}
		return d.id, nil
	})
	host.HandleFunc("POST /demo/session", func(w http.ResponseWriter, r *http.Request) {
		d.mu.RLock()
		defer d.mu.RUnlock()
		launch.ServeHTTP(w, r)
	})
	host.HandleFunc("POST /demo/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != parentOrigin || r.Header.Get("X-Demo-CSRF") != d.csrf || r.URL.RawQuery != "" || r.ContentLength != 0 {
			jsonReply(w, 403, map[string]string{"error": "denied"})
			return
		}
		d.mu.Lock()
		d.revoked = true
		d.mu.Unlock()
		jsonReply(w, 200, map[string]string{"status": "revoked"})
	})
	host.HandleFunc("GET /demo/config", func(w http.ResponseWriter, r *http.Request) {
		// No CORS. A custom mutation header plus exact Origin protects the demo.
		if r.URL.RawQuery != "" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != parentOrigin) {
			jsonReply(w, 403, map[string]string{"error": "denied"})
			return
		}
		jsonReply(w, 200, map[string]string{"csrf": d.csrf, "installationId": d.id.InstallationID})
	})
	frame.HandleFunc("GET /demo/identity", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != appOrigin) {
			jsonReply(w, 403, map[string]string{"error": "denied"})
			return
		}
		d.mu.RLock()
		defer d.mu.RUnlock()
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			jsonReply(w, 401, map[string]string{"error": "identity_required"})
			return
		}
		claims, err := d.verifier.Authenticate(r.Context(), strings.TrimPrefix(header, "Bearer "), "demo-client", d.id)
		if err != nil {
			jsonReply(w, 403, map[string]string{"error": "access_denied"})
			return
		}
		jsonReply(w, 200, map[string]any{"merchantId": claims.MerchantID, "actorId": claims.ActorID, "appId": claims.AppID, "installationId": claims.InstallationID, "expiresAt": claims.ExpiresAt, "status": "connected", "demo": true})
	})
	static := func(mux *http.ServeMux, page string) {
		mux.HandleFunc("GET /emisell-ui.css", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write([]byte(appui.CSS))
		})
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { serveAsset(w, "web/"+page, "text/html; charset=utf-8") })
		mux.HandleFunc("GET /demo.css", func(w http.ResponseWriter, r *http.Request) { serveAsset(w, "web/demo.css", "text/css") })
		mux.HandleFunc("GET /demo.js", func(w http.ResponseWriter, r *http.Request) {
			serveAsset(w, "web/"+strings.TrimSuffix(page, ".html")+".js", "text/javascript")
		})
		mux.HandleFunc("GET /bridge.mjs", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(contract.BrowserBridge))
		})
	}
	static(host, "host.html")
	static(frame, "app.html")
	frame.HandleFunc("GET /seller", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, "web/app.html", "text/html; charset=utf-8")
	})
	frame.Handle("GET /seller/identity", sellerIdentity(&http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}))
	return guarded(host, parentOrigin, false), guarded(frame, appOrigin, true)
}

func serveAsset(w http.ResponseWriter, path, mime string) {
	b, err := assets.ReadFile(path)
	if err != nil {
		http.Error(w, "asset unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", mime)
	_, _ = w.Write(b)
}

func guarded(next http.Handler, origin string, frame bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		ancestors := "'none'"
		if frame {
			ancestors = parentOrigin
			if r.URL.Path == "/seller" {
				ancestors = "http://localhost:3000"
			}
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-src "+appOrigin+"; frame-ancestors "+ancestors+"; base-uri 'none'; form-action 'none'")
		if r.Host != strings.TrimPrefix(origin, "http://") {
			http.Error(w, "invalid host", 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// The demo backend delegates validation to Core's loopback-only introspection.
// Only the short-lived identity travels here; never a seller cookie or API key.
func sellerIdentity(client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != appOrigin) || !strings.HasPrefix(auth, "Bearer ") || len(auth) > 4103 {
			jsonReply(w, 403, map[string]string{"error": "access_denied"})
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), "GET", "http://127.0.0.1:8000/v1/app-platform/core/embedded-demo/identity", nil)
		if err != nil {
			jsonReply(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		req.Header.Set("Authorization", auth)
		res, err := client.Do(req)
		if err != nil {
			jsonReply(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		defer res.Body.Close()
		body, err := io.ReadAll(io.LimitReader(res.Body, 8193))
		if res.StatusCode == 401 || res.StatusCode == 403 {
			jsonReply(w, 403, map[string]string{"error": "access_denied"})
			return
		}
		if err != nil || len(body) > 8192 || res.StatusCode != 200 || !json.Valid(body) {
			jsonReply(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
}

func main() {
	d, err := newDemo()
	if err != nil {
		log.Fatal("cannot initialize isolated demo")
	}
	parent, child := d.handlers()
	a, err := net.Listen("tcp4", "127.0.0.1:4320")
	if err != nil {
		log.Fatal(err)
	}
	b, err := net.Listen("tcp4", "127.0.0.1:4321")
	if err != nil {
		_ = a.Close()
		log.Fatal(err)
	}
	servers := []*http.Server{{Handler: parent, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}, {Handler: child, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	failures := make(chan error, 2)
	go func() { failures <- servers[0].Serve(a) }()
	go func() { failures <- servers[1].Serve(b) }()
	fmt.Println("Isolated embedded demo:", parentOrigin, "— synthetic identity, no merchant database or provider calls")
	select {
	case <-ctx.Done():
	case <-failures:
	}
	for _, s := range servers {
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.Shutdown(shutdown)
		cancel()
	}
}
