-- Credential storage is deliberately outside extension configuration/version snapshots.
ALTER TABLE app_installations ADD CONSTRAINT installation_app_identity UNIQUE (id, app_id);
ALTER TABLE app_extensions ADD CONSTRAINT extension_app_identity UNIQUE (id, app_id);

CREATE TABLE extension_connections (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    installation_id uuid NOT NULL,
    extension_id uuid NOT NULL,
    runtime_name text NOT NULL CHECK (char_length(runtime_name) BETWEEN 3 AND 80),
    scopes jsonb NOT NULL CHECK (jsonb_typeof(scopes) = 'array' AND jsonb_array_length(scopes) BETWEEN 1 AND 32),
    status text NOT NULL CHECK (status IN ('active', 'revoked')),
    revision bigint NOT NULL CHECK (revision > 0),
    runtime_expires_at timestamptz NOT NULL,
    token_hash text UNIQUE CHECK (token_hash IS NULL OR token_hash ~ '^[a-f0-9]{64}$'),
    secret_ciphertext bytea,
    encryption_key_version integer NOT NULL CHECK (encryption_key_version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (installation_id, extension_id),
    FOREIGN KEY (installation_id, app_id) REFERENCES app_installations(id, app_id) ON DELETE CASCADE,
    FOREIGN KEY (extension_id, app_id) REFERENCES app_extensions(id, app_id) ON DELETE CASCADE,
    CHECK ((status = 'active' AND token_hash IS NOT NULL AND secret_ciphertext IS NOT NULL AND octet_length(secret_ciphertext) > 28)
        OR (status = 'revoked' AND token_hash IS NULL AND secret_ciphertext IS NULL))
);

-- Runtime access is not attributed to a human operator in audit_events.
CREATE TABLE extension_credential_access_events (
    id uuid PRIMARY KEY,
    connection_id uuid NOT NULL REFERENCES extension_connections(id) ON DELETE CASCADE,
    connection_revision bigint NOT NULL,
    scope text NOT NULL,
    request_id text NOT NULL CHECK (char_length(request_id) BETWEEN 16 AND 128),
    created_at timestamptz NOT NULL
);
CREATE INDEX extension_credential_access_connection_idx ON extension_credential_access_events(connection_id, created_at DESC);
