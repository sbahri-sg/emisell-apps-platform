// Core reference knows only capability contracts and its own inbox.
package main

import (
	"connectrpc.com/connect"
	"context"
	referencecore "emisell.app/platform/examples/core-reference"
	"emisell.app/platform/internal/event/natsbus"
	"emisell.app/platform/internal/platform/config"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/pkg/sdk"
	pay "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	operation := flag.String("operation", "shipping-rates", "payment-create|payment-status|payment-capture|payment-refund|shipping-rates|shipping-create|shipping-track|consume")
	key := flag.String("key", "", "stable business idempotency key, required for invocation")
	resource := flag.String("resource", "", "existing capability resource ID")
	reference := flag.String("reference", "core-reference", "business reference")
	flag.Parse()
	var cred localfiles.CoreCredential
	if err := localfiles.Read(".local/core.json", &cred); err != nil {
		return errors.New("run cli init-core first")
	}
	if *operation == "consume" {
		return consume(cred)
	}
	if *key == "" {
		return errors.New("--key is required; reuse it only for the same business operation")
	}
	client, err := sdk.NewLocalClient(cred.Address, cred.Token)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var message proto.Message
	switch *operation {
	case "payment-create":
		r, e := client.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: cred.TenantID, IdempotencyKey: *key, Reference: *reference, AmountMinor: 12000, Currency: "IDR"}))
		err = e
		if r != nil {
			message = r.Msg
		}
	case "payment-status":
		r, e := client.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: cred.TenantID, IdempotencyKey: *key, ResourceId: *resource}))
		err = e
		if r != nil {
			message = r.Msg
		}
	case "payment-capture":
		r, e := client.Payment.Capture(ctx, connect.NewRequest(&pay.CaptureRequest{TenantId: cred.TenantID, IdempotencyKey: *key, ResourceId: *resource}))
		err = e
		if r != nil {
			message = r.Msg
		}
	case "payment-refund":
		r, e := client.Payment.Refund(ctx, connect.NewRequest(&pay.RefundRequest{TenantId: cred.TenantID, IdempotencyKey: *key, ResourceId: *resource}))
		err = e
		if r != nil {
			message = r.Msg
		}
	case "shipping-rates":
		r, e := client.Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{TenantId: cred.TenantID, IdempotencyKey: *key, WeightGrams: 1000, DestinationZone: "ID-JKT"}))
		err = e
		if r != nil {
			message = r.Msg
		}
	case "shipping-create":
		r, e := client.Shipping.Create(ctx, connect.NewRequest(&ship.CreateRequest{TenantId: cred.TenantID, IdempotencyKey: *key, Reference: *reference, WeightGrams: 1000, DestinationZone: "ID-JKT"}))
		err = e
		if r != nil {
			message = r.Msg
		}
	case "shipping-track":
		r, e := client.Shipping.Track(ctx, connect.NewRequest(&ship.TrackRequest{TenantId: cred.TenantID, IdempotencyKey: *key, ResourceId: *resource}))
		err = e
		if r != nil {
			message = r.Msg
		}
	default:
		return errors.New("unknown operation")
	}
	if err != nil {
		return err
	}
	raw, err := protojson.MarshalOptions{Indent: "  "}.Marshal(message)
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}
func consume(cred localfiles.CoreCredential) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Read()
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("reference inbox database unavailable")
	}
	defer pool.Close()
	nc, err := natsbus.Connect(cred.Broker)
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	consumer, err := js.Consumer(ctx, natsbus.Stream, "core_"+cred.TenantID)
	if err != nil {
		return errors.New("tenant consumer unavailable; start worker first")
	}
	if consumer.CachedInfo().Config.FilterSubject != "emisell.events."+cred.TenantID+".>" {
		return errors.New("invalid tenant consumer filter")
	}
	inbox := referencecore.Inbox{Pool: pool}
	fmt.Println("Core reference consumer ready; durable inbox enabled.")
	for ctx.Err() == nil {
		opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		outcome, err := inbox.ConsumeOne(opCtx, consumer, cred.TenantID)
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Consumer retry: broker/database unavailable; no acknowledgement sent.")
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
			continue
		}
		if outcome != "idle" {
			fmt.Println("event:", outcome)
		}
	}
	return nil
}
