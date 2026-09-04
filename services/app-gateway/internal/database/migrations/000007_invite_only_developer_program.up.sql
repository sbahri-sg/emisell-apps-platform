CREATE TABLE developer_applications (
    id uuid PRIMARY KEY,
    platform_organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    company_name text NOT NULL CHECK (char_length(company_name) BETWEEN 2 AND 120),
    company_domain text NOT NULL CHECK (char_length(company_domain) BETWEEN 3 AND 253),
    contact_name text NOT NULL CHECK (char_length(contact_name) BETWEEN 2 AND 120),
    contact_email text NOT NULL CHECK (char_length(contact_email) BETWEEN 3 AND 254),
    requested_app_name text NOT NULL CHECK (char_length(requested_app_name) BETWEEN 3 AND 80),
    app_type text NOT NULL CHECK (app_type IN ('payment', 'shipping', 'erp', 'marketing', 'custom')),
    use_case text NOT NULL CHECK (char_length(use_case) BETWEEN 20 AND 2000),
    requested_scopes jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(requested_scopes) = 'array'),
    status text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'under_review', 'approved', 'invited', 'active', 'rejected')),
    review_notes text CHECK (review_notes IS NULL OR char_length(review_notes) <= 2000),
    organization_id uuid REFERENCES organizations(id) ON DELETE RESTRICT,
    submitted_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reviewed_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0)
);

CREATE INDEX developer_applications_platform_status_idx
    ON developer_applications (platform_organization_id, status, created_at DESC, id DESC);
CREATE INDEX developer_applications_platform_created_idx
    ON developer_applications (platform_organization_id, created_at DESC, id DESC);
CREATE INDEX developer_applications_contact_email_idx
    ON developer_applications (lower(contact_email));

CREATE TABLE organization_entitlements (
    organization_id uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    sandbox_access boolean NOT NULL DEFAULT true,
    production_access boolean NOT NULL DEFAULT false,
    max_apps integer NOT NULL DEFAULT 3 CHECK (max_apps BETWEEN 1 AND 100),
    max_webhooks integer NOT NULL DEFAULT 20 CHECK (max_webhooks BETWEEN 1 AND 1000),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE developer_invitations (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES developer_applications(id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email text NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
    role text NOT NULL DEFAULT 'owner' CHECK (role = 'owner'),
    token_hash char(64) NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    accepted_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    revoked_at timestamptz,
    CONSTRAINT developer_invitations_expiry_after_creation CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX developer_invitations_token_hash_unique
    ON developer_invitations (token_hash);
CREATE UNIQUE INDEX developer_invitations_one_pending_per_application
    ON developer_invitations (application_id)
    WHERE status = 'pending';
CREATE INDEX developer_invitations_application_created_idx
    ON developer_invitations (application_id, created_at DESC);
CREATE INDEX developer_invitations_pending_expiry_idx
    ON developer_invitations (expires_at)
    WHERE status = 'pending';

COMMENT ON TABLE developer_applications IS 'Invite-only developer onboarding requests recorded and reviewed by Emisell platform operators.';
COMMENT ON COLUMN developer_invitations.token_hash IS 'SHA-256 digest of a one-time invitation token. Raw tokens are never persisted.';
COMMENT ON COLUMN organization_entitlements.production_access IS 'Production access is intentionally separate from developer onboarding and defaults to false.';
