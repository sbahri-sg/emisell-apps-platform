# Emisell App Platform API guide

For the integrated operator guide, searchable reference, and guarded OpenAPI/Postman downloads, open **Admin Console → Documentation** (`/admin/docs`). The primary surface is deliberately limited to **Internal Emisell Gateway** and **Partner API**. Admin/Developer dashboard CRUD, identity tooling, managed runtime operations, pilots, and planned blueprints are implementation references under the collapsed advanced section—not additional integration products. See [`admin-guide.md`](./admin-guide.md) for access controls and usage. Partner integration remains separately documented in [`provider-api.md`](./provider-api.md).

## Current implementation

The Go App Gateway currently implements browser identity sessions and organization switching, invite-only Developer Onboarding, operator-curated App Store publication, a signed Emisell Backend merchant bridge, durable Apps, Versions, Extensions, Scopes, Credentials, OAuth installation authorization, isolated merchant consent and sessions, installation access-token authentication, a scope-protected merchant profile, signed Webhook Delivery, and Merchant Installations slices.

| Area                    | Status      | Endpoints                                                                                       |
| ----------------------- | ----------- | ----------------------------------------------------------------------------------------------- |
| Operations              | Implemented | `GET /healthz`, `GET /readyz`                                                                   |
| Identity                | Implemented | OIDC login/callback, development login, session, membership list, organization switch, logout   |
| Developer program       | Implemented | internal intake/review/approve/reject/invite/revoke plus identity-bound acceptance              |
| Developer organizations | Implemented | operator-only list and detail with entitlement, app usage, and memberships                      |
| Apps                    | Implemented | list, create, get, update, archive                                                              |
| Versions                | Implemented | list, create, get, release, rollback                                                            |
| Extensions              | Implemented | list, create, update, disable                                                                   |
| Scopes                  | Implemented | list, replace                                                                                   |
| Credentials             | Implemented | list metadata, create, rotate, revoke                                                           |
| Webhooks                | Implemented | subscriptions, event queue, signed delivery, retry, delivery history                            |
| OAuth                   | Implemented | developer simulation and merchant consent, PKCE exchange, version-pinned installation           |
| App catalog             | Implemented | operator-curated publish/hide flow plus safe merchant catalog list/detail                        |
| Emisell Backend bridge  | Implemented | dedicated RS256 service identity, one-time session grant, merchant cookie exchange                |
| Shipping rates bridge   | Gated       | internal API Kurir calculation; exact installed-extension binding; disabled by default            |
| Merchant session        | Implemented | development sandbox login and production Emisell Backend bridge, safe session, protected logout   |
| Merchant API            | Implemented | consent preview/approval, Connected Apps/uninstall, token-authenticated `read_merchant` profile |
| Installations           | Implemented | list, install, get, review/upgrade version, suspend/resume, uninstall                           |

The complete OpenAPI 3.1 contract is in `docs/openapi.json`. With Docker running, interactive Swagger UI is available at `http://localhost:8082`.

## Local access

Docker exposes the API at `http://localhost:8081`. Running the Go process directly uses `http://localhost:8080`.

For command-line development, control-plane `/v1` requests accept:

```text
Authorization: Bearer emisell-local-dev-token
X-Organization-Id: 01995f72-0000-7000-8000-000000000001
```

Installation resource routes use their own opaque installation bearer token and never accept this control-plane credential. Merchant Portal routes use a separate server-side merchant cookie and never accept a developer session as merchant authority.

The development authenticator also accepts `X-Emisell-Role` to exercise `owner`, `admin`, `developer`, and `analyst` authorization. This fallback is development-only. The dashboard upgrades it to a server-side browser session through `POST /auth/development-login`; it does not treat the browser's organization header as session authority.

In production, API clients can use RS256 JWTs signed by the configured RSA public key. App Gateway verifies `alg`, `kid`, signature, `iss`, `aud`, `iat`, `nbf`, `exp`, UUID actor claims, role, and that `X-Organization-Id` matches the signed `organization_id` claim. The signed `email` claim binds invitation acceptance, while `platform_operator: true` gates internal review operations. Production startup fails if required issuer, audience, key, encryption, or PostgreSQL configuration is missing.

## Browser identity and organization switching

Browser login uses the OpenID Connect authorization-code flow with PKCE S256, signed ID-token validation, a hashed single-use `state`, a hashed nonce, and an encrypted short-lived PKCE verifier. The integration is provider-neutral and is enabled with `AUTH_OIDC_*` server variables; no client secret is exposed through `NEXT_PUBLIC_*`.

After login, the browser receives an opaque `HttpOnly`, `SameSite=Lax` session cookie. PostgreSQL stores only its SHA-256 digest, an absolute expiry, an idle expiry, and the active organization. Unsafe requests require the separate `X-CSRF-Token` value. Logout revokes the server record before clearing both cookies.

Organization selection is authoritative on the server:

| Method | Path                        | Purpose                                                      |
| ------ | --------------------------- | ------------------------------------------------------------ |
| `GET`  | `/v1/session`               | Current identity and active organization                     |
| `GET`  | `/v1/session/organizations` | Active memberships belonging to the current user             |
| `POST` | `/v1/session/organization`  | Verify membership and update the active session organization |
| `POST` | `/v1/session/logout`        | Revoke the current browser session                           |

The switch endpoint rejects organizations outside the user's memberships. If a membership or organization is suspended, subsequent session authentication clears it as the active tenant. OIDC currently verifies a pinned RSA key; remote JWKS discovery and automatic rotation remain a production hardening follow-up.

## Merchant App Store, identity, and consent

`/merchant/app-store`, `/install`, and `/merchant/apps` are outside both console layouts. They use the
separate `emisell_merchant_session` and `emisell_merchant_csrf` cookies. The
session is bound server-side to a persisted merchant identity and environment; preview,
approval, logout, and uninstall mutations require the matching CSRF header.

| Method   | Path                                          | Purpose                                                               |
| -------- | --------------------------------------------- | --------------------------------------------------------------------- |
| `POST`   | `/auth/sandbox-merchant-login`                | Development-only creation of an Emisell-managed test merchant session |
| `POST`   | `/v1/integrations/emisell/merchant-session-grants` | Server-to-server creation of a one-time Emisell merchant bridge    |
| `GET`    | `/auth/emisell-merchant/exchange`             | Consume the one-time code, set merchant cookies, and redirect          |
| `GET`    | `/v1/catalog/apps`                            | List operator-published, active App Store listings                     |
| `GET`    | `/v1/merchant/session`                        | Safe merchant identity and session expiry                             |
| `POST`   | `/v1/merchant/session/logout`                 | Server-side merchant-session revocation                               |
| `POST`   | `/v1/merchant/oauth/preview`                  | Validate the client request and render trustworthy app/scope details  |
| `POST`   | `/v1/merchant/oauth/authorize`                | Approve required plus selected optional scopes                        |
| `GET`    | `/v1/merchant/installations`                  | List active/suspended apps for the authenticated merchant             |
| `DELETE` | `/v1/merchant/installations/{installationId}` | Merchant-owned, idempotent uninstall                                  |

The preview and authorize bodies contain OAuth request values only. Merchant ID,
name, domain, user ID, organization ID, role, and environment are derived on the
server. Production merchant identity is accepted only through a separately
configured Emisell Backend RS256 issuer. Emisell Backend remains responsible for
verifying the actor's access to the selected store before signing the short-lived
claim. See [`emisell-backend-integration.md`](./emisell-backend-integration.md)
for the end-to-end contract and [`merchant-installation.md`](./merchant-installation.md)
for the OAuth consent flow.

## Dashboard integration

The frontend is split into two consoles plus the narrow merchant surface. `/overview` is the organization-scoped Developer Console. `/admin` is the internal Emisell Admin Console; `/admin/app-catalog` is its operator-only publication queue. `/merchant/app-store`, `/install`, and `/merchant/apps` use merchant identity and do not bootstrap developer state. Hiding navigation is not treated as authorization. See `docs/admin-developer-consoles.md` for the console route map and trust boundary.

The dashboard uses the typed client in `lib/app-platform/client.ts` for all implemented endpoints. Its local browser configuration is:

```text
NEXT_PUBLIC_APP_GATEWAY_URL=http://localhost:8081
NEXT_PUBLIC_APP_GATEWAY_TOKEN=emisell-local-dev-token
NEXT_PUBLIC_ORGANIZATION_ID=01995f72-0000-7000-8000-000000000001
NEXT_PUBLIC_ENABLE_DEVELOPMENT_LOGIN=true
NEXT_PUBLIC_AUTH_CSRF_COOKIE_NAME=emisell_csrf
NEXT_PUBLIC_MERCHANT_CSRF_COOKIE_NAME=emisell_merchant_csrf
```

Apps are loaded across all cursor pages. Create, archive, create-version, release, rollback, and create-extension requests generate a unique idempotency key. App and extension updates send the last observed `revision`, and version releases send `expectedActiveVersionId` so stale browser state cannot overwrite a newer release.

These public token and organization variables are strictly a local bootstrap fallback. Do not put production bearer tokens in `NEXT_PUBLIC_*`; production browsers use the OIDC/session flow.

## Conventions

- IDs are opaque UUIDv7-compatible values.
- Timestamps are UTC ISO 8601 values.
- JSON bodies reject unknown fields and payloads larger than 1 MiB.
- List endpoints use an opaque `cursor` and `limit` between 1 and 100.
- `Idempotency-Key` must contain 16–128 characters. A key is scoped to the organization and logical action and is retained for 24 hours.
- Mutable app updates require the last observed `revision`; a stale value returns `409`.
- Cross-organization resource access returns `404` to avoid exposing resource existence.

## Invite-only developer program

The dashboard route `/admin/developer-requests`, all `/v1/internal/developer-*` operations, and the internal organization inventory are restricted to signed Emisell platform operators. They are intentionally not granted to organization owners.

| Method        | Path                                                              | Notes                                                                        |
| ------------- | ----------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `GET`         | `/v1/session`                                                     | Returns signed actor fields used to hide internal navigation                 |
| `GET`, `POST` | `/v1/internal/developer-applications`                             | List the queue or manually record a selected candidate                       |
| `POST`        | `/v1/internal/developer-applications/{applicationId}/review`      | Submitted → under review                                                     |
| `POST`        | `/v1/internal/developer-applications/{applicationId}/approve`     | Atomically creates organization, sandbox entitlement, and one-time invite    |
| `POST`        | `/v1/internal/developer-applications/{applicationId}/reject`      | Requires review notes                                                        |
| `POST`        | `/v1/internal/developer-applications/{applicationId}/invitations` | Revokes any pending code and returns a new code once                         |
| `POST`        | `/v1/internal/developer-invitations/{invitationId}/revoke`        | Invalidates a pending code                                                   |
| `POST`        | `/v1/developer-invitations/accept`                                | Matches the code to the signed identity email and activates owner membership |

The approval response contains `invitationToken` exactly once. Only its SHA-256 digest is stored. The raw code is absent from list/get responses, audit metadata, and logs. Production access remains `false`; onboarding grants sandbox access only. See `docs/developer-onboarding.md` for operator checks and safe delivery rules.

## Developer organization inventory

The Admin Console uses a separate read model instead of inferring organizations from the onboarding queue. Only organizations with an `organization_entitlements` record are included, so the internal Emisell platform workspace is excluded.

| Method | Path                                          | Notes                                                                                                      |
| ------ | --------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `GET`  | `/v1/internal/organizations`                  | Cursor pagination plus name/slug search and `active`/`suspended` filtering                                 |
| `GET`  | `/v1/internal/organizations/{organizationId}` | Status, sandbox/production flags, app and webhook limits, non-archived app count, and accepted memberships |

Both routes require `platform_operator=true`. They are read-only: this phase does not add organization suspension, entitlement mutation, production approval, or membership management.

## Apps

| Method   | Path               | Capability   | Notes                                              |
| -------- | ------------------ | ------------ | -------------------------------------------------- |
| `GET`    | `/v1/apps`         | `app.read`   | Supports `search`, `status`, `cursor`, and `limit` |
| `POST`   | `/v1/apps`         | `app.write`  | Creates a draft; requires idempotency key          |
| `GET`    | `/v1/apps/{appId}` | `app.read`   | Organization-scoped lookup                         |
| `PATCH`  | `/v1/apps/{appId}` | `app.write`  | Requires current `revision`                        |
| `DELETE` | `/v1/apps/{appId}` | `app.manage` | Archives; requires idempotency key                 |

During invite-only access, create and update requests accept only `distribution: "custom"`. Public distribution is rejected server-side.

Create an app:

```sh
curl -X POST http://localhost:8081/v1/apps \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: create-loyalty-app-0001' \
  -H 'Content-Type: application/json' \
  --data '{
    "name": "Loyalty Connect",
    "description": "Merchant loyalty integration",
    "distribution": "custom",
    "appUrl": "https://apps.example.com/loyalty",
    "contactEmail": "developers@example.com"
  }'
```

Update an app after reading its current revision:

```sh
curl -X PATCH http://localhost:8081/v1/apps/{appId} \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Content-Type: application/json' \
  --data '{"description":"Updated description","revision":1}'
```

## Versions

| Method | Path                                             | Capability       | Notes                                   |
| ------ | ------------------------------------------------ | ---------------- | --------------------------------------- |
| `GET`  | `/v1/apps/{appId}/versions`                      | `app.read`       | Cursor paginated                        |
| `POST` | `/v1/apps/{appId}/versions`                      | `release.create` | Captures an immutable snapshot          |
| `GET`  | `/v1/apps/{appId}/versions/{versionId}`          | `app.read`       | Returns snapshot and configuration hash |
| `POST` | `/v1/apps/{appId}/versions/{versionId}/release`  | `release.manage` | Draft → active                          |
| `POST` | `/v1/apps/{appId}/versions/{versionId}/rollback` | `release.manage` | Released → active                       |

Create and release a version:

```sh
curl -X POST http://localhost:8081/v1/apps/{appId}/versions \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: create-version-1-0-0' \
  -H 'Content-Type: application/json' \
  --data '{"version":"1.0.0","releaseNote":"Initial release"}'

curl -X POST http://localhost:8081/v1/apps/{appId}/versions/{versionId}/release \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: release-version-1-0-0' \
  -H 'Content-Type: application/json' \
  --data '{}'
```

When replacing an already-active version, send `expectedActiveVersionId` to prevent a stale release action from overwriting a newer release.

## Extensions

| Method   | Path                                        | Capability  | Notes                                                |
| -------- | ------------------------------------------- | ----------- | ---------------------------------------------------- |
| `GET`    | `/v1/apps/{appId}/extensions`               | `app.read`  | Lists the mutable working configuration              |
| `POST`   | `/v1/apps/{appId}/extensions`               | `app.write` | Creates a draft; requires idempotency key            |
| `PATCH`  | `/v1/apps/{appId}/extensions/{extensionId}` | `app.write` | Requires current `revision`; changes return to draft |
| `DELETE` | `/v1/apps/{appId}/extensions/{extensionId}` | `app.write` | Soft-disables the extension for future snapshots     |

Payment and shipping are extension types; their runtime gateways remain separate services. Creating an extension only registers its control-plane configuration.

```sh
curl -X POST http://localhost:8081/v1/apps/{appId}/extensions \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: create-payment-extension-0001' \
  -H 'Content-Type: application/json' \
  --data '{
    "name":"Payments Runtime",
    "type":"payment",
    "runtimeUrl":"https://extensions.example.com/payments",
    "configuration":{"mode":"sandbox"}
  }'
```

## Scopes

| Method | Path                      | Capability  | Notes                                     |
| ------ | ------------------------- | ----------- | ----------------------------------------- |
| `GET`  | `/v1/apps/{appId}/scopes` | `app.read`  | Lists required and optional scopes        |
| `PUT`  | `/v1/apps/{appId}/scopes` | `app.write` | Atomically replaces the working scope set |
| `GET`  | `/v1/scope-catalog` | Public | Lists the authoritative registered scopes and availability |

Scope names are controlled by Emisell and must exist in `GET /v1/scope-catalog`; a matching prefix alone is not sufficient. Duplicate names are rejected, and an app can request at most 100 scopes. Entries marked `planned` are not generally available. `read_products` has an explicitly operator-enabled pilot, disabled by default; other planned resource scopes have no endpoint. An app version containing a planned scope cannot be published to the merchant App Store.

The provider-facing source of truth is [`provider-api.md`](./provider-api.md) and [`provider-openapi.json`](./provider-openapi.json). Unimplemented routes are omitted; implemented-but-gated Products routes are explicitly labeled pilot. The internal boundary is documented in [`emisell-resource-api.md`](./emisell-resource-api.md), with configuration/testing in [`resource-pilot.md`](./resource-pilot.md). Existence in a contract is not proof of deployment or general availability.

```sh
curl -X PUT http://localhost:8081/v1/apps/{appId}/scopes \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Content-Type: application/json' \
  --data '{"scopes":[
    {"scope":"write_products","access":"required"},
    {"scope":"read_orders","access":"optional"}
  ]}'
```

Creating an App Version reads the current non-disabled extensions, all scopes, and active webhook subscriptions in the same request flow, then stores them inside the immutable version snapshot and configuration hash.

## Credentials

Credential metadata can be read only by owners and admins. Plaintext client secrets are returned only after create or rotate, are encrypted with AES-256-GCM before persistence, and never appear in list responses or audit metadata.

| Method   | Path                                                 | Capability          | Notes                                             |
| -------- | ---------------------------------------------------- | ------------------- | ------------------------------------------------- |
| `GET`    | `/v1/apps/{appId}/credentials`                       | `credential.read`   | Metadata and fingerprints only                    |
| `POST`   | `/v1/apps/{appId}/credentials`                       | `credential.manage` | Returns the secret once; requires idempotency key |
| `POST`   | `/v1/apps/{appId}/credentials/{credentialId}/rotate` | `credential.manage` | Replaces the secret; requires idempotency key     |
| `DELETE` | `/v1/apps/{appId}/credentials/{credentialId}`        | `credential.manage` | Revokes access; requires idempotency key          |

```sh
curl -X POST http://localhost:8081/v1/apps/{appId}/credentials \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: create-sandbox-credential-0001' \
  -H 'Content-Type: application/json' \
  --data '{"environment":"sandbox"}'
```

Sandbox and production credential requests are checked against the organization's persisted entitlement. Approved developer organizations start with sandbox access only; production requests return `403` until a separate production approval flow grants access.

`SECRET_ENCRYPTION_KEY_BASE64` must decode to 32 bytes. The checked-in example value is for local development only; production must use a secret manager and an independently generated key.

## Webhooks

Webhooks require HTTPS endpoints without embedded credentials. Private, loopback, link-local, and other restricted address ranges are rejected both during configuration and after DNS resolution during delivery. Redirects are not followed. Each subscription receives a unique signing secret, encrypted at rest and returned once during creation. Endpoint/status updates require the last observed revision; delete is a soft-disable that preserves audit history.

`GET /v1/webhook-event-catalog` is the persisted source of truth for the event selector and provider documentation. A subscription is accepted only when its event exists in that catalog and has `availability=available`; a catalog entry with a `requiredScope` also requires that scope in the app working configuration. The initial real event is `app/uninstalled`, produced transactionally by App Platform when an installation is disconnected. Resource events sourced from Emisell Backend remain `planned` until their trusted producer contract is implemented and verified.

| Method   | Path                                               | Capability         | Notes                                                 |
| -------- | -------------------------------------------------- | ------------------ | ----------------------------------------------------- |
| `GET`    | `/v1/apps/{appId}/webhooks`                        | `app.read`         | Cursor paginated                                      |
| `POST`   | `/v1/apps/{appId}/webhooks`                        | `app.write`        | Returns signing secret once; requires idempotency key |
| `PATCH`  | `/v1/apps/{appId}/webhooks/{webhookId}`            | `app.write`        | Update HTTPS endpoint or active/paused status         |
| `DELETE` | `/v1/apps/{appId}/webhooks/{webhookId}`            | `app.write`        | Soft-disables the subscription                        |
| `GET`    | `/v1/apps/{appId}/webhooks/{webhookId}/deliveries` | `app.read`         | Outcome metadata only; no payload or secret           |
| `GET`    | `/v1/webhook-event-catalog`                       | Public             | Persisted available/planned event definitions         |
| `POST`   | `/v1/apps/{appId}/webhook-events`                  | `webhook.dispatch` | Owner/admin development test only                      |

```sh
curl -X POST http://localhost:8081/v1/apps/{appId}/webhooks \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: create-lifecycle-webhook-0001' \
  -H 'Content-Type: application/json' \
  --data '{"event":"app/uninstalled","endpointUrl":"https://hooks.example.com/lifecycle"}'
```

Delivery requests include `X-Emisell-Event`, `X-Emisell-Event-Id`, `X-Emisell-Delivery-Id`, and `X-Emisell-Webhook-Signature`. The signature format is `t=<unix>,v1=<hex HMAC-SHA256>` over `timestamp + "." + raw_request_body`. Verify it with a constant-time comparison and reject stale timestamps before parsing or processing the body. See `docs/oauth-webhooks.md` for the safe verification sequence and retry policy.

Only subscriptions contained in the active immutable version are eligible. Retryable network errors, timeouts, HTTP `429`, and HTTP `5xx` responses use bounded exponential backoff. Other `4xx` responses are recorded as final failures. Delivery history exposes status, attempt, HTTP status, duration, and a coarse error code; it never returns payloads, signing secrets, or response bodies.

### Development webhook tester

The app-level Webhooks page exposes `Send test event` only when the active released version contains at least one available webhook subscription. Its event selector is the intersection of that immutable snapshot and the persisted event catalog—not from a React constant or unreleased working configuration. After queueing an event, the console polls delivery metadata for the exact event and shows pending, delivered, failed, partial, or timed-out status. A timeout in the console does not cancel the durable worker; retry processing continues in the background.

If an endpoint, event, or subscription status changed after the last release, the console warns that a new version must be released before the new configuration can be tested. Test payloads must contain non-sensitive JSON objects and remain subject to the same validation, signing, SSRF protection, queue, and retry behavior as normal webhook events.

## OAuth installation authorization

The authorization flow is for confidential server-side clients. The Developer Console simulator approval endpoint is an internal tool restricted to an authenticated Emisell platform operator. External developers test through a merchant-bound development request and the normal merchant consent endpoints. The merchant approval endpoints require an authenticated merchant session plus CSRF; its environment must match the OAuth credential. The token endpoint always uses HTTP Basic client authentication and form-encoded OAuth parameters.

| Method | Path                           | Authentication                | Notes                                                                                     |
| ------ | ------------------------------ | ----------------------------- | ----------------------------------------------------------------------------------------- |
| `POST` | `/v1/oauth/authorizations`     | Emisell platform operator     | Internal sandbox simulator creates a five-minute, single-use code; PKCE S256 required     |
| `POST` | `/v1/merchant/oauth/preview`   | Merchant session + CSRF       | Resolves the active app/version and validated required/optional scope display             |
| `POST` | `/v1/merchant/oauth/authorize` | Merchant session + CSRF       | Creates the code from session-derived merchant identity and selected valid scopes         |
| `POST` | `/oauth/token`                 | HTTP Basic client credentials | Atomically consumes the code, creates the installation, and returns the access token once |
| `GET`  | `/v1/installation-context`     | Installation bearer token     | Resolves the active installation and effective scopes without exposing secret material    |
| `GET`  | `/v1/merchant/profile`         | Installation bearer token     | Requires `read_merchant`; returns only the installation-bound merchant identity snapshot  |

Security invariants:

- `state` is required and echoed unchanged to the exact registered callback;
- callback comparison is exact against the active version snapshot—wildcards and prefix matching are not used;
- the authorization code, access token, and client secret are never persisted in plaintext;
- authorization code consumption and installation/token creation commit atomically;
- credential environment, active app version, required scopes, expiration, PKCE, and one-time use are rechecked during exchange;
- successful authorization and token responses use `Cache-Control: no-store`;
- merchant identity and developer organization are resolved from authenticated server state, never accepted from consent-body overrides;
- in real integrations the client secret belongs only in server-side configuration or a secret vault, never in merchant-facing browser JavaScript or a URL.

Generate PKCE and retain the verifier only in the initiating server session. Exchange the returned code using environment variables or a secret manager; never paste real secrets into documentation or shell history:

```sh
curl -X POST "$EMISELL_GATEWAY_URL/oauth/token" \
  -u "$OAUTH_CLIENT_ID:$OAUTH_CLIENT_SECRET" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=authorization_code' \
  --data-urlencode "code=$OAUTH_AUTHORIZATION_CODE" \
  --data-urlencode "redirect_uri=$OAUTH_REDIRECT_URI" \
  --data-urlencode "code_verifier=$OAUTH_CODE_VERIFIER"
```

The returned installation access token is distinct from the developer-dashboard JWT and is not accepted by control-plane `/v1/apps` routes. It can authenticate only the installation boundary:

```sh
curl "$EMISELL_GATEWAY_URL/v1/installation-context" \
  -H "Authorization: Bearer $EMISELL_INSTALLATION_TOKEN"
```

The gateway hashes the presented token before lookup, then verifies token expiration/revocation, credential validity, active app and developer organization, active installation status, environment entitlement, and effective scopes. The response contains safe app, merchant, environment, installed-version, scope, and expiry context; it never echoes the token, its digest, or a client secret. Effective scopes are the intersection of the token grant and the installation's current grants, so stale or expanded claims fail closed.

The first scope-protected resource returns only the merchant identity already captured by the installation:

```sh
curl "$EMISELL_GATEWAY_URL/v1/merchant/profile" \
  -H "Authorization: Bearer $EMISELL_INSTALLATION_TOKEN"
```

The token must contain `read_merchant`. Missing scope returns `403` with an `insufficient_scope` Bearer challenge; an invalid authentication chain returns `401`. This endpoint does not invent a merchant catalog, orders, customers, payments, or shipping data model. Those resource APIs should be added only when the corresponding Emisell source-of-truth contracts exist.

Suspending or uninstalling an installation makes its token return `401`. Resuming a suspended installation restores access only while the underlying token and credential remain valid. Credential revocation, app or organization suspension, environment removal, and token expiry also reject access.

### Sandbox OAuth simulator

The Developer Console includes `Simulate OAuth install` on the Sandbox
installations page. It is an owner/admin-only development tool that calls the
same authorization and token endpoints described above; it is not a mocked or
separate installation path.

Before running it, the selected app needs:

1. an HTTPS app URL captured by its active released version;
2. an active sandbox credential;
3. the one-time client secret saved when that credential was created or rotated.

The simulator generates the verifier, S256 challenge, and state with Web Crypto,
reviews required and optional access, validates the returned callback state,
exchanges the one-time code, immediately authenticates the returned token
against `/v1/installation-context`, and calls `/v1/merchant/profile` to prove
scope enforcement. For this local sandbox tool only, the pasted
client secret and returned access token live in React memory until the dialog closes.
They are never written to browser storage, URLs, logs, or the repository. The
simulator sends the secret only as HTTP Basic authentication to `/oauth/token`.

This browser-assisted exchange is a development convenience, not the production
integration pattern. A real app must generate PKCE/state and exchange its code
from its own trusted backend, with the secret supplied by a secret manager.

### Sandbox Merchant Install portal

The merchant-facing `/install` route is the realistic sandbox integration path.
The external app backend creates the state and PKCE challenge, then redirects the
merchant to the portal using a standard authorization URL. The portal validates
the request through `/v1/merchant/oauth/preview`, displays resolved app and scope
information, and authorizes only after explicit merchant approval. It never
receives the client secret or verifier. The callback code is exchanged by the
external app backend, not browser JavaScript.

The `/merchant/apps` route lists only installations matching the authenticated
authenticated merchant and environment. Merchant-initiated uninstall resolves ownership on the server,
uses the normal transactional uninstall path, records
`installation.uninstalled_by_merchant`, and immediately revokes active tokens.
See [`merchant-installation.md`](./merchant-installation.md) for URL parameters,
endpoint contracts, cancellation behavior, and production requirements.

## Transaction guarantees

The PostgreSQL repository commits the following in one transaction:

1. lock the app and idempotency scope;
2. verify revision, version state, and expected active version;
3. mark the previous active version as released;
4. activate the selected immutable snapshot;
5. update the app's active version and revision;
6. record the idempotency result and append the audit event.

Only one active version per app is also enforced by a partial unique database index.

## Merchant installations

An app must have an active version before it can be installed. Each installation is pinned to that immutable version, captures the merchant display identity, environment, and granted scopes, and enforces every required scope from the version snapshot.

The Developer Console creates **development test requests** from Merchant ID only. App Gateway resolves merchant context internally and allows this flow only while the app status is `Development`. The merchant must complete normal OAuth consent and the provider must exchange the authorization code before an installation exists. Once the app is `Released`, new installations start from the App Store.

| Method   | Path                                                      | Capability            | Notes                                                      |
| -------- | --------------------------------------------------------- | --------------------- | ---------------------------------------------------------- |
| `GET`    | `/v1/apps/{appId}/installations`                          | `app.read`            | Cursor-paginated history                                   |
| `POST`   | `/v1/apps/{appId}/installations`                          | Platform operator + `installation.manage` | Internal sandbox fixture; not the developer test-install flow |
| `GET`    | `/v1/apps/{appId}/test-install-requests`                  | `app.read`            | Development invitations and current state                 |
| `POST`   | `/v1/apps/{appId}/test-install-requests`                  | `installation.manage` | Resolves Merchant ID and returns a seven-day provider launch link |
| `GET`    | `/v1/apps/{appId}/installations/{installationId}`         | `app.read`            | Organization-scoped lookup                                 |
| `PATCH`  | `/v1/apps/{appId}/installations/{installationId}`         | `installation.manage` | Suspend/resume with current revision                       |
| `POST`   | `/v1/apps/{appId}/installations/{installationId}/upgrade` | `installation.manage` | Owner/admin only; atomic, revision-checked, and idempotent |
| `DELETE` | `/v1/apps/{appId}/installations/{installationId}`         | `installation.manage` | Soft-uninstall; requires idempotency key                   |

```sh
curl -X POST http://localhost:8081/v1/apps/{appId}/test-install-requests \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: test-install-demo-0001' \
  -H 'Content-Type: application/json' \
  --data '{"merchantId":"cmmerchantdemo000000000001"}'
```

Merchant name, domain, internal context, version, and scopes are resolved by App Gateway from Merchant ID and app state. The returned launch link is sent to that merchant and must continue through the provider's PKCE OAuth flow. See [`development-test-installation.md`](./development-test-installation.md).

An installation never moves just because a new version is released. The console
shows `Update available` and compares extensions, webhooks, and scopes before an
owner or admin confirms. The upgrade request must name both the active target
version and the currently installed version:

```sh
curl -X POST http://localhost:8081/v1/apps/{appId}/installations/{installationId}/upgrade \
  -H 'Authorization: Bearer emisell-local-dev-token' \
  -H 'X-Organization-Id: 01995f72-0000-7000-8000-000000000001' \
  -H 'Idempotency-Key: upgrade-demo-merchant-0001' \
  -H 'Content-Type: application/json' \
  --data '{
    "targetVersionId":"01995f72-0000-7000-8000-000000000120",
    "expectedInstalledVersionId":"01995f72-0000-7000-8000-000000000110",
    "revision":1
  }'
```

The database transaction updates the pinned version, granted scopes, active
OAuth token grants, audit event, and idempotency record together. Target-version
required scopes are added, scopes removed from the target are dropped, existing
optional scopes are preserved when still defined, and newly introduced optional
scopes are not auto-granted. Production merchant re-consent remains a separate
future flow; this endpoint is currently exposed through the sandbox console.

## Production JWT configuration

The trusted identity issuer must issue these claims: `sub` (UUID user), `organization_id` (UUID tenant), `role`, `iss`, `aud`, `iat`, and `exp`. The user, organization, and membership must already be provisioned in PostgreSQL so audited mutations can satisfy their foreign keys.

```text
APP_ENV=production
REPOSITORY_DRIVER=postgres
AUTH_JWT_ISSUER=https://identity.example.com
AUTH_JWT_AUDIENCE=emisell-app-platform
AUTH_JWT_KEY_ID=production-key-1
AUTH_JWT_PUBLIC_KEY_BASE64=<base64-encoded RSA public key PEM>
AUTH_JWT_CLOCK_SKEW=30s
SECRET_ENCRYPTION_KEY_BASE64=<independent 32-byte key>
OAUTH_CODE_TTL=5m
OAUTH_ACCESS_TOKEN_TTL=1h
WEBHOOK_REQUEST_TIMEOUT=8s
WEBHOOK_MAX_ATTEMPTS=5
```

Request-supplied development user and role headers are ignored by the production authenticator. Key rotation currently uses a controlled restart with the next public key; remote JWKS refresh can be added when the final identity provider is selected.

Do not place any production JWT, OAuth client secret, access token, signing secret, database password, or encryption key in this repository. The values shown above are placeholders; hosted values must come from the deployment secret manager.

## Errors

Errors use a stable envelope:

```json
{
  "error": {
    "code": "conflict",
    "message": "resource conflict: active version changed",
    "requestId": "01a056df-0000-7000-8000-000000000000"
  }
}
```

| Status | Meaning                                                  |
| ------ | -------------------------------------------------------- |
| `401`  | Missing or invalid access token or organization header   |
| `403`  | Role lacks the required capability                       |
| `404`  | Resource not found within the active organization        |
| `409`  | Revision, uniqueness, or workflow state conflict         |
| `422`  | Invalid path value, query, headers, or JSON body         |
| `503`  | Readiness check failed because PostgreSQL is unavailable |
