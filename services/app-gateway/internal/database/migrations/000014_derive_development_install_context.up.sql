ALTER TABLE development_install_requests
    DROP CONSTRAINT IF EXISTS development_install_requests_environment_check;

ALTER TABLE development_install_requests
    ADD CONSTRAINT development_install_requests_environment_check
    CHECK (environment IN ('sandbox', 'production'));

DROP INDEX development_install_requests_pending_unique;

CREATE UNIQUE INDEX development_install_requests_pending_unique
    ON development_install_requests (app_id, merchant_id)
    WHERE status = 'pending';

COMMENT ON TABLE development_install_requests IS
    'Merchant-ID development install invitations. Merchant context is derived internally; merchant consent and normal OAuth PKCE remain mandatory.';
