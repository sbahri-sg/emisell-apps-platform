package service

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/catalogmanifest"
	"emisell.app/platform/pkg/webhookconfig"
	"encoding/json"
	"testing"
)

func TestWebhookConfigurationBoundWithoutPublicEndpoint(t *testing.T) {
	d := AppDocument{Name: "Shipping", Summary: "Rates", Description: "Details", Version: "1.0.0", Capability: "shipping/v1", Scopes: []string{"orders.read", "shipping.read", "shipping.write"}, Endpoint: "https://example.invalid/app"}
	d.AccessScopes = &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{}}
	d.Webhooks = &webhookconfig.Config{APIVersion: webhookconfig.APIVersion, Subscriptions: []webhookconfig.Subscription{{Topics: []string{"products.created"}, URI: "https://example.com/events"}}}
	if err := d.Validate(true); err != nil {
		t.Fatal(err)
	}
	candidate := CatalogCandidate{AppID: "app_1", OrganizationID: "org_1", Document: d}
	m := catalogManifest(candidate)
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	p, err := catalogmanifest.Sign(m, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	if bytes.Contains(raw, []byte("https://example.com/events")) {
		t.Fatal("receiver leaked into public catalog")
	}
	candidate.Document.Webhooks.Subscriptions[0].URI = "https://other.com/events"
	next := catalogManifest(candidate)
	if m.SourceSHA256 == next.SourceSHA256 {
		t.Fatal("configuration not bound to source hash")
	}
	p.Manifest.SourceSHA256 = next.SourceSHA256
	if catalogmanifest.Verify(p, pub) == nil {
		t.Fatal("changed source accepted with old signature")
	}
}
