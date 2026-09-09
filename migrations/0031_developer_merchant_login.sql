CREATE TABLE platform_identity.developer_core_links (
 account_id text PRIMARY KEY REFERENCES platform_identity.portal_accounts(id),
 core_subject text NOT NULL UNIQUE,
 core_email text NOT NULL,
 display_name text NOT NULL,
 stores jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE platform_identity.developer_login_requests (
 id text PRIMARY KEY,
 verifier_hash text NOT NULL,
 portal_origin text NOT NULL,
 link_account_id text NOT NULL DEFAULT '',
 core_subject text,
 core_email text,
 display_name text,
 stores jsonb,
 code_hash text UNIQUE,
 expires_at timestamptz NOT NULL
);
CREATE INDEX developer_login_requests_expiry ON platform_identity.developer_login_requests(expires_at);
