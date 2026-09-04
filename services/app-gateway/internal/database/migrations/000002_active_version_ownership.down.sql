ALTER TABLE apps DROP CONSTRAINT apps_active_version_fk;

ALTER TABLE app_versions DROP CONSTRAINT app_versions_app_id_id_unique;

ALTER TABLE apps
    ADD CONSTRAINT apps_active_version_fk
    FOREIGN KEY (active_version_id)
    REFERENCES app_versions(id)
    ON DELETE SET NULL
    DEFERRABLE INITIALLY DEFERRED;
