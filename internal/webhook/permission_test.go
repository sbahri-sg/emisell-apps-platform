package webhook

import (
	"emisell.app/platform/internal/apppermission"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/pkg/appmanifest"
	"testing"
)

func TestSubscriptionRequiresCurrentIdentityConsentGrantAndReview(t *testing.T) {
	id := apppermission.Identity{MerchantID: "merchant", AppID: "any-app", InstallationID: "installation"}
	base := apppermission.Grant{Identity: id, Active: true, ConsentedScopes: []string{"read_products"}, GrantedScopes: []string{"read_products"}}
	sub := Subscription{Identity: id, Topic: "test.product.changed.v1", ReleaseVersion: "1.0.0", Active: true, Approved: true}
	allowed := func(g apppermission.Grant, s Subscription, required []string) bool {
		return AllowsSubscription(id, g, s, sub.Topic, "1.0.0", required)
	}
	if !allowed(base, sub, []string{"read_products"}) {
		t.Fatal("valid subscription rejected")
	}
	for _, mutate := range []func(*apppermission.Grant){
		func(g *apppermission.Grant) { g.Active = false }, func(g *apppermission.Grant) { g.Revoked = true },
		func(g *apppermission.Grant) { g.MerchantID = "other" }, func(g *apppermission.Grant) { g.InstallationID = "other" },
		func(g *apppermission.Grant) { g.GrantedScopes = nil }, func(g *apppermission.Grant) { g.ConsentedScopes = nil },
	} {
		g := base
		mutate(&g)
		if allowed(g, sub, []string{"read_products"}) {
			t.Fatal("invalid grant accepted")
		}
	}
	for _, mutate := range []func(*Subscription){
		func(s *Subscription) { s.Approved = false }, func(s *Subscription) { s.Active = false }, func(s *Subscription) { s.MerchantID = "other" },
		func(s *Subscription) { s.Topic = "*" }, func(s *Subscription) { s.ReleaseVersion = "2.0.0" },
	} {
		s := sub
		mutate(&s)
		if allowed(base, s, []string{"read_products"}) {
			t.Fatal("invalid subscription accepted")
		}
	}
	for _, required := range [][]string{nil, {"*"}, {"write_products"}, {"read_products", "read_orders"}} {
		if allowed(base, sub, required) {
			t.Fatal("scope escalation", required)
		}
	}
}

func TestLocalWebhookRequiresSignedSubscriptionAndAllGrantedScopes(t *testing.T) {
	app := appmanifest.Manifest{ID: "fixture", Version: "1.0.0", ExecutionProfile: "local-remote", Subscriptions: []string{"event"}, Scopes: []string{"payments.read", "payments.write"}}
	ins := domain.Installation{ID: "installation", AppID: app.ID, Version: app.Version, Status: "active", Scopes: app.Scopes}
	if !allowsLocalWebhook("merchant", ins.ID, "event", ins, app) {
		t.Fatal("valid rejected")
	}
	ins.Scopes = []string{"payments.read"}
	if allowsLocalWebhook("merchant", ins.ID, "event", ins, app) {
		t.Fatal("revoked scope accepted")
	}
	ins.Scopes = app.Scopes
	app.Subscriptions = nil
	if allowsLocalWebhook("merchant", ins.ID, "event", ins, app) {
		t.Fatal("unsubscribed event accepted")
	}
}
