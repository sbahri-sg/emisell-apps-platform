-- Additive remote CONFIGURATION pipeline. No executable registry/install changes.
CREATE TABLE platform_app.integration_releases (
 id text PRIMARY KEY, organization_id text NOT NULL, app_id text NOT NULL,
 submission_id text NOT NULL UNIQUE, version text NOT NULL,
 manifest jsonb NOT NULL, sha256 text NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'),
 package jsonb, status text NOT NULL CHECK(status IN ('submitted','approved','rejected','signed','suspended')),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(app_id,version),
 CHECK(status <> 'signed' OR package IS NOT NULL),
 CHECK(package IS NULL OR status IN ('signed','suspended'))
);
CREATE INDEX integration_owner ON platform_app.integration_releases(organization_id,created_at DESC,id);
CREATE TABLE platform_app.integration_requests (
 actor_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL,
 release_id text NOT NULL REFERENCES platform_app.integration_releases(id),
 PRIMARY KEY(actor_id,request_key)
);
CREATE TABLE platform_app.integration_audit (
 id text PRIMARY KEY, release_id text NOT NULL REFERENCES platform_app.integration_releases(id),
 actor_id text NOT NULL, action text NOT NULL, reason text NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_app.protect_integration_release() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.organization_id IS DISTINCT FROM OLD.organization_id
 OR NEW.app_id IS DISTINCT FROM OLD.app_id OR NEW.submission_id IS DISTINCT FROM OLD.submission_id
 OR NEW.version IS DISTINCT FROM OLD.version OR NEW.manifest IS DISTINCT FROM OLD.manifest
 OR NEW.sha256 IS DISTINCT FROM OLD.sha256 OR NEW.created_at IS DISTINCT FROM OLD.created_at
 OR (OLD.package IS NOT NULL AND NEW.package IS DISTINCT FROM OLD.package)
 THEN RAISE EXCEPTION 'integration snapshot and signature are immutable'; END IF;
 IF NEW.revision <> OLD.revision + 1 OR NOT (
   (OLD.status='submitted' AND NEW.status IN ('approved','rejected')) OR
   (OLD.status='approved' AND NEW.status IN ('signed','suspended')) OR
   (OLD.status='signed' AND NEW.status='suspended')
 ) THEN RAISE EXCEPTION 'invalid integration state transition'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER integration_release_immutable BEFORE UPDATE ON platform_app.integration_releases
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_integration_release();
