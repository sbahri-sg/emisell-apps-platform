package domain

import (
	"errors"
	"testing"
	"time"

	"emisell.app/platform/internal/platform/fault"
)

func TestInstallIntentSnapshotAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	owner := IntentOwner{TenantID: "tenant", ServiceID: "core", ActorID: "staff"}
	release := IntentRelease{AppID: "app", Version: "1.0.0", ManifestDigest: "digest", Scopes: []string{"orders.read"}}
	v := NewInstallIntent("intent", owner, release, now)
	if v.ExpiresAt.Sub(v.CreatedAt) != IntentTTL || v.State != "pending" || len(v.ConsentDigest) != 64 {
		t.Fatal("invalid initial snapshot", v)
	}
	variants := []InstallIntent{
		NewInstallIntent("other", owner, release, now),
		NewInstallIntent("intent", IntentOwner{TenantID: "other", ServiceID: "core", ActorID: "staff"}, release, now),
		NewInstallIntent("intent", IntentOwner{TenantID: "tenant", ServiceID: "other", ActorID: "staff"}, release, now),
		NewInstallIntent("intent", IntentOwner{TenantID: "tenant", ServiceID: "core", ActorID: "other"}, release, now),
		NewInstallIntent("intent", owner, IntentRelease{AppID: "app", Version: "2.0.0"}, now),
		NewInstallIntent("intent", owner, release, now.Add(time.Second)),
	}
	for _, other := range variants {
		if v.ConsentDigest == other.ConsentDigest {
			t.Fatal("digest not bound to snapshot")
		}
	}
	for _, decision := range []string{"consent", "deny"} {
		if _, err := v.Decide(v.ConsentDigest, decision, v.ExpiresAt); !errors.Is(err, fault.Conflict) {
			t.Fatal("expiry boundary accepted", err)
		}
	}
	if _, err := v.Decide("wrong", "consent", now); !errors.Is(err, fault.Conflict) {
		t.Fatal("wrong digest accepted")
	}
	if _, err := v.Decide(v.ConsentDigest, "unspecified", now); !errors.Is(err, fault.Invalid) {
		t.Fatal("unknown decision accepted")
	}
	approved, err := v.Decide(v.ConsentDigest, "consent", now.Add(time.Second))
	if err != nil || approved.State != "consented" || approved.DecidedAt == nil || approved.ConsentDigest != v.ConsentDigest {
		t.Fatal("invalid consent", approved, err)
	}
	if _, err = approved.Decide(v.ConsentDigest, "deny", now.Add(2*time.Second)); !errors.Is(err, fault.Conflict) {
		t.Fatal("second decision accepted")
	}
	if approved.Effective(v.ExpiresAt).State != "expired" || approved.State != "consented" {
		t.Fatal("expiry projection mutated persisted value")
	}
	denied, err := v.Decide(v.ConsentDigest, "deny", now)
	if err != nil || denied.Effective(v.ExpiresAt).State != "denied" {
		t.Fatal("denial should remain terminal")
	}
}
