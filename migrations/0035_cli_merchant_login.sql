-- CLI login reuses Core merchant SSO, with separate proof-bound consumption.
ALTER TABLE platform_identity.developer_login_requests
 ADD COLUMN login_kind text NOT NULL DEFAULT 'browser' CHECK (login_kind IN ('browser','cli')),
 ADD COLUMN cli_confirmed boolean NOT NULL DEFAULT false;
