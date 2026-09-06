package appclient

import (
	"context"
	"errors"
	"testing"
	"time"
)

type lockedClient struct {
	Repository
	client Client
	locked bool
}

func (r *lockedClient) WithClient(_ context.Context, _ string, fn func(Client) error) error {
	r.locked = true
	defer func() { r.locked = false }()
	return fn(r.client)
}

func TestBoundReadyCurrentState(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	b := Binding{ReleaseID: "release", OrganizationID: "org", AppID: "app", Digest: "digest"}
	base := Client{ID: "client", Binding: b, Status: "verified", VerifiedUntil: &future, SecretHash: "hash"}
	for _, tc := range []struct {
		name   string
		change func(*Client)
	}{
		{"revoked", func(c *Client) { c.Status = "revoked" }},
		{"expired", func(c *Client) { c.VerifiedUntil = &past }},
		{"no proof", func(c *Client) { c.VerifiedUntil = nil }},
		{"no secret", func(c *Client) { c.SecretHash = "" }},
		{"other release", func(c *Client) { c.Binding.ReleaseID = "other" }},
		{"other org", func(c *Client) { c.Binding.OrganizationID = "other" }},
		{"other client", func(c *Client) { c.ID = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &lockedClient{client: base}
			tc.change(&r.client)
			called := false
			err := (Service{Repo: r}).WithBoundReady(context.Background(), base.ID, b, func(Client) error { called = true; return nil })
			if err == nil || called || r.locked {
				t.Fatal("invalid client accepted or lock leaked", err)
			}
		})
	}
	r := &lockedClient{client: base}
	sentinel := errors.New("callback failed")
	err := (Service{Repo: r}).WithBoundReady(context.Background(), base.ID, b, func(Client) error {
		if !r.locked {
			t.Fatal("callback outside lock")
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) || r.locked {
		t.Fatal("failure/cleanup", err)
	}
}
