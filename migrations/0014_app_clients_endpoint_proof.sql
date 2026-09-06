-- Confidential developer-app identities, NOT Core keys or installation tokens.
CREATE TABLE platform_oauth.app_clients (
 id text PRIMARY KEY, organization_id text NOT NULL, release_id text NOT NULL UNIQUE,
 binding jsonb NOT NULL, status text NOT NULL CHECK(status IN ('pending','verified','revoked')),
 revision integer NOT NULL CHECK(revision>0), challenge_id text NOT NULL, challenge text NOT NULL,
 challenge_expires_at timestamptz NOT NULL, verified_until timestamptz,
 last_attempt_at timestamptz, last_result text NOT NULL,
 secret_hash text NOT NULL DEFAULT '' CHECK(secret_hash='' OR secret_hash ~ '^[0-9a-f]{64}$'),
 secret_version integer NOT NULL DEFAULT 0 CHECK(secret_version>=0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(status='verified' OR secret_hash=''), CHECK(status<>'verified' OR verified_until IS NOT NULL)
);
CREATE INDEX app_clients_owner ON platform_oauth.app_clients(organization_id,created_at DESC,id);
CREATE TABLE platform_oauth.app_client_requests (
 actor_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL,
 client_id text NOT NULL REFERENCES platform_oauth.app_clients(id), PRIMARY KEY(actor_id,request_key)
);
CREATE TABLE platform_oauth.app_client_audit (
 id text PRIMARY KEY, client_id text NOT NULL REFERENCES platform_oauth.app_clients(id),
 actor_id text NOT NULL, action text NOT NULL, reason text NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_oauth.protect_app_client() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.organization_id IS DISTINCT FROM OLD.organization_id
 OR NEW.release_id IS DISTINCT FROM OLD.release_id OR NEW.binding IS DISTINCT FROM OLD.binding
 OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.revision <> OLD.revision+1
 OR (OLD.status='revoked' AND NEW.status<>'revoked')
 THEN RAISE EXCEPTION 'immutable client binding or invalid transition'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER app_client_binding_immutable BEFORE UPDATE ON platform_oauth.app_clients
 FOR EACH ROW EXECUTE FUNCTION platform_oauth.protect_app_client();
