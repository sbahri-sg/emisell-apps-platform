package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"emisell-app-platform/services/app-gateway/internal/security"
)

// Isolated service tests; transaction/persistence behaviour is verified separately with PostgreSQL.
type connectionFixture struct {
	state       ports.ExtensionConnectionState
	commitError error
	accesses    int
}

func (f *connectionFixture) WithExtensionConnection(_ context.Context, sel ports.ExtensionConnectionSelector, use func(*ports.ExtensionConnectionState) error) error {
	if sel.TokenHash != "" {
		if f.state.Connection == nil || f.state.Connection.TokenHash != sel.TokenHash {
			return domain.ErrUnauthorized
		}
	} else if sel.OrganizationID != f.state.App.OrganizationID || sel.AppID != f.state.App.ID || sel.InstallationID != f.state.Installation.ID || sel.ExtensionID != f.state.Extension.ID {
		return domain.ErrNotFound
	}
	state := f.state
	state.Write, state.AccessScope = nil, ""
	if err := use(&state); err != nil {
		return err
	}
	if f.commitError != nil {
		return f.commitError
	}
	if state.Write != nil {
		f.state.Connection = state.Write
	}
	if state.AccessScope != "" {
		f.accesses++
	}
	return nil
}

func connectionServiceFixture(t *testing.T) (*ExtensionConnectionService, *connectionFixture, ProvisionExtensionConnection) {
	t.Helper()
	newID := func() string {
		id, err := ids.NewUUIDv7()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	org, app, install, ext, version := newID(), newID(), newID(), newID(), newID()
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	f := &connectionFixture{state: ports.ExtensionConnectionState{
		App:                domain.App{ID: app, OrganizationID: org, Status: domain.AppStatusActive},
		Extension:          domain.AppExtension{ID: ext, AppID: app, Status: domain.ExtensionStatusActive},
		Installation:       domain.AppInstallation{ID: install, AppID: app, MerchantID: "merchant-test-a", InstalledVersionID: version, Status: domain.InstallationStatusActive, Environment: domain.EnvironmentSandbox, GrantedScopes: []string{"read_merchant"}},
		Version:            domain.AppVersion{ID: version, Snapshot: domain.VersionSnapshot{Extensions: []domain.SnapshotExtension{{ExtensionID: ext}}, Scopes: []domain.SnapshotScope{{Scope: "read_merchant"}}}},
		OrganizationActive: true, Entitled: true,
	}}
	cipher, err := security.NewSecretBox([]byte(strings.Repeat("t", 32)))
	if err != nil {
		t.Fatal(err)
	}
	service := NewExtensionConnectionService(f, cipher, 1, ids.NewUUIDv7, func() time.Time { return now }, DevelopmentResourcePilot{})
	cmd := ProvisionExtensionConnection{Selector: ports.ExtensionConnectionSelector{OrganizationID: org, AppID: app, InstallationID: install, ExtensionID: ext}, ActorID: newID(), RuntimeName: "managed-test-runtime", Scopes: []string{"read_merchant"}, Secret: map[string]string{"apiKey": "synthetic-provider-secret"}, RuntimeExpiresAt: now.Add(time.Hour)}
	return service, f, cmd
}

func TestExtensionConnectionLifecycleAndRedaction(t *testing.T) {
	s, f, cmd := connectionServiceFixture(t)
	ctx := t.Context()
	initial, err := s.Get(ctx, cmd.Selector)
	if err != nil || initial.Mode != "external" || initial.Connection != nil {
		t.Fatal("unmanaged connection must stay external", err)
	}
	created, err := s.Provision(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.RuntimeToken) != 46 || f.state.Connection.TokenHash == created.RuntimeToken {
		t.Fatal("invalid token storage")
	}
	if strings.Contains(string(f.state.Connection.Ciphertext), cmd.Secret["apiKey"]) {
		t.Fatal("unencrypted secret")
	}
	meta, err := s.Get(ctx, cmd.Selector)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(meta)
	for _, forbidden := range []string{created.RuntimeToken, cmd.Secret["apiKey"], "tokenHash", "ciphertext", "apiKey"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatal("metadata exposed protected material")
		}
	}
	resolved, err := s.Resolve(ctx, created.RuntimeToken, "read_merchant", "unit-test-request-0001")
	if err != nil || resolved.MerchantID != "merchant-test-a" || resolved.Secret["apiKey"] != cmd.Secret["apiKey"] || f.accesses != 1 {
		t.Fatal("resolve failed", err)
	}
	if value, err := s.Provision(ctx, cmd); !errors.Is(err, domain.ErrConflict) || value.RuntimeToken != "" {
		t.Fatal("revision replay must not return a token", err)
	}
	rotated, err := s.Rotate(ctx, cmd.Selector, cmd.ActorID, 1, cmd.RuntimeExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, created.RuntimeToken, "read_merchant", "unit-test-request-0002"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("old runtime token remains valid", err)
	}
	if _, err := s.Resolve(ctx, rotated.RuntimeToken, "read_merchant", "unit-test-request-0003"); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(ctx, cmd.Selector, cmd.ActorID, 2); err != nil {
		t.Fatal(err)
	}
	if f.state.Connection.Ciphertext != nil || f.state.Connection.TokenHash != "" {
		t.Fatal("revocation retained protected material")
	}
	if _, err := s.Resolve(ctx, rotated.RuntimeToken, "read_merchant", "unit-test-request-0004"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("revoked token accepted", err)
	}
}

func TestExtensionConnectionRechecksAuthorization(t *testing.T) {
	cases := map[string]func(*connectionFixture){
		"organization suspended":          func(f *connectionFixture) { f.state.OrganizationActive = false },
		"entitlement removed":             func(f *connectionFixture) { f.state.Entitled = false },
		"app archived":                    func(f *connectionFixture) { f.state.App.Status = domain.AppStatusArchived },
		"installation suspended":          func(f *connectionFixture) { f.state.Installation.Status = domain.InstallationStatusSuspended },
		"installation uninstalled":        func(f *connectionFixture) { f.state.Installation.Status = domain.InstallationStatusUninstalled },
		"extension disabled":              func(f *connectionFixture) { f.state.Extension.Status = domain.ExtensionStatusDisabled },
		"not in installed version":        func(f *connectionFixture) { f.state.Version.Snapshot.Extensions = nil },
		"merchant scope removed":          func(f *connectionFixture) { f.state.Installation.GrantedScopes = nil },
		"installed version scope removed": func(f *connectionFixture) { f.state.Version.Snapshot.Scopes = nil },
		"connection allowlist removed":    func(f *connectionFixture) { f.state.Connection.Scopes = []string{"read_orders"} },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			s, f, cmd := connectionServiceFixture(t)
			created, err := s.Provision(t.Context(), cmd)
			if err != nil {
				t.Fatal(err)
			}
			change(f)
			value, err := s.Resolve(t.Context(), created.RuntimeToken, "read_merchant", "unit-negative-request-01")
			if !errors.Is(err, domain.ErrForbidden) || value.Secret != nil || f.accesses != 0 {
				t.Fatal("unauthorized credential release", err)
			}
		})
	}
}

func TestExtensionConnectionFailureDoesNotReleaseSecrets(t *testing.T) {
	for _, kind := range []string{"expiry", "key version", "ciphertext swap", "audit commit"} {
		t.Run(kind, func(t *testing.T) {
			s, f, cmd := connectionServiceFixture(t)
			created, err := s.Provision(t.Context(), cmd)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "expiry":
				f.state.Connection.RuntimeExpiresAt = s.now()
			case "key version":
				f.state.Connection.KeyVersion = 2
			case "ciphertext swap":
				f.state.Connection.InstallationID = "00000000-0000-4000-8000-000000000001"
				f.state.Installation.ID = f.state.Connection.InstallationID
			case "audit commit":
				f.commitError = errors.New("synthetic commit failure")
			}
			value, err := s.Resolve(t.Context(), created.RuntimeToken, "read_merchant", "unit-failure-request-01")
			if err == nil || value.Secret != nil || f.accesses != 0 {
				t.Fatal("failed request exposed secret")
			}
		})
	}
	s, f, cmd := connectionServiceFixture(t)
	f.commitError = errors.New("synthetic commit failure")
	if value, err := s.Provision(t.Context(), cmd); err == nil || value.RuntimeToken != "" {
		t.Fatal("failed commit exposed token")
	}
}

func TestExtensionConnectionRejectsUnavailableScopesAndInvalidProvision(t *testing.T) {
	for _, kind := range []string{"runtime whitespace", "expiry", "too long expiry", "empty secret", "large secret", "duplicate scopes", "unavailable scope"} {
		t.Run(kind, func(t *testing.T) {
			s, f, cmd := connectionServiceFixture(t)
			switch kind {
			case "runtime whitespace":
				cmd.RuntimeName = "   "
			case "expiry":
				cmd.RuntimeExpiresAt = s.now()
			case "too long expiry":
				cmd.RuntimeExpiresAt = s.now().Add(31 * 24 * time.Hour)
			case "empty secret":
				cmd.Secret = nil
			case "large secret":
				cmd.Secret = map[string]string{"apiKey": strings.Repeat("x", 8192)}
			case "duplicate scopes":
				cmd.Scopes = []string{"read_merchant", "read_merchant"}
			case "unavailable scope":
				cmd.Scopes = []string{"read_products"}
				f.state.Installation.GrantedScopes = cmd.Scopes
				f.state.Version.Snapshot.Scopes = []domain.SnapshotScope{{Scope: "read_products"}}
			}
			if value, err := s.Provision(t.Context(), cmd); err == nil || value.RuntimeToken != "" || f.state.Connection != nil {
				t.Fatal("invalid provision accepted")
			}
		})
	}
}
