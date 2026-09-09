# Private product apps — local increment, 9 September 2026

The developer portal now creates `private-products/v1` applications, separate from the historical shipping/public-review pipeline. This follows the user's decision to develop apps for their own stores first. It does not change historical shipping documents, signatures, installations or marketplace policy.

## Implemented contract

- Initial configuration and stable `eai_` client identity are created atomically with a signed private product version. Migration 0034 is additive; it does not backfill older apps.
- Required resource access is exactly `read_products`. No fixture capability scopes, optional permissions, owner/staff data, order access, product writes, custom data, webhooks or executable URL are accepted.
- The signature uses an explicit configured resource key with domain separation `emisell.private-products/v1`. Signing is integrity/authenticity, not a claim of marketplace review or code scanning.
- The private source verifies the immutable signed snapshot, current linked Core actor, enabled developer account, current organization membership and stable client binding. It does not authorize from the cached stores profile. Core must freshly check merchant membership and app-management permission on every request.
- Consent, consume, activation, revoke and receipts reuse the installation lifecycle. The `resource-app/v1` install contract remains backward-compatible. Required scope snapshots cannot be expanded from request input. Creating an app does not create a merchant assignment, consent, installation or grant.
- Only the app owner can install/use this private app through current Core authorization. A different staff actor cannot use the developer's private app merely because they can manage apps in the same store.
- Confidential `eai_` credentials are accepted only by the authenticated Core resource delegation handler and still require the matching current installation/grant. Invalid legacy clients never fall back to private authentication. Reads reuse existing Core product endpoints; this does not create a new product API.
- These initial apps are headless. No iframe/external URL is invented, Open app remains unavailable, and no embedded identity token or legacy installation token is issued. A UI-bearing release and general app OAuth flow remain separate work.

## Rollout and recovery

Back up the local database before applying migration 0034, then start the matching backend before the frontend default changes. The current local composition needs configured resource signing, runtime and app credential encryption. Missing configuration denies creation/installation. This increment is not a production deployment.

Create a fresh product app for testing; do not rewrite active shipping versions. Working draft edits still cannot replace the initial active snapshot. Later version publication/upgrades need a versioned transition and renewed consent where permissions change.

Rollback the frontend default before rolling back the backend; retain migration/data and private app records. Old binaries must not be used for private installs. Existing shipping and reviewed-UI records are unchanged. Uninstall remains available if the owner is disabled or the source stops authorizing.

## Verification

`TestPrivateProductCreateConsentAndIsolation` uses the isolated `emisell_local_test` database to cover app/credential creation, retry, foreign ownership, consent, activation, exact read-only grants, cross-store access, disabling the owner, immutable versions and uninstall. `TestPrivateProductScopeAndSignature` rejects write/staff/order permissions and modified signed identities. Existing reviewed-resource lifecycle tests remain enabled. No user store is installed during testing.

The isolated test also exercises the existing HTTP resource delegation with a mock Core product endpoint: incorrect secrets and order reads fail before reaching Core; product reads succeed; credential rotation invalidates the previous secret.

Local verification: targeted private-product and existing reviewed-resource lifecycle tests passed, portal tests passed 93/93, TypeScript and the portal build passed. The wider backend suite still reports existing failures in the password-based developer CLI contract, isolated shipping runner setup, and two older resource-scope expectation tests. Selected-file lint reports the existing synchronous state update in the navigation effect (`components/portal.tsx`); this increment does not change that effect. These are not reported as a clean full-suite run.

Local rollout applied migration 0034 after a private database backup, restarted the matching Apps Platform backend, and enabled the existing resource/reviewed-UI contract in the local API-service configuration. No production deployment was performed. A fresh `Product Reader Test` app was created for the same verified linked owner as the historical test app; its consent preparation returned `read_products` and pending consent. No consent, installation or grant was created for the merchant. Use the fresh app's Install action, not the historical shipping app URL.
