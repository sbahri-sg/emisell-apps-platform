-- Reference client-owned storage. Never read by platform domain modules.
CREATE SCHEMA IF NOT EXISTS reference_core;
CREATE TABLE IF NOT EXISTS reference_core.inbox (
 tenant_id text NOT NULL,event_id text NOT NULL,envelope jsonb NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(tenant_id,event_id)
);
CREATE TABLE IF NOT EXISTS reference_core.receipts (
 tenant_id text NOT NULL,event_type text NOT NULL,count bigint NOT NULL,
 PRIMARY KEY(tenant_id,event_type)
);
CREATE TABLE IF NOT EXISTS reference_core.dead_letters (
 tenant_id text NOT NULL,stream_sequence bigint NOT NULL,reason text NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(tenant_id,stream_sequence)
);
