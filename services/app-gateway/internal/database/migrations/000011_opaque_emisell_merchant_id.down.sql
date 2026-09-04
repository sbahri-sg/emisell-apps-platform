DO $$
BEGIN
    IF EXISTS (
        SELECT merchant_id FROM merchant_identities
        WHERE merchant_id !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'
        UNION ALL
        SELECT merchant_id FROM app_installations
        WHERE merchant_id !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'
        UNION ALL
        SELECT merchant_id FROM oauth_authorizations
        WHERE merchant_id !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'
        UNION ALL
        SELECT merchant_id FROM merchant_session_grants
        WHERE merchant_id !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'
    ) THEN
        RAISE EXCEPTION 'cannot roll back migration 000011 while non-UUID Emisell merchant identifiers exist';
    END IF;
END $$;

ALTER TABLE identity_sessions
    DROP CONSTRAINT identity_sessions_merchant_binding_fk,
    DROP CONSTRAINT identity_sessions_merchant_id_format_check;

ALTER TABLE merchant_identities
    DROP CONSTRAINT merchant_identities_merchant_id_format_check;
ALTER TABLE app_installations
    DROP CONSTRAINT app_installations_merchant_id_format_check;
ALTER TABLE oauth_authorizations
    DROP CONSTRAINT oauth_authorizations_merchant_id_format_check;
ALTER TABLE merchant_session_grants
    DROP CONSTRAINT merchant_session_grants_merchant_id_format_check;

ALTER TABLE merchant_identities
    ALTER COLUMN merchant_id TYPE uuid USING merchant_id::uuid;
ALTER TABLE identity_sessions
    ALTER COLUMN merchant_id TYPE uuid USING merchant_id::uuid;
ALTER TABLE app_installations
    ALTER COLUMN merchant_id TYPE uuid USING merchant_id::uuid;
ALTER TABLE oauth_authorizations
    ALTER COLUMN merchant_id TYPE uuid USING merchant_id::uuid;
ALTER TABLE merchant_session_grants
    ALTER COLUMN merchant_id TYPE uuid USING merchant_id::uuid;

ALTER TABLE identity_sessions
    ADD CONSTRAINT identity_sessions_merchant_binding_fk
        FOREIGN KEY (merchant_id, user_id, merchant_environment)
        REFERENCES merchant_identities (merchant_id, user_id, environment)
        ON DELETE CASCADE;
