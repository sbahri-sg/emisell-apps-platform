ALTER TABLE platform_app.test_assignments ADD COLUMN ui_release_id text REFERENCES platform_app.ui_releases(id);
ALTER TABLE platform_app.test_assignments DROP CONSTRAINT test_assignment_one_source;
ALTER TABLE platform_app.test_assignments ADD CONSTRAINT test_assignment_one_source
 CHECK(num_nonnulls(release_id,managed_release_id,ui_release_id)=1);
CREATE UNIQUE INDEX test_assignment_ui_current ON platform_app.test_assignments(ui_release_id,merchant_id) WHERE status IN ('requested','approved');
CREATE FUNCTION platform_app.protect_ui_assignment_source() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.ui_release_id IS DISTINCT FROM OLD.ui_release_id THEN RAISE EXCEPTION 'UI assignment source is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER test_assignment_ui_immutable BEFORE UPDATE ON platform_app.test_assignments FOR EACH ROW EXECUTE FUNCTION platform_app.protect_ui_assignment_source();
