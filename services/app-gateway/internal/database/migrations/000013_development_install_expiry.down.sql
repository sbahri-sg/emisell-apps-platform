UPDATE development_install_requests
SET status = 'cancelled'
WHERE status = 'expired';

ALTER TABLE development_install_requests
    DROP CONSTRAINT development_install_requests_status_check,
    ADD CONSTRAINT development_install_requests_status_check
    CHECK (status IN ('pending', 'authorized', 'cancelled'));
