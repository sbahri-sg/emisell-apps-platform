-- Separate credentials: existing tenant-bound service accounts are never elevated.
CREATE TABLE platform_identity.core_platform_keys (
    id text PRIMARY KEY CHECK (id LIKE 'platformkey\_%' ESCAPE '\'),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    token_hash text NOT NULL UNIQUE,
    actor_id text NOT NULL REFERENCES platform_identity.portal_accounts(id),
    request_key text NOT NULL,
    request_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    UNIQUE (actor_id, request_key)
);
CREATE TABLE platform_identity.core_platform_key_audit (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key_id text NOT NULL REFERENCES platform_identity.core_platform_keys(id),
    action text NOT NULL CHECK (action IN ('issued', 'revoked')),
    actor_id text NOT NULL REFERENCES platform_identity.portal_accounts(id),
    occurred_at timestamptz NOT NULL DEFAULT now()
);
