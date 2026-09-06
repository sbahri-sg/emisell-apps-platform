package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	referencecore "emisell.app/platform/examples/core-reference"
	caprepo "emisell.app/platform/internal/capability/postgres"
	"emisell.app/platform/internal/event"
	"emisell.app/platform/internal/event/natsbus"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/pkg/sdk/events"
	"github.com/nats-io/nats.go/jetstream"
)

type brokerFixture struct {
	binary, path string
	config       localfiles.EventConfig
	process      *exec.Cmd
	done         chan error
}

func brokerSetup(t *testing.T, tenant ...string) *brokerFixture {
	t.Helper()
	binary := os.Getenv("EMISELL_NATS_SERVER")
	if binary == "" {
		t.Skip("set EMISELL_NATS_SERVER to pinned nats-server binary for restart/ACL integration tests")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	dir := t.TempDir()
	cfg, raw := localfiles.LocalBrokerConfig()
	// Bind this private test broker's Core credential to exactly one fixture
	// tenant. Do not grant the producer permission to ACK Core messages.
	if len(tenant) > 0 {
		if !events.Token.MatchString(tenant[0]) {
			t.Fatal("invalid fixture tenant")
		}
		raw = bytes.ReplaceAll(raw, []byte("core_local-store"), []byte("core_"+tenant[0]))
	}
	var config map[string]any
	if err = json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	config["port"] = port
	config["jetstream"].(map[string]any)["store_dir"] = filepath.Join(dir, "store")
	raw, err = json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte(`\u003e`), []byte(">"))
	path := filepath.Join(dir, "nats.conf")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Worker.URL = "nats://127.0.0.1:" + strconv.Itoa(port)
	cfg.Core.URL = cfg.Worker.URL
	b := &brokerFixture{binary: binary, path: path, config: cfg}
	b.start(t)
	t.Cleanup(func() { b.stop(t) })
	return b
}
func (b *brokerFixture) start(t *testing.T) {
	t.Helper()
	b.process = exec.Command(b.binary, "-a", "127.0.0.1", "-c", b.path)
	b.process.Stdout = io.Discard
	b.process.Stderr = io.Discard
	if err := b.process.Start(); err != nil {
		t.Fatal(err)
	}
	b.done = make(chan error, 1)
	go func() { b.done <- b.process.Wait() }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		nc, err := natsbus.Connect(b.config.Worker)
		if err == nil {
			nc.Close()
			return
		}
		select {
		case err := <-b.done:
			t.Fatal("test broker exited", err)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("test broker did not start")
}
func (b *brokerFixture) stop(t *testing.T) {
	t.Helper()
	if b.process == nil {
		return
	}
	b.process.Process.Signal(os.Interrupt)
	select {
	case <-b.done:
	case <-time.After(5 * time.Second):
		b.process.Process.Kill()
		<-b.done
	}
	b.process = nil
}

type publisherFunc func(context.Context, event.Envelope) error

func (fn publisherFunc) Publish(ctx context.Context, e event.Envelope) error { return fn(ctx, e) }

func TestJetStreamOutboxInboxRecoveryAndTenantACL(t *testing.T) {
	broker := brokerSetup(t)
	f := setup(t)
	ctx := context.Background()
	nc, err := natsbus.Connect(broker.config.Worker)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err = natsbus.Provision(ctx, js, []string{"local-store", "foreign"}); err != nil {
		t.Fatal(err)
	}
	boxes := []event.Outbox{(installrepo.Repository{Pool: f.pool}).Outbox(), (caprepo.Repository{Pool: f.pool}).Outbox()}
	publisher := natsbus.Publisher{JS: js}
	// Existing test rows are delivered too, never deleted or marked via a fake ACK.
	for _, box := range boxes {
		for range 1000 {
			result, e := box.DeliverOne(ctx, publisher)
			if e != nil {
				t.Fatal(e)
			}
			if !result.Found {
				break
			}
		}
	}
	if err = referencecore.Init(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
	inbox := referencecore.Inbox{Pool: f.pool}
	coreConn, err := natsbus.Connect(broker.config.Core)
	if err != nil {
		t.Fatal(err)
	}
	defer coreConn.Close()
	coreJS, err := jetstream.New(coreConn)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := coreJS.Consumer(ctx, natsbus.Stream, "core_local-store")
	if err != nil {
		t.Fatal("least-privilege consumer lookup", err)
	}
	// Drain prior test deliveries addressed to local-store; unrelated tenants cannot arrive.
	for range 1000 {
		msg, e := consumer.Next(jetstream.FetchMaxWait(100 * time.Millisecond))
		if e != nil {
			break
		}
		if e = msg.DoubleAck(ctx); e != nil {
			t.Fatal(e)
		}
	}
	newEvent := func(module string) event.Envelope {
		e := event.New("emisell.capability.invoked.v1", "local-store", "core-local-store", key(), key(), map[string]string{"capability": "payment/v1", "operation": "create"})
		raw, _ := json.Marshal(e)
		if _, err = f.pool.Exec(ctx, `INSERT INTO platform_`+module+`.events(id,tenant_id,envelope,occurred_at) VALUES($1,$2,$3,$4)`, e.ID, e.TenantID, raw, e.OccurredAt); err != nil {
			t.Fatal(err)
		}
		return e
	}
	makeDue := func(module, id string) {
		if _, err = f.pool.Exec(ctx, `UPDATE platform_`+module+`.events SET next_attempt_at=now() WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	state := func(module, id string) (bool, int, bool) {
		var published, dead bool
		var attempts int
		if err = f.pool.QueryRow(ctx, `SELECT published_at IS NOT NULL,attempts,dead_at IS NOT NULL FROM platform_`+module+`.events WHERE id=$1`, id).Scan(&published, &attempts, &dead); err != nil {
			t.Fatal(err)
		}
		return published, attempts, dead
	}
	deliver := func(box event.Outbox, p event.Publisher, want string) {
		t.Helper()
		r, e := box.DeliverOne(ctx, p)
		if e != nil || !r.Found || r.Outcome != want {
			t.Fatalf("delivery=%+v err=%v want=%s", r, e, want)
		}
	}

	var receiptsBefore int64
	if err = f.pool.QueryRow(ctx, `SELECT COALESCE((SELECT count FROM reference_core.receipts WHERE tenant_id='local-store' AND event_type='emisell.capability.invoked.v1'),0)`).Scan(&receiptsBefore); err != nil {
		t.Fatal(err)
	}
	e := newEvent("capability")
	deliver(boxes[1], publisher, "published")
	if published, attempts, _ := state("capability", e.ID); !published || attempts != 1 {
		t.Fatal("publish acknowledgement not persisted")
	}
	msg, err := consumer.Next(jetstream.FetchMaxWait(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	md, _ := msg.Metadata()
	outcome, err := inbox.Apply(ctx, "local-store", md.Sequence.Stream, msg.Subject(), msg.Data())
	if err != nil || outcome != "applied" {
		t.Fatal(outcome, err)
	}
	// Crash window: committed side effect, no broker ACK. Re-delivery is deduped.
	redelivered, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	if err != nil {
		redelivered, err = consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	}
	if err != nil {
		t.Fatal("unacked event not redelivered", err)
	}
	outcome, err = inbox.Apply(ctx, "local-store", md.Sequence.Stream, redelivered.Subject(), redelivered.Data())
	if err != nil || outcome != "duplicate" {
		t.Fatal("inbox duplicated effect", outcome, err)
	}
	if err = redelivered.DoubleAck(ctx); err != nil {
		t.Fatal("consumer ACK denied", err)
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM reference_core.inbox WHERE tenant_id=$1 AND event_id=$2`, e.TenantID, e.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("missing inbox deduplication", err, count)
	}
	var receiptsAfter int64
	if err = f.pool.QueryRow(ctx, `SELECT count FROM reference_core.receipts WHERE tenant_id='local-store' AND event_type='emisell.capability.invoked.v1'`).Scan(&receiptsAfter); err != nil || receiptsAfter != receiptsBefore+1 {
		t.Fatal("reference side effect executed more than once", err)
	}

	uncertain := newEvent("installation")
	deliver(boxes[0], publisherFunc(func(ctx context.Context, e event.Envelope) error {
		if err := publisher.Publish(ctx, e); err != nil {
			return err
		}
		return errors.New("simulated lost acknowledgement")
	}), "retry")
	stream, err := js.Stream(ctx, natsbus.Stream)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := stream.Info(ctx)
	makeDue("installation", uncertain.ID)
	deliver(boxes[0], publisher, "published")
	after, _ := stream.Info(ctx)
	if after.State.Msgs != before.State.Msgs {
		t.Fatal("broker deduplication failed")
	}

	// A real broker restart uses the same on-disk store and durable consumer.
	broker.stop(t)
	disconnected := newEvent("capability")
	deliver(boxes[1], publisher, "retry")
	if published, _, _ := state("capability", disconnected.ID); published {
		t.Fatal("unacknowledged event marked delivered")
	}
	broker.start(t)
	deadline := time.Now().Add(8 * time.Second)
	for !nc.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !nc.IsConnected() {
		t.Fatal("publisher failed to reconnect")
	}
	makeDue("capability", disconnected.ID)
	deliver(boxes[1], publisher, "published")
	if _, err = js.Consumer(ctx, natsbus.Stream, "core_local-store"); err != nil {
		t.Fatal("durable consumer lost on restart", err)
	}

	exhausted := newEvent("installation")
	if _, err = f.pool.Exec(ctx, `UPDATE platform_installation.events SET attempts=11 WHERE id=$1`, exhausted.ID); err != nil {
		t.Fatal(err)
	}
	deliver(boxes[0], publisherFunc(func(context.Context, event.Envelope) error { return errors.New("offline") }), "dead")
	if published, attempts, dead := state("installation", exhausted.ID); published || attempts != 12 || !dead {
		t.Fatal("retry exhaustion not quarantined")
	}
	if err = boxes[0].Replay(ctx, exhausted.ID, "test-operator", "broker connection recovered"); err != nil {
		t.Fatal(err)
	}
	deliver(boxes[0], publisher, "published")
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.outbox_replays WHERE event_id=$1`, exhausted.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("replay missing audit", err)
	}

	invalid := newEvent("capability")
	if _, err = f.pool.Exec(ctx, `UPDATE platform_capability.events SET envelope='{}'::jsonb WHERE id=$1`, invalid.ID); err != nil {
		t.Fatal(err)
	}
	deliver(boxes[1], publisher, "dead")
	raw, _ := json.Marshal(event.New("emisell.app.activated.v1", "foreign", "operator", key(), key(), map[string]string{"status": "active"}))
	outcome, err = inbox.Apply(ctx, "local-store", 999999, "emisell.events.foreign.emisell.app.activated.v1", raw)
	if err != nil || outcome != "quarantined" {
		t.Fatal("foreign consumer payload accepted", outcome, err)
	}

	// Broker ACL denies changing filters, viewing another durable, publishing or reading stream data.
	deniedCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	_, err = coreJS.Consumer(deniedCtx, natsbus.Stream, "core_foreign")
	cancel()
	if err == nil {
		t.Fatal("cross-tenant consumer permitted")
	}
	deniedCtx, cancel = context.WithTimeout(ctx, 400*time.Millisecond)
	_, err = coreJS.Publish(deniedCtx, events.Subject(e), []byte("{}"))
	cancel()
	if err == nil {
		t.Fatal("Core consumer could publish platform events")
	}
	deniedCtx, cancel = context.WithTimeout(ctx, 400*time.Millisecond)
	_, err = coreJS.CreateConsumer(deniedCtx, natsbus.Stream, jetstream.ConsumerConfig{Name: "core_local-store", FilterSubject: "emisell.events.>", AckPolicy: jetstream.AckExplicitPolicy})
	cancel()
	if err == nil {
		t.Fatal("Core consumer could widen filter")
	}
}
