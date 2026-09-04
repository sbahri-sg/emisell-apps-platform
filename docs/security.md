# Security model and deployment checklist

## Trust boundaries

- Developer control plane: protected by a trusted RS256 identity token and organization-scoped authorization.
- Merchant Portal: server session explicitly bound to user, store, and sandbox/production environment, with separate cookies and CSRF protection; production entry requires a signed Emisell Backend bridge.
- OAuth client: a confidential server application authenticated with a client ID and secret.
- Merchant installation: pinned to an immutable app version and an explicit granted-scope set.
- Webhook receiver: an external HTTPS service authenticated by an HMAC signature.
- PostgreSQL: authoritative state for grants, token digests, delivery attempts, idempotency, and audit records.
- Developer program: internal review actions require a signed platform-operator claim independent from organization roles; acceptance is bound to the signed email claim.

Payment and shipping runtimes remain outside this control plane. The App Gateway merchant profile endpoint demonstrates installation-token and scope enforcement, but extension runtimes must apply the same checks at their own trust boundary.

## Secret handling

The repository must never contain a production JWT, OAuth client secret, authorization code, access token, webhook signing secret, database password, or encryption key.

- Client and signing secrets are encrypted with AES-256-GCM and shown only at creation or rotation.
- Authorization codes and installation access tokens are stored only as SHA-256 digests.
- One-time developer invitation codes are stored only as SHA-256 digests and are never returned by list or history APIs.
- API list/history responses expose fingerprints and coarse outcomes, not secret material or webhook payloads.
- Production configuration must come from a secret manager with access logging and rotation support.
- `NEXT_PUBLIC_*` values are browser-visible. They must never carry production bearer tokens or credentials.

Local example values are intentionally public development fixtures. They provide convenience, not security, and must never be copied into a hosted environment.

## Required production controls

- Set `APP_ENV=production` and `REPOSITORY_DRIVER=postgres`.
- Provide an independent random 32-byte encryption key and controlled key version.
- Configure the trusted JWT issuer, audience, key ID, RSA public key, and small clock skew.
- Configure the identity provider to sign verified `email` and tightly administered `platform_operator` claims. Never derive operator access from browser headers or organization owner status.
- Provision users, organizations, and memberships before accepting audited mutations.
- Terminate TLS at a trusted proxy and permit only known dashboard origins.
- Keep PostgreSQL private, require TLS, use a least-privileged database role, and enable encrypted backups.
- Send structured logs to a protected sink; do not log authorization headers, request bodies, form bodies, query strings containing codes, or secret environment values.
- Alert on repeated OAuth failures, credential rotation/revocation, webhook final failures, unusual event volume, and cross-tenant access attempts.
- Define retention and deletion policies for webhook payloads, delivery metadata, expired grants/tokens, idempotency records, and audit events.

## Implemented defenses

- Fail-closed production startup when JWT, encryption, or PostgreSQL configuration is absent.
- RS256 algorithm/key pinning and issuer, audience, expiry, issued-at, subject, organization, and role validation.
- Tenant-scoped repository queries and `404` responses for cross-tenant resources.
- Owner/admin capability boundaries for credentials, install approval, direct installation, and event publishing.
- A separate signed platform-operator boundary for developer intake, review, approval, rejection, invitation rotation, and revocation.
- Identity-bound developer acceptance, hashed 256-bit invitation codes, bounded expiration, one pending invitation per application, and atomic owner-membership activation.
- Exact OAuth redirect matching, required state, mandatory PKCE S256, short code lifetime, single-use codes, and atomic exchange.
- Session-derived merchant consent, signed Emisell Backend store identity, hashed one-time session grants, server-resolved app/tenant/environment, fixed required scopes, bounded optional grants, and merchant-owned Connected Apps/uninstall.
- Opaque installation-token digest lookup, active installation/credential/app/organization/entitlement validation, fail-closed effective scopes, an authoritative scope registry, and a `read_merchant`-protected profile resource.
- Opaque Emisell Merchant IDs stored separately from credentials; the provider cannot select a tenant with a request header or parameter.
- Optimistic revisions, transactional idempotency, row locks, unique lifecycle constraints, and append-only application audit events.
- HTTPS-only webhooks, SSRF-resistant DNS/IP checks, no redirect following, bounded timeouts, payload size limits, HMAC signatures, leases, and capped retries.
- Unknown JSON-field rejection and bounded request bodies.

## Known boundaries

- Admin documentation data/downloads are guarded on the frontend server using gateway-verified operator identity, not only a page guard. Docker Swagger is separate, has no login, and is bound to 127.0.0.1. Do not expose its raw contract mounts through a public proxy. See [`admin-guide.md`](./admin-guide.md) for same-host cookie routing requirements.

- JWT verification currently uses a configured RSA public key. Automatic JWKS discovery and rotation should be added after the final identity provider is selected.
- Production merchant entry has a signed, one-time Emisell Backend bridge, but merchant-to-store authorization policy, account recovery, remote JWKS rotation, rate limiting, abuse controls, and production launch review still require operational completion.
- Installation access tokens are intentionally not accepted by developer control-plane routes. The first App Gateway merchant resource is protected, while distributed token validation/introspection for independent extension runtimes remains a separate integration.
- Managed Emisell credentials have a separate default-off connection API with operator provisioning, encrypted installation–extension binding, hashed runtime tokens, per-resolve scope/lifecycle checks, rotation, revocation and committed access audits. This is not a payment/shipping execution adapter or general third-party secret vault. A runtime may retain a fetched secret despite platform revocation, so upstream key revocation and trusted runtime design remain mandatory. See [managed-extensions.md](./managed-extensions.md) for limitations, secure rollout and tests.
- Products have an operator-enabled pilot, disabled by default in both services and constrained to an explicit backend merchant allowlist. Inventory, orders, customers, fulfillment and resource webhooks remain unimplemented. Public scope availability remains `planned`, blocking catalog publication until review. See [`resource-pilot.md`](./resource-pilot.md) for trust keys, query isolation, per-process limits and outstanding deployment/database checks.
- Webhook payload retention is not yet automatically purged. Add a scheduled retention job before production event volume is enabled.
- Invitation delivery is currently manual. Use a verified secure channel; add a transactional email provider only with secret-managed credentials, delivery auditing, and no code leakage into URLs or analytics.

## Release gate

Before production, perform threat modeling and penetration testing for tenant isolation, OAuth mix-up/code replay, PKCE downgrade, CSRF, SSRF/DNS rebinding, webhook replay, signature timing, secret leakage, injection, rate limiting, denial of service, and backup restoration. Do not treat passing unit/integration tests as a substitute for this review.
