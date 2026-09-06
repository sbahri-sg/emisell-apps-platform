package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io/fs"
	"sort"
)

//go:embed *.sql
var files embed.FS

// Verify refuses to start against stale or edited migrations without applying DDL.
func Verify(ctx context.Context, pool *pgxpool.Pool) error {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	for _, name := range names {
		body, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		var existing string
		if err = pool.QueryRow(ctx, "SELECT checksum FROM platform_migrations WHERE name=$1", name).Scan(&existing); err != nil {
			return fmt.Errorf("schema not current; run cli migrate")
		}
		if existing != fmt.Sprintf("%x", sha256.Sum256(body)) {
			return fmt.Errorf("migration checksum mismatch: %s", name)
		}
	}
	return nil
}

// Apply is explicit: the server never silently changes the schema.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(59141001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS platform_migrations(name text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(body))
		var existing string
		rows, err := tx.Query(ctx, "SELECT checksum FROM platform_migrations WHERE name=$1", name)
		if err != nil {
			return err
		}
		if rows.Next() {
			if err = rows.Scan(&existing); err != nil {
				rows.Close()
				return err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if existing != "" {
			if existing != sum {
				return fmt.Errorf("migration checksum mismatch: %s", name)
			}
			continue
		}
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO platform_migrations(name,checksum) VALUES($1,$2)", name, sum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
