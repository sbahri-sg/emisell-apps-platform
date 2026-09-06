package embedded

import (
	"context"
	"crypto/ed25519"
	"net/url"

	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/oauth/embedded/postgres"
	"emisell.app/platform/internal/platform/fault"
)

// CurrentBinding joins OAuth-owned client and launch state. It does not resolve
// distribution eligibility: the caller holds the signed release/assignment locks
// before invoking WithBinding, and checks installation authorization inside fn.
type CurrentBinding struct {
	Clients      appclient.Service
	Launches     postgres.Repository
	Key          ed25519.PublicKey
	ParentOrigin string
}

func (s CurrentBinding) WithBinding(ctx context.Context, client string, release appclient.Binding, fn func(Binding) error) error {
	if fn == nil || len(s.Key) != ed25519.PublicKeySize || s.ParentOrigin == "" {
		return fault.Unavailable
	}
	return s.Clients.WithBoundReady(ctx, client, release, func(c appclient.Client) error {
		return s.Launches.WithApproved(ctx, c.Binding.AppID, c.ID, c.Binding.Digest, s.Key, func(r postgres.Record) error {
			endpoint, err := url.Parse(c.Binding.Endpoint)
			launch, launchErr := url.Parse(r.Launch.URL)
			if err != nil || launchErr != nil || endpoint.Scheme != "https" || endpoint.Host == "" ||
				endpoint.Scheme != launch.Scheme || endpoint.Host != launch.Host || r.Launch.ParentOrigin != s.ParentOrigin {
				return fault.Forbidden
			}
			return fn(Binding{Launch: r.Launch, Signature: r.Signature})
		})
	})
}
