DROP INDEX IF EXISTS app_installations_status_updated_idx;

ALTER TABLE app_installations
    DROP CONSTRAINT IF EXISTS app_installations_domain_check,
    DROP CONSTRAINT IF EXISTS app_installations_merchant_name_check,
    DROP CONSTRAINT app_installations_status_check,
    ADD CONSTRAINT app_installations_status_check CHECK (status IN ('pending', 'active', 'suspended', 'uninstalled')),
    DROP COLUMN revision,
    DROP COLUMN installed_by,
    DROP COLUMN installed_version_id,
    DROP COLUMN merchant_domain,
    DROP COLUMN merchant_name;
