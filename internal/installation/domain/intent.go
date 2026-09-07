package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
	"emisell.app/platform/pkg/embedded"
)

const IntentTTL = 10 * time.Minute

const InstallPolicy = "local-reviewed-fixture/v1"

const ManagedShippingPolicy = "managed-shipping-local/v1"
const ManagedShippingProfile = "managed-shipping-local"
const ProviderAppPolicy = "provider-app/v1"
const ProviderAppProfile = "provider-app"

// Immutable provenance. A local engine grant cannot be replayed in production.
type ManagedSource struct {
	ReleaseID    string `json:"releaseId"`
	AssignmentID string `json:"assignmentId"`
	MerchantID   string `json:"merchantId"`
	Environment  string `json:"environment"`
}

// IntentOwner contains a trusted Core assertion, not a browser session.
type IntentOwner struct {
	TenantID  string `json:"tenantId"`
	ServiceID string `json:"serviceId"`
	ActorID   string `json:"actorId"`
}

type IntentRelease struct {
	InstallPolicy    string                               `json:"installPolicy,omitempty"`
	AppID            string                               `json:"appId"`
	Name             string                               `json:"name"`
	DeveloperID      string                               `json:"developerId"`
	Version          string                               `json:"version"`
	ManifestDigest   string                               `json:"manifestDigest"`
	Scopes           []string                             `json:"scopes"`
	Capabilities     []string                             `json:"capabilities"`
	ExecutionProfile string                               `json:"executionProfile"`
	ShippingProvider *appmanifest.ShippingProviderBinding `json:"shippingProvider,omitempty"`
	ManagedSource    *ManagedSource                       `json:"managedSource,omitempty"`
	UIBinding        *UIBinding                           `json:"uiBinding,omitempty"`
}

const ReviewedUIPolicy = "reviewed-ui/v1"

// UIBinding is consent provenance, not a grant inferred from a catalog URL.
type UIBinding struct {
	AssignmentID string          `json:"assignmentId,omitempty"`
	ReleaseID    string          `json:"releaseId"`
	Launch       embedded.Launch `json:"launch"`
	Signature    string          `json:"signature"`
}

// RoutedCapabilities separates provider app installation from checkout selection.
// Provider lifecycle fixtures do not participate in the legacy capability resolver.
func (r IntentRelease) RoutedCapabilities() []string {
	if r.ExecutionProfile == appmanifest.ShippingProviderFixtureProfile || r.ExecutionProfile == ManagedShippingProfile || r.ExecutionProfile == ProviderAppProfile {
		return []string{}
	}
	return slices.Clone(r.Capabilities)
}

// InstallIntent is only a consent record. It is never an active access grant.
type InstallIntent struct {
	ID            string        `json:"id"`
	Owner         IntentOwner   `json:"owner"`
	Release       IntentRelease `json:"release"`
	ConsentDigest string        `json:"consentDigest"`
	State         string        `json:"state"`
	CreatedAt     time.Time     `json:"createdAt"`
	ExpiresAt     time.Time     `json:"expiresAt"`
	DecidedAt     *time.Time    `json:"decidedAt,omitempty"`
}

func NewInstallIntent(id string, owner IntentOwner, release IntentRelease, now time.Time) InstallIntent {
	if release.UIBinding != nil {
		binding := *release.UIBinding
		release.UIBinding = &binding
	}
	release.Scopes = slices.Clone(release.Scopes)
	release.Capabilities = slices.Clone(release.Capabilities)
	if release.ShippingProvider != nil {
		binding := *release.ShippingProvider
		release.ShippingProvider = &binding
	}
	v := InstallIntent{ID: id, Owner: owner, Release: release, State: "pending", CreatedAt: now.UTC(), ExpiresAt: now.UTC().Add(IntentTTL)}
	// Hash a fixed-shape immutable snapshot, not mutable state or decision time.
	raw, _ := json.Marshal(struct {
		ID                   string
		Owner                IntentOwner
		Release              IntentRelease
		CreatedAt, ExpiresAt time.Time
	}{v.ID, v.Owner, v.Release, v.CreatedAt, v.ExpiresAt})
	v.ConsentDigest = fmt.Sprintf("%x", sha256.Sum256(raw))
	return v
}

func (v InstallIntent) Effective(now time.Time) InstallIntent {
	if (v.State == "pending" || v.State == "consented") && !now.Before(v.ExpiresAt) {
		v.State = "expired"
	}
	return v
}

func (v InstallIntent) Decide(digest, decision string, now time.Time) (InstallIntent, error) {
	if decision != "consent" && decision != "deny" {
		return InstallIntent{}, fault.Invalid
	}
	if v.Effective(now).State != "pending" {
		return InstallIntent{}, fault.Conflict
	}
	if digest != v.ConsentDigest {
		return InstallIntent{}, fault.Conflict
	}
	v.State = "denied"
	if decision == "consent" {
		v.State = "consented"
	}
	now = now.UTC()
	v.DecidedAt = &now
	return v, nil
}
