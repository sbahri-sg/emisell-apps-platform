package application

import (
	"emisell-app-platform/services/app-gateway/internal/domain"
	"testing"
)

func TestDevelopmentResourcePilotBoundaries(t *testing.T) {
	pilot := DevelopmentResourcePilot{ReadProductsMerchants: map[string]bool{"merchant-a": true}}
	products := []domain.SnapshotScope{{Scope: "read_products", Access: "required"}}
	merchant := domain.MerchantIdentity{MerchantID: "merchant-a", Environment: domain.EnvironmentSandbox}
	if got := pilot.unavailable(products, merchant); got != "" {
		t.Fatalf("approved pilot rejected: %s", got)
	}
	if got := (DevelopmentResourcePilot{}).unavailable(products, merchant); got != "read_products" {
		t.Fatal("default pilot must be disabled")
	}
	merchant.MerchantID = "merchant-b"
	if got := pilot.unavailable(products, merchant); got != "read_products" {
		t.Fatal("unapproved merchant allowed")
	}
	merchant.MerchantID = "merchant-a"
	merchant.Environment = domain.EnvironmentProduction
	if got := pilot.unavailable(products, merchant); got != "read_products" {
		t.Fatal("production merchant allowed")
	}
	merchant.Environment = domain.EnvironmentSandbox
	for _, name := range []string{"write_products", "read_orders", "read_unknown"} {
		if got := pilot.unavailable([]domain.SnapshotScope{{Scope: name}}, merchant); got != name {
			t.Fatalf("unrelated scope allowed: %s", name)
		}
	}
	if firstUnavailableVersionScope(products) != "read_products" {
		t.Fatal("pilot must not change catalog eligibility")
	}
}

func TestDevelopmentLoopbackAppURL(t *testing.T) {
	for _, raw := range []string{"http://localhost:3014/", "http://127.0.0.1:3014/", "http://[::1]:3014/"} {
		if validateAppURL(&raw, true) != nil {
			t.Fatalf("local URL rejected: %s", raw)
		}
		if validateAppURL(&raw, false) == nil {
			t.Fatal("HTTP allowed by default")
		}
		if catalogLaunchURLAllowed(&raw) {
			t.Fatal("local HTTP app is eligible for App Store publication")
		}
	}
	for _, raw := range []string{"http://localhost.evil.invalid/", "http://192.168.0.1/", "http://localhost@evil.invalid/", "http://127.0.0.2/", "ftp://localhost/", "http://localhost/#fragment"} {
		if validateAppURL(&raw, true) == nil {
			t.Fatalf("unsafe URL allowed: %s", raw)
		}
	}
}
