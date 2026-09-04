ALTER TABLE apps DROP CONSTRAINT apps_active_version_fk;

ALTER TABLE app_versions
    ADD CONSTRAINT app_versions_app_id_id_unique UNIQUE (app_id, id);

ALTER TABLE apps
    ADD CONSTRAINT apps_active_version_fk
    FOREIGN KEY (id, active_version_id)
    REFERENCES app_versions(app_id, id)
    DEFERRABLE INITIALLY DEFERRED;

COMMENT ON CONSTRAINT apps_active_version_fk ON apps IS 'Ensures an app can only reference one of its own versions.';
