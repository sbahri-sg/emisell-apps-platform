-- Explicit resource pilot enrollment. No automatic enrollment or grants.
CREATE TABLE platform_app.resource_release_assignments (
 id text PRIMARY KEY,
 merchant_id text NOT NULL,
 app_id text NOT NULL,
 version text NOT NULL,
 release_id text NOT NULL,
 envelope jsonb NOT NULL,
 signature bytea NOT NULL CHECK(octet_length(signature)=64),
 status text NOT NULL DEFAULT 'approved' CHECK(status IN ('approved','revoked')),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(merchant_id,app_id,version)
);
CREATE FUNCTION platform_app.protect_resource_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.merchant_id IS DISTINCT FROM OLD.merchant_id
 OR NEW.app_id IS DISTINCT FROM OLD.app_id OR NEW.version IS DISTINCT FROM OLD.version
 OR NEW.release_id IS DISTINCT FROM OLD.release_id OR NEW.envelope IS DISTINCT FROM OLD.envelope
 OR NEW.signature IS DISTINCT FROM OLD.signature OR NEW.created_at IS DISTINCT FROM OLD.created_at
 OR OLD.status <> 'approved' OR NEW.status <> 'revoked'
 THEN RAISE EXCEPTION 'immutable resource assignment'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER resource_assignment_immutable BEFORE UPDATE ON platform_app.resource_release_assignments
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_resource_assignment();
