package main

import (
	"context"
	"emisell.app/platform/internal/identity"
	repo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/config"
	"emisell.app/platform/migrations"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/mail"
	"strings"
	"time"
)

// Explicit one-account provisioning; never creates demo organizations or fixtures.
func bootstrapAdmin(email string, input io.Reader) error {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return fmt.Errorf("invalid admin email")
	}
	raw, err := io.ReadAll(io.LimitReader(input, 258))
	if err != nil {
		return fmt.Errorf("password input failed")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if len(password) < 16 || len(password) > 256 {
		return fmt.Errorf("password must contain 16 to 256 bytes")
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
	if err = migrations.Verify(ctx, pool); err != nil {
		return err
	}
	if err = (repo.Repository{Pool: pool}).SeedPortal(ctx, identity.PortalPrincipal{ID: "platform-primary-admin", Email: email, Surface: "admin", Role: "administrator"}, password); err != nil {
		return err
	}
	fmt.Println("Primary administrator verified. Existing credentials are never overwritten.")
	return nil
}
