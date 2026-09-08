# Resource UI test distribution

Signed `emisell.ui-resource-release/v1` releases can be assigned through the
existing test distribution workflow. This is distribution metadata, not an
installation, endpoint verification, seller consent, or resource-access grant.

1. Developer submits `POST /api/v1/developer/test-assignments` with
   `releaseKind: "ui_resource"`, the signed `releaseId`, a merchant ID, a reason,
   and an `Idempotency-Key`.
2. Administrator reviews and approves the assignment using the existing
   `/api/v1/admin/test-assignments/{id}/status` endpoint, current revision,
   reason, and idempotency key. Merchant existence is checked at approval.
3. Seller's authenticated Core session lists only approved assignments for
   that merchant. Resource rows use `executionProfile: "resource-app/v1"`.
4. When finished, the seller chooses **Stop testing / Hentikan pengujian** on
   the assignment and confirms. The current merchant Apps manager can revoke
   only their store's approved assignment. It disappears from the test list;
   history is retained. This does not uninstall an already installed app.

Migration 0027 adds an immutable resource-release reference to the shared
assignment table. Requests, decisions, revocation, audit history, uniqueness,
and pagination use the same workflow as other test assignments. The digest is
the signed outer resource package digest, including its required permissions.

The local reviewed-UI composition exposes resource distribution only when its
explicit resource signing key is configured. Installation is available only
when the signed release, approved merchant assignment, verified confidential
client, approved signed HTTPS launch, and product adapter are all current.
Otherwise metadata remains blocked with `resource_installation_not_available`.
No existing identity-only release is relabeled as a resource app.

The runtime consumes the audited shared test assignment directly; the separate
`ui_resource_assignments` prototype is not populated or required. Approval alone
does not create a grant. The seller reviews `read_products`, explicitly consents,
then installs and activates. Installed metadata appears in Settings and the Apps
menu; opening the application requires another current launch check.

## Local product test runtime

This is a development-only adapter, not a public production OAuth API. It is
disabled by default. `.local/resource-runtime.json` requires `environment` set
to `development`, a configured API-service `origin`, RSA `keyId` and private
`privateKeyPem`. The server refuses this configuration in production. Keep
private files outside source control and never include them in browser assets.
API-service separately pins the public key, sandbox environment and explicit
merchant allowlist through its `APP_PLATFORM_RESOURCE_*` configuration.

The embedded test uses a fresh identity from the authorized seller Dashboard.
Its backend verifies current identity and sends its confidential client secret
to the loopback-only API-service reviewed-UI product adapter. API-service
rechecks the live Core session and calls the private Platform product boundary
with its server-held platform credential. Platform verifies the confidential
client and the current release/assignment/installation/`read_products` grant,
then signs a short-lived merchant-bound request to API-service's private product
API. Browser-supplied merchant IDs or scopes cannot override this binding.

The local starter `/api/products` requires explicit `verifySession` and
`readProducts` backend adapters; identity verification alone cannot read data.
The CLI template's product page displays five products per page, projecting only ID,
name and price, plus opaque `meta.nextCursor`. The local endpoint accepts only
`q` (trimmed name/SKU search, at most 100 UTF-8 bytes), `cursor` (at most 1024
characters) and `limit` (1–20, default 5). Unknown or duplicate filters and
identity overrides are rejected. Search is performed in merchant-scoped SQL;
literal `%`/`_` wildcards are escaped. Cursors are bound to the merchant,
installation, app, environment and filters; changing search resets pagination.
Responses are not cached and the demo reads on demand, not by background polling.
Write access and order/customer access are not part of this test.

Generate the reusable React Router starter with `emisell app init --path
product-reader --parent-origin https://seller.emisell.test` using the local
0.4.0 candidate, then run `npm install` in that project. README and TESTING.md
describe operator configuration and the complete review/testing/consent flow.
`server/backend.mjs` loads private local settings; `server/local-products.mjs`
uses an explicitly configured private client-secret file, never browser
configuration or a repository secret. npm publication and production deployment
are separate. The legacy HTML generators are removed, not existing projects.

Validation covers signed-release ownership, idempotent assignment requests,
admin approval, merchant isolation, immutable source, cursor bounds, consent,
activation, installed-list metadata, signed launch, confidential-client checks,
bounded product reads, rejection of foreign identities or unconsented scopes,
revocation/uninstall, and secret-stripping projections. Revocation tests use an
isolated database; they do not revoke the user's real test installation.

## Seller stop contract

`POST /v1/app-platform/core/test-apps/{assignmentId}/stop` accepts only `{}`,
authenticated seller cookies, the trusted Dashboard Origin, the preview marker
and an `Idempotency-Key`. Identity/permission overrides from the browser are
rejected. It follows the local app-mutations feature gate.

Core calls `TestDistributionService/StopAssignment` using a current full Core
key, authoritative `merchantId` / `coreActorId`, `assignmentId` and request key.
The response contains only `merchantId`, `assignmentId`, `status: revoked`.
Foreign assignments are not found; requested/rejected assignments cannot be
changed by the seller. The existing immutable target, terminal revoke,
transaction lock and audit tables are reused, without a new migration.
Seller audit actors are namespaced as `core:<service>:<merchant>:<actor>`.
Duplicate requests cannot reactivate a revoked assignment or duplicate its
audit. A fresh developer request and admin approval are needed to test again.
