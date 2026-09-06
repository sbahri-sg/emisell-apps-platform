-- Consent records only: no active permission, installation or token is created.
CREATE TABLE platform_installation.install_intents (
 id text PRIMARY KEY,
 tenant_id text NOT NULL,
 service_id text NOT NULL,
 actor_id text NOT NULL,
 snapshot jsonb NOT NULL,
 state text NOT NULL CHECK(state IN ('pending','consented','denied')),
 created_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL CHECK(expires_at > created_at),
 decided_at timestamptz,
 CHECK ((state='pending' AND decided_at IS NULL) OR (state!='pending' AND decided_at IS NOT NULL))
);
CREATE INDEX install_intents_owner ON platform_installation.install_intents(tenant_id,service_id,actor_id,id);

CREATE TABLE platform_installation.intent_requests (
 tenant_id text NOT NULL, service_id text NOT NULL, actor_id text NOT NULL,
 request_key text NOT NULL, request_hash text NOT NULL,
 intent_id text NOT NULL REFERENCES platform_installation.install_intents(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,service_id,actor_id,request_key)
);
CREATE TABLE platform_installation.intent_audit (
 id text PRIMARY KEY,
 intent_id text NOT NULL REFERENCES platform_installation.install_intents(id),
 tenant_id text NOT NULL, service_id text NOT NULL, actor_id text NOT NULL,
 action text NOT NULL CHECK(action IN ('prepared','consented','denied')),
 consent_digest text NOT NULL,
 occurred_at timestamptz NOT NULL
);
CREATE INDEX intent_audit_tenant ON platform_installation.intent_audit(tenant_id,occurred_at,id);

CREATE FUNCTION platform_installation.protect_intent_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
 OR NEW.service_id IS DISTINCT FROM OLD.service_id OR NEW.actor_id IS DISTINCT FROM OLD.actor_id
 OR NEW.snapshot IS DISTINCT FROM OLD.snapshot OR NEW.created_at IS DISTINCT FROM OLD.created_at
 OR NEW.expires_at IS DISTINCT FROM OLD.expires_at THEN
  RAISE EXCEPTION 'intent snapshot is immutable';
 END IF;
 IF OLD.state != 'pending' OR NEW.state NOT IN ('consented','denied') THEN
  RAISE EXCEPTION 'intent decision is single use';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER immutable_intent_snapshot BEFORE UPDATE ON platform_installation.install_intents
 FOR EACH ROW EXECUTE FUNCTION platform_installation.protect_intent_snapshot();
