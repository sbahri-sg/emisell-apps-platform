-- Contact information is app metadata, not an account or ownership change.
CREATE TABLE platform_app.contacts (
 app_id text PRIMARY KEY REFERENCES platform_app.drafts(id),
 email text NOT NULL CHECK(length(email) BETWEEN 3 AND 254),
 revision integer NOT NULL CHECK(revision > 0),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE platform_app.contact_audit (
 id text PRIMARY KEY,
 app_id text NOT NULL REFERENCES platform_app.drafts(id),
 actor_id text NOT NULL,
 revision integer NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now()
);
