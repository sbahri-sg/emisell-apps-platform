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
			Path           string `json:"path"`
			Status         string `json:"status"`
			UpdatedAfter   string `json:"updatedAfter"`
			View           string `json:"view"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF || in.Limit < 1 || in.Limit > 20 {
			fail(400, "invalid_argument")
			return
		}
		if in.Path == "" {
			in.Path = "/v1/products"
		}
		query := url.Values{}
		if in.View != "" {
			query.Set("view", in.View)
		}
		_, detailID := resourceclient.ResourceOperation(in.Path, query)
		if detailID == "" {
			query.Set("limit", strconv.Itoa(in.Limit))
		}
		if in.Cursor != "" {
			query.Set("cursor", in.Cursor)
		}
		if in.Q != "" {
			query.Set("q", in.Q)
		}
		if in.Status != "" {
			query.Set("status", in.Status)
		}
		if in.UpdatedAfter != "" {
			query.Set("updatedAfter", in.UpdatedAfter)
		}
		if resourceclient.ValidateResourceQuery(in.Path, query) != nil {
			fail(400, "invalid_argument")
			return
		}
		p, err = p.BindTenant(in.MerchantID)
		if err != nil {
			fail(403, "permission_denied")
			return
		}
		var authenticatedApp string
		if strings.HasPrefix(in.ClientID, "eai_") {
			if s.PrivateResourceAuth == nil {
				fail(403, "permission_denied")
				return
			}
			authenticatedApp, err = s.PrivateResourceAuth(ctx, in.ClientID, in.ClientSecret)
		} else {
			binding, authErr := s.ResourceClients.Authenticate(ctx, in.ClientID, in.ClientSecret)
			authenticatedApp, err = binding.AppID, authErr
		}
		if err != nil || authenticatedApp != in.AppID {
			fail(403, "permission_denied")
			return
		}
		result, err := s.ResourceProducts.ReadExistingForApp(ctx, s.Lifecycle, p, in.Actor, in.InstallationID, in.AppID, in.ClientID, in.Path, query, ids.New("resource"))
		if err != nil {
			fail(403, "resource_access_denied")
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
}
