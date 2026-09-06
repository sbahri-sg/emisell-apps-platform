-- Additive reference-app-owned local fixture upgrade.
CREATE TABLE IF NOT EXISTS reference_remote.callbacks (
 id text PRIMARY KEY,tenant_id text NOT NULL,installation_id text NOT NULL,
 owner_key text NOT NULL,body jsonb NOT NULL,
 status text NOT NULL DEFAULT 'pending',attempts integer NOT NULL DEFAULT 0,
 next_at timestamptz NOT NULL DEFAULT now(),created_at timestamptz NOT NULL DEFAULT now(),
 last_error text NOT NULL DEFAULT '',completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS reference_callbacks_due ON reference_remote.callbacks(owner_key,next_at,id) WHERE status='pending';
