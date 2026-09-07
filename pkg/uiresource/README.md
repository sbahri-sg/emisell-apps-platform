# UI resource release contract (not enabled)

This separate contract binds validated UI metadata and required `read_products`
permission under a domain-separated Ed25519 signature. A UI-only signature cannot
authorize it. Unsupported, duplicate, optional, order and write scopes fail closed.
Existing `emisell.ui-release/v1` submissions remain identity-only.

This package does not install an app or grant data access. Do not mount it as an
always-ready resource source. Before activation the following must be connected:

Authoring persistence now exists in `internal/app/service/ui_resource_release.go`
and its PostgreSQL repository, with additive migration 0025. The isolated
`TestUIResourceAuthoring` verifies idempotency, owner isolation, scope validation,
review-before-signing, revision conflicts and immutable permissions. It is not
mounted in the live portal and migration 0025 is not applied to the seller DB.

1. Developer submission and admin review/signing persisted with immutable revisions.
2. Verified app-client binding to the outer digest (not the inner UI digest).
3. Seller consent displaying the exact required scope and a new installation/grant
   revision. Existing UI installations must never receive implicit permissions.
4. Current release/client/installation checks at the read boundary, including revoke.
5. Private product adapter and seller UI showing results only after authorization.

No seller data was read to test this package. Unit tests use generated signing
keys and synthetic metadata. Order APIs remain unsupported; this contract must
not be used to advertise them as active.
