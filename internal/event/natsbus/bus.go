package natsbus

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"time"

	"emisell.app/platform/internal/event"
	"emisell.app/platform/pkg/sdk/events"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const Stream = "EMISELL_EVENTS"

type Credentials struct {
	URL      string `json:"url"`
	User     string `json:"user"`
	Password string `json:"password"`
	Inbox    string `json:"inbox"`
}

func Connect(c Credentials) (*nats.Conn, error) {
	u, err := url.Parse(c.URL)
	if err != nil || u.Scheme != "nats" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || c.User == "" || c.Password == "" || c.Inbox == "" {
		return nil, errors.New("invalid local broker configuration")
	}
	nc, err := nats.Connect(c.URL, nats.UserInfo(c.User, c.Password), nats.CustomInboxPrefix(c.Inbox), nats.Timeout(2*time.Second), nats.ReconnectWait(time.Second), nats.MaxReconnects(-1), nats.ReconnectBufSize(0), nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}))
	if err != nil {
		return nil, errors.New("local broker unavailable or authentication failed")
	}
	return nc, nil
}

type Publisher struct{ JS jetstream.JetStream }

func (p Publisher) Publish(ctx context.Context, e event.Envelope) error {
	if e.Validate() != nil {
		return event.ErrPermanent
	}
	raw, err := json.Marshal(e)
	if err != nil || len(raw) > 32<<10 {
		return event.ErrPermanent
	}
	_, err = p.JS.Publish(ctx, events.Subject(e), raw, jetstream.WithMsgID(e.ID), jetstream.WithExpectStream(Stream))
	return err
}

// Provision is operator-only. Core consumers cannot create/change their filters.
func Provision(ctx context.Context, js jetstream.JetStream, tenants []string) error {
	stream, err := js.Stream(ctx, Stream)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		stream, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: Stream, Subjects: []string{"emisell.events.>"}, Storage: jetstream.FileStorage, Retention: jetstream.LimitsPolicy, Discard: jetstream.DiscardNew, MaxBytes: 256 << 20, MaxAge: 7 * 24 * time.Hour, MaxMsgSize: 32 << 10, Duplicates: 24 * time.Hour, Replicas: 1})
	}
	if err != nil {
		return err
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return err
	}
	if !slices.Equal(info.Config.Subjects, []string{"emisell.events.>"}) || info.Config.Storage != jetstream.FileStorage || info.Config.Discard != jetstream.DiscardNew || info.Config.Retention != jetstream.LimitsPolicy {
		return errors.New("unexpected existing stream configuration")
	}
	if info.Config.Duplicates != 24*time.Hour || info.Config.MaxAge != 7*24*time.Hour || info.Config.MaxBytes != 256<<20 || info.Config.MaxMsgSize != 32<<10 || info.Config.Replicas != 1 {
		return errors.New("unexpected stream retention/limits")
	}
	for _, tenant := range tenants {
		if !events.Token.MatchString(tenant) {
			return errors.New("invalid consumer tenant")
		}
		name := "core_" + tenant
		consumer, err := js.Consumer(ctx, Stream, name)
		if errors.Is(err, jetstream.ErrConsumerNotFound) {
			consumer, err = js.CreateConsumer(ctx, Stream, jetstream.ConsumerConfig{Name: name, Durable: name, FilterSubject: "emisell.events." + tenant + ".>", AckPolicy: jetstream.AckExplicitPolicy, DeliverPolicy: jetstream.DeliverAllPolicy, MaxDeliver: -1, BackOff: []time.Duration{5 * time.Second, 30 * time.Second, 120 * time.Second}, MaxAckPending: 16, MaxRequestBatch: 16, MaxRequestExpires: 5 * time.Second})
		}
		if err != nil {
			return err
		}
		ci, err := consumer.Info(ctx)
		if err != nil {
			return err
		}
		if ci.Config.FilterSubject != "emisell.events."+tenant+".>" || len(ci.Config.FilterSubjects) != 0 || ci.Config.AckPolicy != jetstream.AckExplicitPolicy || ci.Config.MaxDeliver != -1 {
			return errors.New("unexpected existing consumer configuration")
		}
	}
	return nil
}
