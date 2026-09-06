package main

import (
	"context"
	"crypto/rand"
	referencecore "emisell.app/platform/examples/core-reference"
	apprepo "emisell.app/platform/internal/app/repository"
	caprepo "emisell.app/platform/internal/capability/postgres"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/platform/config"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/referenceapp"
	hookrepo "emisell.app/platform/internal/webhook/postgres"
	"emisell.app/platform/migrations"
	"emisell.app/platform/pkg/gatewaycontract"
	"emisell.app/platform/pkg/uirelease"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type credential struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && os.Args[1] == "init-ui-release-signing" {
		return localfiles.InitUIReleaseKey()
	}
	// Pure authoring check; this never creates a release, assignment or grant.
	if len(os.Args) == 2 && os.Args[1] == "ui-release-validate" {
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 8193))
		if err != nil {
			return err
		}
		m, err := uirelease.Decode(raw)
		if err != nil {
			return err
		}
		canonical, err := uirelease.Canonical(m)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"valid": true, "schema": m.Schema, "mode": m.Mode, "sha256": uirelease.Digest(canonical), "installable": false})
	}
	if len(os.Args) == 2 && os.Args[1] == "init-managed-shipping-signing" {
		if err := localfiles.InitManagedShippingKey(); err != nil {
			return err
		}
		fmt.Println("Private managed shipping signing key ready. Existing keys unchanged.")
		return nil
	}
	if len(os.Args) == 2 && os.Args[1] == "init-integration-signing" {
		if err := localfiles.InitIntegrationKey(); err != nil {
			return err
		}
		fmt.Println("Private local integration signing key ready. Existing keys unchanged.")
		return nil
	}
	// Documentation export is pure: no credentials, database or network access.
	if len(os.Args) == 2 && os.Args[1] == "gateway-contract" {
		return json.NewEncoder(os.Stdout).Encode(gatewaycontract.Reference())
	}
	if len(os.Args) > 1 && (os.Args[1] == "init-catalog" || os.Args[1] == "catalog-validate" || os.Args[1] == "catalog-verify") {
		return catalogCommand(os.Args[1:])
	}
	if len(os.Args) < 2 || !slices.Contains([]string{"update-admin", "init-portals", "init-local", "migrate", "init-events", "init-core", "rotate-core", "revoke-core", "outbox-status", "replay-event", "init-remote", "webhook-status", "replay-webhook", "retry-cleanup"}, os.Args[1]) {
		return fmt.Errorf("usage: cli init-portals|update-admin <email> (password via stdin)|init-local|migrate|init-events|init-core|rotate-core|revoke-core|outbox-status|replay-event <installation|capability> <event-id> <reason>")
	}
	command := os.Args[1]
	if !slices.Contains([]string{"update-admin", "replay-event", "webhook-status", "replay-webhook", "retry-cleanup"}, command) && len(os.Args) != 2 {
		return fault.Invalid
	}
	if command == "replay-event" && len(os.Args) != 5 {
		return fault.Invalid
	}
	if command == "update-admin" && len(os.Args) != 3 {
		return fault.Invalid
	}
	if command == "webhook-status" && len(os.Args) != 3 {
		return fault.Invalid
	}
	if (command == "replay-webhook" || command == "retry-cleanup") && len(os.Args) != 5 {
		return fault.Invalid
	}
	if command == "init-events" {
		if err := localfiles.InitEvents(); err != nil {
			return err
		}
		fmt.Println("Private local broker configuration ready. No existing credentials rotated.")
		return nil
	}
	cfg, err := config.Read()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database configuration failed")
	}
	defer pool.Close()
	// Maintenance does not implicitly migrate unrelated schema.
	if command == "update-admin" {
		return updateAdmin(ctx, pool, os.Args[2], os.Stdin)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		return err
	}
	if os.Args[1] == "migrate" {
		fmt.Println("Migrations verified.")
		return nil
	}
	if command == "init-portals" {
		return initPortals(ctx, pool)
	}
	if command == "init-remote" {
		if err = localfiles.InitEvents(); err != nil {
			return err
		}
		if err = localfiles.InitRemoteFiles(); err != nil {
			return err
		}
		if err = referenceapp.Init(ctx, pool); err != nil {
			return err
		}
		if err = (apprepo.Postgres{Pool: pool}).SeedRemote(ctx); err != nil {
			return err
		}
		fmt.Println("Local reference app initialized; existing installations and credentials preserved.")
		return nil
	}
	if command == "webhook-status" {
		items, err := (hookrepo.Repository{Pool: pool}).List(ctx, os.Args[2])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(items)
	}
	if command == "replay-webhook" {
		return (hookrepo.Repository{Pool: pool}).Replay(ctx, os.Args[2], os.Args[3], os.Args[4])
	}
	if command == "retry-cleanup" {
		return (installrepo.Repository{Pool: pool}).RetryCleanup(ctx, os.Args[2], os.Args[3], os.Args[4])
	}
	if slices.Contains([]string{"init-core", "rotate-core", "revoke-core"}, command) {
		return core(ctx, pool, cfg, command)
	}
	if command == "outbox-status" || command == "replay-event" {
		boxes := map[string]event.Outbox{"installation": (installrepo.Repository{Pool: pool}).Outbox(), "capability": (caprepo.Repository{Pool: pool}).Outbox()}
		if command == "replay-event" {
			box, ok := boxes[os.Args[2]]
			if !ok {
				return fault.Invalid
			}
			if err := box.Replay(ctx, os.Args[3], "local-operator", os.Args[4]); err != nil {
				return err
			}
			fmt.Println("Dead event scheduled for replay; audit recorded.")
			return nil
		}
		for _, name := range []string{"installation", "capability"} {
			counts, err := boxes[name].Counts(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("%s: pending=%d dead=%d\n", name, counts.Pending, counts.Dead)
		}
		return nil
	}
	if err = os.MkdirAll(".local", 0700); err != nil {
		return err
	}
	path := filepath.Join(".local", "login.json")
	cred := credential{Email: "owner@emisell.local", Password: rand.Text() + "-" + rand.Text()}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(raw, &cred); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		raw, err = json.MarshalIndent(cred, "", "  ")
		if err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(raw)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	} else {
		return err
	}
	if err = (identityrepo.Repository{Pool: pool}).SeedUser(ctx, "local-owner", cred.Email, cred.Password, []identity.Workspace{{ID: "local-store", Name: "Emisell Store"}, {ID: "local-studio", Name: "Nusa Studio"}}); err != nil {
		return err
	}
	if err = (apprepo.Postgres{Pool: pool}).SeedLocal(ctx); err != nil {
		return err
	}
	fmt.Println("Local data initialized. Login credentials are in .local/login.json (private, gitignored). Existing installations are preserved.")
	return nil
}

func core(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, command string) error {
	accounts := identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: pool}}
	const id = "core-local-store"
	if command == "revoke-core" {
		if err := accounts.Revoke(ctx, id, "local-operator"); err != nil {
			return err
		}
		fmt.Println("Local Core service token revoked.")
		return nil
	}
	var existing localfiles.CoreCredential
	err := localfiles.Read(".local/core.json", &existing)
	if err == nil && command == "init-core" {
		principal, err := accounts.Authenticate(ctx, existing.Token)
		if err != nil {
			return fmt.Errorf("Core token expired/revoked; use rotate-core explicitly")
		}
		if principal.ID != id || principal.TenantID != "local-store" {
			return fault.Conflict
		}
		fmt.Println("Existing Core credential preserved.")
		return referencecore.Init(ctx, pool)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var broker localfiles.EventConfig
	if err = localfiles.Read(".local/events.json", &broker); err != nil {
		return fmt.Errorf("run init-events first")
	}
	principal := identity.ServicePrincipal{ID: id, TenantID: "local-store", Scopes: []string{"payments.read", "payments.write", "shipping.read", "shipping.write"}, ExpiresAt: time.Now().Add(24 * time.Hour)}
	token, err := accounts.Issue(ctx, principal, command == "rotate-core", "local-operator")
	if err != nil {
		return err
	}
	credential := localfiles.CoreCredential{ServiceID: id, TenantID: principal.TenantID, Address: "http://" + cfg.RPCAddress, Token: token, ExpiresAt: principal.ExpiresAt, Broker: broker.Core}
	if err = localfiles.Write(".local/core.json", credential, command == "rotate-core"); err != nil {
		return fmt.Errorf("credential persistence failed; token must be explicitly rotated: %w", err)
	}
	if err = referencecore.Init(ctx, pool); err != nil {
		return err
	}
	fmt.Println("Core reference credential ready in .local/core.json (private, 24-hour expiry).")
	return nil
}
