-- Authoring requests only. No merchant routing, delivery or grants are created.
CREATE TABLE platform_webhook.subscription_requests (
 id text PRIMARY KEY,
 organization_id text NOT NULL REFERENCES platform_developer.organizations(id),
 app_id text NOT NULL REFERENCES platform_app.drafts(id),
 draft_revision integer NOT NULL CHECK(draft_revision > 0),
 version text NOT NULL, topic text NOT NULL, required_scope text NOT NULL,
 endpoint text NOT NULL, actor_id text NOT NULL,
 request_key text NOT NULL, request_hash text NOT NULL,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','revoked')),
 created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz,
 UNIQUE(organization_id,actor_id,request_key),
 CHECK((status='revoked')=(revoked_at IS NOT NULL))
);
CREATE UNIQUE INDEX webhook_one_pending_topic ON platform_webhook.subscription_requests(app_id,version,topic) WHERE status='pending';
CREATE INDEX webhook_subscription_owner ON platform_webhook.subscription_requests(organization_id,app_id,id);
CREATE TABLE platform_webhook.subscription_audit (
 id text PRIMARY KEY, subscription_id text NOT NULL REFERENCES platform_webhook.subscription_requests(id),
 actor_id text NOT NULL, action text NOT NULL CHECK(action IN ('requested','revoked')),
 occurred_at timestamptz NOT NULL DEFAULT now()
);
