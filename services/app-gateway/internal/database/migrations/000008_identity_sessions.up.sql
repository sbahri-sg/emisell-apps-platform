CREATE TABLE user_identities (
    id uuid PRIMARY KEY,
    provider text NOT NULL CHECK (char_length(provider) BETWEEN 2 AND 200),
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 500),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (provider, subject)
);

CREATE INDEX user_identities_user_idx ON user_identities (user_id);

CREATE TABLE identity_sessions (
    id uuid PRIMARY KEY,
    token_hash char(64) NOT NULL UNIQUE,
    csrf_token_hash char(64) NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    active_organization_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
    platform_operator boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    idle_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at > created_at),
    CHECK (idle_expires_at > created_at)
);

CREATE INDEX identity_sessions_user_idx ON identity_sessions (user_id, created_at DESC);
CREATE INDEX identity_sessions_active_lookup_idx
    ON identity_sessions (token_hash, expires_at, idle_expires_at)
    WHERE revoked_at IS NULL;
CREATE INDEX identity_sessions_expiry_idx ON identity_sessions (expires_at);

CREATE TABLE oidc_login_states (
    id uuid PRIMARY KEY,
    state_hash char(64) NOT NULL UNIQUE,
    nonce_hash char(64) NOT NULL,
    code_verifier_ciphertext bytea NOT NULL,
    return_to text NOT NULL CHECK (char_length(return_to) BETWEEN 1 AND 500),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at)
);

CREATE INDEX oidc_login_states_expiry_idx ON oidc_login_states (expires_at);

COMMENT ON TABLE identity_sessions IS 'Server-side browser sessions. Only SHA-256 digests of the session and CSRF tokens are persisted.';
COMMENT ON TABLE oidc_login_states IS 'Single-use OIDC authorization state with encrypted PKCE verifier and hashed nonce.';
