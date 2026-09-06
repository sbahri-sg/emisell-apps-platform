package embedded

import (
	"context"
	"crypto/ed25519"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/oauth/embedded/postgres"
	"emisell.app/platform/internal/platform/fault"
	contract "emisell.app/platform/pkg/embedded"
	"errors"
	"net/url"
)

type Reviews struct {
	Repo         postgres.Repository
	Clients      appclient.Service
	Key          ed25519.PrivateKey
	ParentOrigin string
}

func (s Reviews) ForClient(ctx context.Context, p identity.PortalPrincipal, id string) (*postgres.Record, error) {
	if _, _, err := s.Clients.Get(ctx, p, id); err != nil {
		return nil, err
	}
	v, err := s.Repo.ForClient(ctx, id)
	if errors.Is(err, fault.NotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

type LaunchInput struct {
	ClientID string `json:"clientId"`
	URL      string `json:"url"`
	Reason   string `json:"reason"`
	Mode     string `json:"mode,omitempty"`
}

func (s Reviews) Submit(ctx context.Context, p identity.PortalPrincipal, in LaunchInput) (postgres.Record, error) {
	if p.Surface != "developer" || p.ID == "" {
		return postgres.Record{}, fault.Forbidden
	}
	var out postgres.Record
	err := s.Clients.WithReady(ctx, p, in.ClientID, func(c appclient.Client) error {
		base, err := url.Parse(c.Binding.Endpoint)
		if err != nil {
			return fault.Invalid
		}
		launch, err := url.Parse(in.URL)
		if err != nil || launch.Scheme+"://"+launch.Host != base.Scheme+"://"+base.Host {
			return fault.Invalid
		}
		l := contract.Launch{AppID: c.Binding.AppID, ClientID: c.ID, ReleaseDigest: c.Binding.Digest, URL: in.URL, ParentOrigin: s.ParentOrigin, Mode: in.Mode}
		out, err = s.Repo.Submit(ctx, p.ID, l, in.Reason)
		return err
	})
	return out, err
}
func (s Reviews) Get(ctx context.Context, p identity.PortalPrincipal, id string) (postgres.Record, error) {
	v, err := s.Repo.Get(ctx, id)
	if err != nil {
		return v, err
	}
	// The app-client service resolves developer organization ownership. Revoked
	// records remain readable; readiness is only required to submit/approve.
	if _, _, err = s.Clients.Get(ctx, p, v.Launch.ClientID); err != nil {
		return postgres.Record{}, err
	}
	return v, nil
}
func (s Reviews) Decide(ctx context.Context, p identity.PortalPrincipal, id string, revision int, status, reason string) (postgres.Record, error) {
	if p.ID == "" || p.Surface != "admin" || p.Role != "administrator" {
		return postgres.Record{}, fault.Forbidden
	}
	v, err := s.Repo.Get(ctx, id)
	if err != nil {
		return v, err
	}
	if status != "approved" {
		return s.Repo.Review(ctx, p, id, revision, status, reason, nil)
	}
	var out postgres.Record
	err = s.Clients.WithReady(ctx, p, v.Launch.ClientID, func(c appclient.Client) error {
		if c.Binding.AppID != v.Launch.AppID || c.Binding.Digest != v.Launch.ReleaseDigest {
			return fault.Conflict
		}
		var e error
		out, e = s.Repo.Review(ctx, p, id, revision, status, reason, s.Key)
		return e
	})
	return out, err
}
