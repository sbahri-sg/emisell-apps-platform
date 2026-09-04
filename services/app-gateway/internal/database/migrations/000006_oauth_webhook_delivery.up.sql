CREATE TABLE oauth_authorizations (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    credential_id uuid NOT NULL REFERENCES app_credentials(id) ON DELETE RESTRICT,
    client_id text NOT NULL,
    code_hash char(64) NOT NULL UNIQUE,
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL CHECK (char_length(code_challenge) BETWEEN 43 AND 128),
    merchant_id uuid NOT NULL,
    merchant_name text NOT NULL CHECK (char_length(merchant_name) BETWEEN 2 AND 120),
    merchant_domain text,
    environment text NOT NULL CHECK (environment IN ('production', 'sandbox')),
    installed_version_id uuid NOT NULL REFERENCES app_versions(id) ON DELETE RESTRICT,
    granted_scopes jsonb NOT NULL CHECK (jsonb_typeof(granted_scopes) = 'array'),
    approved_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at)
);

CREATE INDEX oauth_authorizations_expiry_idx
    ON oauth_authorizations (expires_at)
    WHERE consumed_at IS NULL;
CREATE INDEX oauth_authorizations_app_created_idx
    ON oauth_authorizations (app_id, created_at DESC);

CREATE TABLE oauth_access_tokens (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    installation_id uuid NOT NULL REFERENCES app_installations(id) ON DELETE CASCADE,
    credential_id uuid NOT NULL REFERENCES app_credentials(id) ON DELETE RESTRICT,
    token_hash char(64) NOT NULL UNIQUE,
    scopes jsonb NOT NULL CHECK (jsonb_typeof(scopes) = 'array'),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at > created_at)
);

CREATE INDEX oauth_access_tokens_installation_idx
    ON oauth_access_tokens (installation_id, created_at DESC);
CREATE INDEX oauth_access_tokens_expiry_idx
    ON oauth_access_tokens (expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE webhook_events (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    event text NOT NULL,
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at timestamptz NOT NULL
);

CREATE INDEX webhook_events_app_created_idx
    ON webhook_events (app_id, created_at DESC, id DESC);

ALTER TABLE webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_event_fk
    FOREIGN KEY (event_id) REFERENCES webhook_events(id) ON DELETE CASCADE NOT VALID,
    ADD COLUMN claimed_at timestamptz,
    ADD COLUMN completed_at timestamptz,
    ADD COLUMN response_time_ms bigint CHECK (response_time_ms IS NULL OR response_time_ms >= 0);

DROP INDEX webhook_deliveries_retry_idx;
CREATE INDEX webhook_deliveries_retry_idx
    ON webhook_deliveries (next_attempt_at, attempted_at, id)
    WHERE status IN ('pending', 'failed');
CREATE INDEX webhook_deliveries_subscription_attempt_idx
    ON webhook_deliveries (subscription_id, attempted_at DESC, id DESC);

COMMENT ON COLUMN oauth_authorizations.code_hash IS 'SHA-256 digest only; plaintext authorization codes are never persisted.';
COMMENT ON COLUMN oauth_access_tokens.token_hash IS 'SHA-256 digest only; plaintext access tokens are returned once and never persisted.';
COMMENT ON COLUMN webhook_events.payload IS 'Delivery payload. Do not publish credentials, tokens, or unnecessary personal data.';
