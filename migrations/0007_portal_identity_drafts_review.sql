-- Additive control-plane domains. No merchant identity or installation changes.
CREATE TABLE platform_identity.portal_accounts (
 id text PRIMARY KEY, email text NOT NULL, password_hash text NOT NULL,
 surface text NOT NULL CHECK(surface IN ('admin','developer')),
 role text NOT NULL CHECK(role IN ('administrator','reviewer','operator','developer')),
 enabled boolean NOT NULL DEFAULT true,
 CHECK ((surface='developer' AND role='developer') OR (surface='admin' AND role <> 'developer')),
 UNIQUE(surface,email)
);
CREATE TABLE platform_identity.portal_sessions (
 token_hash text PRIMARY KEY, account_id text NOT NULL REFERENCES platform_identity.portal_accounts(id),
 surface text NOT NULL, expires_at timestamptz NOT NULL
);
CREATE TABLE platform_identity.portal_audit (
 id text PRIMARY KEY, actor_id text NOT NULL, action text NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE SCHEMA platform_developer;
CREATE TABLE platform_developer.organizations (id text PRIMARY KEY, name text NOT NULL);
CREATE TABLE platform_developer.memberships (
 account_id text PRIMARY KEY, organization_id text NOT NULL REFERENCES platform_developer.organizations(id),
 role text NOT NULL CHECK(role='owner')
);
CREATE TABLE platform_app.drafts (
 id text PRIMARY KEY, organization_id text NOT NULL, revision integer NOT NULL CHECK(revision > 0),
 document jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX app_drafts_owner ON platform_app.drafts(organization_id,updated_at DESC,id);
CREATE TABLE platform_app.draft_requests (
 organization_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL, response jsonb NOT NULL,
 PRIMARY KEY(organization_id,request_key)
);
CREATE TABLE platform_app.draft_audit (
 id text PRIMARY KEY, app_id text NOT NULL, organization_id text NOT NULL, actor_id text NOT NULL,
 revision integer NOT NULL, action text NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE SCHEMA platform_review;
CREATE TABLE platform_review.submissions (
 id text PRIMARY KEY, app_id text NOT NULL, organization_id text NOT NULL, submitter_id text NOT NULL,
 draft_revision integer NOT NULL, version text NOT NULL, snapshot jsonb NOT NULL,
 status text NOT NULL CHECK(status IN ('submitted','changes_requested','approved','rejected')),
 created_at timestamptz NOT NULL DEFAULT now(), decided_at timestamptz, reviewer_id text, feedback text NOT NULL DEFAULT '',
 UNIQUE(app_id,draft_revision)
);
CREATE UNIQUE INDEX review_one_pending ON platform_review.submissions(app_id) WHERE status='submitted';
CREATE INDEX review_queue ON platform_review.submissions(created_at DESC,id);
CREATE INDEX review_owner ON platform_review.submissions(organization_id,created_at DESC,id);
CREATE TABLE platform_review.requests (
 actor_id text NOT NULL, request_key text NOT NULL, request_hash text NOT NULL, response jsonb NOT NULL,
 PRIMARY KEY(actor_id,request_key)
);
CREATE TABLE platform_review.audit (
 id text PRIMARY KEY, submission_id text NOT NULL, actor_id text NOT NULL, action text NOT NULL,
 feedback text NOT NULL DEFAULT '', occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION platform_review.protect_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.app_id IS DISTINCT FROM OLD.app_id
 OR NEW.organization_id IS DISTINCT FROM OLD.organization_id OR NEW.submitter_id IS DISTINCT FROM OLD.submitter_id
 OR NEW.draft_revision IS DISTINCT FROM OLD.draft_revision OR NEW.version IS DISTINCT FROM OLD.version
 OR NEW.snapshot IS DISTINCT FROM OLD.snapshot OR NEW.created_at IS DISTINCT FROM OLD.created_at
 THEN RAISE EXCEPTION 'submission snapshot is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER review_snapshot_immutable BEFORE UPDATE ON platform_review.submissions
 FOR EACH ROW EXECUTE FUNCTION platform_review.protect_snapshot();
