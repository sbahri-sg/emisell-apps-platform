-- Private, headless product readers have a separate signed installation source.
-- No conversion of shipping drafts, review records or existing grants.
CREATE TABLE platform_app.private_product_versions (
 id text PRIMARY KEY,
 app_id text NOT NULL REFERENCES platform_app.drafts(id),
 organization_id text NOT NULL REFERENCES platform_developer.organizations(id),
 owner_account_id text NOT NULL REFERENCES platform_identity.portal_accounts(id),
 version text NOT NULL,
 document jsonb NOT NULL,
 signature bytea NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(app_id,version)
);
CREATE FUNCTION platform_app.protect_private_product_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'private product version is immutable';
END;
$$;
CREATE TRIGGER private_product_version_immutable BEFORE UPDATE OR DELETE ON platform_app.private_product_versions
FOR EACH ROW EXECUTE FUNCTION platform_app.protect_private_product_version();
