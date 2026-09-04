DROP INDEX IF EXISTS app_credentials_active_idx;
DROP INDEX IF EXISTS webhook_subscriptions_active_endpoint_unique;

ALTER TABLE webhook_subscriptions
    ADD CONSTRAINT webhook_subscriptions_app_id_event_endpoint_url_key UNIQUE (app_id, event, endpoint_url),
    DROP CONSTRAINT webhook_subscriptions_status_check,
    ADD CONSTRAINT webhook_subscriptions_status_check CHECK (status IN ('active', 'paused', 'disabled')),
    DROP COLUMN encryption_key_version;

ALTER TABLE app_credentials
    DROP CONSTRAINT app_credentials_environment_check,
    DROP COLUMN expires_at,
    DROP COLUMN last_used_at,
    DROP COLUMN environment;
