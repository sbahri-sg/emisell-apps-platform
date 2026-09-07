package bootstrap_test

import (
	"context"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/oauth/appclient"
	"errors"
	"testing"
	"time"
)

type resourceClientRepo struct {
	appclient.Repository
	value  appclient.Client
	locked bool
}

func (r *resourceClientRepo) WithClient(_ context.Context, _ string, fn func(appclient.Client) error) error {
	r.locked = true
	defer func() { r.locked = false }()
	return fn(r.value)
}

type resourceClientRelease struct {
	binding appclient.Binding
	locked  bool
}

func (r *resourceClientRelease) WithBinding(_ context.Context, _, _ string, fn func(appclient.Binding) error) error {
	r.locked = true
	defer func() { r.locked = false }()
	return fn(r.binding)
}

func TestResourceClientSourceCurrentState(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	for _, scenario := range []string{"valid", "revoked", "expired", "secret missing", "digest drift", "organization drift", "version drift", "client drift", "profile drift", "source unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			binding := appclient.Binding{ReleaseID: "release_resource", OrganizationID: "org_resource", AppID: "app_resource", Version: "1.0.0", Digest: "digest"}
			client := &resourceClientRepo{value: appclient.Client{ID: "client_resource", Binding: binding, Status: "verified", VerifiedUntil: &future, SecretHash: "hash"}}
			signed := &resourceClientRelease{binding: binding}
			source := &reviewedSource{merchant: "merchant", release: domain.IntentRelease{AppID: binding.AppID, DeveloperID: binding.OrganizationID, Version: binding.Version, ManifestDigest: binding.Digest, InstallPolicy: domain.ResourceAppPolicy, ExecutionProfile: domain.ResourceAppPolicy, ResourceBinding: &domain.ResourceBinding{ClientID: client.value.ID, ReleaseID: binding.ReleaseID}}}
			gate := bootstrap.ResourceClientSource{Source: source, Clients: appclient.Service{Repo: client, Releases: signed}}
			switch scenario {
			case "revoked":
				client.value.Status = "revoked"
			case "expired":
				client.value.VerifiedUntil = &past
			case "secret missing":
				client.value.SecretHash = ""
			case "digest drift":
				signed.binding.Digest = "changed"
			case "organization drift":
				signed.binding.OrganizationID = "other"
			case "version drift":
				signed.binding.Version = "2.0.0"
			case "client drift":
				client.value.ID = "other"
			case "profile drift":
				source.release.ExecutionProfile = domain.ReviewedUIPolicy
			case "source unavailable":
				gate.Source = nil
			}
			called := false
			sentinel := errors.New("callback result")
			err := gate.WithRelease(context.Background(), "merchant", binding.AppID, binding.Version, func(domain.IntentRelease) error {
				called = true
				if !client.locked || !signed.locked {
					t.Fatal("locks not retained")
				}
				return sentinel
			})
			if scenario == "valid" {
				if !called || !errors.Is(err, sentinel) {
					t.Fatal("valid binding rejected", err)
				}
			} else if called || err == nil {
				t.Fatal("invalid binding accepted")
			}
			if client.locked || signed.locked {
				t.Fatal("lock leaked")
			}
		})
	}
}
