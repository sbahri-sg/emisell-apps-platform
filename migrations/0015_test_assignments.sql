-- Distribution permission only; never an installation, grant, token or runtime registration.
CREATE TABLE platform_app.test_assignments (
 id text PRIMARY KEY, organization_id text NOT NULL,
 release_id text NOT NULL REFERENCES platform_app.integration_releases(id),
 release_sha256 text NOT NULL CHECK(release_sha256 ~ '^[0-9a-f]{64}$'),
 merchant_id text NOT NULL CHECK(merchant_id ~ '^[A-Za-z0-9_-]{1,100}$'),
 status text NOT NULL CHECK(status IN ('requested','approved','rejected','revoked')),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX test_assignment_current ON platform_app.test_assignments(release_id,merchant_id)
 WHERE status IN ('requested','approved');
CREATE INDEX test_assignment_org ON platform_app.test_assignments(organization_id,id);
CREATE INDEX test_assignment_merchant ON platform_app.test_assignments(merchant_id,id) WHERE status='approved';
CREATE TABLE platform_app.test_assignment_requests (
 actor_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL,
 assignment_id text NOT NULL REFERENCES platform_app.test_assignments(id), PRIMARY KEY(actor_id,request_key)
);
CREATE TABLE platform_app.test_assignment_audit (
 id text PRIMARY KEY, assignment_id text NOT NULL REFERENCES platform_app.test_assignments(id),
 actor_id text NOT NULL, action text NOT NULL, reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000),
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_app.protect_test_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.organization_id IS DISTINCT FROM OLD.organization_id
 OR NEW.release_id IS DISTINCT FROM OLD.release_id OR NEW.release_sha256 IS DISTINCT FROM OLD.release_sha256
 OR NEW.merchant_id IS DISTINCT FROM OLD.merchant_id OR NEW.created_at IS DISTINCT FROM OLD.created_at
 THEN RAISE EXCEPTION 'test assignment target is immutable'; END IF;
 IF NEW.revision <> OLD.revision + 1 OR NOT (
 (OLD.status='requested' AND NEW.status IN ('approved','rejected')) OR
 (OLD.status='approved' AND NEW.status='revoked')) THEN RAISE EXCEPTION 'invalid assignment transition'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER test_assignment_immutable BEFORE UPDATE ON platform_app.test_assignments
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_test_assignment();
