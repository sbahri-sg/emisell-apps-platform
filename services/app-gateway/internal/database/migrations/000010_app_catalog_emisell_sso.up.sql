ALTER TABLE merchant_identities
    DROP CONSTRAINT merchant_identities_environment_check;

ALTER TABLE merchant_identities
    ADD CONSTRAINT merchant_identities_environment_check
    CHECK (environment IN ('sandbox', 'production'));

ALTER TABLE merchant_identities
    DROP CONSTRAINT merchant_identities_pkey,
    DROP CONSTRAINT merchant_identities_user_id_key;

ALTER TABLE merchant_identities
    ADD CONSTRAINT merchant_identities_pkey PRIMARY KEY (merchant_id, user_id, environment);

ALTER TABLE identity_sessions
    ADD COLUMN merchant_id uuid,
    ADD COLUMN merchant_environment text CHECK (merchant_environment IN ('sandbox', 'production')),
    ADD CONSTRAINT identity_sessions_merchant_binding_check CHECK (
        (merchant_id IS NULL AND merchant_environment IS NULL) OR
        (merchant_id IS NOT NULL AND merchant_environment IS NOT NULL)
    ),
    ADD CONSTRAINT identity_sessions_merchant_binding_fk
        FOREIGN KEY (merchant_id, user_id, merchant_environment)
        REFERENCES merchant_identities (merchant_id, user_id, environment)
        ON DELETE CASCADE;

CREATE INDEX identity_sessions_merchant_idx
    ON identity_sessions (merchant_id, merchant_environment, created_at DESC)
    WHERE merchant_id IS NOT NULL;

CREATE TABLE app_catalog_listings (
    app_id uuid PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    category text NOT NULL CHECK (category IN ('payment', 'shipping', 'erp', 'marketing', 'operations', 'custom')),
    status text NOT NULL CHECK (status IN ('draft', 'published', 'hidden')),
    featured boolean NOT NULL DEFAULT false,
    published_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    published_at timestamptz,
    updated_at timestamptz NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    CHECK (status <> 'published' OR (published_by IS NOT NULL AND published_at IS NOT NULL))
);

CREATE INDEX app_catalog_listings_status_idx
    ON app_catalog_listings (status, featured DESC, published_at DESC, app_id DESC);

CREATE INDEX app_catalog_listings_category_idx
    ON app_catalog_listings (category, status, published_at DESC);

CREATE TABLE merchant_session_grants (
    id uuid PRIMARY KEY,
    code_hash text NOT NULL UNIQUE CHECK (code_hash ~ '^[0-9a-f]{64}$'),
    source_jti text NOT NULL UNIQUE CHECK (char_length(source_jti) BETWEEN 16 AND 128),
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 200),
    actor_email text NOT NULL,
    actor_display_name text NOT NULL CHECK (char_length(actor_display_name) BETWEEN 1 AND 120),
    merchant_id uuid NOT NULL,
    merchant_name text NOT NULL CHECK (char_length(merchant_name) BETWEEN 2 AND 120),
    merchant_domain text,
    environment text NOT NULL CHECK (environment IN ('sandbox', 'production')),
    permissions jsonb NOT NULL CHECK (jsonb_typeof(permissions) = 'array'),
    return_to text NOT NULL CHECK (return_to ~ '^/[^/].*' OR return_to = '/'),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at)
);

CREATE INDEX merchant_session_grants_expiry_idx
    ON merchant_session_grants (expires_at)
    WHERE consumed_at IS NULL;

COMMENT ON TABLE app_catalog_listings IS 'Emisell operator-curated App Store visibility. Active app/version eligibility is rechecked on every catalog read.';
COMMENT ON TABLE merchant_session_grants IS 'Short-lived, one-time backend-to-browser SSO bridge. Only code digests are persisted.';
COMMENT ON COLUMN identity_sessions.merchant_id IS 'Explicit store binding for an isolated merchant session; null for developer/admin sessions.';
