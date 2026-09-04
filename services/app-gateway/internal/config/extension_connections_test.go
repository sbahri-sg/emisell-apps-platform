package config

import (
	"strings"
	"testing"
)

func TestExtensionConnectionsDefaultOff(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("REPOSITORY_DRIVER", "memory")
	t.Setenv("AUTH_OIDC_ENABLED", "false")
	t.Setenv("EXTENSION_CONNECTIONS_ENABLED", "")
	configuration, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.ExtensionConnectionsEnabled {
		t.Fatal("managed extension credentials must default off")
	}
	t.Setenv("EXTENSION_CONNECTIONS_ENABLED", "true")
	configuration, err = Load()
	if err != nil || !configuration.ExtensionConnectionsEnabled {
		t.Fatal("explicit opt-in failed", err)
	}
	t.Setenv("EXTENSION_CONNECTIONS_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Fatal("invalid feature flag must fail")
	}
	t.Setenv("EXTENSION_CONNECTIONS_ENABLED", "true")
	t.Setenv("APP_ENV", "production")
	t.Setenv("SECRET_ENCRYPTION_KEY_BASE64", "r8PQPmYOmajyNIx3Gn4bEISAPze1pvrCs8vB/WOjjjc=")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "public development encryption key") {
		t.Fatal("production managed credentials must reject the public development key")
	}
}
