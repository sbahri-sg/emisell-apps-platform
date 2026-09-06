package webhook

import (
	"context"
	"emisell.app/platform/internal/event/natsbus"
	"emisell.app/platform/pkg/sdk/events"
	"errors"
	"github.com/nats-io/nats.go/jetstream"
	"time"
)

const ConsumerName = "webhook_router"

func Provision(ctx context.Context, js jetstream.JetStream) (jetstream.Consumer, error) {
	c, err := js.Consumer(ctx, natsbus.Stream, ConsumerName)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		c, err = js.CreateConsumer(ctx, natsbus.Stream, jetstream.ConsumerConfig{Name: ConsumerName, Durable: ConsumerName, FilterSubject: "emisell.events.*.emisell.capability.invoked.v1", AckPolicy: jetstream.AckExplicitPolicy, DeliverPolicy: jetstream.DeliverAllPolicy, MaxDeliver: -1, BackOff: []time.Duration{10 * time.Second, 30 * time.Second, 120 * time.Second}, MaxAckPending: 16, MaxRequestBatch: 16, MaxRequestExpires: 5 * time.Second})
	}
	if err != nil {
		return nil, err
	}
	info, err := c.Info(ctx)
	if err != nil {
		return nil, err
	}
	if info.Config.FilterSubject != "emisell.events.*.emisell.capability.invoked.v1" || len(info.Config.FilterSubjects) != 0 || info.Config.AckPolicy != jetstream.AckExplicitPolicy || info.Config.MaxDeliver != -1 {
		return nil, errors.New("unexpected webhook consumer configuration")
	}
	return c, nil
}
func (s Service) RouteBatch(ctx context.Context, c jetstream.Consumer) error {
	batch, err := c.Fetch(16, jetstream.FetchMaxWait(100*time.Millisecond))
	if err != nil {
		return err
	}
	for msg := range batch.Messages() {
		e, err := events.Decode(msg.Data())
		if err != nil || msg.Subject() != events.Subject(e) {
			if err = msg.Term(); err != nil {
				return err
			}
			continue
		}
		if err = s.Ingest(ctx, e); err != nil {
			return err
		}
		if err = msg.DoubleAck(ctx); err != nil {
			return err
		}
	}
	err = batch.Error()
	if errors.Is(err, jetstream.ErrNoMessages) {
		return nil
	}
	return err
}
