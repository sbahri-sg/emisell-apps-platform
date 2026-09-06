-- Catalog metadata is not an executable app release. Existing registry untouched.
CREATE TABLE platform_app.catalog_releases (
 id text PRIMARY KEY, app_id text NOT NULL, organization_id text NOT NULL,
 submission_id text NOT NULL UNIQUE, version text NOT NULL,
 package jsonb NOT NULL, status text NOT NULL CHECK(status IN ('signed','published','suspended')),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(app_id,version)
);
CREATE UNIQUE INDEX catalog_one_public_version ON platform_app.catalog_releases(app_id) WHERE status='published';
CREATE INDEX catalog_owner ON platform_app.catalog_releases(organization_id,created_at DESC,id);
CREATE TABLE platform_app.catalog_requests (
 actor_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL,
 response jsonb NOT NULL, PRIMARY KEY(actor_id,request_key)
);
CREATE TABLE platform_app.catalog_audit (
 id text PRIMARY KEY, release_id text NOT NULL, actor_id text NOT NULL, action text NOT NULL,
 reason text NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_app.protect_catalog_package() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.app_id IS DISTINCT FROM OLD.app_id
 OR NEW.organization_id IS DISTINCT FROM OLD.organization_id OR NEW.submission_id IS DISTINCT FROM OLD.submission_id
 OR NEW.version IS DISTINCT FROM OLD.version OR NEW.package IS DISTINCT FROM OLD.package
 OR NEW.created_at IS DISTINCT FROM OLD.created_at
 THEN RAISE EXCEPTION 'signed catalog package is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER catalog_package_immutable BEFORE UPDATE ON platform_app.catalog_releases
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_catalog_package();
