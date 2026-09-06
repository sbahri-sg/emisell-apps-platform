-- Existing event envelopes/IDs stay immutable; only delivery metadata changes.
ALTER TABLE platform_installation.events
 ADD COLUMN attempts integer NOT NULL DEFAULT 0,
 ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
 ADD COLUMN dead_at timestamptz,
 ADD COLUMN last_error text;
ALTER TABLE platform_capability.events
 ADD COLUMN attempts integer NOT NULL DEFAULT 0,
 ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
 ADD COLUMN dead_at timestamptz,
 ADD COLUMN last_error text;
CREATE INDEX installation_outbox_due ON platform_installation.events(next_attempt_at,id) WHERE published_at IS NULL AND dead_at IS NULL;
CREATE INDEX capability_outbox_due ON platform_capability.events(next_attempt_at,id) WHERE published_at IS NULL AND dead_at IS NULL;
CREATE TABLE platform_installation.outbox_replays (
 id bigserial PRIMARY KEY, event_id text NOT NULL REFERENCES platform_installation.events(id),
 actor text NOT NULL, reason text NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE platform_capability.outbox_replays (
 id bigserial PRIMARY KEY, event_id text NOT NULL REFERENCES platform_capability.events(id),
 actor text NOT NULL, reason text NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now()
);
