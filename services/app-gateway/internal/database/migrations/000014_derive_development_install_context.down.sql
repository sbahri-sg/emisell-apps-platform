DROP INDEX development_install_requests_pending_unique;

CREATE UNIQUE INDEX development_install_requests_pending_unique
    ON development_install_requests (app_id, merchant_id, environment)
    WHERE status = 'pending';

ALTER TABLE development_install_requests
    DROP CONSTRAINT IF EXISTS development_install_requests_environment_check;

ALTER TABLE development_install_requests
    ADD CONSTRAINT development_install_requests_environment_check
    CHECK (environment = 'sandbox');

COMMENT ON TABLE development_install_requests IS
    'Developer-created sandbox install invitations. Merchant consent and normal OAuth PKCE remain mandatory.';
