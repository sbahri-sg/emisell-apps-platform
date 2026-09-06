package event

import (
	"context"
	"errors"
)

// ErrPermanent quarantines invalid payloads without repeating an unsafe publish.
var ErrPermanent = errors.New("invalid_event")

type Publisher interface {
	Publish(context.Context, Envelope) error
}
type Delivery struct {
	Found   bool
	Outcome string
}
type Counts struct{ Pending, Dead int64 }

// Outbox is implemented by each producing module; relay never reads its tables.
type Outbox interface {
	DeliverOne(context.Context, Publisher) (Delivery, error)
	Counts(context.Context) (Counts, error)
	Replay(context.Context, string, string, string) error
}
