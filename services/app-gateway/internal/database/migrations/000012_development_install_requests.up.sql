CREATE TABLE development_install_requests (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    merchant_id text NOT NULL,
    merchant_name text NOT NULL CHECK (char_length(merchant_name) BETWEEN 2 AND 120),
    merchant_domain text,
    environment text NOT NULL CHECK (environment = 'sandbox'),
    version_id uuid NOT NULL REFERENCES app_versions(id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('pending', 'authorized', 'cancelled')),
    launch_url text NOT NULL CHECK (launch_url ~ '^https://'),
    requested_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    authorized_at timestamptz,
    authorization_id uuid REFERENCES oauth_authorizations(id) ON DELETE SET NULL,
    CHECK (merchant_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    CHECK (merchant_domain IS NULL OR merchant_domain ~ '^[a-z0-9.-]+$'),
    CHECK (expires_at > created_at),
    CHECK (
        (status = 'authorized' AND authorized_at IS NOT NULL AND authorization_id IS NOT NULL) OR
        (status <> 'authorized' AND authorized_at IS NULL AND authorization_id IS NULL)
    )
);

CREATE UNIQUE INDEX development_install_requests_pending_unique
    ON development_install_requests (app_id, merchant_id, environment)
    WHERE status = 'pending';

CREATE INDEX development_install_requests_app_created_idx
    ON development_install_requests (app_id, created_at DESC, id DESC);

CREATE INDEX development_install_requests_expiry_idx
    ON development_install_requests (expires_at)
    WHERE status = 'pending';

COMMENT ON TABLE development_install_requests IS 'Developer-created sandbox install invitations. Merchant consent and normal OAuth PKCE remain mandatory.';
COMMENT ON COLUMN development_install_requests.id IS 'Opaque binding handle, not a merchant credential; authorization also requires a matching merchant session and OAuth client.';
