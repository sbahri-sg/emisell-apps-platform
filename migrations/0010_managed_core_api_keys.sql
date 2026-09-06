-- Additive metadata for administrator-issued Core keys. Legacy CLI keys untouched.
CREATE TABLE platform_identity.managed_service_keys (
 service_id text PRIMARY KEY REFERENCES platform_identity.service_accounts(id),
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 80),
 actor_id text NOT NULL REFERENCES platform_identity.portal_accounts(id),
 request_key text NOT NULL,
 request_hash text NOT NULL,
 UNIQUE(actor_id,request_key)
);
CREATE INDEX managed_service_keys_actor ON platform_identity.managed_service_keys(actor_id);
-- Secrets remain SHA-256 hashes only in service_accounts. No secret in metadata/audit.
