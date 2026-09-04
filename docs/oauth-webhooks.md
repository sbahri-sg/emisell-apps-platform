# OAuth and webhook integration

This guide describes the secure integration contract. It intentionally contains no usable credential, token, signing secret, private key, or production host value.

## Authorization-code installation

Authorization has two entry points over the same grant engine: the owner/admin
Developer Console simulator and the authenticated merchant consent screen.
Production merchant sessions enter through the signed Emisell Backend one-time
bridge; the grant engine remains the same.

### Required sequence

1. The app backend generates a cryptographically random `state` value and PKCE verifier.
2. It stores both in the initiating server-side session; neither belongs in a global variable or browser storage shared across users.
3. The browser opens `/install` with the client ID, exact callback, requested scopes, state, and S256 challenge. Merchant and environment are not supplied as authority by this request.
4. Emisell verifies the active credential, environment, active immutable version, exact callback, requested scopes, and required scopes.
5. The one-time code is returned and appended to the callback together with the unchanged state.
6. The app backend compares state before exchanging the code.
7. The backend calls `/oauth/token` over HTTPS using HTTP Basic client authentication, the exact callback, and original verifier.
8. Emisell atomically consumes the code, creates the version-pinned installation, persists only an access-token digest, and returns the plaintext access token once.
9. The app backend presents that token to installation APIs; Emisell resolves the current installation context and checks required scopes on every request.

Authorization codes expire after five minutes by default. A code cannot be replayed, used with another client, callback, credential, or verifier, or exchanged after the active app version changes.

### PKCE calculation

Use 32 or more bytes from a cryptographically secure random generator. Encode the verifier with unpadded base64url, then compute:

```text
code_challenge = BASE64URL_NO_PADDING(SHA256(code_verifier))
code_challenge_method = S256
```

Do not use `plain` PKCE. Do not log the verifier, authorization code, client secret, or access token.

### Developer Console simulator

`Simulate OAuth install` is a sandbox-only owner/admin tool for validating this
same sequence before the external app backend is ready. It generates PKCE and
state with Web Crypto, calls the normal approval endpoint, validates the callback
state, then calls the normal token endpoint and verifies the returned token
against `/v1/installation-context`. It does not bypass credential,
redirect, scope, version, environment, expiry, replay, or tenant checks.

The simulator accepts a previously saved sandbox client secret and keeps it only
in the mounted dialog's memory. It clears the secret after exchange and never
writes the secret, verifier, code, or access token to browser storage. The
authorization and token responses are marked `Cache-Control: no-store`. Closing
the result dialog removes the one-time access token from the UI.

This is deliberately not the production client architecture. External apps must
perform token exchange on their trusted backend and obtain credentials from a
secret manager.

### Merchant consent

`/install` uses a separate merchant session bound to one user, store, and
sandbox/production environment. The session token is
`HttpOnly`; unsafe session operations require the independent merchant CSRF
cookie value in `X-CSRF-Token`. The consent preview resolves the app, immutable
active version, exact callback, and scopes before displaying them. The authorize
request cannot supply merchant identity, developer organization, developer role,
or environment; those values are resolved from the session and OAuth client.

Required scopes are always selected. The merchant may select only optional
scopes requested by the app and defined in the active version. Approval calls
the normal authorization engine and returns an exact callback URL containing a
single-use code and unchanged state. Cancel returns `access_denied` only to a
callback that already passed server validation. Token exchange remains the
external app backend's responsibility.

The merchant can review active and suspended installations at `/merchant/apps`
and uninstall only an installation bound to its own merchant ID. Uninstall uses
an idempotency key, preserves audit history, and revokes active installation
tokens. See [`merchant-installation.md`](./merchant-installation.md) for the full
integration contract and [`emisell-backend-integration.md`](./emisell-backend-integration.md)
for the production identity boundary.

### Token boundaries

The installation token is not a developer dashboard session and is not accepted by `/v1/apps` control-plane endpoints. `GET /v1/installation-context` returns only safe installation identity plus effective scopes. `GET /v1/merchant/profile` is the first resource endpoint and requires `read_merchant`; it returns only the merchant identity snapshot already bound to the installation. The callable provider surface is intentionally limited to [`provider-openapi.json`](./provider-openapi.json); scopes marked `planned` do not imply that a resource endpoint exists.

Authentication hashes the presented token, then validates token expiration and revocation, the issuing credential, active app and developer organization, active installation state, and the persisted environment entitlement. Effective scopes are computed as the intersection of the token grant and current installation grants. A missing required scope returns `403`; an invalid or inactive authentication chain returns `401` without revealing which record failed.

Keep the token server-side and apply the shared installation scope guard on every merchant or extension resource request. The merchant profile handler is the reference implementation: invalid authentication returns `401`, missing scope returns `403` plus an `insufficient_scope` challenge, and successful responses are marked `no-store`. Suspending or uninstalling an installation immediately rejects its token. Resuming works only if the token, credential, app, organization, and entitlement are still valid. Credential revocation and token expiry remain final for that token.

## Signed webhook delivery

An active immutable app version determines which subscriptions may receive an event. Publishing an event creates durable delivery attempts. A worker claims due attempts with row locking, sends signed HTTPS requests, and records only operational outcome metadata.

### Request headers

```text
Content-Type: application/json
User-Agent: Emisell-Webhooks/1.0
X-Emisell-Event: app/uninstalled
X-Emisell-Event-Id: <opaque UUID>
X-Emisell-Delivery-Id: <opaque UUID>
X-Emisell-Webhook-Signature: t=<unix>,v1=<lowercase hex HMAC-SHA256>
```

For installation-bound lifecycle and resource events, the signed JSON envelope includes the opaque tenant context:

```json
{
  "id": "<event UUID>",
  "event": "app/uninstalled",
  "apiVersion": "2026-09-01",
  "source": "app_platform",
  "createdAt": "2026-09-01T00:00:00Z",
  "merchantId": "<opaque Emisell Merchant.id>",
  "installationId": "<installation UUID>",
  "data": {
    "appId": "<app UUID>",
    "uninstalledAt": "2026-09-01T00:00:00Z"
  }
}
```

Do not infer tenant identity from payload resource fields. Use the envelope `merchantId` and `installationId`, which App Platform derives from the persisted installation.

The signed message is the UTF-8 byte sequence:

```text
<timestamp>.<raw request body>
```

### Verification sequence

1. Read and retain the raw request bytes before JSON parsing.
2. Parse `t` and `v1`; reject missing, duplicated, or malformed values.
3. Reject timestamps outside a small tolerance such as five minutes.
4. Compute HMAC-SHA256 over `timestamp + "." + raw_body` using the subscription secret.
5. Compare the expected and supplied signatures using a constant-time function.
6. After signature verification, require `X-Emisell-Event-Id` to match the signed envelope `id`. Deduplicate side effects durably by event ID and installation. Do not deduplicate only by `X-Emisell-Delivery-Id`: each retry has a different attempt ID for the same event.
7. Return `2xx` only after the event is durably accepted. Process long-running work asynchronously.

During secret rotation, receivers may temporarily verify against both the current and immediately previous secret. Remove the previous secret after the maximum delivery/retry window.

### Retry policy

- Retry: DNS/network failures, timeouts, HTTP `429`, and HTTP `5xx`.
- Do not automatically retry other HTTP `4xx` responses.
- Default attempts: five total.
- Default backoff: 5 seconds, then exponential growth capped at 15 minutes.
- Each attempt has its own immutable ID and outcome metadata.

Receivers must be idempotent because a timeout can occur after the receiver committed its work but before Emisell observed the response.

### Testing from the Developer Console

Use `Send test event` on the app Webhooks page after the subscription has been included in an active released version. The tester publishes a `source=test` event through the normal durable queue and automatically follows outcome metadata for the generated event. It does not bypass catalog validation, signing, destination validation, retry policy, or snapshot pinning, and it never displays the event payload, signing secret, or receiver response body in delivery history.

### Network and data safety

- HTTPS is mandatory.
- Embedded URL credentials and URL fragments are rejected.
- Redirects are not followed.
- Loopback, private, link-local, unspecified, and multicast destinations are blocked both for literal IPs and resolved DNS addresses.
- Event payloads are limited to 256 KiB.
- Do not publish passwords, client secrets, tokens, private keys, full payment-card data, or personal information that the receiver does not need.
- Delivery history never returns the payload, signing secret, or receiver response body.

## Operational ownership

Rotate a credential or signing secret immediately after suspected disclosure. Review audit events for approvals, exchanges, event publication, credential rotation, subscription updates, and installation changes. Production keys and secrets must be injected through the deployment secret manager rather than `.env`, command-line arguments, container images, or repository files.
