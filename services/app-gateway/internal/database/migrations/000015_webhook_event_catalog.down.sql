DROP INDEX IF EXISTS webhook_events_installation_created_idx;

ALTER TABLE webhook_events
    DROP COLUMN IF EXISTS installation_id,
    DROP COLUMN IF EXISTS merchant_id,
    DROP COLUMN IF EXISTS source;

DROP TABLE IF EXISTS webhook_event_definitions;
