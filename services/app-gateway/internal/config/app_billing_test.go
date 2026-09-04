package config

import (
	"strings"
	"testing"
)

func TestAppBillingExplicitOptInAndNoLiveDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("REPOSITORY_DRIVER", "memory")
	t.Setenv("AUTH_OIDC_ENABLED", "false")
	t.Setenv("APP_BILLING_ENABLED", "")
	t.Setenv("APP_BILLING_LIVE_ENABLED", "")
	c, err := Load()
	if err != nil || c.AppBillingEnabled || c.AppBillingLiveEnabled {
		t.Fatal("billing must be off by default", err)
	}
	t.Setenv("APP_BILLING_ENABLED", "true")
	c, err = Load()
	if err != nil || !c.AppBillingEnabled || c.AppBillingLiveEnabled {
		t.Fatal("test mode opt-in", err)
	}
	t.Setenv("APP_BILLING_LIVE_ENABLED", "true")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "live app billing requires") {
		t.Fatal("live billing was allowed in development")
	}
	t.Setenv("APP_BILLING_LIVE_ENABLED", "false")
	t.Setenv("APP_BILLING_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Fatal("invalid feature flag accepted")
	}
}
