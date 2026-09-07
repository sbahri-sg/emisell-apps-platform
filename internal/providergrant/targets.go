package providergrant

import (
	"context"
	"encoding/json"
	"net/http"
)

const TargetsPath = "/internal/provider-grants/v1/targets"

type Target struct {
	MerchantID     string `json:"merchantId"`
	ProviderCode   string `json:"providerCode"`
	AppID          string `json:"appId"`
	InstallationID string `json:"installationId"`
}
type TargetPage struct {
	Targets []Target `json:"targets"`
	Next    string   `json:"next"`
}
type TargetSource interface {
	Targets(context.Context, map[string]string, string) (TargetPage, error)
}

func (s Service) targets(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	q := r.URL.Query()
	after := q.Get("after")
	if len(q) > 1 || len(q["after"]) > 1 || (len(q) == 1 && len(q["after"]) == 0) || (after != "" && !id.MatchString(after)) {
		w.WriteHeader(400)
		return
	}
	source, ok := s.Source.(TargetSource)
	if !ok {
		w.WriteHeader(503)
		return
	}
	v, err := source.Targets(r.Context(), s.Enrolled, after)
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func (p Postgres) Targets(ctx context.Context, enrolled map[string]string, after string) (TargetPage, error) {
	result := TargetPage{Targets: []Target{}}
	rows, err := p.Pool.Query(ctx, `SELECT i.tenant_id,i.app_id,i.id,c.release->'shippingProvider'->>'providerCode'
 FROM platform_installation.installations i JOIN platform_installation.intent_consumptions c
 ON c.tenant_id=i.tenant_id AND c.installation_id=i.id AND c.intent_id=i.intent_id
 JOIN platform_installation.access_grants g ON g.tenant_id=i.tenant_id AND g.installation_id=i.id
 WHERE i.id>$1 AND c.release->>'appId'=i.app_id AND c.release->'shippingProvider'->>'engine'='api-kurir'
 AND c.release->'shippingProvider'->>'providerCode'=($2::jsonb->>i.app_id) ORDER BY i.id LIMIT 100`, after, enrolled)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Target
		if err = rows.Scan(&t.MerchantID, &t.AppID, &t.InstallationID, &t.ProviderCode); err != nil {
			return result, err
		}
		result.Targets = append(result.Targets, t)
	}
	if len(result.Targets) == 100 {
		result.Next = result.Targets[99].InstallationID
	}
	return result, rows.Err()
}
