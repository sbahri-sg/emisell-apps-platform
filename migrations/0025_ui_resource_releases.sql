-- UI resource authoring; does not create assignments, installations or grants.
CREATE TABLE platform_app.ui_resource_releases (
 id text PRIMARY KEY, app_id text NOT NULL, organization_id text NOT NULL,
 version text NOT NULL, document jsonb NOT NULL,
 UNIQUE(app_id,version)
);
CREATE TABLE platform_app.ui_resource_release_requests (
 actor_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL,
 release_id text NOT NULL REFERENCES platform_app.ui_resource_releases(id),
 PRIMARY KEY(actor_id,request_key)
);
CREATE TABLE platform_app.ui_resource_release_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 release_id text NOT NULL REFERENCES platform_app.ui_resource_releases(id),
 actor_id text NOT NULL, action text NOT NULL, reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000),
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_app.protect_ui_resource_release() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.app_id IS DISTINCT FROM OLD.app_id
 OR NEW.organization_id IS DISTINCT FROM OLD.organization_id OR NEW.version IS DISTINCT FROM OLD.version
 OR NEW.document->'manifest' IS DISTINCT FROM OLD.document->'manifest'
 OR NEW.document->>'id' IS DISTINCT FROM OLD.document->>'id'
 OR (NEW.document->>'revision')::integer <> (OLD.document->>'revision')::integer+1
 OR (OLD.document ? 'package' AND NEW.document->'package' IS DISTINCT FROM OLD.document->'package')
 THEN RAISE EXCEPTION 'immutable UI release'; END IF;
 IF NOT ((OLD.document->>'status'='submitted' AND NEW.document->>'status' IN ('approved','rejected'))
 OR (OLD.document->>'status'='approved' AND NEW.document->>'status' IN ('signed','suspended'))
 OR (OLD.document->>'status'='signed' AND NEW.document->>'status'='suspended'))
 THEN RAISE EXCEPTION 'invalid UI release transition'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER ui_resource_release_immutable BEFORE UPDATE ON platform_app.ui_resource_releases
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_ui_resource_release();
