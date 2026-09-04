CREATE TABLE organizations (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 2 AND 120),
    slug text NOT NULL CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX organizations_slug_unique ON organizations (lower(slug));

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

CREATE TABLE organization_memberships (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'analyst')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE INDEX organization_memberships_user_idx ON organization_memberships (user_id);

CREATE TABLE apps (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    name text NOT NULL CHECK (char_length(name) BETWEEN 3 AND 80),
    slug text NOT NULL CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    description text,
    distribution text NOT NULL CHECK (distribution IN ('public', 'custom')),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    app_url text,
    contact_email text,
    active_version_id uuid,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0)
);

CREATE UNIQUE INDEX apps_active_slug_unique
    ON apps (organization_id, lower(slug))
    WHERE status <> 'archived';
CREATE INDEX apps_organization_created_idx
    ON apps (organization_id, created_at DESC, id DESC);
CREATE INDEX apps_organization_status_idx
    ON apps (organization_id, status, created_at DESC);

CREATE TABLE app_versions (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version text NOT NULL CHECK (version ~ '^[0-9]+\.[0-9]+\.[0-9]+$'),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'released', 'active')),
    release_note text,
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot) = 'object'),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    released_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    released_at timestamptz,
    UNIQUE (app_id, version)
);

CREATE UNIQUE INDEX app_versions_one_active_per_app
    ON app_versions (app_id)
    WHERE status = 'active';
CREATE INDEX app_versions_app_created_idx
    ON app_versions (app_id, created_at DESC, id DESC);

ALTER TABLE apps
    ADD CONSTRAINT apps_active_version_fk
    FOREIGN KEY (active_version_id)
    REFERENCES app_versions(id)
    ON DELETE SET NULL
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE app_extensions (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    name text NOT NULL,
    type text NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'disabled')),
    runtime_url text,
    configuration jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(configuration) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    UNIQUE (app_id, name)
);

CREATE INDEX app_extensions_app_idx ON app_extensions (app_id, created_at DESC);

CREATE TABLE app_scopes (
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    scope text NOT NULL,
    access text NOT NULL CHECK (access IN ('required', 'optional')),
    description text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (app_id, scope)
);

CREATE TABLE app_credentials (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    client_id text NOT NULL,
    secret_ciphertext bytea NOT NULL,
    secret_fingerprint text NOT NULL,
    encryption_key_version integer NOT NULL CHECK (encryption_key_version > 0),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    rotated_at timestamptz,
    revoked_at timestamptz,
    UNIQUE (client_id)
);

CREATE INDEX app_credentials_app_idx ON app_credentials (app_id, created_at DESC);

CREATE TABLE webhook_subscriptions (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    event text NOT NULL,
    endpoint_url text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'disabled')),
    signing_secret_ciphertext bytea,
    signing_secret_fingerprint text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    UNIQUE (app_id, event, endpoint_url)
);

CREATE INDEX webhook_subscriptions_app_idx ON webhook_subscriptions (app_id, created_at DESC);

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY,
    subscription_id uuid NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_id uuid NOT NULL,
    attempt integer NOT NULL CHECK (attempt > 0),
    status text NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    response_status integer,
    error_code text,
    attempted_at timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz,
    UNIQUE (subscription_id, event_id, attempt)
);

CREATE INDEX webhook_deliveries_retry_idx
    ON webhook_deliveries (status, next_attempt_at)
    WHERE status IN ('pending', 'failed');

CREATE TABLE app_installations (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    merchant_id uuid NOT NULL,
    environment text NOT NULL CHECK (environment IN ('production', 'sandbox')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'suspended', 'uninstalled')),
    granted_scopes jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(granted_scopes) = 'array'),
    installed_at timestamptz,
    uninstalled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX app_installations_active_unique
    ON app_installations (app_id, merchant_id, environment)
    WHERE status <> 'uninstalled';
CREATE INDEX app_installations_app_idx ON app_installations (app_id, created_at DESC);

CREATE TABLE idempotency_keys (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    action text NOT NULL,
    key text NOT NULL CHECK (char_length(key) BETWEEN 16 AND 128),
    resource_type text NOT NULL,
    resource_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL DEFAULT (now() + interval '24 hours'),
    PRIMARY KEY (organization_id, action, key)
);

CREATE INDEX idempotency_keys_expiry_idx ON idempotency_keys (expires_at);

CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at timestamptz NOT NULL
);

CREATE INDEX audit_events_organization_created_idx
    ON audit_events (organization_id, created_at DESC, id DESC);
CREATE INDEX audit_events_resource_idx
    ON audit_events (resource_type, resource_id, created_at DESC);

COMMENT ON TABLE app_versions IS 'Immutable configuration snapshots. Only status, released_by, and released_at change during activation.';
COMMENT ON TABLE audit_events IS 'Append-only security and configuration history.';
COMMENT ON COLUMN app_credentials.secret_ciphertext IS 'Encrypted secret bytes; raw secret values must never be returned after creation or rotation.';
