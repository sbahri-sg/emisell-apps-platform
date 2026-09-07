-- Resource UI distribution uses the same reviewed/audited assignment workflow.
-- This is metadata distribution only; it never creates installation authority.
ALTER TABLE platform_app.test_assignments ADD COLUMN resource_release_id text REFERENCES platform_app.ui_resource_releases(id);
ALTER TABLE platform_app.test_assignments DROP CONSTRAINT test_assignment_one_source;
ALTER TABLE platform_app.test_assignments ADD CONSTRAINT test_assignment_one_source
 CHECK(num_nonnulls(release_id,managed_release_id,ui_release_id,resource_release_id)=1);
CREATE UNIQUE INDEX test_assignment_resource_current ON platform_app.test_assignments(resource_release_id,merchant_id) WHERE status IN ('requested','approved');
CREATE FUNCTION platform_app.protect_resource_test_assignment_source() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.resource_release_id IS DISTINCT FROM OLD.resource_release_id THEN RAISE EXCEPTION 'resource assignment source is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER test_assignment_resource_immutable BEFORE UPDATE ON platform_app.test_assignments FOR EACH ROW EXECUTE FUNCTION platform_app.protect_resource_test_assignment_source();
