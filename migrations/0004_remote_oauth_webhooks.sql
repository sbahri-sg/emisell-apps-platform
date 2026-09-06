-- Additive lifecycle extension: routing/scopes are removed while cleanup retries.
ALTER TABLE platform_installation.installations DROP CONSTRAINT installations_status_check;
ALTER TABLE platform_installation.installations ADD CONSTRAINT installations_status_check CHECK(status IN ('pending','active','disabling','uninstalled'));
ALTER TABLE platform_installation.installations
 ADD COLUMN cleanup_attempts integer NOT NULL DEFAULT 0,
 ADD COLUMN cleanup_next_at timestamptz NOT NULL DEFAULT now();
CREATE TABLE platform_installation.cleanup_audit (
 id bigserial PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,
 reason text NOT NULL,occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE SCHEMA platform_oauth;
CREATE TABLE platform_oauth.states (
 state_hash text PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,
 actor_id text NOT NULL,session_hash text NOT NULL,request_key text NOT NULL,
 material bytea NOT NULL,expires_at timestamptz NOT NULL,used_at timestamptz,
 UNIQUE(tenant_id,installation_id,session_hash,request_key)
);
CREATE TABLE platform_oauth.connections (
 tenant_id text NOT NULL,installation_id text NOT NULL,
 status text NOT NULL CHECK(status IN ('connected','needs_connection','revoked')),
 secret bytea,updated_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(tenant_id,installation_id)
);
CREATE TABLE platform_oauth.audit (
 id bigserial PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,
 actor_id text NOT NULL,action text NOT NULL,occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE SCHEMA platform_webhook;
CREATE TABLE platform_webhook.deliveries (
 id text PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,event_id text NOT NULL,
 body bytea NOT NULL,status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','delivered','dead','cancelled')),
 attempts integer NOT NULL DEFAULT 0,next_at timestamptz NOT NULL DEFAULT now(),
 created_at timestamptz NOT NULL DEFAULT now(),last_error text,
 UNIQUE(tenant_id,installation_id,event_id)
);
CREATE INDEX webhook_due ON platform_webhook.deliveries(next_at) WHERE status='pending';
CREATE TABLE platform_webhook.inbox (event_id text PRIMARY KEY,received_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE platform_webhook.audit (
 id bigserial PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,delivery_id text NOT NULL,
 action text NOT NULL,reason text NOT NULL DEFAULT '',occurred_at timestamptz NOT NULL DEFAULT now()
);
