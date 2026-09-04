package config

import "testing"

func TestDevelopmentPilotConfiguration(t *testing.T) {
	t.Setenv("DEVELOPMENT_LOOPBACK_APP_HTTP", "")
	t.Setenv("EMISELL_RESOURCE_TEST_MERCHANT_IDS", "")
	pilot, local, err := LoadDevelopmentPilot("production", false)
	if err != nil || local || len(pilot.ReadProductsMerchants) != 0 {
		t.Fatal("pilot must default off")
	}
	t.Setenv("DEVELOPMENT_LOOPBACK_APP_HTTP", "true")
	if _, _, err := LoadDevelopmentPilot("production", true); err == nil {
		t.Fatal("production accepted local HTTP")
	}
	t.Setenv("DEVELOPMENT_LOOPBACK_APP_HTTP", "false")
	t.Setenv("EMISELL_RESOURCE_TEST_MERCHANT_IDS", "merchant-a,merchant-b")
	if _, _, err := LoadDevelopmentPilot("production", true); err == nil {
		t.Fatal("production accepted test merchants")
	}
	if _, _, err := LoadDevelopmentPilot("development", false); err == nil {
		t.Fatal("pilot accepted missing adapter")
	}
	pilot, _, err = LoadDevelopmentPilot("development", true)
	if err != nil || !pilot.ReadProductsMerchants["merchant-a"] || !pilot.ReadProductsMerchants["merchant-b"] {
		t.Fatal("valid pilot rejected")
	}
	t.Setenv("EMISELL_RESOURCE_TEST_MERCHANT_IDS", "merchant-a,")
	if _, _, err := LoadDevelopmentPilot("development", true); err == nil {
		t.Fatal("empty merchant ID accepted")
	}
}
