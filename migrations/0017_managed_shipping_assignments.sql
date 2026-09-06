-- Preserve existing integration assignments and their FK; each row has exactly
-- one independently verified release source. Distribution still is not a grant.
ALTER TABLE platform_app.test_assignments ALTER COLUMN release_id DROP NOT NULL;
ALTER TABLE platform_app.test_assignments ADD COLUMN managed_release_id text
 REFERENCES platform_app.managed_shipping_releases(id);
ALTER TABLE platform_app.test_assignments ADD CONSTRAINT test_assignment_one_source
 CHECK ((release_id IS NOT NULL)::int + (managed_release_id IS NOT NULL)::int = 1);
CREATE UNIQUE INDEX test_assignment_managed_current
 ON platform_app.test_assignments(managed_release_id,merchant_id)
 WHERE status IN ('requested','approved');
CREATE FUNCTION platform_app.protect_managed_assignment_source() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.managed_release_id IS DISTINCT FROM OLD.managed_release_id THEN
  RAISE EXCEPTION 'managed assignment source is immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER test_assignment_managed_immutable BEFORE UPDATE ON platform_app.test_assignments
 FOR EACH ROW EXECUTE FUNCTION platform_app.protect_managed_assignment_source();
