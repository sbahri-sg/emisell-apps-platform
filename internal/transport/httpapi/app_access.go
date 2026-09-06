package httpapi

import (
	"net/http"
	"strings"

	"emisell.app/platform/internal/oauth/apptoken"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/merchantid"
)

// No browser, admin session or Core key can substitute for an app token.
func (s Server) appInstallationAccess(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" || r.URL.RawQuery != "" {
		s.fail(w, r, fault.Forbidden)
		return
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		s.fail(w, r, fault.Unauthenticated)
		return
	}
	merchant, err := merchantid.Resolve(r.Header.Get("X-Emisell-Merchant-ID"), r.Header.Get("X-Emisell-Tenant-ID"))
	if err != nil || len(r.Header.Values("X-Emisell-Merchant-ID")) > 1 || len(r.Header.Values("X-Emisell-Tenant-ID")) > 1 {
		s.fail(w, r, fault.Invalid)
		return
	}
	v, err := s.AppAccess.CheckAppToken(r.Context(), strings.TrimPrefix(header, "Bearer "), merchant, r.Header.Get("X-Emisell-App-ID"), r.Header.Get("X-Emisell-Installation-ID"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Explicit public DTO: no actor/service IDs, consent snapshot or secret material.
	out := map[string]any{"merchantId": v.Owner.TenantID, "appId": v.Release.AppID, "installationId": v.Installation.ID, "version": v.Release.Version, "status": v.Installation.Status, "grantState": v.GrantState, "scopes": v.GrantedScopes, "audience": apptoken.Audience, "resourceGatewayAllowed": false}
	if r.Header.Get("X-Emisell-Merchant-ID") == "" && r.Header.Get("X-Emisell-Tenant-ID") != "" {
		delete(out, "merchantId")
		out["tenantId"] = v.Owner.TenantID // Only legacy callers receive the alias.
	}
	write(w, 200, out)
}
