# Managed extension credentials

Status: implemented credential/authorization foundation, **disabled by default**. Verified with synthetic data in disposable PostgreSQL 17. This does not prove production readiness or execute Payment Gateway / Shipping Gateway operations. Existing external developer apps and OAuth flows are unchanged.

## Ownership

| Component | Owns | Does not receive |
| --- | --- | --- |
| Emisell Backend | Merchant identity, business data, merchant authorization for its own resources | Extension/provider secrets |
| App Platform | Installation and version binding, granted scopes, encrypted managed credentials, runtime-token hashes, access checks/audits | An unauthenticated merchant ID as proof of authorization |
| Emisell-managed runtime | Its own operator-provisioned runtime token; just-in-time credential for its bound installation–extension | Arbitrary selection of another merchant, installation or extension |
| External developer app | Its own OAuth client secret and installation access tokens; its own provider-side secrets | Managed extension credentials or platform operator privileges |

`merchantId` remains an opaque identifier, not a secret or access token. Authenticated Backend → Platform requests may supply it; platform resolves and validates installation context. Platform → Backend resource calls still require a signed service assertion and Backend tenant filtering. Do not remove those checks.

This is an Emisell-managed extension design, not a claim that Shopify stores every third-party provider credential centrally. A third-party developer's service remains outside Emisell's trust boundary.

## Connection model and isolation

One connection binds `appId + installationId + extensionId`. A PostgreSQL unique constraint prevents duplicate installation–extension pairs; composite foreign keys prohibit cross-app bindings.

- No connection: metadata reports `mode: external`, `connection: null`. Existing OAuth provider integration continues unchanged.
- Operator provisions a connection: `mode: emisell_managed`, `status: active`.
- Operator revokes it: metadata remains `emisell_managed / revoked`; token hash and encrypted credential are cleared. It does **not** silently fall back to external mode.

This mode labels credential ownership only. It does not switch execution automatically, designate a third-party server as trusted, or disable the app's other OAuth features. Provision only after Emisell has approved and controls the target runtime. `runtimeName` is an operator label, not an independent machine identity.

Managed secrets never enter `AppExtension.configuration`, version snapshots, developer credential lists, or management GET responses. Treat ordinary configuration as non-secret metadata; this foundation does not scan or rewrite existing arbitrary configuration, snapshots or backups. Audit legacy data separately if secrets were previously entered there.

Secrets use the existing AES-256-GCM secret box with a random nonce, encryption-key version, and encrypted purpose/connection/app/installation/extension/runtime binding. Moving ciphertext between connections fails validation. The key lives outside the database. Raw runtime tokens are random `er_` bearer tokens (32 random bytes), returned once after commit; only SHA-256 hashes are stored.

## Provisioning: operator only

Apply migration `000016_extension_connections.up.sql` to a reviewed target database and enable `EXTENSION_CONNECTIONS_ENABLED=true` in the gateway, then restart it. The supplied Docker Compose and both environment examples default to `false`. No existing gateway/database is automatically changed by the isolated test command.

Before enabling outside local development: use PostgreSQL; a private secret-manager-provided `SECRET_ENCRYPTION_KEY_BASE64` (32 random bytes, **not** the public development key); authenticated operator identities; HTTPS ingress and a restricted internal network; disable request/response-body capture and redact Authorization in proxies/APM. Default development credentials and plaintext public ingress are unsuitable.

Management base path:

```text
/v1/internal/organizations/{organizationId}/apps/{appId}/installations/{installationId}/extensions/{extensionId}/connection
```

All four operations require a verified `platformOperator` identity, not merely organization `owner` or `admin`. Bearer control-plane authentication or an authenticated operator session is accepted; browser-session mutations also require CSRF protection. The organization in the path must actually own the app and both child records.

| Method | Path suffix | Purpose |
| --- | --- | --- |
| GET | base | Metadata only; obtain current revision |
| PUT | base | Create/replace provider secret, allowed scopes and runtime label; also rotates runtime token |
| POST | `/rotate` | Rotate runtime token and expiry without replacing provider secret |
| DELETE | base | Revoke token and clear stored provider secret |

Illustrative PUT body (replace IDs in the path, secret, and expiry before individual execution):

```json
{
  "revision": 0,
  "runtimeName": "managed-example-runtime",
  "scopes": ["read_merchant"],
  "secret": { "apiKey": "<provider-api-key>" },
  "runtimeExpiresAt": "2026-09-04T00:00:00Z"
}
```

`revision: 0` creates; replacements use the revision returned by GET. Runtime expiry must be later than the current time and within 30 days; the static example is not a reusable expiry. Scope names come from the existing catalog, not an extension-specific hardcoded list. Each must be available and included in both the installed immutable version and merchant grants. `read_products` remains permitted only by the existing explicitly configured development pilot; this feature does not make it generally available. Payment/shipping execution scopes are not activated here.

Secret input is an opaque map of 1–32 nonempty string values, at most 8192 JSON-encoded bytes; the HTTP body is limited to 16 KiB. Unknown fields and query parameters are rejected. Examples use `apiKey` only to illustrate the map, not to prescribe a provider schema. The platform does not validate whether an upstream provider accepts that credential.

Provision/rotate returns `data.runtimeToken` once and metadata; never the submitted provider secret. Put the token directly in the **bound internal runtime's** secret manager, never frontend state, logs, Git, browser storage, shared Postman values or a third-party developer app. There is no token-readback endpoint. After an uncertain write response, GET current revision and rotate; do not blindly retry a mutation.

Rotation body: `{"revision": 1, "runtimeExpiresAt": "2026-09-04T00:00:00Z"}`. Revocation body: `{"revision": 2}`. Use actual current revisions, not these fixed samples.

## Runtime credential access

```text
POST /v1/runtime/extension-credentials/resolve
Authorization: Bearer <extension_runtime_token>
Content-Type: application/json

{"scope":"read_merchant"}
```

This is a server-to-server route, not a frontend route. OAuth `es_at_` installation tokens and control-plane JWTs do not authenticate it. Cookie and Origin headers are rejected. Header rejection reduces accidental browser use; it is not a substitute for keeping runtime tokens private and restricting ingress.

The request cannot choose merchant, app, installation, extension or environment. Query/body selectors are rejected; identity-override headers have no effect. The response derives `connectionId`, `appId`, `installationId`, `extensionId`, `merchantId`, `installedVersionId`, and `scope` from the authenticated connection, plus its decrypted `secret` map. Responses use `Cache-Control: no-store`.

Every resolve locks and rechecks:

1. Token hash, active connection and expiry.
2. Active app, active organization and environment entitlement.
3. Active installation; current extension must not be disabled and must exist in the installed snapshot.
4. Requested scope ∈ connection allowlist ∩ installed version scopes ∩ merchant grants ∩ currently available scope policy.
5. Encryption-key version and credential purpose/binding.
6. Successful access audit insertion and transaction commit **before** returning plaintext.

The app's newest version or a draft edit cannot expand installed permissions. Suspended/uninstalled installations and disabled extensions cannot resolve credentials. The current foundation blocks access but does not automatically erase stored connection ciphertext upon uninstall; the operator must revoke the connection for erasure. Suspend/reactivate can restore access to an otherwise valid connection; use explicit revocation for permanent invalidation. Revoke upstream provider keys separately when required.

The runtime is trusted code: scope checks on secret retrieval cannot prevent misuse of a provider secret after retrieval. The future execution adapter must enforce its operation, bind merchant from the resolved identity, validate inputs and upstream responses, and avoid arbitrary target URLs. Never treat a successful resolve as permission for every payment/shipping action. Do not persist, log, send to the client, or long-cache returned credentials. Platform revocation prevents future retrieval; it cannot erase a copied secret or invalidate an upstream API key by itself.

Merchant-account/business eligibility remains authoritative in Emisell Backend. Credential resolution does not query Backend merchant status or authorize a financial/shipping operation; the eventual operation integration must validate that business context as well.

## Operations and failure handling

| Status | Meaning / next action |
| --- | --- |
| 401 | Missing, invalid, expired or revoked runtime token; wrong credential type; browser headers |
| 403 | Operator privilege missing, inactive lifecycle, missing entitlement or denied scope |
| 404 | Management target not owned by selected organization/app, or connection missing for rotation/revocation |
| 409 | Revision changed; reload metadata before an explicit retry |
| 422 | Invalid input, extra selectors, oversized request or invalid expiry |
| 429 | Runtime throttle; honor `Retry-After: 60` |
| 500 | Database, encryption/key binding or audit failure; no credential/token returned; correlate request ID |
| 503 | Feature is disabled |

Management mutations use existing operator audit records; successful runtime access uses `extension_credential_access_events` with connection ID, revision, scope, request ID and timestamp. Neither stores plaintext credentials or tokens. Runtime failures use existing HTTP status/request-ID logs, not a new failed-access audit feed. No runtime-audit dashboard or retention scheduler is implemented here; set retention and monitoring policies before production. The built-in throttle is per gateway process, 120 attempts/minute per socket IP, maximum 10,000 buckets; forwarded IP headers are ignored. An ingress proxy may therefore share one bucket. Configure distributed/edge throttling for a replicated deployment.

Disabling the feature stops API access (503), not already running provider operations. Key-version mismatch fails closed. Automatic key-ring migration/re-encryption is not implemented: do not change the active encryption key without a reviewed migration/backup plan for **all** existing credential types. Migration rollback refuses to drop populated connection storage; prefer forward recovery. Database backup deletion and upstream key revocation are separate operations.

## Documentation and verification

- Admin Console → Documentation → **Admin Control Plane**: provision, metadata, rotation and revocation.
- **Managed Extension Runtime**: machine-only resolve. Provider Runtime export deliberately excludes managed/internal contracts.
- OpenAPI/Postman downloads remain operator-protected. Managed requests are skipped in Postman unless the private environment explicitly sets `enable_managed_extensions=true`; never execute an entire collection against production. Spec availability is not proof the gateway flag is enabled.

```sh
npm run backend:check
npm run test:extensions
npm run validate:contracts
npm run test:docs
```

`test:extensions` creates its own labelled loopback PostgreSQL 17 container with random credentials and tmpfs data, runs the migration and integration tests, then removes only that container. It never reads `.env` or uses `DATABASE_URL`/`TEST_DATABASE_URL` from the application. Tests cover two-merchant isolation, operator/runtime authentication separation, selector rejection, encryption, ciphertext binding, persistence across repository recreation, lifecycle/scope rechecks, audit failure, concurrent rotation, revocation and metadata redaction. Service tests additionally cover expiry, unavailable scope, installed snapshot checks, invalid provisioning and failed commit. This is not an upstream-provider end-to-end transaction test.
