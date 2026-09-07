package bootstrap

import (
	"context"
	"emisell.app/platform/internal/installation/domain"
	"testing"
)

func TestProviderSourceRequiresLocalAdmission(t *testing.T) {
	for _, env := range []string{"", "production", "staging", "development"} {
		s := ProviderInstallSource{Environment: env}
		called := false
		err := s.WithRelease(context.Background(), "merchant", "app", "1.0.0", func(domain.IntentRelease) error { called = true; return nil })
		if err == nil || called {
			t.Fatal("incomplete or non-local source allowed", env)
		}
	}
	if _, err := InternalHandlerWithLocalProviderApps(nil, nil, nil, nil, LocalProviderInstall{}); err == nil {
		t.Fatal("incomplete composition allowed")
	}
}
