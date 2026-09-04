# Admin documentation center

Entry point: **Admin Console → Documentation**, at `/admin/docs` on the frontend (local: `http://localhost:3003/admin/docs`). This is an internal operator guide, not the public developer portal. It extends the existing Admin shell without changing payment or shipping runtimes.

Use **Gateway integration** (`/admin/docs?contract=emisell&view=gateway`) for the generated Backend Emisell → App Platform → API Kurir map. It shows both trust boundaries, safe fields/headers, ownership, failure policy and sandbox checklist. The view is generated from OpenAPI metadata and never contains a service-key value or provider credential.

## Included surfaces

- **Start here:** manual developer admission through app publication.
- **API reference:** searchable operations, method filter, authentication, fields/constraints, request examples, response schemas/statuses, OpenAPI and Postman downloads.
- **Authentication:** identity, token, cookie, and CSRF requirements by caller.
- **Troubleshooting:** auth, revisions, invalid inputs, invitations, catalog eligibility, webhook delivery, and docs connectivity.

The reference, counts, fields, examples, and downloads are generated from the checked-in OpenAPI documents. The UI does not maintain another API-path list. Examples illustrate schemas, not recorded API responses. Replace placeholder IDs and illustrative revisions/states with current valid data before use.

## Contracts by caller

| Contract | Caller and purpose | Source |
| --- | --- | --- |
| Admin Control Plane | Emisell operator: developer intake/review, invitations, read-only organization inventory, catalog decisions and default-off managed credential provisioning/rotation/revocation | Filtered `openapi.json`, internal routes only |
| Developer Management | Organization member: app configuration, immutable versions, scopes, credentials, webhooks, installations | Filtered `openapi.json`, app management routes |
| Provider Runtime | Developer app backend: OAuth exchange and installation-scoped resources | `provider-openapi.json` |
| Managed Extension Runtime — default-off | Internal Emisell runtime: credential resolution bound to one installation–extension; not payment/shipping execution | Filtered `openapi.json`; see [managed-extensions.md](./managed-extensions.md) |
| Emisell Backend Integration | Emisell Backend → Platform session bridge; merchant browser catalog, consent, connected apps | Filtered `openapi.json`; auth differs per operation |
| Shipping rate bridge — default-off | Emisell Backend → App Platform → API Kurir calculation, with merchant/extension isolation | Filtered `openapi.json`; see [shipping-rate-bridge.md](./shipping-rate-bridge.md) |
| Identity & Local Tooling | Login, session, invitation acceptance, health, internal test tooling | Filtered `openapi.json` |
| Emisell Resource API — pilot | App Gateway → Emisell Backend product reads, default disabled | `emisell-resource-openapi.json`; `implemented_gated`, not general availability |

The product boundary has cross-repository tests against the current api-service Prisma schema in disposable PostgreSQL. Gateway test storage is in-memory; this is not verification of a running deployment. Operators must still provision keys, approve test merchants and check deployment connectivity. Base price/stock are not variant/display price or available inventory. See [`resource-pilot.md`](./resource-pilot.md) for exact evidence, repeatable tests and remaining rollout gates.

Identity/tooling includes development-only and operator-only simulator operations; these are not production installation flows. In Emisell integration, the service JWT is only accepted by the merchant-session-grant endpoint, not merchant-session operations. Merchant profile is a Provider Runtime resource, not a merchant browser session API.

## Operator walkthrough

### 1. Record a selected developer

Open **Developer requests**. Verify business identity, domain, contact email, app type, use case, and scope requirements outside the platform. Create a candidate with the form or `createDeveloperApplication` reference. There is no public signup endpoint.

The source schema contains a custom-app example using `read_merchant`. It does not imply that products, orders, shipping, or payment resource APIs are available to providers.

### 2. Review the application

Read the latest application state and revision, move it to review, and record non-sensitive notes. Approval/rejection uses the latest revision. A stale revision returns `409`; reload and reconsider before retrying. Do not put personal documents, credentials, or invitation codes in review notes.

### 3. Approve and deliver the invitation

Approval creates the organization, development entitlement, and initial owner invitation together. Production access remains false. The raw code is returned only once; list/detail responses never expose it or its digest.

Deliver it through a verified secure channel, never query strings, analytics, tickets, logs, or shared screenshots. If it may have leaked, revoke and reissue. Invitation rotation is for reissue, not a mandatory extra step after approval.

### 4. The developer accepts

The invited person signs in with a verified email matching the invitation and enters the code at `/accept-invitation`. Acceptance consumes it once and activates membership. Default local expiry is 48 hours, configurable at the gateway. The operator must not accept as the developer.

### 5. Configure and test an installation

The developer configures app URL/callback, extensions, scopes, credentials, and webhook subscriptions in the organization workspace. Create the required version snapshot and test-install invitation for the selected Merchant ID. Follow [`development-test-installation.md`](./development-test-installation.md) for preconditions and OAuth handoff.

Merchant ID identifies a destination, not permission. The merchant still needs a verified session and explicit consent. The provider exchanges the code from its backend using client authentication and PKCE, then uses the installation token. Never give the provider an operator token.

**Development / Released** app labels reflect catalog publication. A released configuration version alone does not publish an app. Internal sandbox/production entitlements remain security controls even without a separate sandbox selector in the developer test-install UI.

### 6. Review and publish the listing

Open **App catalog** and inspect `eligible` / `blockingReason`. Verify active version, required metadata and launch URL, credential, available scopes, and integration readiness. Use the latest listing revision (zero only for an initial listing).

Choose **Review app** to inspect saved configuration, active-version scopes and installation evidence without opening another organization's Developer Console. The report does not test network connectivity or assert end-to-end success. **Publish reviewed version** carries the inspected revision and version; refresh and reconsider after a 409. Read [integration handoff](./integration-handoff.md) for the exact limits, API and test checklist.

Publication is an explicit operator decision. Published eligible listings become discoverable through the catalog API; publishing does not grant production entitlement. Hide a listing when it must no longer be discoverable. This is not organization suspension or production approval.

## Documentation authorization

- `platform_operator=true` is verified by the Go gateway, independently of organization owner/admin roles.
- The page uses the existing operator guard. Internal reference data and downloads use the **frontend** `GET /admin/docs/content` endpoint, which verifies gateway `/v1/session` on every request before loading a contract.
- Queries: `contract=admin|developer|provider|emisell|identity|resource`, `format=reference|openapi|postman`. Unknown selection returns `400`; missing identity `401`; non-operator `403`; unavailable identity verification `502`.
- Only the configured developer cookie, Authorization, and X-Organization-Id are forwarded to a configured gateway. Merchant cookies, role/operator flags, host headers, and arbitrary destination URLs are not forwarded.
- No server-side fallback credential is supplied, even in development. The existing local frontend may send its public development fixture, still verified by the gateway. Never set production secrets in NEXT_PUBLIC_*.
- Responses use private/no-store, nosniff, and no-referrer. Credentials/session contents are never documentation payloads. Source contracts are imported server-side and are not emitted as frontend static assets.
- Vite development file serving denies the source `docs/` directory, including raw/import and filesystem URLs. Internal server imports remain available; browse contracts through the guarded documentation endpoint or loopback Swagger viewer.
- Configure **frontend server** `APP_GATEWAY_INTERNAL_URL`: local default `http://localhost:8081`, Docker `http://app-gateway:8080`. Keep `AUTH_SESSION_COOKIE_NAME` consistent with the gateway.
- The session cookie must reach the frontend docs endpoint. Localhost ports share a hostname. For deployment use a reviewed same-host routing/cookie setup; a gateway-only host cookie on a different subdomain will not automatically reach the frontend server. Never compensate by putting credentials in URLs/public environment variables.

Actual Admin mutations remain authorized by the Go gateway. Documentation does not grant new capabilities.

## Safe OpenAPI / Postman usage

Choose a contract in API reference, then **OpenAPI** or **Postman**. Downloads are generated after operator verification. No populated credential values, response-history captures, or scripts that copy responses into shared variables are exported.

1. Import into a local/private workspace.
2. Set baseUrl to the intended environment; implemented contracts default to localhost:8081. Do not target production without readiness checks and authority.
3. Fill auth variables privately or through a vault. They are deliberately blank. Match the credential type required by that request.
4. Replace path/query placeholders and illustrative body values with current data. Optional query fields are disabled until needed.
5. Review mutating requests individually; never run the entire collection blindly. Reuse an idempotency key only for the same logical operation where required by the contract.
6. Never share populated secret environments or raw token responses/screenshots.

For bearer-or-session APIs, examples choose bearer. Cookie-only APIs include cookie/CSRF placeholders. Use browser login for browser navigation flows; examples do not automate IdP consent. The gated resource collection uses an invalid example host. Both internal and provider Products requests call `pm.execution.skipRequest()` unless the private environment explicitly sets `enable_resource_pilot=true`. The API Kurir rate request is skipped unless `enable_api_kurir_rate_bridge=true`. These switches only remove documentation guards; they do not enable backend features. See the [official Postman execution reference](https://learning.postman.com/latest-v-12/docs/tests-and-scripts/write-scripts/postman-sandbox-reference/pm-execution), [`resource-pilot.md`](./resource-pilot.md), and [`shipping-rate-bridge.md`](./shipping-rate-bridge.md). Do not remove guards or run the collection in a runner that ignores them.

Docker Swagger at localhost:8082 is a separate raw-contract development viewer. It has **no Admin login**, is bound to 127.0.0.1, and must not be exposed through a public production proxy. Restricting `/admin/docs` does not protect another server serving those files.

## Explicitly not implemented

- Dedicated Audit UI or operator audit-search API; internal audit records do exist.
- Platform-operator account/role management.
- Organization suspend/reactivate or entitlement mutation from Admin UI/API.
- Production-access review/approval workflow.
- Products are implemented as a default-disabled pilot, not generally available. Orders/inventory/customer/fulfillment routes and resource events remain unimplemented/planned.
- Payment execution, shipping shipment/label/pickup/tracking, and arbitrary external-provider dispatch. Only the internal API Kurir rate calculation bridge is implemented, gated and disabled by default.

## Troubleshooting and verification

Use the troubleshooting tab for auth, revisions, validation, invitations, catalog, and webhook checks. A docs `502` indicates failed identity verification/connectivity; it must never fall back to granting access.

Escalation reports should include method/path, timestamp, HTTP status, and gateway requestId. Redact Authorization, cookies, CSRF, invitation codes, client secrets, and webhook secrets.

Run `npm run validate:contracts` and `npm run test:docs` after contract/generator changes. Tests compare method/path coverage to the Go router, verify caller grouping, filtered refs, generated Postman auth/CSRF, placeholders, planned safeguards, and operator access. Both run in `npm run check`.
