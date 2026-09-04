ALTER TABLE app_installations
    ADD COLUMN merchant_name text,
    ADD COLUMN merchant_domain text,
    ADD COLUMN installed_version_id uuid REFERENCES app_versions(id) ON DELETE RESTRICT,
    ADD COLUMN installed_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);

UPDATE app_installations i
SET merchant_name = 'Merchant ' || left(i.merchant_id::text, 8),
    installed_version_id = a.active_version_id,
    installed_by = a.created_by,
    installed_at = COALESCE(i.installed_at, i.created_at)
FROM apps a
WHERE a.id = i.app_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM app_installations
        WHERE installed_version_id IS NULL OR installed_by IS NULL
    ) THEN
        RAISE EXCEPTION 'existing installations must reference an active app version before migration';
    END IF;
END $$;

ALTER TABLE app_installations
    ALTER COLUMN merchant_name SET NOT NULL,
    ALTER COLUMN installed_version_id SET NOT NULL,
    ALTER COLUMN installed_by SET NOT NULL,
    ALTER COLUMN installed_at SET NOT NULL;

ALTER TABLE app_installations
    DROP CONSTRAINT app_installations_status_check,
    ADD CONSTRAINT app_installations_status_check
    CHECK (status IN ('active', 'suspended', 'uninstalled')),
    ADD CONSTRAINT app_installations_merchant_name_check
    CHECK (char_length(merchant_name) BETWEEN 2 AND 120),
    ADD CONSTRAINT app_installations_domain_check
    CHECK (merchant_domain IS NULL OR merchant_domain ~ '^[a-z0-9.-]+$');

CREATE INDEX app_installations_status_updated_idx
    ON app_installations (status, updated_at DESC, id DESC)
    WHERE status <> 'uninstalled';
