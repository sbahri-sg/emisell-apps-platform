package main

import (
	"context"
	"emisell.app/platform/internal/platform/config"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/platform/localhttp"
	"emisell.app/platform/internal/platform/secretbox"
	"emisell.app/platform/internal/referenceapp"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("local reference app stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	simulate := flag.String("simulate", "", "explicit local capture|refund; no real money")
	tenant := flag.String("tenant", "", "local workspace ID")
	installation := flag.String("installation", "", "connected local installation ID")
	resource := flag.String("resource", "", "payment resource ID")
	key := flag.String("key", "", "stable simulation idempotency key")
	flag.Parse()
	cfg, err := config.Read()
	if err != nil {
		return err
	}
	var remote localfiles.RemoteConfig
	if err = localfiles.Read(".local/remote-app.json", &remote); err != nil {
		return errors.New("run init-remote first")
	}
	if _, err = localhttp.Client(remote.Origin); err != nil {
		return err
	}
	origin, _ := url.Parse(remote.Origin)
	box, err := secretbox.New(remote.EncryptionKey)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("reference database configuration failed")
	}
	defer pool.Close()
	var ready bool
	if err = pool.QueryRow(ctx, "SELECT to_regclass('reference_remote.callbacks') IS NOT NULL").Scan(&ready); err != nil || !ready {
		return errors.New("run init-remote first")
	}
	app := referenceapp.Server{Pool: pool, Config: remote, Box: &box}
	if *simulate != "" {
		opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		result, err := app.Simulate(opCtx, *tenant, *installation, *resource, *simulate, *key)
		if err != nil {
			return err
		}
		fmt.Printf("Simulasi %s tersimpan; callback diantrikan. Resource: %s\n", result.Resource.Status, result.Resource.ID)
		return nil
	}
	server := &http.Server{Addr: origin.Host, Handler: app.Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 8 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	defer server.Close()
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				outcome, err := app.DeliverOne(opCtx, "http://127.0.0.1:8087")
				cancel()
				if err != nil {
					slog.Warn("reference callback retry", "reason", "database_or_key_unavailable")
				} else if outcome != "idle" {
					slog.Info("reference callback", "outcome", outcome)
				}
			}
		}
	}()
	go func() {
		slog.Info("local reference app ready", "address", origin.Host)
		done <- server.ListenAndServe()
	}()
	select {
	case err = <-done:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
