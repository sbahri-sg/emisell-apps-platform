package main

import (
	"context"
	"emisell.app/platform/internal/bootstrap"
	caprepo "emisell.app/platform/internal/capability/postgres"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/event/natsbus"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/platform/config"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/webhook"
	hookrepo "emisell.app/platform/internal/webhook/postgres"
	"emisell.app/platform/migrations"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Read()
	if err != nil {
		return err
	}
	var credentials localfiles.EventConfig
	if err = localfiles.Read(".local/events.json", &credentials); err != nil {
		return fmt.Errorf("run init-events first")
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("database configuration failed")
	}
	defer pool.Close()
	if err = migrations.Verify(ctx, pool); err != nil {
		return err
	}
	nc, err := natsbus.Connect(credentials.Worker)
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return errors.New("JetStream initialization failed")
	}
	setupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = natsbus.Provision(setupCtx, js, []string{"local-store"})
	cancel()
	if err != nil {
		return errors.New("JetStream provision failed; check broker configuration")
	}
	boxes := map[string]event.Outbox{"installation": (installrepo.Repository{Pool: pool}).Outbox(), "capability": (caprepo.Repository{Pool: pool}).Outbox()}
	metrics := prometheus.NewRegistry()
	outcomes := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "emisell_outbox_attempts_total", Help: "Committed outbox delivery outcomes."}, []string{"module", "outcome"})
	pending := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "emisell_outbox_pending", Help: "Unpublished, retryable events."}, []string{"module"})
	dead := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "emisell_outbox_dead", Help: "Quarantined outbox events requiring operator review."}, []string{"module"})
	metrics.MustRegister(outcomes, pending, dead)
	var hooks *webhook.Service
	var consumer jetstream.Consumer
	var remoteConfig localfiles.RemoteConfig
	if err = localfiles.Read(".local/remote-platform.json", &remoteConfig); err == nil {
		connectionPool, poolErr := pgxpool.New(ctx, cfg.DatabaseURL)
		if poolErr != nil {
			return errors.New("connection pool unavailable")
		}
		defer connectionPool.Close()
		queuePool, poolErr := pgxpool.New(ctx, cfg.DatabaseURL)
		if poolErr != nil {
			return errors.New("queue pool unavailable")
		}
		defer queuePool.Close()
		connection, err := bootstrap.Connections(pool, connectionPool, remoteConfig)
		if err != nil {
			return err
		}
		hooks = &webhook.Service{Repo: hookrepo.Repository{Pool: queuePool}, Connections: connection}
		setup, cancel := context.WithTimeout(ctx, 5*time.Second)
		consumer, err = webhook.Provision(setup, js)
		cancel()
		if err != nil {
			return errors.New("webhook consumer provision failed")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	hookOutcomes := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "emisell_webhook_attempts_total", Help: "Durable webhook delivery outcomes."}, []string{"outcome"})
	metrics.MustRegister(hookOutcomes)
	hookPending := prometheus.NewGauge(prometheus.GaugeOpts{Name: "emisell_webhook_pending", Help: "Pending webhook deliveries."})
	hookDead := prometheus.NewGauge(prometheus.GaugeOpts{Name: "emisell_webhook_dead", Help: "Quarantined webhook deliveries."})
	metrics.MustRegister(hookPending, hookDead)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{}))
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !nc.IsConnected() {
			http.Error(w, "broker unavailable", 503)
			return
		}
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		fmt.Fprintln(w, "ready")
	})
	monitoring := &http.Server{Addr: "127.0.0.1:8089", Handler: mux, ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second}
	defer monitoring.Close()
	serverErr := make(chan error, 1)
	go func() { serverErr <- monitoring.ListenAndServe() }()
	slog.Info("outbox worker ready", "monitoring", "127.0.0.1:8089")
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serverErr:
			return err
		case <-tick.C:
		}
		for _, name := range []string{"installation", "capability"} {
			batchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			for range 20 {
				result, err := boxes[name].DeliverOne(batchCtx, natsbus.Publisher{JS: js})
				if err != nil {
					slog.Warn("outbox storage unavailable", "module", name)
					break
				}
				if !result.Found {
					break
				}
				outcomes.WithLabelValues(name, result.Outcome).Inc()
				if result.Outcome != "published" {
					slog.Warn("outbox delivery deferred", "module", name, "outcome", result.Outcome)
					break
				}
			}
			counts, err := boxes[name].Counts(batchCtx)
			if err == nil {
				pending.WithLabelValues(name).Set(float64(counts.Pending))
				dead.WithLabelValues(name).Set(float64(counts.Dead))
			}
			cancel()
		}
		if hooks != nil {
			batch, cancel := context.WithTimeout(ctx, 8*time.Second)
			if err = hooks.RouteBatch(batch, consumer); err != nil {
				slog.Warn("webhook routing deferred")
			}
			for range 10 {
				outcome, err := hooks.DeliverOne(batch)
				if err != nil {
					slog.Warn("webhook storage unavailable")
					break
				}
				if outcome == "idle" {
					break
				}
				hookOutcomes.WithLabelValues(outcome).Inc()
			}
			cancel()
			cleanup, cancel := context.WithTimeout(ctx, 8*time.Second)
			if counts, err := hooks.Repo.Counts(cleanup); err == nil {
				hookPending.Set(float64(counts.Pending))
				hookDead.Set(float64(counts.Dead))
			}
			repo := installrepo.Repository{Pool: pool}
			items, err := repo.PendingCleanup(cleanup)
			if err == nil {
				for _, item := range items {
					if err = repo.CompleteCleanup(cleanup, item, hooks.Connections.Revoke); err != nil {
						slog.Warn("remote uninstall cleanup deferred", "installation_id", item.ID)
					}
				}
			}
			cancel()
		}
	}
}
