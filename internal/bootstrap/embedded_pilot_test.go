package bootstrap_test

import (
	"context"
	"errors"
	"testing"

	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	service "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
)

func TestEmbeddedPilotPersistentConsentAndAccess(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, principal, _, _ := lifecycleClient(t, f)
	pilot, err := service.NewLocalEmbeddedPilot("development", f.tenant)
	if err != nil {
		t.Fatal(err)
	}
	intents := service.Intents{Repo: installrepo.Repository{Pool: f.pool}, Apps: appservice.Registry{Repo: apprepo.Postgres{Pool: f.caps}}, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, EmbeddedPilot: pilot}
	lifecycle := service.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}, Intents: intents}
	prepare := func() domain.InstallIntent {
		v, e := intents.Prepare(ctx, principal, "staff", key(), service.PrepareIntent{AppID: service.EmbeddedPilotApp, Version: "1.0.0"})
		if e != nil {
			t.Fatal(e)
		}
		if v.Release.InstallPolicy != service.EmbeddedPilotPolicy || len(v.Release.Scopes) != 0 || len(v.Release.Capabilities) != 0 {
			t.Fatal("unexpected commerce permission")
		}
		return v
	}
	install := func() domain.Access {
		v := prepare()
		if _, e := lifecycle.Consume(ctx, principal, "staff", key(), v.ID, v.ConsentDigest); e == nil {
			t.Fatal("consent bypassed")
		}
		_, e := intents.Decide(ctx, principal, "staff", key(), service.DecideIntent{ID: v.ID, Digest: v.ConsentDigest, Decision: "consent"})
		if e != nil {
			t.Fatal(e)
		}
		k := key()
		r, e := lifecycle.Consume(ctx, principal, "staff", k, v.ID, v.ConsentDigest)
		if e != nil {
			t.Fatal(e)
		}
		retry, e := lifecycle.Consume(ctx, principal, "staff", k, v.ID, v.ConsentDigest)
		if e != nil || retry.Access.Installation.ID != r.Access.Installation.ID {
			t.Fatal("consume retry failed", e)
		}
		return r.Access
	}
	a := install()
	called := 0
	check := func(id, actor string) error {
		return lifecycle.WithEmbeddedPilotAccess(ctx, principal, actor, id, func(domain.Access) error { called++; return nil })
	}
	if check(a.Installation.ID, "staff") == nil {
		t.Fatal("pending installation launched")
	}
	_, err = lifecycle.Execute(ctx, principal, "staff", key(), a.Installation.ID, "activate")
	if err != nil || check(a.Installation.ID, "staff") != nil || called != 1 {
		t.Fatal("active access failed", err)
	}
	if check(a.Installation.ID, "other-staff") == nil {
		t.Fatal("actor isolation bypassed")
	}
	visible, err := lifecycle.Get(ctx, principal, "staff", a.Installation.ID)
	items, _, listErr := lifecycle.List(ctx, principal, "staff", "", 20)
	if listErr != nil || len(items) != 1 || items[0].ID != a.Installation.ID {
		t.Fatal("installed pilot missing from list", listErr)
	}
	if err != nil || !visible.LocalEmbeddedAccess {
		t.Fatal("live Get omitted embedded access", err)
	}
	if _, err = lifecycle.Execute(ctx, principal, "staff", key(), a.Installation.ID, "issue_token"); !errors.Is(err, fault.Forbidden) {
		t.Fatal("resource token granted", err)
	}
	// Enrollment removal blocks access immediately, without deleting receipts.
	lifecycle.Intents.EmbeddedPilot = nil
	visible, err = lifecycle.Get(ctx, principal, "staff", a.Installation.ID)
	if err != nil || visible.LocalEmbeddedAccess {
		t.Fatal("disabled enrollment retained embedded access", err)
	}
	if check(a.Installation.ID, "staff") == nil {
		t.Fatal("missing enrollment accepted")
	}
	// Uninstall must remain possible even after pilot disablement.
	_, err = lifecycle.Execute(ctx, principal, "staff", key(), a.Installation.ID, "uninstall")
	if err != nil {
		t.Fatal(err)
	}
	lifecycle.Intents.EmbeddedPilot = pilot
	if check(a.Installation.ID, "staff") == nil {
		t.Fatal("uninstalled access accepted")
	}
	replacement := install()
	if replacement.Installation.ID == a.Installation.ID {
		t.Fatal("reinstall reused identity")
	}
	if check(a.Installation.ID, "staff") == nil {
		t.Fatal("old identity gained replacement access")
	}
	// Default composition rejects new preparation even when the app ID is known.
	intents.EmbeddedPilot = nil
	if _, err = intents.Prepare(ctx, principal, "staff", key(), service.PrepareIntent{AppID: service.EmbeddedPilotApp, Version: "1.0.0"}); err == nil {
		t.Fatal("pilot enabled implicitly")
	}
	for _, environment := range []string{"", "production", "staging"} {
		if _, err = service.NewLocalEmbeddedPilot(environment, f.tenant); err == nil {
			t.Fatal("non-development enrollment accepted")
		}
	}
}
