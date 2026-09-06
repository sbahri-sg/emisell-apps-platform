-- Additive client-owned projection, keyed independently of any provider.
CREATE TABLE IF NOT EXISTS reference_core.payments (
 tenant_id text NOT NULL,resource_id text NOT NULL,installation_id text NOT NULL,
 reference text NOT NULL,status text NOT NULL,amount_minor bigint NOT NULL,currency text NOT NULL,
 revision bigint NOT NULL,event_id text NOT NULL,updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,resource_id)
);
