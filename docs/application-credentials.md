# Stable application credentials

Application creation now provisions a stable `eai_` client and encrypted `ecs_` secret in the same PostgreSQL transaction as the draft. Normal GET/list/create responses never contain the secret. The owner developer uses explicit, Origin-checked, audited POST reveal; rotation checks the expected version and an idempotency key. Rotation invalidates the old secret immediately, with no change to the client ID, releases or store grants.

This is application identity, not OAuth token issuance or resource access. Existing release-bound `eac_` clients, proof gates, authentication, signed release snapshots and installation bindings remain unchanged. Do not substitute `eai_` into a legacy resource/embedded adapter that still requires a release-bound client. `/api/v1/app/client-check` accepts either identity format and continues returning no OAuth, installation, or resource privileges.

## Upgrade

1. Back up the database and configure a persistent 32-byte, raw-standard-base64 `EMISELL_APP_CREDENTIAL_KEY` through the server secret manager. It is used only by Apps Platform, never Dashboard or API-service. Do not regenerate it on deploy. Losing it makes stored secrets unrecoverable; changing it requires an explicit encryption-key migration.
2. Apply the additive migration: `go run ./cmd/cli migrate`.
3. Run `go run ./cmd/cli init-app-credentials` to provision existing drafts without altering existing app identities or release clients. In development only, this command can create `.local/application-credentials.json` with owner-only permissions when no environment key exists. Production requires the environment key.
4. Start the updated server, then refresh App settings. New drafts provision credentials atomically; no GET or page mount creates credentials.

Metadata: `GET /api/v1/developer/apps/{id}/credentials`. Reveal: `POST .../credentials/reveal` with `{ "version": 1 }`. Rotate: `POST .../credentials/rotate` with the expected version and `Idempotency-Key`. Metadata and secret responses use `Cache-Control: no-store`. API ownership checks use the authenticated developer organization, never a browser-supplied organization ID. Administrator sessions cannot reveal developer secrets through these endpoints.

The UI masks by default, clears plaintext after 60 seconds/on hidden tab/on unmount, and discards late responses. Clipboard writes occur only on explicit Copy actions. The clipboard is controlled by the user/device and is not automatically cleared.

Not included: full OAuth authorization-code flow, replacing installed legacy release-client bindings, cloud event transports, or deployment to production.
