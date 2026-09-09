-- Active developer configuration is separate from executable releases and store grants.
ALTER TABLE platform_app.drafts ADD COLUMN active_version jsonb;

-- Upgrade only untouched, never-submitted apps. Existing release/review history stays intact.
UPDATE platform_app.drafts d
SET active_version=jsonb_build_object(
  'document',d.document,'revision',d.revision,'activatedAt',d.updated_at)
WHERE d.revision=1
  AND NOT EXISTS(SELECT 1 FROM platform_review.submissions s WHERE s.app_id=d.id);

INSERT INTO platform_app.draft_audit(id,app_id,organization_id,actor_id,revision,action)
SELECT 'initial-version-upgrade-' || id,id,organization_id,'migration-0033',1,'initial_version_activated'
FROM platform_app.drafts WHERE active_version IS NOT NULL;

-- Editing the working draft cannot silently replace the active snapshot.
CREATE FUNCTION platform_app.protect_initial_active_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.active_version IS DISTINCT FROM OLD.active_version THEN
    RAISE EXCEPTION 'active app configuration is immutable';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER active_app_version_immutable BEFORE UPDATE ON platform_app.drafts
FOR EACH ROW EXECUTE FUNCTION platform_app.protect_initial_active_version();
