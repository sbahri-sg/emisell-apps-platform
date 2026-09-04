-- Reviewed recovery only. Refuse accidental destruction of provisioned credentials/audit.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM extension_connections) THEN
        RAISE EXCEPTION 'extension connections exist; export/review retention before manual rollback';
    END IF;
END $$;
DROP TABLE extension_credential_access_events;
DROP TABLE extension_connections;
ALTER TABLE app_extensions DROP CONSTRAINT extension_app_identity;
ALTER TABLE app_installations DROP CONSTRAINT installation_app_identity;
