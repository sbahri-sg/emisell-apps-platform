package oauth

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appapi"
	"encoding/json"
	"time"
)

// VerifyCallback must run under the lifecycle gate. Access-token expiry does
// not invalidate the installation signing key; disconnect/revoke does.
func (s *Service) VerifyCallback(ctx context.Context, tenant, ins, delivery, timestamp, signature string, body []byte) error {
	status, sealed, err := s.Repo.Load(ctx, tenant, ins)
	if err != nil {
		return err
	}
	if status != "connected" {
		return fault.Unauthenticated
	}
	raw, err := s.Box.Open("connection:"+tenant+":"+ins, sealed)
	if err != nil {
		return fault.Unavailable
	}
	var c credentials
	if json.Unmarshal(raw, &c) != nil {
		return fault.Unavailable
	}
	if !appapi.VerifyCallback(c.WebhookSecret, tenant, ins, delivery, timestamp, signature, body, time.Now()) {
		return fault.Unauthenticated
	}
	return nil
}
