package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"emisell-app-platform/services/app-gateway/internal/config"
	"emisell-app-platform/services/app-gateway/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configuration, err := config.LoadDatabase()
	if err != nil {
		logger.Error("load database configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, database.Options{
		URL:             configuration.URL,
		MaxConnections:  configuration.MaxConnections,
		MinConnections:  configuration.MinConnections,
		ConnectTimeout:  configuration.ConnectTimeout,
		ApplicationName: "emisell-app-gateway-migrate",
	})
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		logger.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations complete")
}
