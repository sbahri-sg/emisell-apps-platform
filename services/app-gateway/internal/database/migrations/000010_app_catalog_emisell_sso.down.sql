DROP TABLE IF EXISTS merchant_session_grants;
DROP TABLE IF EXISTS app_catalog_listings;

ALTER TABLE identity_sessions
    DROP CONSTRAINT identity_sessions_merchant_binding_fk,
    DROP CONSTRAINT identity_sessions_merchant_binding_check,
    DROP COLUMN merchant_environment,
    DROP COLUMN merchant_id;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM merchant_identities WHERE environment = 'production') THEN
        RAISE EXCEPTION 'cannot roll back migration 000010 while production merchant identities exist';
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM merchant_identities GROUP BY merchant_id HAVING count(*) > 1
    ) OR EXISTS (
        SELECT 1 FROM merchant_identities GROUP BY user_id HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot roll back migration 000010 while multi-store or multi-user merchant bindings exist';
    END IF;
END $$;

ALTER TABLE merchant_identities
    DROP CONSTRAINT merchant_identities_pkey;

ALTER TABLE merchant_identities
    ADD CONSTRAINT merchant_identities_pkey PRIMARY KEY (merchant_id),
    ADD CONSTRAINT merchant_identities_user_id_key UNIQUE (user_id);

ALTER TABLE merchant_identities
    DROP CONSTRAINT merchant_identities_environment_check;

ALTER TABLE merchant_identities
    ADD CONSTRAINT merchant_identities_environment_check
    CHECK (environment = 'sandbox');
