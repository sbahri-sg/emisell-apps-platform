package domain

import (
	"emisell.app/platform/pkg/appmanifest"
	"testing"
)

func TestLifecycleConsentAndRevocation(t *testing.T) {
	app := appmanifest.Manifest{ID: "pay", Version: "1.0.0", Scopes: []string{"payments.read", "payments.write"}, Capabilities: []string{"payment/v1"}}
	action := Action{Type: "install", AppID: app.ID, Version: app.Version, Grants: []string{"payments.read"}}
	if _, _, err := Transition(nil, app, action); err == nil {
		t.Fatal("missing consent accepted")
	}
	action.Grants = app.Scopes
	first, changed, err := Transition(nil, app, action)
	if err != nil || !changed || first.Status != "pending" {
		t.Fatalf("install: %v", err)
	}
	replay, changed, err := Transition(first, app, action)
	if err != nil || changed || replay.ID != first.ID {
		t.Fatal("duplicate install")
	}
	active, _, err := Transition(first, app, Action{Type: "activate"})
	if err != nil || active.Status != "active" {
		t.Fatal("activate")
	}
	removed, _, err := Transition(active, app, Action{Type: "uninstall"})
	if err != nil || len(removed.Scopes) != 0 || len(removed.Capabilities) != 0 {
		t.Fatal("access not revoked")
	}
	if _, _, err = Transition(removed, app, Action{Type: "activate"}); err == nil {
		t.Fatal("uninstalled reactivated")
	}
	next, _, err := Transition(removed, app, action)
	if err != nil || next.ID == first.ID {
		t.Fatal("reinstall must have a new identity")
	}
	if active.Status != "active" || len(active.Scopes) != 2 {
		t.Fatal("transition mutated original")
	}
}
