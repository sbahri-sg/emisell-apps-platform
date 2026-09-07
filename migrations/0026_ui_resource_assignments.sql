-- Explicit merchant distribution; never creates installation or consent.
CREATE TABLE platform_app.ui_resource_assignments (
 id text PRIMARY KEY,
 merchant_id text NOT NULL,
 organization_id text NOT NULL,
 app_id text NOT NULL,
 version text NOT NULL,
 release_id text NOT NULL REFERENCES platform_app.ui_resource_releases(id),
 client_id text NOT NULL REFERENCES platform_oauth.app_clients(id),
 status text NOT NULL CHECK(status IN ('approved','revoked')),
 actor_id text NOT NULL,
 reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(merchant_id,app_id,version)
);
CREATE FUNCTION platform_app.protect_ui_resource_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (to_jsonb(NEW)-'status') IS DISTINCT FROM (to_jsonb(OLD)-'status')
 OR OLD.status <> 'approved' OR NEW.status <> 'revoked'
 THEN RAISE EXCEPTION 'immutable UI resource assignment'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER ui_resource_assignment_immutable BEFORE UPDATE ON platform_app.ui_resource_assignments
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_ui_resource_assignment();
