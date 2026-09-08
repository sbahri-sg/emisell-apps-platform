-- On-demand first-party Core registration. Existing merchant references stay
-- untouched; no tenant grants/memberships are backfilled or auto-approved.
CREATE TABLE platform_identity.merchant_reference_audit (
 merchant_id text PRIMARY KEY REFERENCES platform_identity.workspaces(id),
 service_id text NOT NULL REFERENCES platform_identity.core_platform_keys(id),
 core_actor_id text NOT NULL CHECK (char_length(core_actor_id) BETWEEN 1 AND 128),
 created_at timestamptz NOT NULL DEFAULT now()
);
