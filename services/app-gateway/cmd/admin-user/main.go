// Admin account lifecycle is deliberately a server-side CLI, not public signup.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"emisell-app-platform/services/app-gateway/internal/adapters/postgres"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/config"
	"emisell-app-platform/services/app-gateway/internal/database"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/security"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readPassword(stdin bool, minimum int) (string, error) {
	if stdin {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			return "", fmt.Errorf("--password-stdin requires a pipe, not an interactive terminal")
		}
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 258))
		if err != nil {
			return "", err
		}
		return strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r"), nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("use an interactive terminal or --password-stdin from a secret manager")
	}
	fmt.Fprintf(os.Stderr, "Password (minimum %d characters, hidden): ", minimum)
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", fmt.Errorf("passwords do not match")
	}
	return string(first), nil
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: admin-user <migrate|create|reset-password|disable> --email EMAIL [--name NAME] [--password-stdin]")
	}
	action := os.Args[1]
	if action != "migrate" && action != "create" && action != "reset-password" && action != "disable" {
		return fmt.Errorf("unknown admin-user command")
	}
	flags := flag.NewFlagSet("admin-user", flag.ContinueOnError)
	email := flags.String("email", "", "admin account email")
	name := flags.String("name", "", "display name (create only)")
	platformOrg := flags.String("organization-id", os.Getenv("ADMIN_PLATFORM_ORGANIZATION_ID"), "internal Emisell organization (create only)")
	stdin := flags.Bool("password-stdin", false, "read password from a pipe, never a command-line argument")
	weakDevelopmentPassword := flags.Bool("allow-weak-development-password", false, "allow 12 characters only when APP_ENV=development")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *weakDevelopmentPassword && os.Getenv("APP_ENV") != "development" {
		return fmt.Errorf("weak-password override is available only when APP_ENV=development")
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	var normalized, hash string
	var err error
	if action != "migrate" {
		if action == "create" && *platformOrg == "" && os.Getenv("APP_ENV") == "development" {
			*platformOrg = os.Getenv("DEVELOPMENT_ORGANIZATION_ID")
		}
		if action == "create" && *platformOrg == "" {
			return fmt.Errorf("--organization-id or ADMIN_PLATFORM_ORGANIZATION_ID is required")
		}
		normalized, err = application.NormalizeAdminEmail(*email)
		if err != nil {
			return err
		}
		if action == "create" && (strings.TrimSpace(*name) == "" || utf8.RuneCountInString(*name) > 120) {
			return fmt.Errorf("name must be 1–120 characters")
		}
		if action == "create" || action == "reset-password" {
			minimum := 15
			if *weakDevelopmentPassword {
				minimum = 12
			}
			password, err := readPassword(*stdin, minimum)
			if err != nil {
				return err
			}
			if *weakDevelopmentPassword {
				hash, err = security.HashDevelopmentPassword(password)
			} else {
				hash, err = security.HashPassword(password)
			}
			if err != nil {
				return err
			}
		}
	}
	cfg, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("invalid database configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, database.Options{URL: cfg.URL, MaxConnections: 2, ConnectTimeout: cfg.ConnectTimeout, ApplicationName: "emisell-admin-user"})
	if err != nil {
		return fmt.Errorf("could not connect to the configured database")
	}
	defer pool.Close()
	if action == "migrate" {
		if err := database.MigrateAdminLogin(ctx, pool); err != nil {
			return fmt.Errorf("admin migration failed; ensure core schema exists")
		}
		fmt.Println("Admin login schema is ready. No other feature migrations were applied.")
		return nil
	}
	repo := postgres.NewRepository(pool, ids.NewUUIDv7, time.Now)
	if action == "create" {
		id, err := ids.NewUUIDv7()
		if err != nil {
			return err
		}
		err = repo.CreateAdminAccount(ctx, domain.AdminAccount{UserID: id, OrganizationID: *platformOrg, Email: normalized, DisplayName: strings.TrimSpace(*name), PasswordHash: hash})
		if errors.Is(err, domain.ErrConflict) {
			return fmt.Errorf("email already belongs to an account; existing users are not promoted automatically")
		}
		if err != nil {
			return fmt.Errorf("could not create admin; ensure the admin schema is migrated")
		}
	} else {
		err = repo.UpdateAdminPassword(ctx, normalized, hash, action == "disable", time.Now().UTC())
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("admin account not found")
		}
		if err != nil {
			return fmt.Errorf("could not update admin account")
		}
	}
	fmt.Printf("Admin operation %s completed for %s.\n", action, normalized)
	if action != "create" {
		fmt.Println("All existing admin sessions for this account have been revoked. Resetting a password does not re-enable a disabled account.")
	}
	return nil
}
