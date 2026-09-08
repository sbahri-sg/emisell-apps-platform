package main

import (
	"context"
	"crypto/ed25519"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/oauth/endpointproof"
	"emisell.app/platform/internal/platform/config"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/providergrant"
	"emisell.app/platform/internal/resourceclient"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/internal/transport/connectapi"
	"emisell.app/platform/migrations"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg, err := config.Read()
	if err != nil {
		return err
	}
	proofVerifier, err := readProofVerifier()
	if err != nil {
		return err
	}
	if os.Getenv("EMISELL_ENV") == "production" {
		for _, name := range []string{"remote-platform.json", "managed-engine.json", "reviewed-ui.json", "embedded-pilot.json"} {
			if _, e := os.Stat(".local/" + name); !errors.Is(e, os.ErrNotExist) {
				return errors.New("production cannot load local simulator or pilot configuration")
			}
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("database configuration failed")
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return errors.New("database unavailable; start local PostgreSQL")
	}
	// Readiness requires the explicit migration command to have completed.
	if err = migrations.Verify(ctx, pool); err != nil {
		return err
	}
	caps, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("capability database configuration failed")
	}
	defer caps.Close()
	clientPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("app-client database configuration failed")
	}
	defer clientPool.Close()
	connectionsPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("connection database configuration failed")
	}
	defer connectionsPool.Close()
	var remoteConfig localfiles.RemoteConfig
	var connections *oauth.Service
	if err = localfiles.Read(".local/remote-platform.json", &remoteConfig); err == nil {
		connections, err = bootstrap.Connections(pool, connectionsPool, remoteConfig)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tp := sdktrace.NewTracerProvider()
	var signer appservice.CatalogSigner
	key, keyErr := localfiles.ReadCatalogKey()
	if keyErr == nil {
		signer = review.CatalogSigner{Key: key}
	} else if !errors.Is(keyErr, os.ErrNotExist) {
		return keyErr
	}
	otel.SetTracerProvider(tp)
	var integrationSigner appservice.IntegrationSigner
	integrationKey, integrationErr := localfiles.ReadIntegrationKey()
	if integrationErr == nil {
		integrationSigner = review.IntegrationSigner{Key: integrationKey}
	} else if !errors.Is(integrationErr, os.ErrNotExist) {
		return integrationErr
	}
	otel.SetTextMapPropagator(propagation.TraceContext{})
	var managedSigner appservice.ManagedShippingSigner
	managedKey, managedErr := localfiles.ReadManagedShippingKey()
	if managedErr == nil {
		managedSigner = review.ManagedShippingSigner{Key: managedKey}
	} else if !errors.Is(managedErr, os.ErrNotExist) {
		return managedErr
	}
	defer tp.Shutdown(context.Background())
	uiKey, uiErr := localfiles.ReadUIReleaseKey()
	if uiErr != nil && !errors.Is(uiErr, os.ErrNotExist) {
		return uiErr
	}
	var uiConfig struct {
		Environment  string `json:"environment"`
		ParentOrigin string `json:"parentOrigin"`
		LaunchSeed   []byte `json:"launchSeed"`
	}
	var uiRuntime *bootstrap.ReviewedUIRuntime
	var launchKey ed25519.PrivateKey
	if e := localfiles.Read(".local/reviewed-ui.json", &uiConfig); e == nil {
		if uiConfig.Environment != "development" || os.Getenv("NODE_ENV") == "production" || os.Getenv("EMISELL_ENV") == "production" || len(uiConfig.LaunchSeed) != ed25519.SeedSize {
			return errors.New("invalid reviewed UI development configuration")
		}
		launchKey = ed25519.NewKeyFromSeed(uiConfig.LaunchSeed)
		uiRuntime, e = bootstrap.NewReviewedUIRuntime(caps, clientPool, uiKey, launchKey, uiConfig.ParentOrigin)
		if e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	resourceKey, err := readUIResourceKey()
	if err != nil {
		return err
	}
	if uiRuntime != nil {
		uiRuntime.Source.ResourceKey = resourceKey
		var resources struct {
			Environment   string `json:"environment"`
			Origin        string `json:"origin"`
			KeyID         string `json:"keyId"`
			PrivateKeyPEM string `json:"privateKeyPem"`
		}
		if e := localfiles.Read(".local/resource-runtime.json", &resources); e == nil {
			if resources.Environment != "development" || os.Getenv("NODE_ENV") == "production" || os.Getenv("EMISELL_ENV") == "production" || resourceKey == nil {
				return errors.New("invalid local resource runtime")
			}
			uiRuntime.Source.Products, err = resourceclient.NewProducts(resourceclient.Options{Origin: resources.Origin, KeyID: resources.KeyID, PrivateKeyPEM: []byte(resources.PrivateKeyPEM), Environment: "sandbox", AllowHTTP: true, Timeout: 5 * time.Second})
			if err != nil {
				return errors.New("invalid local resource runtime")
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return e
		}
	}
	managedEnabled := false
	internalHandler := bootstrap.InternalHandlerWithManagedReleases(pool, caps, logger, integrationSigner, managedSigner, connections)
	if uiRuntime != nil {
		internalHandler, err = bootstrap.InternalHandlerWithReviewedUI(pool, caps, logger, integrationSigner, managedSigner, *uiRuntime, connections)
		if err != nil {
			return err
		}
	}
	httpHandler := bootstrap.HandlerWithManagedShipping(pool, caps, clientPool, cfg.Origin, logger, signer, integrationSigner, managedSigner, proofVerifier, connections)
	var engineConfig bootstrap.LocalEngineConfig
	if e := localfiles.Read(".local/managed-engine.json", &engineConfig); e == nil {
		local, e := engineConfig.LocalManaged()
		if e != nil {
			return e
		}
		managedEnabled = true
		local.ReviewedUI = uiRuntime
		var pilotConfig struct {
			Environment string `json:"environment"`
			MerchantID  string `json:"merchantId"`
		}
		if e := localfiles.Read(".local/embedded-pilot.json", &pilotConfig); e == nil {
			if os.Getenv("NODE_ENV") == "production" || os.Getenv("EMISELL_ENV") == "production" {
				return errors.New("embedded pilot forbidden in production")
			}
			local.EmbeddedPilot, e = installservice.NewLocalEmbeddedPilot(pilotConfig.Environment, pilotConfig.MerchantID)
			if e != nil {
				return e
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return e
		}
		httpHandler = bootstrap.HandlerWithLocalManagedShipping(pool, caps, clientPool, cfg.Origin, logger, signer, integrationSigner, managedSigner, proofVerifier, connections)
		internalHandler, e = bootstrap.InternalHandlerWithLocalManaged(pool, caps, logger, integrationSigner, managedSigner, local, connections)
		if e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if uiRuntime != nil {
		httpHandler = bootstrap.HandlerWithReviewedUIRuntime(pool, caps, clientPool, cfg.Origin, logger, signer, integrationSigner, managedSigner, proofVerifier, managedEnabled, bootstrap.EmbeddedReviewConfig{Runtime: uiRuntime, UIReleaseKey: uiKey, ResourceReleaseKey: resourceKey, Key: launchKey, ParentOrigin: uiConfig.ParentOrigin}, connections)
	}
	httpHandler = bootstrap.AddUIReleaseRoutes(httpHandler, pool, cfg.Origin, logger, uiKey)
	if resourceKey != nil {
		httpHandler = bootstrap.AddUIResourceReleaseRoutes(httpHandler, pool, cfg.Origin, logger, resourceKey)
	}
	internalHandler, err = providergrant.Attach(internalHandler, os.Getenv("EMISELL_PROVIDER_GRANT_FILE"), providergrant.Postgres{Pool: pool})
	if err != nil {
		return err
	}
	httpHandler, err = connectapi.ExposeCore(httpHandler, internalHandler, cfg.Origin, os.Getenv("EMISELL_CORE_HTTP_ENABLED"))
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.Address, Handler: httpHandler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	rpc := &http.Server{Addr: cfg.RPCAddress, Handler: internalHandler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 2)
	defer server.Close()
	defer rpc.Close()
	go func() { logger.Info("internal RPC ready", "address", cfg.RPCAddress); done <- rpc.ListenAndServe() }()
	go func() {
		mode := "local-simulator"
		if os.Getenv("EMISELL_ENV") == "production" {
			mode = "production-control-plane"
		}
		logger.Info("server ready", "address", cfg.Address, "mode", mode)
		done <- server.ListenAndServe()
	}()
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return errors.Join(server.Shutdown(shutdown), rpc.Shutdown(shutdown))
	}
}

func readProofVerifier() (*endpointproof.Verifier, error) {
	var c struct {
		Environment    string `json:"environment"`
		Origin         string `json:"origin"`
		CertificatePEM string `json:"certificatePem"`
	}
	err := localfiles.Read(".local/endpoint-proof.json", &c)
	if errors.Is(err, os.ErrNotExist) {
		return endpointproof.New(), nil
	}
	if err != nil {
		return nil, err
	}
	if os.Getenv("EMISELL_ENV") == "production" || os.Getenv("NODE_ENV") == "production" {
		return nil, errors.New("local endpoint proof forbidden in production")
	}
	return endpointproof.NewDevelopment(c.Environment, c.Origin, []byte(c.CertificatePEM))
}

// Local authoring only; no installation/grant runtime is enabled by this key.
func readUIResourceKey() (ed25519.PrivateKey, error) {
	var c struct {
		Environment string `json:"environment"`
		Seed        []byte `json:"seed"`
	}
	err := localfiles.Read(".local/ui-resource-signing.json", &c)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if c.Environment != "development" || os.Getenv("EMISELL_ENV") == "production" || os.Getenv("NODE_ENV") == "production" || len(c.Seed) != ed25519.SeedSize {
		return nil, errors.New("invalid local UI resource signing configuration")
	}
	return ed25519.NewKeyFromSeed(c.Seed), nil
}
