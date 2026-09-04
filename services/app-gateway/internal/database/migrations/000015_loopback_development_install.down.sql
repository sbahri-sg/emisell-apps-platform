-- Fails safely if local HTTP invitations remain; never delete user records on rollback.
ALTER TABLE development_install_requests
    DROP CONSTRAINT development_install_requests_launch_url_check;
ALTER TABLE development_install_requests
    ADD CONSTRAINT development_install_requests_launch_url_check CHECK (launch_url ~ '^https://');
