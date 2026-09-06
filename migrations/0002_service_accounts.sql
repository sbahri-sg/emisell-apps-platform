-- Additive. Browser identities and sessions retain their existing behavior.
CREATE TABLE platform_identity.service_accounts (
 id text PRIMARY KEY,
 tenant_id text NOT NULL REFERENCES platform_identity.workspaces(id),
 token_hash text UNIQUE NOT NULL,
 scopes text[] NOT NULL,
 expires_at timestamptz NOT NULL,
 revoked_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE platform_identity.service_account_audit (
 id bigserial PRIMARY KEY,
 service_id text NOT NULL,
 tenant_id text NOT NULL,
 action text NOT NULL CHECK(action IN ('issued','rotated','revoked')),
 actor text NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now()
);
