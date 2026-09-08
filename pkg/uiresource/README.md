# Reviewed UI resource release contract

This contract binds validated UI metadata and a nonempty, sorted, unique subset of
`read_catalogs`, `read_collections`, `read_inventory`, `read_locations`, `read_orders`, `read_products`, `read_shipping` under a domain-separated Ed25519
signature. A UI-only signature cannot authorize it. Unsupported, duplicate,
optional and write scopes fail closed.
Existing `emisell.ui-release/v1` submissions remain identity-only.

This package alone does not install an app or grant data access. The reviewed
local runtime connects the checks below; it is not an always-ready resource source:

Authoring persistence now exists in `internal/app/service/ui_resource_release.go`
and its PostgreSQL repository, with additive migration 0025. The isolated
`TestUIResourceAuthoring` verifies idempotency, owner isolation, scope validation,
review-before-signing, revision conflicts and immutable permissions. It is not
evidence of a production deployment or a migration applied to the seller DB.

1. Developer submission and admin review/signing persisted with immutable revisions.
2. Verified app-client binding to the outer digest (not the inner UI digest).
3. Seller consent displaying the exact required scope and a new installation/grant
   revision. Existing UI installations must never receive implicit permissions.
4. Current release/client/installation checks at the read boundary, including revoke.
5. Existing Emisell product/order/shipping/catalog/collection/location endpoints and UI showing results only
   after authorization. The adapter chooses exactly one scope per operation.

No seller data was read to test this package. Unit tests use generated signing
keys and synthetic metadata; isolated lifecycle tests cover all seven permissions.
Inventory uses existing product paths with the explicit `view=inventory` selector;
no selector retains `read_products`, never an automatic scope-based fallback.
Catalog/collection declarations use the Emisell-owned `emisell-reviewed-read/v1`
profile, not the pinned external reference. Old profile/digest bindings are retained.
Existing installations never gain added scopes implicitly. Public production
activation and live acceptance testing are separate rollout steps.
