# Emisell App Platform

Developer control plane for building, versioning, releasing, and operating apps across the Emisell ecosystem.

## Architecture

```text
Developer Console + Merchant Portal (React + Next App Router conventions + Vinext/Vite)
                         │
                         │ OpenAPI / HTTP
                         ▼
App Gateway (Go)
  ├── OIDC identity and server-side organization sessions
  ├── Curated merchant App Store and isolated merchant sessions
  ├── Emisell Backend signed identity bridge and one-time exchange
  ├── Invite-only developer review and sandbox entitlements
  ├── Apps and immutable versions
  ├── Extensions and scopes
  ├── Credentials, PKCE OAuth, and installation token authentication
  ├── Durable signed webhook delivery and retry
  ├── Installations and RBAC
  └── Audit history
         │
         ├── Payment extension runtime (later)
         └── Shipping extension runtime (later)
```

Payment and shipping are extensions, not responsibilities of the App Gateway control plane.

## Local development

Admin Console login is intentionally separate from Developer login. After the core database exists, apply only its independent schema and create the first account with a hidden password prompt:

```sh
docker compose run --rm app-gateway go run ./cmd/admin-user migrate
docker compose run --rm app-gateway go run ./cmd/admin-user create --email admin@example.com --name "Emisell Admin"
```

The command uses `DEVELOPMENT_ORGANIZATION_ID` locally. Production requires `ADMIN_PLATFORM_ORGANIZATION_ID` or `--organization-id`. See [docs/authentication.md](docs/authentication.md). No Admin account or password is seeded.

### Docker Compose (recommended)

Start the complete local stack with one command:

```sh
cp .env.example .env
npm run docker:up
```

This starts:

- Dashboard at `http://localhost:3003`
- Internal Admin documentation center at `http://localhost:3003/admin/docs`, with operator-verified OpenAPI/Postman downloads
- Go App Gateway at `http://localhost:8081` (container port `8080`)
- Raw Partner API Swagger at `http://localhost:8082` (loopback-only, no application login); internal and roadmap contracts remain in authenticated Admin Documentation
- PostgreSQL at `localhost:5432`

Frontend and Go source changes reload automatically. Follow or stop the stack with:

```sh
npm run docker:logs
npm run docker:down
```

Compose applies migrations before starting App Gateway. Users, sandbox/production merchant identities, curated catalog listings, hashed browser sessions, one-time Emisell session grants, single-use OIDC state, developer applications, hashed invitations, entitlements, apps, versions, extensions, scopes, encrypted credentials, OAuth grants/tokens, webhook delivery, installations, idempotency records, and audit events are persisted in PostgreSQL and survive service restarts. Payment and shipping runtimes are intentionally not part of this stack.

The dashboard reads identity, organization memberships, Developer Requests, Apps, Versions, Extensions, Scopes, Credentials, and Webhooks directly from App Gateway. The local bearer token and organization ID in `.env.example` are used only to bootstrap a development browser session. Production uses provider-neutral OIDC authorization code + PKCE and server-side sessions; provider credentials must be supplied outside the repository.

If the default host ports are occupied, change `FRONTEND_PORT`, `GATEWAY_PORT`, or `POSTGRES_PORT` in `.env`.

### Without Docker

Frontend:

```sh
npm install
npm run dev
```

Migrate and run Go App Gateway:

```sh
cd services/app-gateway
go run ./cmd/migrate
cd ../..
npm run backend:run
```

The frontend runs on `http://localhost:3003` in the current local setup. The Go service runs on `http://localhost:8080`.

## Contracts and validation

For a runnable third-party developer example, see [Product Reader](./examples/product-reader/README.md). Run `npm run example:lab -- /absolute/path/to/api-service` for an isolated local console, merchant consent, Go provider backend, and two disposable PostgreSQL databases. It does not connect to existing merchant data or publish an app. `npm run example:check -- /absolute/path/to/api-service` tests the full HTTP flow, service restart, and uninstall; `npm run example:test` runs the provider unit/security tests.

- `docs/data-model.md` defines entities, state transitions, RBAC, and security invariants.
- `docs/database.md` documents tables, migrations, and persistence invariants.
- `docs/api-guide.md` documents implemented endpoints and development examples.
- `docs/developer-onboarding.md` documents the invite-only developer program and secure operator workflow.
- `docs/authentication.md` documents OIDC, session, CSRF, and organization-switching security boundaries.
- `docs/merchant-installation.md` documents merchant consent, callback, token exchange, and Connected Apps.
- `docs/development-test-installation.md` documents pre-release app testing by Merchant ID without bypassing OAuth consent.
- `docs/emisell-backend-integration.md` is the practical App Store, Emisell Backend SSO, install, and Connected Apps integration guide.
- `docs/admin-developer-consoles.md` documents the separate Admin and Developer Console routes and authorization boundary.
- `docs/integration-handoff.md` explains API-backed app inspection in both consoles, review-before-publication with stale-version protection, evidence limits and remaining integration gates. Run `npm run test:integration-readiness` for isolated PostgreSQL verification.
- `docs/admin-guide.md` explains the integrated operator playbook, caller-specific contracts, secure downloads, and troubleshooting.
- `docs/openapi.json` defines the OpenAPI 3.1 surface.
- `docs/provider-api.md` explains the approved-provider OAuth, tenant, scope, and error-handling contract.
- `docs/provider-openapi.json` contains only the public endpoints that providers can actually call.
- `docs/emisell-resource-api.md` defines the App Gateway → Emisell Backend security and tenant contract.
- `docs/emisell-resource-openapi.json` describes the implemented-but-gated `read_products` pilot; it does not claim general availability.
- `docs/resource-pilot.md` provides configuration for both services, key handling, test-merchant allowlist, verification, and rollback. The integration is disabled by default.
- `lib/app-platform` contains shared TypeScript domain and request/response contracts.
- `services/app-gateway` contains the Go control-plane service.

Run the complete project verification:

```sh
npm run check
```

This validates OpenAPI references, documentation generation/access tests, TypeScript, frontend lint/build, Go vet, and Go tests. Documentation tests also compare every implemented gateway method/path with the contract to catch missing or invented routes.

For the optional cross-repository Products test, run `node scripts/test-app-platform-resources.mjs /absolute/path/to/emisell-app-platform` from the Emisell `api-service` checkout. This provisions and removes its own isolated PostgreSQL database; it does not use existing merchant data or run the backend's normal seed. See `docs/resource-pilot.md` for prerequisites and the test's limitations.

## Current implementation status

- UI foundation and route-based navigation: complete
- Separate Admin Console and Developer Console layouts/navigation: complete
- Invite-only developer intake, review, one-time invitation, and sandbox activation: complete
- Core data model and OpenAPI contract: complete
- Go service foundation: complete
- PostgreSQL migrations and durable control-plane repository including Merchant Installations: complete
- Transactional release/rollback and installation upgrade, idempotency, and audit history: complete
- Frontend Apps/Versions/Extensions/Scopes/Credentials/Webhooks/Installations integration, including version-diff review: complete
- Fail-closed RS256 production JWT verification: complete
- Server-side browser sessions, CSRF protection, and membership-verified organization switching: complete
- Provider-neutral OIDC authorization code + PKCE integration: complete foundation; provider configuration required
- PKCE authorization-code installation flow and owner/admin sandbox simulator: complete
- Merchant-bound development test install requests with seven-day expiry and OAuth/consent enforcement: complete
- Installation access-token authentication, live context resolution, and scope guard: complete
- Scope-protected `read_merchant` profile resource and simulator verification: complete
- Curated merchant App Store, Admin catalog publication, Merchant Install/Consent, Connected Apps, and uninstall: complete
- Dedicated Emisell Backend RS256 identity bridge with hashed one-time merchant-session grants: complete
- Signed webhook queue, worker, bounded retry, and safe delivery log: complete
- Production signing-key provisioning/rotation, same-site deployment, rate limits, and security review: required before launch

## App plans and consolidated billing

The default-off billing foundation adds **Plans & pricing** to app details and **Plan & billing** to Connected Apps. It supports immutable Free/Paid monthly plans, explicit consent, integer prorata, invoice line-item snapshots and idempotent payment reconciliation. No live billing is activated. Read [the billing guide](./docs/app-billing.md) before enabling the module; the `api-service` adapter is not yet wired to the existing renewal/payment entrypoints. Run `npm run test:billing` for an isolated disposable PostgreSQL test without touching the running development database.
