-- Stable application identity; independent of signed release evidence and store grants.
CREATE TABLE platform_oauth.application_credentials (
 client_id text PRIMARY KEY,
 organization_id text NOT NULL,
 app_id text NOT NULL UNIQUE,
 secret_hash text NOT NULL CHECK(secret_hash ~ '^[0-9a-f]{64}$'),
 secret_ciphertext bytea NOT NULL,
 version integer NOT NULL CHECK(version > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 secret_created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id, app_id)
);
CREATE TABLE platform_oauth.application_credential_audit (
 id text PRIMARY KEY, client_id text NOT NULL REFERENCES platform_oauth.application_credentials(client_id),
 actor_id text NOT NULL, action text NOT NULL, version integer NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE platform_oauth.application_credential_requests (
 organization_id text NOT NULL, actor_id text NOT NULL, request_key text NOT NULL,
 app_id text NOT NULL, expected_version integer NOT NULL, resulting_version integer NOT NULL,
 PRIMARY KEY(organization_id,actor_id,request_key)
);
