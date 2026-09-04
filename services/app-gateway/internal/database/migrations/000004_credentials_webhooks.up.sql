ALTER TABLE app_credentials
    ADD COLUMN environment text NOT NULL DEFAULT 'production',
    ADD COLUMN last_used_at timestamptz,
    ADD COLUMN expires_at timestamptz;

ALTER TABLE app_credentials
    ADD CONSTRAINT app_credentials_environment_check
    CHECK (environment IN ('production', 'sandbox'));

ALTER TABLE webhook_subscriptions
    ADD COLUMN encryption_key_version integer NOT NULL DEFAULT 1
    CHECK (encryption_key_version > 0);

ALTER TABLE webhook_subscriptions
    DROP CONSTRAINT webhook_subscriptions_status_check,
    ADD CONSTRAINT webhook_subscriptions_status_check
    CHECK (status IN ('pending', 'active', 'paused', 'failing', 'disabled'));

ALTER TABLE webhook_subscriptions
    DROP CONSTRAINT webhook_subscriptions_app_id_event_endpoint_url_key;

CREATE UNIQUE INDEX webhook_subscriptions_active_endpoint_unique
    ON webhook_subscriptions (app_id, event, endpoint_url)
    WHERE status <> 'disabled';

CREATE INDEX app_credentials_active_idx
    ON app_credentials (app_id, environment, created_at DESC)
    WHERE status = 'active';

COMMENT ON COLUMN webhook_subscriptions.signing_secret_ciphertext IS 'AES-256-GCM encrypted signing secret; plaintext is only returned during creation.';
