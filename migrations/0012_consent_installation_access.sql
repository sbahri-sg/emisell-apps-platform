-- Additive local-fixture lifecycle; existing intents, installations and keys stay unchanged.
ALTER TABLE platform_installation.installations ADD COLUMN intent_id text
 REFERENCES platform_installation.install_intents(id);

CREATE TABLE platform_installation.intent_consumptions (
 intent_id text PRIMARY KEY REFERENCES platform_installation.install_intents(id),
 tenant_id text NOT NULL, service_id text NOT NULL, actor_id text NOT NULL,
 installation_id text NOT NULL, release jsonb NOT NULL, consent_digest text NOT NULL,
 consumed_at timestamptz NOT NULL,
 UNIQUE(tenant_id,installation_id)
);
CREATE TABLE platform_installation.access_grants (
 tenant_id text NOT NULL, installation_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('pending','active','revoked')),
 scopes jsonb NOT NULL, revoked_at timestamptz,
 PRIMARY KEY(tenant_id,installation_id),
 FOREIGN KEY(tenant_id,installation_id) REFERENCES platform_installation.intent_consumptions(tenant_id,installation_id),
 CHECK ((state='revoked') = (revoked_at IS NOT NULL))
);
CREATE TABLE platform_installation.app_tokens (
 id text PRIMARY KEY, tenant_id text NOT NULL, installation_id text NOT NULL,
 token_hash text UNIQUE NOT NULL CHECK(token_hash ~ '^[a-f0-9]{64}$'),
 audience text NOT NULL CHECK(audience='emisell.app-platform.local/installation-access'),
 created_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
 revoked_at timestamptz,
 FOREIGN KEY(tenant_id,installation_id) REFERENCES platform_installation.access_grants(tenant_id,installation_id),
 CHECK(expires_at > created_at AND expires_at <= created_at + interval '15 minutes')
);
CREATE UNIQUE INDEX one_live_app_token ON platform_installation.app_tokens(tenant_id,installation_id) WHERE revoked_at IS NULL;
CREATE TABLE platform_installation.access_requests (
 tenant_id text NOT NULL, service_id text NOT NULL, actor_id text NOT NULL,
 request_key text NOT NULL, request_hash text NOT NULL,
 installation_id text NOT NULL, token_id text REFERENCES platform_installation.app_tokens(id),
 PRIMARY KEY(tenant_id,service_id,actor_id,request_key),
 FOREIGN KEY(tenant_id,installation_id) REFERENCES platform_installation.intent_consumptions(tenant_id,installation_id)
);
CREATE TABLE platform_installation.access_audit (
 id text PRIMARY KEY, tenant_id text NOT NULL, service_id text NOT NULL, actor_id text NOT NULL,
 installation_id text NOT NULL, intent_id text NOT NULL,
 action text NOT NULL CHECK(action IN ('consumed','activated','token_issued','uninstalled')),
 consent_digest text NOT NULL, occurred_at timestamptz NOT NULL
);
CREATE INDEX access_audit_installation ON platform_installation.access_audit(tenant_id,installation_id,occurred_at);

-- Every path, including the compatibility lifecycle and cleanup worker, revokes
-- access in the same transaction before disabled state becomes observable.
CREATE FUNCTION platform_installation.revoke_installation_access() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status IN ('disabling','uninstalled') THEN
  UPDATE platform_installation.access_grants SET state='revoked',scopes='[]'::jsonb,revoked_at=COALESCE(revoked_at,clock_timestamp())
   WHERE tenant_id=OLD.tenant_id AND installation_id=OLD.id AND state!='revoked';
  UPDATE platform_installation.app_tokens SET revoked_at=clock_timestamp()
   WHERE tenant_id=OLD.tenant_id AND installation_id=OLD.id AND revoked_at IS NULL;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER revoke_installation_access BEFORE UPDATE ON platform_installation.installations
 FOR EACH ROW EXECUTE FUNCTION platform_installation.revoke_installation_access();
