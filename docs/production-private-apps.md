# Production private-app composition

Decision, 9 September 2026: private headless product readers are separate from
reviewed embedded UI and shipping pilots. The user approved production refactoring
and a narrowly scoped API-service installation change. Earlier local-only notes
still apply to those pilots, but not to this private-app lifecycle.

## What changes

- App creation signs a private version and creates encrypted stable credentials
  without requiring `reviewed-ui.json` or a launch signing key.
- An explicit private runtime handles consent, installation, activation, current
  resource grants and uninstall. Only the existing exact `read_products` policy
  is accepted. Current Core owner/developer identity and signed version remain
  authoritative. Merchant consent is never inferred from creation or login.
- No fallback to a fixture registry, managed shipping, reviewed UI or marketplace
  release. No new schema, scope, public token exchange or embedded launch.
- API-service's production adapter accepts this narrow lifecycle, while local
  simulator/embedded/shipping policies remain denied. Legacy explicit disable
  flags still work. URL and Core key are sufficient for the Core connection;
  platform operational signing/encryption secrets remain server-side.

## Operator configuration

Retain `EMISELL_APP_CREDENTIAL_KEY`: 32 random bytes, raw standard Base64 without
padding. This is the persistent encryption key, not the Core API key. Losing it
prevents decryption; never regenerate it on startup or normal deployment.

Set `EMISELL_PRIVATE_APPS_FILE` to an absolute private regular JSON file, mode
0600. Both production Compose files map it to
`/app/.local/production-private-apps.json` in the existing private key volume.
Despite the volume mount name, its schema and keys are production-specific:

```json
{
  "environment": "production",
  "transport": "https",
  "releaseSeed": "<32 random bytes encoded as standard padded Base64>",
  "coreOrigin": "https://<verified-api-service-host>",
  "keyId": "<production-resource-signing-key-id>",
  "privateKeyPem": "<RSA private key PEM, at least 2048 bits>"
}
```

Generate keys once on the intended server using a cryptographic RNG, retain a
private recoverable backup, and never copy development keys. Install the matching
RSA **public** key at API-service's existing resource trust boundary, with its
production environment, cursor secret and explicit merchant enrollment. The
resource transport defaults to HTTPS certificate verification, no redirects or
environment proxy, bounded timeouts and narrowly scoped 45-second assertions.
Changing enrollment is an operator decision, not a wildcard migration.

The user confirmed API-service is internal (`localhost:8000` on its own host).
For an isolated operator-controlled Docker network, explicitly choose
`"transport": "private-network"` and a reachable `http://<internal-core-host>:8000`
origin. This is signed service traffic, **not encrypted transport**; use HTTPS
outside a trusted private network. Every new connection resolves all addresses,
rejects public/mixed/metadata/link-local peers and pins a numeric loopback/private
address. No environment proxies or redirects are allowed. `localhost` inside
the Platform container is not the API-service container; use its verified network
alias. No global AllowHTTP switch or public HTTP bypass was introduced.

Missing, malformed, public-readable or local config fails startup. Production
rejects `reviewed-ui.json`, `resource-runtime.json`, `ui-resource-signing.json`,
engine/simulator/pilot config. Public-network HTTP remains rejected.

## Rollout and recovery

1. Confirm the merchant origin and internal Core network destination; configure seller redirects
   and Dashboard public origin independently of internal Docker URLs.
2. Back up PostgreSQL, source and all existing keys. Provision the production
   private-app keys and matching Core connection/resource trust without printing
   secrets. Existing keys and accounts must not be replaced.
3. Build exact commits and explicitly apply pending migrations 0029–0035. This
   refactor adds no migration. Migration 0032 retires unlinked legacy sessions;
   inspect the earlier release checklist before applying it.
4. Deploy Platform, API-service and Dashboard in that order. Check public docs,
   login/CLI authorization, merchant picker and read-only consent. Do not install
   on a real store merely to verify deployment.
5. Preserve backups and exact prior images. After the new lifecycle is in use,
   prefer forward fixes. Never roll back immutable signed rows, copy local keys,
reset grants or use a binary that does not understand the applied migrations.

The user elected to set Dashboard/environment addresses themselves. No live
environment, service key, container or database was changed by this refactor.
`localhost:3000` is a local testing address, not a production public redirect.

## Evidence and remaining boundaries

The persistent private-app test uses the actual independent authoring and RPC
composition, checks consent before activation, identity/merchant isolation,
credential rotation, revoked owner/uninstall and rejection of historical fixtures.
API-service tests exercise consent→consume→activate→details→uninstall without
reviewed UI and reject extra scopes/foreign identity/embedded responses.

These tests use isolated local data. They do not establish that a particular
server has matching keys, trust/enrollment or a reachable HTTPS resource backend.
The public private-app token exchange and embedded/shipping runtime remain separate
work; installed headless apps do not acquire an invented Open app URL.
