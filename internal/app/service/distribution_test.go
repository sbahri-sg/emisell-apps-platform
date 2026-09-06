package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"emisell.app/platform/pkg/catalogmanifest"
	"emisell.app/platform/pkg/integrationmanifest"
)

func TestPaymentSchemaRemainsValidButDistributionIsDenied(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	m := integrationmanifest.Manifest{Schema: integrationmanifest.Schema, Policy: integrationmanifest.Policy, SubmissionID: "sub_historical", Metadata: catalogmanifest.Manifest{Schema: catalogmanifest.Schema, Policy: catalogmanifest.Policy, AppID: "app_historical", DeveloperID: "org_historical", Name: "Legacy payment", Summary: "Historical fixture", Description: "Historical signed payment configuration", Version: "1.0.0", Capability: "payment/v1", Scopes: []string{"orders.read", "payments.read", "payments.write"}, Runtime: "remote", Pricing: "free", SourceSHA256: strings.Repeat("a", 64)}, Config: integrationmanifest.Config{Protocol: integrationmanifest.Protocol, Endpoint: "https://app.example.com/api", CallbackURL: "https://app.example.com/oauth", HealthURL: "https://app.example.com/health"}}
	p, err := integrationmanifest.Sign(m, key)
	if err != nil || integrationmanifest.Verify(p, pub) != nil {
		t.Fatal("legacy signature incompatible", err)
	}
	if !integrationmanifest.Inspect(m).Valid {
		t.Fatal("schema was incorrectly changed")
	}
	r := distributionReport(integrationmanifest.Inspect(m), m.Metadata.Capability)
	if r.Valid || r.Installable || len(r.Blockers) == 0 {
		t.Fatal("payment eligible for public distribution")
	}
	if _, err = catalogmanifest.Sign(m.Metadata, key); err != nil {
		t.Fatal("legacy catalog incompatible", err)
	}
	for _, c := range []string{"payment/v1", "", "shipping/v2", "unknown"} {
		if PublicDistributionAllowed(c) || ValidatePublicDistribution(c) == nil {
			t.Fatal("non-public capability accepted", c)
		}
	}
	if !PublicDistributionAllowed("shipping/v1") || ValidatePublicDistribution("shipping/v1") != nil {
		t.Fatal("shipping blocked")
	}
}
