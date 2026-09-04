-- Manually provisioned platform administrators. No public registration.
CREATE TABLE admin_password_accounts (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    platform_organization_id uuid NOT NULL REFERENCES organizations(id),
    email text NOT NULL UNIQUE CHECK (email = lower(email)),
    display_name text NOT NULL,
    password_hash text NOT NULL,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Intentionally separate from developer and merchant sessions.
CREATE TABLE admin_identity_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES admin_password_accounts(user_id),
    token_hash text NOT NULL UNIQUE,
    csrf_token_hash text NOT NULL,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    idle_expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX admin_sessions_user_id ON admin_identity_sessions(user_id);
CREATE INDEX admin_sessions_expiry ON admin_identity_sessions(expires_at);

-- Only digests of the IP/email bucket are stored. Shared, bounded-window limits.
CREATE TABLE admin_login_attempts (
    bucket_hash text PRIMARY KEY,
    attempts integer NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE INDEX admin_login_attempts_expiry ON admin_login_attempts(expires_at);
