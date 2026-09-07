// Package resourcerelease verifies explicitly signed resource pilot assignments.
// A catalog approval or client-supplied flag cannot become resource authority.
package resourcerelease

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"reflect"
	"time"
)

const Schema = "emisell.resource-assignment/v1"

type Envelope struct {
	Schema      string               `json:"schema"`
	MerchantID  string               `json:"merchantId"`
	Environment string               `json:"environment"`
	ExpiresAt   time.Time            `json:"expiresAt"`
	Release     domain.IntentRelease `json:"release"`
	Client      appclient.Binding    `json:"client"`
}

type Source struct {
	Pool        *pgxpool.Pool
	PublicKey   ed25519.PublicKey
	Environment string
	Clients     appclient.Service
}

// Digest excludes its own field, but commits all scope and webhook provenance.
func Digest(r domain.IntentRelease) string {
	r.ManifestDigest = ""
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s Source) verify(e Envelope, signature []byte) error {
	if len(s.PublicKey) != ed25519.PublicKeySize || (s.Environment != "sandbox" && s.Environment != "production") {
		return fault.Unavailable
	}
	raw, err := json.Marshal(e)
	if err != nil || !ed25519.Verify(s.PublicKey, raw, signature) || e.Schema != Schema || e.Environment != s.Environment || e.MerchantID == "" || !e.ExpiresAt.After(time.Now()) {
		return fault.Forbidden
	}
	r := e.Release
	b := r.ResourceBinding
	c := e.Client
	if r.InstallPolicy != domain.ResourceAppPolicy || r.ExecutionProfile != domain.ResourceAppPolicy || b == nil || r.ManifestDigest != Digest(r) || r.ManagedSource != nil || r.UIBinding != nil || r.ShippingProvider != nil || len(r.Capabilities) != 0 {
		return fault.Forbidden
	}
	// Only the currently implemented read boundary; new scopes require evidence.
	if b.AccessScopes.Validate() != nil || !reflect.DeepEqual(r.Scopes, []string{"read_products"}) || !reflect.DeepEqual(b.AccessScopes.Canonical().Required, r.Scopes) || len(b.AccessScopes.Optional) != 0 {
		return fault.Forbidden
	}
	if b.Webhooks != nil && b.Webhooks.Validate(&b.AccessScopes) != nil {
		return fault.Forbidden
	}
	if b.ClientID == "" || b.ReleaseID == "" || c.ReleaseID != b.ReleaseID || c.OrganizationID != r.DeveloperID || c.AppID != r.AppID || c.Version != r.Version || c.Digest != r.ManifestDigest || c.Name != r.Name {
		return fault.Forbidden
	}
	return nil
}

// Lock order is assignment -> client -> installation. Revocation of either
// assignment or client waits for an in-flight read and blocks subsequent calls.
func (s Source) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if s.Pool == nil || s.Clients.Repo == nil {
		return fault.Unavailable
	}
	if fn == nil {
		return fault.Invalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var raw, signature []byte
	var releaseID string
	err = tx.QueryRow(ctx, `SELECT envelope,signature,release_id FROM platform_app.resource_release_assignments WHERE merchant_id=$1 AND app_id=$2 AND version=$3 AND status='approved' FOR SHARE`, merchant, app, version).Scan(&raw, &signature, &releaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return err
	}
	var e Envelope
	if json.Unmarshal(raw, &e) != nil || e.MerchantID != merchant || e.Release.AppID != app || e.Release.Version != version {
		return fault.Forbidden
	}
	if err = s.verify(e, signature); err != nil {
		return err
	}
	if e.Release.ResourceBinding.ReleaseID != releaseID {
		return fault.Forbidden
	}
	err = s.Clients.WithBoundReady(ctx, e.Release.ResourceBinding.ClientID, e.Client, func(appclient.Client) error { return fn(e.Release) })
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReadyResource is used inside WithRelease's retained assignment/client locks.
// It checks signed provenance again, without reacquiring non-reentrant client locks.
func (s Source) ReadyResource(ctx context.Context, r domain.IntentRelease) error {
	if s.Pool == nil || r.ResourceBinding == nil || service.ResourceMerchant(ctx) == "" {
		return fault.Unavailable
	}
	rows, err := s.Pool.Query(ctx, `SELECT envelope,signature FROM platform_app.resource_release_assignments WHERE release_id=$1 AND app_id=$2 AND version=$3 AND merchant_id=$4 AND status='approved'`, r.ResourceBinding.ReleaseID, r.AppID, r.Version, service.ResourceMerchant(ctx))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var raw, sig []byte
		if err = rows.Scan(&raw, &sig); err != nil {
			return err
		}
		var e Envelope
		if json.Unmarshal(raw, &e) == nil && reflect.DeepEqual(e.Release, r) && s.verify(e, sig) == nil {
			return nil
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return fault.Forbidden
}
