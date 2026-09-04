# Emisell App Platform — Core data model

## Scope

This model defines the App Platform control plane. Payment Gateway and Shipping Gateway remain independent runtimes and connect through versioned extensions. The control plane owns configuration, permissions, credentials, releases, installations, and audit history; it does not process payments or shipping labels.

## Core relationships

```text
Organization
├── Membership ── User
│                 ├── UserIdentity
│                 ├── IdentitySession ── optional active Organization or explicit Merchant binding
│                 └── MerchantIdentity (user + store + environment binding)
├── OrganizationEntitlement
└── App
    ├── AppCatalogListing (operator publication decision)
    ├── AppVersion ── immutable AppVersionSnapshot
    ├── AppExtension
    ├── AppScope
	├── AppCredential ── OAuthAuthorization ── OAuthAccessToken
	├── WebhookSubscription ── WebhookEvent ── WebhookDelivery
    └── Installation ── Merchant
```

The platform organization also owns `DeveloperApplication` records. Approval creates the candidate organization, its sandbox entitlement, and a hashed `DeveloperInvitation`; acceptance creates the candidate's owner membership.

All tenant-owned records carry an `organizationId` directly or inherit it through `appId`. Repository queries must always be organization-scoped.

## Entity ownership

| Entity                  | Purpose                                           | Important constraints                                                            |
| ----------------------- | ------------------------------------------------- | -------------------------------------------------------------------------------- |
| Organization            | Tenant boundary for developers and apps           | Slug is globally unique                                                          |
| Membership              | User role inside an organization                  | One membership per organization/user pair                                        |
| UserIdentity            | Trusted OIDC provider subject linked to a user    | Provider + subject is unique                                                     |
| IdentitySession         | Revocable browser session and active tenant       | Token/CSRF digests only; absolute + idle expiry                                  |
| MerchantIdentity        | Merchant identity bound to user/store/environment | Composite binding supports multiple stores and actors; not developer authority     |
| MerchantSessionGrant    | One-time Emisell Backend to browser bridge        | Code digest and source JWT `jti` unique; short expiry; single use                    |
| AppCatalogListing       | Operator-controlled App Store visibility          | Separate from app status; draft/published/hidden with revision and audit             |
| App                     | Developer-facing application record               | Slug is unique inside an organization; archive instead of hard delete            |
| AppVersion              | Immutable release snapshot                        | Semantic version is unique per app; exactly one active version per active app    |
| AppExtension            | Mutable extension configuration                   | Runtime URL is HTTPS; included in releases through snapshots                     |
| AppScope                | Current requested permissions                     | Scope name is unique per app; access is required or optional                     |
| AppCredential           | OAuth-style client credential metadata            | Raw secrets are returned once, then only encrypted bytes and fingerprints remain |
| WebhookSubscription     | Event-to-endpoint mapping                         | Event + endpoint must be unique per app                                          |
| WebhookDelivery         | Individual delivery attempt                       | Append-only operational record with retention policy                             |
| OAuthAuthorization      | Short-lived merchant consent grant                | Code stored as digest, exact redirect, PKCE S256, single use                     |
| OAuthAccessToken        | Installation-scoped access grant                  | Token stored as digest, bounded expiration, revocable                            |
| Installation            | App installation at a merchant                    | One active installation per app/merchant/environment tuple                       |
| AuditEvent              | Security and configuration history                | Append-only and organization-scoped                                              |
| DeveloperApplication    | Emisell-managed candidate review                  | Platform-operator only; optimistic revision and explicit state machine           |
| DeveloperInvitation     | Identity-bound owner invitation                   | Raw code returned once; digest persisted; one pending per application            |
| OrganizationEntitlement | Sandbox/production access boundary                | Sandbox enabled on approval; production disabled by default                      |

## State transitions

### App

```text
draft → active → archived
  └────────────→ archived
archived → active (explicit restore only)
```

An app can become active after its first version is released. Archiving does not uninstall it from merchants.

### Developer application

```text
submitted → under_review → invited → active
     └───────────────→ rejected
                       approved ← revoked invite
                          └────→ invited
```

Organization roles never authorize these review transitions. They require the independently signed platform-operator claim. Invitation acceptance additionally requires the signed identity email to match the invited email.

### Version

```text
draft → active
active → released
released → active  (rollback)
```

- A draft is mutable until release.
- Release creates or verifies the immutable snapshot, marks the previous active version as released, and makes the selected version active in one transaction.
- Rollback activates an existing released snapshot; it never edits or copies the snapshot.
- Released and active snapshots cannot be mutated.

### Extension working configuration

```text
draft → active    (included in a released version)
active → draft    (configuration edited)
draft/active → disabled
```

Disabled extensions are excluded from newly created version snapshots. Existing immutable versions retain the extension configuration and can still be reactivated through rollback.

### Installation

```text
active ⇄ suspended
  └────→ uninstalled

active@version A ── owner/admin approval ──→ active@version B
```

Installation requires an active immutable app version and records its ID, granted scopes, environment, and merchant identity snapshot. A release does not auto-upgrade existing installations. Manual upgrade uses the installed version ID plus revision as concurrency guards and atomically updates the version pin, safe scope grant set, active OAuth token grants, idempotency record, and audit event. Uninstalling records `uninstalledAt`; historical audit records remain and the same merchant can install again later.

### Credential and webhook

```text
credential: active → revoked
              └────→ expired  (derived from expiresAt)

webhook: active ⇄ paused
          └────→ disabled  (soft delete)
```

Credential secrets can be replaced only through rotation. Webhook signing secrets are unique per subscription. Both use a versioned encryption key so future key rotation can be introduced without changing the public contract.

### OAuth authorization

```text
approved → consumed
    └────→ expired
```

The active version supplies the exact redirect URL and registered scopes. Successful code exchange creates the installation and hashed access-token record in the same transaction. Plaintext codes and tokens are returned once and are never recoverable from PostgreSQL. Runtime authentication resolves an access-token digest through its credential, app, organization, installation, and entitlement chain; effective scopes are the fail-closed intersection of token and installation grants. The `read_merchant` resource remains a projection of the installation snapshot. A merchant browser session explicitly stores its selected merchant and environment binding; the same user can safely hold sessions for multiple stores and one store can be managed by multiple users. This binding authorizes consent and Connected Apps ownership but never broadens the installation snapshot.

### App catalog listing

```text
draft → published ⇄ hidden
```

Publication is an Emisell operator decision, not a derived app status. The write path rechecks organization, app, active immutable version, HTTPS launch URL, and scope availability eligibility under row locks. App versions containing a planned or unregistered provider scope cannot be published. Public reads repeat those checks so a suspended organization, archived app, invalidated active version, or unavailable scope disappears without relying on a later cleanup job.

### Webhook delivery

```text
pending → delivered
   └────→ failed → pending retry ... → delivered/final failed
```

Each retry is a separate immutable attempt. Claims are leased so another worker can recover abandoned work. Payloads are internal-only; dashboard/API delivery history exposes only safe operational metadata.

`WebhookEventDefinition` is the persisted registry used by API validation, Developer Console, and provider documentation. `available` events may be subscribed; `planned` events reserve a truthful contract without pretending that an Emisell Backend producer exists. Installation-bound events persist the opaque Merchant ID and installation ID as delivery context. The first available lifecycle producer is `app/uninstalled`, queued in the same transaction that marks the installation uninstalled and revokes its access tokens.

## Version snapshot

Each release captures:

- extension type, public configuration, and runtime reference;
- required and optional scopes;
- webhook event subscriptions and endpoints;
- OAuth redirect URLs;
- a deterministic configuration hash.

Runtime secrets are referenced, never embedded in snapshots.

## Security rules

1. App Platform-owned IDs are opaque UUIDv7-compatible strings. External Emisell subject and Merchant IDs are opaque stable strings such as CUIDs and must never be parsed, guessed, or treated as credentials.
2. All timestamps use UTC ISO 8601.
3. Credential and webhook signing secrets are returned once, encrypted at rest, and represented later by fingerprints.
4. Mutating API calls require an idempotency key where duplicate execution could create or rotate a resource.
5. Updates use a `revision` number for optimistic concurrency; stale updates return `409 conflict`.
6. Release, rollback, installation upgrade, credential rotation, role changes, and archive actions always produce audit events.
7. API authorization checks organization membership before resolving the resource, preventing cross-tenant existence leaks.
8. OAuth redirect URIs use exact matching and authorization requires state plus PKCE S256.
9. Webhook subscriptions must reference an available persisted event definition; delivery refuses restricted network ranges after DNS resolution, follows no redirects, and signs the raw body with HMAC-SHA256.
10. Developer invitation codes have 256 bits of randomness, are stored only as SHA-256 digests, expire within a bounded TTL, and are bound to the signed identity email.
11. Developer approval grants sandbox access only; production enablement is a separate future review.
12. Browser sessions store only SHA-256 token digests, require CSRF validation for unsafe requests, and never accept the active organization from browser storage.
13. Organization switching verifies an active membership in the same transaction that updates the session.
14. Installation tokens are distinct from developer control-plane identity, resolve only active and entitled installations, and authorize only the effective scopes persisted for that installation.
15. Merchant consent identity is derived from a separate server-side session bound to user, store, and environment; consent bodies cannot select merchant, developer organization, role, or environment.
16. Merchant uninstall first resolves installation ownership by session merchant ID and environment, then uses the transactional audited uninstall path.
17. App scopes must exist in the Emisell scope registry; a syntactically valid but unregistered scope is rejected.
17. Emisell Backend service JWTs use a dedicated issuer/audience/key, last at most five minutes, require unique `jti` plus `apps.install`, and can create only short-lived hashed one-time session grants.
18. Only operator-published eligible apps appear in the public catalog; credentials, internal extension configuration, secrets, and webhooks are excluded from catalog responses.

## Roles

| Capability                     | Owner | Admin | Developer | Analyst |
| ------------------------------ | :---: | :---: | :-------: | :-----: |
| View apps and analytics        |   ✓   |   ✓   |     ✓     |    ✓    |
| Edit app configuration         |   ✓   |   ✓   |     ✓     |    —    |
| Create draft versions          |   ✓   |   ✓   |     ✓     |    —    |
| Release or roll back           |   ✓   |   ✓   |     —     |    —    |
| Manage credentials             |   ✓   |   ✓   |     —     |    —    |
| Manage team and roles          |   ✓   |   ✓   |     —     |    —    |
| Organization security settings |   ✓   |   —   |     —     |    —    |

## API conventions

- Base path: `/v1`
- Authentication: bearer access token for API clients, an organization-bound browser session for consoles, or a separate merchant-bound session for merchant routes
- Tenant selection: signed bearer claim + matching `X-Organization-Id`, or the membership-verified active organization stored in the browser session
- Pagination: opaque cursor with a maximum page size of 100
- Mutations: `Idempotency-Key` header on create, release, rollback, and rotation operations
- Errors: stable machine-readable `code`, human-readable `message`, `requestId`, and optional `details`
- Concurrency: clients send the last observed `revision` when updating mutable resources

## Physical persistence

The PostgreSQL schema is defined by the ordered migrations in `services/app-gateway/internal/database/migrations`. Apps, immutable versions, extensions, scopes, credentials, webhook subscriptions, catalog listings, merchant identity/session-grant bindings, installations, idempotency records, and audit events are implemented through the PostgreSQL repository. Release, rollback, catalog publication, and installation upgrade use transactions plus row locks, and the database independently enforces one active version per app.

See `docs/database.md` for the migration workflow, tables, and enforced invariants.

## App subscription billing extension

Migration `000017_app_billing` adds `app_plans`, `merchant_app_billing`, `app_billing_quotes`, `app_subscriptions`, `app_subscription_charges`, `app_billing_invoices`, and `app_billing_events`. Pricing, merchant consent, installation lifecycle, paid-feature entitlement and Emisell invoice settlement are distinct concerns. No existing app/installation is automatically subscribed. See [app-billing.md](./app-billing.md) for schema constraints, ownership, concurrency and rollout limits.
