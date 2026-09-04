DROP INDEX IF EXISTS webhook_deliveries_subscription_attempt_idx;
DROP INDEX IF EXISTS webhook_deliveries_retry_idx;
ALTER TABLE webhook_deliveries
    DROP COLUMN IF EXISTS response_time_ms,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS claimed_at,
    DROP CONSTRAINT IF EXISTS webhook_deliveries_event_fk;
CREATE INDEX webhook_deliveries_retry_idx
    ON webhook_deliveries (status, next_attempt_at)
    WHERE status IN ('pending', 'failed');
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS oauth_access_tokens;
DROP TABLE IF EXISTS oauth_authorizations;
