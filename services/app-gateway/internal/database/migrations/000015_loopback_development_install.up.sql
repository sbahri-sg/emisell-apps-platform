-- Local pilot only: production-context invitations continue to require HTTPS.
-- Application configuration additionally gates loopback HTTP to APP_ENV=development.
ALTER TABLE development_install_requests
    DROP CONSTRAINT development_install_requests_launch_url_check;
ALTER TABLE development_install_requests
    ADD CONSTRAINT development_install_requests_launch_url_check CHECK (
        launch_url ~ '^https://' OR (
            environment = 'sandbox' AND
            launch_url ~ '^http://(localhost|127\.0\.0\.1|\[::1\])(:[0-9]{1,5})?(/[^[:space:]]*)?$'
        )
    );
