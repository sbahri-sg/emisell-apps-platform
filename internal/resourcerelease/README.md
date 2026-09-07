# Signed resource assignment source

This package supplies a real PostgreSQL `ManagedReleases`/`ResourceReadiness`
implementation. It is not a public authoring endpoint or an automatic grant.
Migration 0024 is additive, enrolls nobody, and is not applied by server startup.

Each immutable envelope signs the schema discriminator, merchant, environment,
expiry, complete intent release (scope/webhook provenance) and exact confidential
app-client binding with Ed25519. `Digest` hashes the release with its own digest
field cleared. Sign canonical Go JSON of `Envelope`; do not sign PostgreSQL JSONB
text. Use a dedicated trusted signing key; catalog/UI signatures are not accepted.
The trust key must be operator configuration, never submitted alongside a request.

`WithRelease` locks the approved assignment, verifies signature, expiry and indexed
identity, then uses existing `appclient.WithBoundReady` to lock and check the current
client. The lifecycle obtains the installation lock inside that callback.
`ReadyResource` rechecks the exact merchant assignment from authoritative managed
context; an assignment for a different merchant cannot satisfy readiness.
Revocation waits for in-flight callbacks and denies later access. Records cannot
be rewritten or moved from revoked back to approved.

Composition requires assigning this same Source to `Intents.Managed` and
`Lifecycle.Resources` and then updating `Lifecycle.Intents`. Do not wrap it in
ResourceClientSource: this package already holds the signed assignment/client
locks. Do not replace existing UI/shipping sources wholesale; runtime selection
must be explicit and cannot fall back after a resource denial.

The supported slice is required read_products only, no optional scopes. This is
not evidence to mark the universal catalog Active. No default bootstrap, public
OAuth/resource route, release-signing UI or delivery worker is enabled here.
The prerequisite to mounting it remains an audited authoring/signing/assignment
workflow and explicit runtime configuration. No direct database enrollment or
schema change has been performed in a seller/production database.

Verification: `TestPersistedResourceReleaseConsentAndRevocation` uses isolated
PostgreSQL for assignment, app-client and installation/grant state. It checks
consent, activation, allowed read, extra-scope rejection, wrong environment/key,
cross-merchant denial, client/assignment revocation and uninstall. It does not
claim public token issuance or full HTTP/webhook production coverage.
