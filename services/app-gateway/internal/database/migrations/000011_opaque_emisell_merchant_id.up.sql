ALTER TABLE identity_sessions
    DROP CONSTRAINT identity_sessions_merchant_binding_fk;

ALTER TABLE merchant_identities
    ALTER COLUMN merchant_id TYPE text USING merchant_id::text;

ALTER TABLE identity_sessions
    ALTER COLUMN merchant_id TYPE text USING merchant_id::text;

ALTER TABLE app_installations
    ALTER COLUMN merchant_id TYPE text USING merchant_id::text;

ALTER TABLE oauth_authorizations
    ALTER COLUMN merchant_id TYPE text USING merchant_id::text;

ALTER TABLE merchant_session_grants
    ALTER COLUMN merchant_id TYPE text USING merchant_id::text;

ALTER TABLE merchant_identities
    ADD CONSTRAINT merchant_identities_merchant_id_format_check
    CHECK (merchant_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$');

ALTER TABLE identity_sessions
    ADD CONSTRAINT identity_sessions_merchant_id_format_check
    CHECK (merchant_id IS NULL OR merchant_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    ADD CONSTRAINT identity_sessions_merchant_binding_fk
        FOREIGN KEY (merchant_id, user_id, merchant_environment)
        REFERENCES merchant_identities (merchant_id, user_id, environment)
        ON DELETE CASCADE;

ALTER TABLE app_installations
    ADD CONSTRAINT app_installations_merchant_id_format_check
    CHECK (merchant_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$');

ALTER TABLE oauth_authorizations
    ADD CONSTRAINT oauth_authorizations_merchant_id_format_check
    CHECK (merchant_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$');

ALTER TABLE merchant_session_grants
    ADD CONSTRAINT merchant_session_grants_merchant_id_format_check
    CHECK (merchant_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$');

COMMENT ON COLUMN merchant_identities.merchant_id IS 'Opaque stable Merchant.id from Emisell. This external identifier is not required to be a UUID.';
COMMENT ON COLUMN app_installations.merchant_id IS 'Opaque stable Merchant.id from Emisell. Authentication and authorization are enforced separately.';
