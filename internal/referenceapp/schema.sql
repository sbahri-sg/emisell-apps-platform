-- Owned exclusively by the local reference app; no platform tables are read.
CREATE SCHEMA IF NOT EXISTS reference_remote;
CREATE TABLE IF NOT EXISTS reference_remote.codes (
 hash text PRIMARY KEY,form_hash text NOT NULL,request jsonb NOT NULL,
 approved boolean NOT NULL DEFAULT false,used boolean NOT NULL DEFAULT false,
 expires_at timestamptz NOT NULL DEFAULT now()+interval '5 minutes'
);
CREATE TABLE IF NOT EXISTS reference_remote.grants (
 id text PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,
 access_hash text NOT NULL UNIQUE,refresh_hash text NOT NULL UNIQUE,
 hook bytea NOT NULL,access_expires timestamptz NOT NULL,
 expires_at timestamptz NOT NULL DEFAULT now()+interval '8 hours',revoked boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS reference_remote.refresh_history(hash text PRIMARY KEY,grant_id text NOT NULL);
CREATE TABLE IF NOT EXISTS reference_remote.idempotency (
 tenant_id text NOT NULL,installation_id text NOT NULL,key text NOT NULL,hash text NOT NULL,response jsonb NOT NULL,
 PRIMARY KEY(tenant_id,installation_id,key)
);
CREATE TABLE IF NOT EXISTS reference_remote.resources (
 tenant_id text NOT NULL,installation_id text NOT NULL,id text NOT NULL,data jsonb NOT NULL,
 PRIMARY KEY(tenant_id,installation_id,id)
);
CREATE TABLE IF NOT EXISTS reference_remote.webhook_inbox (
 tenant_id text NOT NULL,installation_id text NOT NULL,delivery_id text NOT NULL,
 event_id text NOT NULL,body_hash text NOT NULL,received_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,installation_id,delivery_id),UNIQUE(tenant_id,installation_id,event_id)
);
CREATE TABLE IF NOT EXISTS reference_remote.audit (
 id bigserial PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,action text NOT NULL,occurred_at timestamptz NOT NULL DEFAULT now()
);
