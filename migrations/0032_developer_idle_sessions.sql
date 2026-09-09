-- Read-only requests do not refresh developer activity.
ALTER TABLE platform_identity.portal_sessions ADD COLUMN last_active_at timestamptz NOT NULL DEFAULT now();
-- Retire legacy sessions without deleting accounts, apps, or audit history.
DELETE FROM platform_identity.portal_sessions s WHERE s.surface='developer'
 AND NOT EXISTS(SELECT 1 FROM platform_identity.developer_core_links l WHERE l.account_id=s.account_id);
UPDATE platform_identity.portal_sessions SET expires_at=least(expires_at,now()+interval '1 hour') WHERE surface='developer';
