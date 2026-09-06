-- Reviewed engine/provider bindings; no existing grants, providers or secrets changed.
CREATE TABLE platform_app.managed_shipping_releases (
 id text PRIMARY KEY, organization_id text NOT NULL, app_id text NOT NULL,
 draft_revision integer NOT NULL CHECK(draft_revision>0), version text NOT NULL,
 manifest jsonb NOT NULL, sha256 text NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'),
 package jsonb, status text NOT NULL CHECK(status IN ('submitted','approved','rejected','signed','suspended')),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(app_id,version),
 CHECK(status <> 'signed' OR package IS NOT NULL),
 CHECK(package IS NULL OR status IN ('signed','suspended'))
);
CREATE INDEX managed_shipping_owner ON platform_app.managed_shipping_releases(organization_id,created_at DESC,id);
CREATE TABLE platform_app.managed_shipping_requests (
 actor_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL,
 release_id text NOT NULL REFERENCES platform_app.managed_shipping_releases(id), PRIMARY KEY(actor_id,request_key)
);
CREATE TABLE platform_app.managed_shipping_audit (
 id text PRIMARY KEY, release_id text NOT NULL REFERENCES platform_app.managed_shipping_releases(id),
 actor_id text NOT NULL, action text NOT NULL, reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000),
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_app.protect_managed_shipping_release() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.organization_id IS DISTINCT FROM OLD.organization_id
 OR NEW.app_id IS DISTINCT FROM OLD.app_id OR NEW.draft_revision IS DISTINCT FROM OLD.draft_revision
 OR NEW.version IS DISTINCT FROM OLD.version OR NEW.manifest IS DISTINCT FROM OLD.manifest
 OR NEW.sha256 IS DISTINCT FROM OLD.sha256 OR NEW.created_at IS DISTINCT FROM OLD.created_at
 OR (OLD.package IS NOT NULL AND NEW.package IS DISTINCT FROM OLD.package)
 THEN RAISE EXCEPTION 'managed shipping snapshot and signature are immutable'; END IF;
 IF NEW.revision <> OLD.revision + 1 OR NOT (
 (OLD.status='submitted' AND NEW.status IN ('approved','rejected')) OR
 (OLD.status='approved' AND NEW.status IN ('signed','suspended')) OR
 (OLD.status='signed' AND NEW.status='suspended')) THEN RAISE EXCEPTION 'invalid managed shipping transition'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER managed_shipping_release_immutable BEFORE UPDATE ON platform_app.managed_shipping_releases
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_managed_shipping_release();
