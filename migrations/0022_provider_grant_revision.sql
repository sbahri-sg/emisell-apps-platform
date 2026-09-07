-- Monotonic access revision, including revocation triggered by uninstall.
CREATE SEQUENCE platform_installation.provider_grant_revision_seq;
ALTER TABLE platform_installation.access_grants ADD COLUMN provider_revision bigint NOT NULL
 DEFAULT nextval('platform_installation.provider_grant_revision_seq');
CREATE FUNCTION platform_installation.bump_provider_grant_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.provider_revision := nextval('platform_installation.provider_grant_revision_seq');
 RETURN NEW;
END $$;
CREATE TRIGGER provider_grant_revision BEFORE UPDATE ON platform_installation.access_grants
 FOR EACH ROW EXECUTE FUNCTION platform_installation.bump_provider_grant_revision();
