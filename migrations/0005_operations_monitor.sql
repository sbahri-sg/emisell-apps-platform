-- Additive monitoring metadata. created_at remains the existing retry-window clock.
ALTER TABLE platform_webhook.deliveries
 ADD COLUMN enqueued_at timestamptz NOT NULL DEFAULT now(),
 ADD COLUMN last_attempt_at timestamptz,
 ADD COLUMN completed_at timestamptz,
 ADD COLUMN revision bigint NOT NULL DEFAULT 0;
UPDATE platform_webhook.deliveries SET enqueued_at=created_at;
CREATE INDEX webhook_tenant_history ON platform_webhook.deliveries(tenant_id,enqueued_at DESC,id DESC);
ALTER TABLE platform_webhook.audit ADD COLUMN actor_id text NOT NULL DEFAULT 'system-worker';
CREATE TABLE platform_webhook.recovery_requests (
 tenant_id text NOT NULL,actor_id text NOT NULL,key text NOT NULL,
 request_hash text NOT NULL,response jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,actor_id,key)
);
ALTER TABLE platform_installation.installations ADD COLUMN cleanup_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE platform_installation.cleanup_audit
 ADD COLUMN actor_id text NOT NULL DEFAULT 'local-operator',
 ADD COLUMN action text NOT NULL DEFAULT 'operator_retry';
CREATE TABLE platform_installation.recovery_requests (
 tenant_id text NOT NULL,actor_id text NOT NULL,key text NOT NULL,
 request_hash text NOT NULL,response jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,actor_id,key)
);
