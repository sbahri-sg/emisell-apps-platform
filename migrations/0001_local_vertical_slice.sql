-- New namespace: deliberately independent of the pre-existing deleted gateway.
CREATE SCHEMA platform_identity;
CREATE SCHEMA platform_app;
CREATE SCHEMA platform_installation;
CREATE SCHEMA platform_capability;

CREATE TABLE platform_identity.users (
 id text PRIMARY KEY, email text UNIQUE NOT NULL, password_hash text NOT NULL
);
CREATE TABLE platform_identity.workspaces (
 id text PRIMARY KEY, name text NOT NULL CHECK(char_length(name) BETWEEN 2 AND 60)
);
CREATE TABLE platform_identity.memberships (
 user_id text NOT NULL REFERENCES platform_identity.users(id),
 tenant_id text NOT NULL REFERENCES platform_identity.workspaces(id),
 role text NOT NULL CHECK(role='owner'), PRIMARY KEY(user_id,tenant_id)
);
CREATE TABLE platform_identity.sessions (
 token_hash text PRIMARY KEY, user_id text NOT NULL REFERENCES platform_identity.users(id),
 expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_expiry ON platform_identity.sessions(expires_at);

CREATE TABLE platform_app.releases (
 app_id text NOT NULL, version text NOT NULL, manifest json NOT NULL,
 signature text NOT NULL, public_key text NOT NULL, PRIMARY KEY(app_id,version)
);
CREATE FUNCTION platform_app.reject_release_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'published releases are immutable'; END $$;
CREATE TRIGGER immutable_release BEFORE UPDATE OR DELETE ON platform_app.releases
 FOR EACH ROW EXECUTE FUNCTION platform_app.reject_release_change();

CREATE TABLE platform_installation.installations (
 tenant_id text NOT NULL, app_id text NOT NULL, id text NOT NULL,
 version text NOT NULL, status text NOT NULL CHECK(status IN ('pending','active','uninstalled')),
 scopes jsonb NOT NULL, capabilities jsonb NOT NULL, installed_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(tenant_id,app_id), UNIQUE(tenant_id,id)
);
CREATE TABLE platform_installation.idempotency (
 tenant_id text NOT NULL, actor_id text NOT NULL, key text NOT NULL,
 request_hash text NOT NULL, response jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,actor_id,key)
);
-- Durable audit + transactional outbox. A future NATS relay marks published_at
-- only after broker acknowledgement; pending rows are not "delivered".
CREATE TABLE platform_installation.events (
 id text PRIMARY KEY, tenant_id text NOT NULL, envelope jsonb NOT NULL,
 occurred_at timestamptz NOT NULL, published_at timestamptz
);
CREATE INDEX installation_events_tenant ON platform_installation.events(tenant_id,occurred_at DESC,id);
CREATE INDEX installation_outbox_pending ON platform_installation.events(occurred_at) WHERE published_at IS NULL;

CREATE TABLE platform_capability.resources (
 tenant_id text NOT NULL, installation_id text NOT NULL, id text NOT NULL,
 capability text NOT NULL, data jsonb NOT NULL, PRIMARY KEY(tenant_id,id)
);
CREATE TABLE platform_capability.idempotency (
 tenant_id text NOT NULL, actor_id text NOT NULL, key text NOT NULL,
 request_hash text NOT NULL, response jsonb NOT NULL, PRIMARY KEY(tenant_id,actor_id,key)
);
CREATE TABLE platform_capability.events (
 id text PRIMARY KEY, tenant_id text NOT NULL, envelope jsonb NOT NULL,
 occurred_at timestamptz NOT NULL, published_at timestamptz
);
