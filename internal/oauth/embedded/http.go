package embedded

import (
	"context"
	identity "emisell.app/platform/pkg/embedded"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

var ErrDenied = errors.New("embedded access denied")
var ErrUnauthenticated = errors.New("embedded session missing")

// BrowserIdentity must authenticate the Core session, validate CSRF, current
// staff membership and permission, and resolve the installation ID to identity.
// Never construct merchant/actor/app identity from request body fields.
type BrowserIdentity func(context.Context, *http.Request, string) (identity.Identity, error)

// LaunchHandler is a mountable Core BFF adapter, not mounted in Platform portals.
// Deploy only with a real BrowserIdentity and current-release resolver.
func LaunchHandler(launcher Launcher, parentOrigin string, auth BrowserIdentity) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		fail := func(status int) { w.WriteHeader(status); _, _ = w.Write([]byte(`{"error":"embedded_launch_failed"}`)) }
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			fail(405)
			return
		}
		if parentOrigin == "" || r.Header.Get("Origin") != parentOrigin || r.URL.RawQuery != "" {
			fail(403)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			fail(415)
			return
		}
		if auth == nil {
			fail(503)
			return
		}
		var body struct {
			InstallationID string `json:"installationId"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		d.DisallowUnknownFields()
		var extra any
		if d.Decode(&body) != nil || d.Decode(&extra) != io.EOF || body.InstallationID == "" {
			fail(400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		id, err := auth(ctx, r, body.InstallationID)
		if err == nil && id.InstallationID != body.InstallationID {
			err = ErrDenied
		}
		var session Session
		if err == nil {
			session, err = launcher.Open(ctx, id, parentOrigin)
		}
		if err != nil {
			switch {
			case errors.Is(err, ErrUnauthenticated):
				fail(401)
			case errors.Is(err, ErrDenied), errors.Is(err, identity.ErrInvalid):
				fail(403)
			default:
				fail(503)
			}
			return
		}
		_ = json.NewEncoder(w).Encode(session)
	})
}
