package bootstrap_test

import (
	"context"
	referencecore "emisell.app/platform/examples/core-reference"
	caprepo "emisell.app/platform/internal/capability/postgres"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/event/natsbus"
	"emisell.app/platform/pkg/sdk/events"
	"encoding/json"
	"github.com/nats-io/nats.go/jetstream"
	"testing"
	"time"
)

func TestPaymentCallbackOutboxJetStreamCoreProjection(t *testing.T) {
	f, ins, r := paymentSetup(t)
	broker := brokerSetup(t, f.tenant)
	ctx := context.Background()
	if err := referencecore.Init(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
	if err := referencecore.Init(ctx, f.pool); err != nil {
		t.Fatal("reference schema not repeatable", err)
	}
	for _, op := range []string{"capture", "refund"} {
		if _, err := f.reference.Simulate(ctx, f.tenant, ins, r.ID, op, key()); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		if got, err := f.reference.DeliverOne(ctx, f.server.URL); err != nil || got != "delivered" {
			t.Fatal(got, err)
		}
	}
	nc, err := natsbus.Connect(broker.config.Worker)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err = natsbus.Provision(ctx, js, []string{f.tenant}); err != nil {
		t.Fatal(err)
	}
	box := (caprepo.Repository{Pool: f.caps}).Outbox()
	for range 5000 {
		result, err := box.DeliverOne(ctx, natsbus.Publisher{JS: js})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Found {
			break
		}
	}
	var pending int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_capability.events WHERE tenant_id=$1 AND published_at IS NULL", f.tenant).Scan(&pending); err != nil || pending != 0 {
		t.Fatal("unpublished payment events", pending, err)
	}
	coreConn, err := natsbus.Connect(broker.config.Core)
	if err != nil {
		t.Fatal(err)
	}
	defer coreConn.Close()
	coreJS, err := jetstream.New(coreConn)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := coreJS.Consumer(ctx, natsbus.Stream, "core_"+f.tenant)
	if err != nil {
		t.Fatal(err)
	}
	inbox := referencecore.Inbox{Pool: f.pool}
	// create invocation + three business states, all delivered by the real outbox.
	for range 4 {
		if got, err := inbox.ConsumeOne(ctx, consumer, f.tenant); err != nil || got != "applied" {
			t.Fatal(got, err)
		}
	}
	assertProjection := func() {
		t.Helper()
		var status string
		var revision int64
		if err := f.pool.QueryRow(ctx, "SELECT status,revision FROM reference_core.payments WHERE tenant_id=$1 AND resource_id=$2", f.tenant, r.ID).Scan(&status, &revision); err != nil || status != "refunded" || revision != 3 {
			t.Fatal("Core projection", status, revision, err)
		}
	}
	assertProjection()
	var latest event.Envelope
	if err = f.pool.QueryRow(ctx, "SELECT envelope FROM platform_capability.events WHERE tenant_id=$1 AND envelope->>'type'=$2 ORDER BY occurred_at DESC LIMIT 1", f.tenant, events.PaymentStatusType).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(latest)
	if got, err := inbox.Apply(ctx, f.tenant, 1000, events.Subject(latest), raw); err != nil || got != "duplicate" {
		t.Fatal(got, err)
	}
	p, err := events.DecodePayment(latest)
	if err != nil {
		t.Fatal(err)
	}
	p.Status = "captured"
	p.Revision = 2
	late := event.New(events.PaymentStatusType, f.tenant, "app-callback", r.ID, key(), p)
	raw, _ = json.Marshal(late)
	if got, err := inbox.Apply(ctx, f.tenant, 1001, events.Subject(late), raw); err != nil || got != "applied" {
		t.Fatal(got, err)
	}
	assertProjection()
	for index, mutate := range []func(*events.PaymentStatus){
		func(p *events.PaymentStatus) { p.Status = "authorized"; p.Revision = 4 },
		func(p *events.PaymentStatus) { p.Status = "captured"; p.Revision = 3 },
		func(p *events.PaymentStatus) { p.AmountMinor++ },
	} {
		bad, _ := events.DecodePayment(latest)
		mutate(&bad)
		e := event.New(events.PaymentStatusType, f.tenant, "app-callback", r.ID, key(), bad)
		raw, _ = json.Marshal(e)
		if got, err := inbox.Apply(ctx, f.tenant, uint64(1002+index), events.Subject(e), raw); err != nil || got != "quarantined" {
			t.Fatal(got, err)
		}
	}
	if got, err := inbox.Apply(ctx, f.other, 1008, events.Subject(latest), raw); err != nil || got != "quarantined" {
		t.Fatal("foreign Core event", got, err)
	}
	assertProjection()
	// Broker restart and a fresh Inbox instance retain delivery ACKs/projection.
	broker.stop(t)
	broker.start(t)
	deadline := time.Now().Add(8 * time.Second)
	for !nc.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	for !coreConn.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	consumer, err = coreJS.Consumer(ctx, natsbus.Stream, "core_"+f.tenant)
	if err != nil {
		t.Fatal(err)
	}
	info, err := consumer.Info(ctx)
	if err != nil || info.NumAckPending != 0 || info.NumPending != 0 {
		t.Fatal("Core ACK durability", err)
	}
	assertProjection()
}
