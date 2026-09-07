// Package providergrant is an independent engine-only contract, not the legacy
// local Emisell grant protocol. Enrollment never creates installation consent.
package providergrant

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"emisell.app/platform/internal/apppermission"
)

const Path = "/internal/provider-grants/v1/check"

var Denied = errors.New("provider grant denied")
var id = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@-]{0,127}$`)
var provider = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type Request struct {
	MerchantID     string `json:"merchantId"`
	ProviderCode   string `json:"providerCode"`
	AppID          string `json:"appId"`
	InstallationID string `json:"installationId"`
	Operation      string `json:"operation"`
}
type Snapshot struct {
	MerchantID     string   `json:"merchantId"`
	ProviderCode   string   `json:"providerCode"`
	AppID          string   `json:"appId"`
	InstallationID string   `json:"installationId"`
	Revision       int64    `json:"revision,string"`
	Active         bool     `json:"active"`
	Revoked        bool     `json:"revoked"`
	Scopes         []string `json:"scopes"`
}
type Source interface {
	Snapshot(context.Context, Request) (Snapshot, error)
}
type Service struct {
	Source   Source
	Enrolled map[string]string
}

func (s Service) Check(ctx context.Context, r Request) (Snapshot, error) {
	if s.Source == nil || !id.MatchString(r.MerchantID) || !id.MatchString(r.AppID) || !id.MatchString(r.InstallationID) || !provider.MatchString(r.ProviderCode) || s.Enrolled[r.AppID] != r.ProviderCode {
		return Snapshot{}, Denied
	}
	required := ""
	switch r.Operation {
	case "binding.read":
	case "rates.read", "settings.read", "tracking.read":
		required = "shipping.read"
	case "settings.write", "shipments.create":
		required = "shipping.write"
	default:
		return Snapshot{}, Denied
	}
	v, err := s.Source.Snapshot(ctx, r)
	if err != nil {
		return Snapshot{}, err
	}
	if v.MerchantID != r.MerchantID || v.AppID != r.AppID || v.InstallationID != r.InstallationID || v.ProviderCode != r.ProviderCode || v.Revision < 1 {
		return Snapshot{}, Denied
	}
	// The source supplies the intersection of consumed consent and current scopes.
	identity := apppermission.Identity{MerchantID: r.MerchantID, AppID: r.AppID, InstallationID: r.InstallationID}
	if required != "" && !apppermission.Allows(identity, apppermission.Grant{
		Identity: apppermission.Identity{MerchantID: v.MerchantID, AppID: v.AppID, InstallationID: v.InstallationID},
		Active:   v.Active, Revoked: v.Revoked, ConsentedScopes: v.Scopes, GrantedScopes: v.Scopes,
	}, []string{required}) {
		return Snapshot{}, Denied
	}
	if !v.Active || v.Revoked {
		v.Scopes = []string{}
	}
	return v, nil
}

// Handler is mounted only on an explicitly configured internal listener.
func Handler(s Service, key string) (http.Handler, error) {
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(key) || s.Source == nil || len(s.Enrolled) == 0 {
		return nil, errors.New("invalid engine grant configuration")
	}
	enrolled := map[string]string{}
	for app, p := range s.Enrolled {
		if !id.MatchString(app) || !provider.MatchString(p) {
			return nil, Denied
		}
		enrolled[app] = p
	}
	s.Enrolled = enrolled
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != Path && r.URL.Path != TargetsPath {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == Path && r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" || len(r.Header.Values("Authorization")) != 1 || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+key)) != 1 {
			w.WriteHeader(403)
			return
		}
		if r.URL.Path == TargetsPath {
			s.targets(w, r)
			return
		}
		var q Request
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if d.Decode(&q) != nil || d.Decode(&struct{}{}) != io.EOF {
			w.WriteHeader(400)
			return
		}
		v, err := s.Check(r.Context(), q)
		if errors.Is(err, Denied) {
			w.WriteHeader(403)
			return
		}
		if err != nil {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}), nil
}
