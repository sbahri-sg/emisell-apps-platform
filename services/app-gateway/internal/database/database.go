package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	URL             string
	MaxConnections  int32
	MinConnections  int32
	ConnectTimeout  time.Duration
	ApplicationName string
}

func Open(ctx context.Context, options Options) (*pgxpool.Pool, error) {
	configuration, err := pgxpool.ParseConfig(options.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	configuration.MaxConns = options.MaxConnections
	configuration.MinConns = options.MinConnections
	configuration.ConnConfig.ConnectTimeout = options.ConnectTimeout
	configuration.ConnConfig.RuntimeParams["application_name"] = options.ApplicationName
	configuration.ConnConfig.RuntimeParams["timezone"] = "UTC"

	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func BootstrapDevelopmentIdentity(ctx context.Context, pool *pgxpool.Pool, organizationID, userID, role, userEmail, displayName string) error {
	organizationSlug := "local-" + strings.ReplaceAll(organizationID, "-", "")
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin development identity bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1::uuid, 'Emisell Local Development', $2)
		ON CONFLICT (id) DO NOTHING`, organizationID, organizationSlug); err != nil {
		return fmt.Errorf("bootstrap development organization: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, display_name)
		VALUES ($1::uuid, $2, $3)
		ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, display_name = EXCLUDED.display_name, updated_at = now()`, userID, userEmail, displayName); err != nil {
		return fmt.Errorf("bootstrap development user: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (organization_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = now()`, organizationID, userID, role); err != nil {
		return fmt.Errorf("bootstrap development membership: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit development identity bootstrap: %w", err)
	}
	return nil
}
