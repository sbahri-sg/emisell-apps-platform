package remote

import (
	"bytes"
	"context"
	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/sdk/events"
	"encoding/json"
	"io"
	"net/http"
	"slices"
)

type Client struct{ Connections *oauth.Service }

func (c Client) Execute(ctx context.Context, tenant, actor, ins, cap, key string, request capability.Request) (*capability.Resource, []capability.Rate, error) {
	connection, err := c.Connections.Access(ctx, tenant, ins)
	if err != nil {
		return nil, nil, err
	}
	invocation := appapi.Invocation{TenantID: tenant, InstallationID: ins, Capability: cap, IdempotencyKey: oauth.Hash(tenant + ":" + ins + ":" + actor + ":" + key), Request: request}
	raw, _ := json.Marshal(invocation)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.Connections.Config.Origin+"/v1/capabilities/invoke", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+connection.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Connections.HTTP.Do(req)
	if err != nil {
		return nil, nil, fault.Unavailable
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
	case 401:
		if err = c.Connections.Disconnect(ctx, tenant, ins); err != nil {
			return nil, nil, err
		}
		return nil, nil, fault.Unavailable
	case 400:
		return nil, nil, fault.Invalid
	case 404:
		return nil, nil, fault.NotFound
	case 409:
		return nil, nil, fault.Conflict
	default:
		return nil, nil, fault.Unavailable
	}
	var result appapi.Response
	d := json.NewDecoder(resp.Body)
	d.DisallowUnknownFields()
	if d.Decode(&result) != nil || d.Decode(&struct{}{}) != io.EOF {
		return nil, nil, fault.Unavailable
	}
	r := result.Resource
	if result.Capability != cap || result.Operation != request.Operation || result.InstallationID != ins || !result.Simulation || r == nil || len(result.Rates) != 0 || cap != "payment/v1" || !events.Token.MatchString(r.ID) || !events.Token.MatchString(r.Reference) || r.Currency != "IDR" || r.AmountMinor <= 0 || r.AmountMinor > 1_000_000_000_000 || !slices.Contains([]string{"authorized", "captured", "refunded"}, r.Status) {
		return nil, nil, fault.Unavailable
	}
	if request.Operation == "create" && (r.Reference != request.Reference || r.AmountMinor != request.AmountMinor || r.Status != "authorized") {
		return nil, nil, fault.Unavailable
	}
	if request.ResourceID != "" && r.ID != request.ResourceID {
		return nil, nil, fault.Unavailable
	}
	if (request.Operation == "capture" && r.Status != "captured") || (request.Operation == "refund" && r.Status != "refunded") {
		return nil, nil, fault.Unavailable
	}
	return r, nil, nil
}
