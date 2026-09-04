CREATE TABLE merchant_identities (
    merchant_id uuid PRIMARY KEY,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    merchant_name text NOT NULL CHECK (char_length(merchant_name) BETWEEN 2 AND 120),
    merchant_domain text,
    environment text NOT NULL CHECK (environment = 'sandbox'),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX merchant_identities_user_lookup_idx
    ON merchant_identities (user_id, environment);

COMMENT ON TABLE merchant_identities IS 'Sandbox merchant identity binding used by the isolated merchant browser session. Production identity requires the selected merchant IdP.';
