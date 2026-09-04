# Browser authentication and organization sessions

## Security boundary

Emisell App Platform supports five intentionally separate authentication modes:

- Admin Console uses an opaque, dedicated session created by email + password for a manually provisioned account. It is the only credential accepted by `/v1/internal/*`; Developer JWTs and browser sessions never fall back into Admin access.

- API clients use a signed RS256 bearer token. The organization header must match the signed organization claim.
- Browsers use an opaque server-side session created through OpenID Connect authorization code + PKCE. The active organization is read from that session, never from local storage or an unsigned browser header.
- Merchants use a distinct opaque server-side session bound to a persisted merchant identity and environment. Sandbox can use the development login; production sessions can only be bridged from authenticated Emisell Backend claims. Merchant sessions cannot authorize Developer or Admin Console operations.
- Emisell Backend uses a dedicated, short-lived RS256 service JWT only to create one-time merchant-session grants. It is never accepted as a developer token or installation token.

The Developer browser session is the default developer-dashboard model. The fixed bearer token and `/auth/development-login` endpoint exist only when `APP_ENV=development`; neither can access Admin APIs.

## Admin email/password login

There is no registration endpoint. Accounts are created only through `cmd/admin-user` after an Emisell operator selects the account and the internal platform organization. The CLI reads passwords from a hidden terminal prompt (or an explicitly piped secret-manager value), requires at least 15 characters, and never accepts passwords as command-line arguments.

Passwords are stored only as versioned PBKDF2-HMAC-SHA256 hashes with 600,000 iterations and a random 128-bit salt. Login performs the same bounded hash work for unknown emails, returns a generic credential error, limits concurrent hashing, and uses durable 15-minute per-email and transport-IP counters. Production should retain an edge rate limit in front of the gateway as defense in depth.

The browser receives `emisell_admin_session` (opaque, `HttpOnly`, `SameSite=Strict`) and `emisell_admin_csrf` (`SameSite=Strict`). Both are `Secure` outside explicit local HTTP development. Only token digests are stored. Admin sessions have an eight-hour absolute lifetime and a 30-minute sliding idle lifetime. Logout, password reset, and account disable revoke sessions in PostgreSQL. The account's platform organization is loaded server-side and cannot be supplied by the browser.

Local setup, without applying unrelated deferred migrations:

```sh
docker compose run --rm app-gateway go run ./cmd/admin-user migrate
docker compose run --rm app-gateway go run ./cmd/admin-user create --email admin@example.com --name "Emisell Admin"
```

Use `reset-password --email ...` to replace a password or `disable --email ...` to disable the account; both revoke all sessions. Never send a password through chat, logs, tickets, environment files, or a shell argument.

For throwaway local development only, the CLI accepts `--allow-weak-development-password` and lowers the minimum to 12 characters. The command refuses this flag unless `APP_ENV=development`; production retains the 15-character minimum. Do not reuse a development password anywhere else.

## OIDC flow

1. `GET /auth/login?return_to=/overview` generates cryptographically random `state`, nonce, and PKCE verifier.
2. PostgreSQL stores the state and nonce only as SHA-256 digests. The PKCE verifier is encrypted with the configured AES-256-GCM key and expires within a bounded login TTL.
3. The browser is redirected to the configured authorization endpoint with `code_challenge_method=S256`.
4. `GET /auth/callback` atomically consumes the state before exchanging the authorization code.
5. App Gateway validates the ID token's RSA signature, pinned `kid`, issuer, audience, time claims, nonce, verified email, and subject.
6. The provider + subject pair is linked to an internal user, then a new server-side session is created.

`return_to` accepts only a relative path beginning with a single `/`. Absolute and protocol-relative destinations fall back to `/overview`, preventing open redirects.

## Session and CSRF controls

The browser receives:

- `emisell_session`: opaque, `HttpOnly`, `SameSite=Lax`, `Secure` in production;
- `emisell_csrf`: opaque, `SameSite=Lax`, `Secure` in production, readable by the frontend so it can send `X-CSRF-Token`.

Only SHA-256 digests of both values are stored. Unsafe cookie-authenticated methods are rejected unless the CSRF header matches the session record using constant-time comparison. Sessions enforce both `AUTH_SESSION_TTL` and a sliding `AUTH_SESSION_IDLE_TTL`. Logout records revocation server-side before expiring cookies.

Never log cookies, CSRF values, authorization codes, ID tokens, client secrets, or decrypted PKCE verifiers.

## Merchant sessions

The Merchant Install and Connected Apps surfaces use different cookie names:

- `emisell_merchant_session`: opaque, `HttpOnly`, `SameSite=Lax`, and `Secure`
  in production-capable configuration;
- `emisell_merchant_csrf`: opaque, readable only so the frontend can send the
  matching `X-CSRF-Token` header.

The same hardened server-side session store is reused, but a merchant request is
accepted only when the session user is linked through `merchant_identities` to a
merchant and its sandbox/production environment. A normal OIDC/developer session
without that binding fails.
Control-plane capabilities and active organization are not consulted as merchant
authority.

`POST /auth/sandbox-merchant-login` provisions or resolves the test identity only
when development authentication is configured. It validates the browser origin
and creates the merchant mapping and session server-side. The later consent body
cannot replace the merchant identity. Production must use a selected merchant
identity provider; the manual development login is not a production fallback.

Production uses `POST /v1/integrations/emisell/merchant-session-grants` with a
dedicated Emisell Backend RS256 assertion, followed by one browser exchange of a
hashed, short-lived code. The request body cannot override signed store identity
or permissions. See [`emisell-backend-integration.md`](./emisell-backend-integration.md).

## Organization switching

`GET /v1/session/organizations` returns memberships for the authenticated user. `POST /v1/session/organization` accepts an organization ID but switches only if an active membership and active organization are found in PostgreSQL. The validation and session update use one serializable transaction.

Every later request reloads the active membership. If it is removed or the organization is suspended, App Gateway clears the active organization and organization capabilities become unavailable.

## Provider configuration

Set these only in the App Gateway's server environment:

```text
AUTH_OIDC_ENABLED=true
AUTH_OIDC_ISSUER=https://identity.example.com
AUTH_OIDC_AUTHORIZATION_ENDPOINT=https://identity.example.com/oauth2/authorize
AUTH_OIDC_TOKEN_ENDPOINT=https://identity.example.com/oauth2/token
AUTH_OIDC_CLIENT_ID=emisell-app-platform
AUTH_OIDC_CLIENT_SECRET=<secret-manager-reference>
AUTH_OIDC_REDIRECT_URL=https://gateway.example.com/auth/callback
AUTH_OIDC_KEY_ID=<pinned-signing-key-id>
AUTH_OIDC_PUBLIC_KEY_BASE64=<base64-encoded-RSA-public-key-PEM>
AUTH_OIDC_SCOPES=openid,profile,email
```

Do not prefix these with `NEXT_PUBLIC_`. The current foundation validates a pinned RSA key. Before production launch, connect a chosen provider, add controlled JWKS refresh/rotation, enforce its account lifecycle policy, and complete a dedicated security review.
