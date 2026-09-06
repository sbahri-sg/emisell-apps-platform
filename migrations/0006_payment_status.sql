-- Additive capability-owned projection/inbox. Do not invent legacy history.
ALTER TABLE platform_capability.resources
 ADD COLUMN observed_at timestamptz,
 ADD COLUMN status_revision bigint NOT NULL DEFAULT 0;
CREATE INDEX capability_payments_page ON platform_capability.resources(tenant_id,id) WHERE capability='payment/v1';
CREATE TABLE platform_capability.callback_inbox (
 tenant_id text NOT NULL, installation_id text NOT NULL, delivery_id text NOT NULL,
 resource_id text NOT NULL, body_hash text NOT NULL, outcome text NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,installation_id,delivery_id)
);
CREATE INDEX capability_payment_history ON platform_capability.events(tenant_id,(envelope->>'subject'),occurred_at,id)
 WHERE envelope->>'type'='emisell.payment.status_changed.v1';
