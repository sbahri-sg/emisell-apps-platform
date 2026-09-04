# Development test installation

Development test installation lets an approved developer send an app to one known merchant while the app status is `Development`.

The developer enters only `Merchant.id` in the Developer Console. Merchant ID selects the intended recipient but **is not authentication or consent**.

## Preconditions

- the app is active and has an active immutable version;
- the app is not published in the Emisell App Store, so its release status is `Development`;
- the app has an HTTPS launch URL (or the explicitly enabled local loopback exception below);
- every scope in the active version is currently `available`, except the narrow operator-gated `read_products` pilot below;
- the Merchant ID has already been registered in Emisell;
- an active credential compatible with the resolved merchant and an exact registered redirect URI exist.

App Store publication is not required. Payment and shipping runtime activation are not implied by a development installation.

## Flow

```text
Developer Console
  → POST /v1/apps/{appId}/test-install-requests { merchantId }
  → App Gateway resolves the merchant context and active version
  → returns provider launchUrl with emisell_test_install_request

Merchant
  → opens the launchUrl
  → provider preserves the request ID in its server session
  → provider starts normal OAuth authorization with PKCE S256
  → adds test_install_request to the /install URL
  → App Platform validates merchant session + request + app + version
  → merchant reviews required/optional scopes and approves
  → provider callback receives the one-time authorization code
  → provider backend exchanges code + verifier for es_at_* token
```

The development request expires after seven days. It becomes `authorized` when merchant consent creates the OAuth authorization; the installation itself is created only when the provider exchanges the authorization code successfully.

## Developer request

```http
POST /v1/apps/{appId}/test-install-requests
Authorization: Bearer <developer-control-plane-token>
X-Organization-ID: <developer-organization-id>
Idempotency-Key: <16-to-128-character-key>
Content-Type: application/json

{
  "merchantId": "cmmerchantdemo000000000001"
}
```

The response contains merchant metadata resolved by App Gateway and an HTTPS `launchUrl` by default. The local-only exception below may return loopback HTTP. Environment is neither accepted from the developer nor exposed in the response; App Gateway derives and validates the merchant context internally.

## Provider handoff

The launch URL contains:

```text
emisell_test_install_request=<request-id>
```

The provider stores the request ID in its server-side installation session. When it redirects the merchant to App Platform consent, it adds:

```text
test_install_request=<same-request-id>
```

alongside the normal `client_id`, exact `redirect_uri`, random session-bound `state`, PKCE S256 `code_challenge`, `code_challenge_method=S256`, and requested `scope` values.

The request ID is a binding handle, not a bearer credential. App Platform still requires a matching merchant session. A request for another Merchant ID, app, version, expired request, or previously authorized request is rejected without disclosing another tenant's details.

## States

| State | Meaning |
| --- | --- |
| `pending` | Link can be used by the matching merchant until `expiresAt` |
| `authorized` | Merchant approved and a one-time OAuth authorization was created |
| `cancelled` | Reserved for a future developer/operator revoke action |
| `expired` | An older pending request was closed when a new request was created |

The UI also displays a pending request as expired immediately after `expiresAt`, even before the next create operation persists the `expired` state.

Installed apps appear in Connected Apps only after the authorization code is exchanged. They use the same uninstall, suspension, token revocation, scope, and audit controls as other installations.

## Security invariants

- developer input never supplies merchant name, domain, user ID, or granted scopes;
- merchant metadata comes from the registered merchant identity;
- an existing pending request for the same app + merchant is rejected;
- a `Released` app rejects development test requests and must be installed through the App Store;
- planned scopes cannot be used for a development installation except the explicit `read_products` pilot below;
- the active app version is rechecked during request creation, consent, and token exchange;
- request authorization and OAuth authorization creation are committed atomically;
- access tokens remain opaque, installation-scoped, short-lived, and returned once;
- launch links, authorization codes, tokens, client secrets, and PKCE verifier values must not be logged.

## Local Product Reader pilot

The [runnable Product Reader example](../examples/product-reader/README.md) provisions isolated databases, merchant fixtures, credentials, a provider backend, and the existing consent UI. Nothing is published and existing `.env` files/databases are not used.

Only in `APP_ENV=development`, an enabled/configured Resource adapter plus `EMISELL_RESOURCE_TEST_MERCHANT_IDS` permits `read_products` for an allowlisted merchant whose resolved internal context is sandbox. The backend must separately allow that same merchant/context. This is checked at test-request creation and consent; it does not mark the public scope available or enable other planned scopes. Disable the resource adapter to stop existing tokens from reading products as well; removing the invitation allowlist alone is not token revocation.

`DEVELOPMENT_LOOPBACK_APP_HTTP=true` optionally permits exact `http://localhost`, `http://127.0.0.1`, and `http://[::1]` app URLs for the local lab. Default remains HTTPS. App Store publication still requires HTTPS. Migration `000015_loopback_development_install` permits these links only for sandbox-context request records; it does not activate the app configuration flag. Both exception settings fail startup outside development. Use HTTPS and normal identity verification for real deployments.
