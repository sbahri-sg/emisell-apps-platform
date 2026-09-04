# PostgreSQL persistence

## Migration workflow

Migrations live in `services/app-gateway/internal/database/migrations` and are embedded into the migration binary. Applied versions are recorded in `schema_migrations`. An advisory lock prevents two migration processes from modifying the schema concurrently.

With Docker:

```sh
npm run db:migrate
```

Without Docker:

```sh
cd services/app-gateway
DATABASE_URL='postgres://emisell:emisell-dev@localhost:5432/emisell_app_platform?sslmode=disable' go run ./cmd/migrate
```

The Compose stack runs migrations before starting App Gateway. Migrations are forward-only in automated environments. A matching `.down.sql` file is kept for reviewed manual recovery; it is never executed automatically.

Migration `000015_loopback_development_install` extends only the development-request launch URL constraint: HTTPS remains valid; exact loopback HTTP is accepted only for sandbox-context records. Application configuration additionally requires `APP_ENV=development` and `DEVELOPMENT_LOOPBACK_APP_HTTP=true`; catalog publication still requires HTTPS. The Product Reader runner applies this to its disposable gateway database, not the existing local/production database. A reviewed rollback refuses to restore the HTTPS-only constraint while incompatible records remain; it never deletes them automatically.

## Tables

| Group                | Tables                                                                                               |
| -------------------- | ---------------------------------------------------------------------------------------------------- |
| Tenancy              | `organizations`, `users`, `organization_memberships`                                                 |
| Browser identity     | `user_identities`, `identity_sessions`, `oidc_login_states`, `merchant_identities`, `merchant_session_grants` |
| App Store catalog    | `app_catalog_listings`                                                                                     |
| Developer onboarding | `developer_applications`, `developer_invitations`, `organization_entitlements`                       |
| App lifecycle        | `apps`, `app_versions`                                                                               |
| Configuration        | `app_extensions`, `app_scopes`                                                                       |
| Security             | `app_credentials`, `oauth_authorizations`, `oauth_access_tokens`, `idempotency_keys`, `audit_events` |
| Events               | `webhook_event_definitions`, `webhook_subscriptions`, `webhook_events`, `webhook_deliveries`         |
| Distribution         | `app_installations`                                                                                  |

The current Go repository reads and writes external user identities, developer and merchant browser sessions, one-time Emisell Backend session grants, OIDC login state, merchant identity bindings, curated catalog listings, developer applications/invitations/entitlements, apps, versions, extensions, scopes, credentials, OAuth grants/tokens, webhook event definitions/subscriptions/events/delivery attempts, merchant installations, idempotency records, and audit events.

## Invariants enforced by PostgreSQL

- active app slugs are unique inside an organization;
- semantic versions are unique inside an app;
- a partial unique index permits only one active version per app;
- a composite foreign key ensures `active_version_id` belongs to the same app;
- version snapshots and configuration payloads must be JSON objects;
- extension types and scope names are constrained at both the service and database layers;
- membership, lifecycle, and access fields use check constraints;
- tenant and app relationships use foreign keys;
- credentials store encrypted bytes and fingerprints, never plaintext secrets;
- credential environments and expiration metadata are constrained independently from secret material;
- webhook event/endpoints are unique among non-disabled subscriptions, allowing safe recreation after deletion;
- the persisted webhook event catalog controls which events are available, planned, platform-produced, backend-produced, and scope-gated;
- webhook signing secrets use the same versioned AES-256-GCM envelope as credentials;
- webhook payloads are retained only in the internal event table and are never returned through delivery-history APIs;
- delivery claims use row locks with `SKIP LOCKED`, a short lease, and unique subscription/event/attempt tuples;
- OAuth authorization codes and access tokens are persisted only as SHA-256 digests;
- authorization codes expire quickly and have a single nullable consumption timestamp;
- token creation, code consumption, and version-pinned installation creation commit atomically;
- active merchant/environment installations are unique per app;
- installations pin an immutable app version and use revision-based state changes;
- uninstall is a soft transition that preserves history while allowing later reinstallation;
- uninstall atomically revokes installation tokens and queues `app/uninstalled` for active subscriptions pinned in the installed version;
- audit events are append-only by application policy;
- idempotency records expire after 24 hours and are replaced only under a transaction-scoped advisory lock;
- only one developer invitation may be pending for an application, and only a SHA-256 token digest is persisted;
- developer approval creates the organization, sandbox-only entitlement, and invitation in one serializable transaction;
- invitation acceptance creates/validates the user, owner membership, active application state, entitlement update, and audit event atomically;
- production access defaults to false and is independent from invitation acceptance.
- external identities are unique by provider and subject, then linked to an internal user;
- browser session and CSRF secrets are persisted only as SHA-256 digests;
- browser sessions enforce both absolute and sliding idle expiry and support explicit revocation;
- the active session organization is nullable and is accepted only when an active membership exists;
- OIDC login state is single-use, time-bounded, and stores only a state/nonce digest plus an encrypted PKCE verifier.
- merchant identities use the opaque stable `Merchant.id` from Emisell (including CUID values); the database does not require external Emisell IDs to be UUIDs;
- a composite merchant/user/environment key permits explicit multi-store bindings while keeping every browser session attached to one verified store;
- Emisell session grant codes and source JWT `jti` values are unique, expiring, single-use, and stored without raw codes;
- catalog publication is a separate operator decision and never inferred only from app status;
- merchant identity rows are constrained to sandbox or production, and production creation is accepted only through the signed Emisell Backend session bridge;
- merchant Connected Apps queries match the session-derived merchant ID and environment before exposing or mutating an installation.

## Development identity

After migrations, the development server idempotently creates the organization, user, and membership configured by:

```text
DEVELOPMENT_ORGANIZATION_ID
DEVELOPMENT_USER_ID
DEVELOPMENT_ROLE
DEVELOPMENT_EMAIL
DEVELOPMENT_DISPLAY_NAME
DEVELOPMENT_PLATFORM_OPERATOR
```

Production authentication remains fail-closed: API bearer identity comes from a trusted signed issuer, while browser identity comes from configured OIDC and server-side membership lookup. Request override headers are development-only.
