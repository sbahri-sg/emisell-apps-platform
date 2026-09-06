-- Additive launch review storage. No existing installation becomes launchable.
CREATE TABLE platform_app.embedded_launches (
 id text PRIMARY KEY,
 app_id text NOT NULL,
 client_id text NOT NULL,
 release_digest text NOT NULL CHECK(release_digest ~ '^[0-9a-f]{64}$'),
 launch jsonb NOT NULL,
 status text NOT NULL DEFAULT 'submitted' CHECK(status IN ('submitted','approved','rejected','revoked')),
 signature text,
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 submitted_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(app_id,client_id,release_digest),
 CHECK((status IN ('approved','revoked') AND signature IS NOT NULL) OR (status IN ('submitted','rejected') AND signature IS NULL))
);
CREATE TABLE platform_app.embedded_launch_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 launch_id text NOT NULL REFERENCES platform_app.embedded_launches(id),
 actor_id text NOT NULL,
 action text NOT NULL,
 reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000),
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_app.protect_embedded_launch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.app_id IS DISTINCT FROM OLD.app_id
 OR NEW.client_id IS DISTINCT FROM OLD.client_id OR NEW.release_digest IS DISTINCT FROM OLD.release_digest
 OR NEW.launch IS DISTINCT FROM OLD.launch OR NEW.submitted_by IS DISTINCT FROM OLD.submitted_by
 OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.revision <> OLD.revision+1
 OR (OLD.signature IS NOT NULL AND NEW.signature IS DISTINCT FROM OLD.signature)
 THEN RAISE EXCEPTION 'immutable embedded launch'; END IF;
 IF NOT ((OLD.status='submitted' AND NEW.status IN ('approved','rejected'))
 OR (OLD.status='approved' AND NEW.status='revoked')) THEN RAISE EXCEPTION 'invalid launch transition'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER embedded_launch_immutable BEFORE UPDATE ON platform_app.embedded_launches
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_embedded_launch();
