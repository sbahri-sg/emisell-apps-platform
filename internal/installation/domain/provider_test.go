package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"emisell.app/platform/pkg/appmanifest"
)

func TestProviderBindingIsInConsentAndNotCheckoutRouting(t *testing.T) {
	r := IntentRelease{AppID: "rajaongkir-provider-reference", ExecutionProfile: appmanifest.ShippingProviderFixtureProfile, Scopes: []string{"shipping.read"}, Capabilities: []string{"shipping/v1"}, ShippingProvider: &appmanifest.ShippingProviderBinding{Engine: "api-kurir", ProviderCode: "rajaongkir"}}
	o := IntentOwner{TenantID: "merchant", ServiceID: "core", ActorID: "owner"}
	now := time.Unix(1000, 0)
	first := NewInstallIntent("intent", o, r, now)
	r.ShippingProvider.ProviderCode = "mengantar"
	r.Scopes[0] = "modified"
	if first.Release.ShippingProvider.ProviderCode != "rajaongkir" || first.Release.Scopes[0] != "shipping.read" {
		t.Fatal("caller mutated immutable consent snapshot")
	}
	r.Scopes[0] = "shipping.read"
	r.ShippingProvider = &appmanifest.ShippingProviderBinding{Engine: "api-kurir", ProviderCode: "kiriminaja"}
	other := NewInstallIntent("intent", o, r, now)
	if first.ConsentDigest == other.ConsentDigest || len(r.RoutedCapabilities()) != 0 {
		t.Fatal("binding/routing not separated")
	}
	r.ExecutionProfile, r.ShippingProvider = "local-simulator", nil
	routed := r.RoutedCapabilities()
	if len(routed) != 1 || routed[0] != "shipping/v1" {
		t.Fatal("legacy routing changed")
	}
	routed[0] = "modified"
	if r.Capabilities[0] != "shipping/v1" {
		t.Fatal("mutable routing alias")
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "shippingProvider") {
		t.Fatal("old consent JSON shape changed")
	}
}
