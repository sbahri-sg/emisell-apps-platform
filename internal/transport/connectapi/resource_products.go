package connectapi

import (
	"context"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/resourceclient"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Private Core delegation, not a public API or browser cookie endpoint. Both
// the live Core session (at api-service) and confidential app client are required.
func (s Server) resourceProductsHandler() http.Handler {
	slots := make(chan struct{}, 8)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fail := func(status int, code string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
		}
		if r.Method != "POST" || r.URL.RawQuery != "" || len(r.Header.Values("Authorization")) != 1 || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			fail(403, "permission_denied")
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			fail(429, "resource_exhausted")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		p, err := s.Accounts.Authenticate(ctx, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil || !p.PlatformFull {
			fail(401, "unauthenticated")
			return
		}
		var in struct {
			MerchantID     string `json:"merchantId"`
			Actor          string `json:"coreActorId"`
			InstallationID string `json:"installationId"`
			AppID          string `json:"appId"`
			ClientID       string `json:"clientId"`
			ClientSecret   string `json:"clientSecret"`
			Limit          int    `json:"limit"`
			Cursor         string `json:"cursor"`
			Q              string `json:"q"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF || in.Limit < 1 || in.Limit > 20 {
			fail(400, "invalid_argument")
			return
		}
		query := url.Values{"limit": {strconv.Itoa(in.Limit)}}
		if in.Cursor != "" {
			query.Set("cursor", in.Cursor)
		}
		if in.Q != "" {
			query.Set("q", in.Q)
		}
		if resourceclient.ValidateQuery(query, false) != nil {
			fail(400, "invalid_argument")
			return
		}
		p, err = p.BindTenant(in.MerchantID)
		if err != nil {
			fail(403, "permission_denied")
			return
		}
		binding, err := s.ResourceClients.Authenticate(ctx, in.ClientID, in.ClientSecret)
		if err != nil || binding.AppID != in.AppID {
			fail(403, "permission_denied")
			return
		}
		result, err := s.ResourceProducts.ReadForApp(ctx, s.Lifecycle, p, in.Actor, in.InstallationID, in.AppID, in.ClientID, query, ids.New("resource"))
		if err != nil {
			fail(403, "resource_access_denied")
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
}
