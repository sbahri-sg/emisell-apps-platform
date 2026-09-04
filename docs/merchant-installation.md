# Merchant installation and consent

Emisell App Platform provides a separate merchant-facing surface for
installing and managing apps. It uses the same OAuth authorization-code engine,
immutable app version, credential, scope, installation, token, and audit records
as the Developer Console simulator. It is not a mocked installation path.

## Current boundary

The merchant flow supports sandbox and production-bound identities:

- `/install` displays the consent screen;
- `/merchant/app-store` displays operator-published apps;
- `/merchant/apps` displays apps connected to the signed-in merchant and environment;
- `/auth/sandbox-merchant-login` creates a development-only merchant session;
- the merchant session and CSRF cookies are distinct from Developer Console
  cookies;
- a merchant session never grants access to developer or internal operator APIs;
- a developer or operator session never becomes a merchant session;
- payment and shipping runtimes remain separate extensions and are not changed
  by this flow.

The manual sandbox identity form is a development tool for Emisell-managed test
merchants. Production accepts merchant identities only through the signed,
short-lived Emisell Backend session bridge described in
[`emisell-backend-integration.md`](./emisell-backend-integration.md).

## End-to-end flow

1. The external app backend generates a cryptographically random `state` and
   PKCE verifier, then stores both in the initiating server-side session.
2. The backend derives the S256 PKCE challenge and redirects the merchant to
   Emisell's `/install` URL.
3. Emisell authenticates the merchant with a separate server-side
   merchant session.
4. The consent API resolves the app from `client_id` and validates the active
   credential for the merchant environment, matching entitlement, active immutable version, exact
   callback, requested scopes, state, and S256 challenge.
5. The consent screen shows the resolved app, merchant, callback host, active
   version, required access, and requested optional access. Merchant identity is
   loaded from the session and is never accepted from the authorization body.
6. Approval creates a five-minute single-use authorization code and redirects
   to the exact registered callback with `code` and the unchanged `state`.
7. The app backend compares `state`, then exchanges the code at `/oauth/token`
   using HTTP Basic client authentication, the exact callback, and the original
   verifier.
8. App Gateway atomically consumes the code, creates the version-pinned
   installation and hashed token record, then returns the access token once.
9. `/merchant/apps` lists the merchant's active or suspended
   installations. Uninstall is merchant-bound, idempotent, audited, and revokes
   active installation tokens.

## Install URL

The app backend constructs this browser URL using values already registered in
the active app version:

```text
http://localhost:3003/install?client_id=<sandbox-client-id>&redirect_uri=<exact-registered-callback>&state=<random-session-bound-state>&code_challenge=<base64url-sha256-challenge>&code_challenge_method=S256&scope=read_merchant%20read_orders
```

For a development test invitation, the provider also appends `test_install_request=<request-id>` after preserving `emisell_test_install_request` from its launch URL. App Platform then binds consent to the intended sandbox merchant, app, environment, and active version. See [`development-test-installation.md`](./development-test-installation.md).

Parameter rules:

| Parameter               | Requirement                                                                                    |
| ----------------------- | ---------------------------------------------------------------------------------------------- |
| `client_id`             | Active sandbox credential client ID                                                            |
| `redirect_uri`          | Exact callback in the active immutable version; no wildcard or prefix match                    |
| `state`                 | Random 16–512 character value retained in the app's server session                             |
| `code_challenge`        | Unpadded base64url SHA-256 of a 43–128 character verifier                                      |
| `code_challenge_method` | Must be `S256`                                                                                 |
| `scope`                 | Space-separated scopes from the active version; required scopes are always included by Emisell |
| `test_install_request`  | Required only for a development invitation; opaque request ID preserved by the provider backend |

The client secret and PKCE verifier must never appear in this URL, browser
storage, frontend JavaScript configuration, analytics, or logs.

## Merchant session endpoints

| Method | Path                           | Purpose                                                 |
| ------ | ------------------------------ | ------------------------------------------------------- |
| `POST` | `/auth/sandbox-merchant-login` | Development-only creation of a sandbox merchant session |
| `POST` | `/v1/integrations/emisell/merchant-session-grants` | Production server-to-server session bridge |
| `GET` | `/auth/emisell-merchant/exchange` | One-time browser exchange and redirect |
| `GET`  | `/v1/merchant/session`         | Return the safe merchant identity and session expiry    |
| `POST` | `/v1/merchant/session/logout`  | Revoke the merchant session and expire its cookies      |

The browser receives `emisell_merchant_session` as an opaque `HttpOnly`,
`SameSite=Lax` cookie and `emisell_merchant_csrf` as a separate readable CSRF
cookie. PostgreSQL stores only SHA-256 digests. Unsafe merchant-session requests
must include the matching value in `X-CSRF-Token`.

The sandbox login accepts an opaque Emisell Merchant ID, display name, and optional hostname solely to
create or resolve a test identity. Later consent requests accept no merchant ID,
name, domain, user ID, organization ID, environment, or developer role from the
browser. Those values come from the authenticated merchant session and resolved
OAuth client.

For production, store/user/environment values come only from the verified
Emisell Backend JWT and cannot be supplied in the request body. The one-time
exchange code is stored only as a SHA-256 digest and is rejected after use or
expiry.

## Consent and connected-app endpoints

| Method   | Path                                          | Purpose                                                                        |
| -------- | --------------------------------------------- | ------------------------------------------------------------------------------ |
| `POST`   | `/v1/merchant/oauth/preview`                  | Validate the request and return safe app, version, callback, and scope details |
| `POST`   | `/v1/merchant/oauth/authorize`                | Approve required plus selected optional access and return the callback URL     |
| `GET`    | `/v1/merchant/installations`                  | List active and suspended apps belonging to the signed-in merchant             |
| `DELETE` | `/v1/merchant/installations/{installationId}` | Uninstall a merchant-owned app; requires `Idempotency-Key`                     |

Preview and authorization are CSRF-protected and reject unknown JSON fields.
Required scopes cannot be removed. Optional scopes can be granted only when the
app requested them and they exist in the active version. The authorization body
cannot broaden access, select another tenant, or install another environment.

Selecting **Cancel** on a valid consent page redirects to the registered callback
with `error=access_denied` and the original `state`; it does not create a grant.
For an invalid or untrusted callback request, the portal displays an error rather
than redirecting to an unverified destination.

## Token exchange

The callback belongs to the external app backend. After validating `state`, that
backend performs the exchange over HTTPS:

```sh
curl -X POST "$EMISELL_GATEWAY_URL/oauth/token" \
  -u "$OAUTH_CLIENT_ID:$OAUTH_CLIENT_SECRET" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=authorization_code' \
  --data-urlencode "code=$OAUTH_AUTHORIZATION_CODE" \
  --data-urlencode "redirect_uri=$OAUTH_REDIRECT_URI" \
  --data-urlencode "code_verifier=$OAUTH_CODE_VERIFIER"
```

The values above are placeholders. Keep production credentials in a secret
manager and keep the returned installation token on the app backend. The token
is not a Developer Console token and cannot access `/v1/apps` or internal routes.

## Production follow-up

Before enabling production merchant installations:

1. connect the selected Emisell merchant identity provider;
2. validate issuer, audience, nonce, verified subject, account status, and remote
   JWKS rotation;
3. define merchant-account-to-store authorization and recovery rules;
4. apply production origin, TLS, rate-limit, abuse, and session policies;
5. perform threat modeling and penetration testing for OAuth mix-up, CSRF,
   session fixation, consent spoofing, code replay, PKCE downgrade, and tenant
   isolation.

The manual sandbox login must not be promoted or exposed as a production login.
