# Emisell App Gateway

Go control-plane service for the Emisell App Platform. It owns app metadata, immutable version snapshots, release state, organization scoping, role enforcement, idempotency, and audit records. Payment and shipping remain external extension runtimes.

## Implemented slice

- `GET /healthz` and `GET /readyz`
- app list, create, read, update, and archive
- version list, create, read, release, and rollback
- extension and scope working-configuration management
- encrypted credential create, rotate, list-metadata, and revoke
- signed webhook create, queue, deliver/retry, observe, update/pause, and soft-delete
- version-pinned merchant install, suspend/resume, list, and uninstall
- five-minute OAuth authorization codes with state, exact redirects, PKCE S256, and atomic token/install exchange
- development bearer authentication with organization and role context
- owner/admin/developer/analyst capability boundaries
- opaque cursor pagination and optimistic app revisions
- durable 24-hour idempotency records for implemented mutations
- serializable, atomic version activation and rollback transactions
- append-only PostgreSQL audit events
- migration runner with advisory locking
- database-aware readiness checks
- UUIDv7 identifiers and structured JSON logging

All resource operations currently documented in OpenAPI are registered by the service.

## Run locally

From the project root, the recommended development workflow is Docker Compose:

```sh
cp .env.example .env
npm run docker:up
npm run docker:logs
```

The service is rebuilt automatically after Go source changes. Compose runs embedded PostgreSQL migrations before starting App Gateway. Interactive OpenAPI documentation is available at `http://localhost:8082`.

To run only the service directly:

```sh
go run ./cmd/migrate
go run ./cmd/server
```

The development API listens on `http://localhost:8080`. API calls require:

```text
Authorization: Bearer emisell-local-dev-token
X-Organization-Id: 01995f72-0000-7000-8000-000000000001
```

Mutating create/release/rollback/archive calls also require an `Idempotency-Key` containing 16–128 characters.

## Architecture

```text
cmd/server
    ↓ composition and lifecycle
internal/httpapi
    ↓ authenticated commands
internal/application
    ↓ repository ports
internal/adapters/postgres
    ↓
internal/domain
```

The PostgreSQL adapter is the default runtime repository. The memory adapter remains available only for isolated unit tests or explicit `REPOSITORY_DRIVER=memory` runs. Version activation, app revision changes, audit insertion, and idempotency storage commit atomically.

See `../../docs/api-guide.md` for endpoint examples and `../../docs/database.md` for the schema and migration workflow.

## Security posture

- Startup fails closed outside `APP_ENV=development` until a production authenticator is supplied.
- The development token is never accepted implicitly; callers must send it.
- Resources are resolved using both organization ID and resource ID.
- Roles are checked before repositories reveal whether a resource exists.
- Unknown JSON fields and request bodies larger than 1 MiB are rejected.
- Tenant relationships, active-version uniqueness, and lifecycle values are constrained by PostgreSQL.
- Credential and webhook secrets use AES-256-GCM, a versioned encryption key, one-time API responses, and non-secret audit metadata.
- OAuth codes and access tokens are stored only as SHA-256 digests; client authentication and PKCE are rechecked at exchange.
- Webhook delivery blocks private/local network destinations after DNS resolution, disables redirects, signs raw payloads, and stores no response bodies.
- Owners/admins manage credentials; developers can manage webhook configuration but cannot read credential metadata.
- Production accepts only configured RS256 JWTs and derives user, organization, and role from signed claims; request override headers are development-only.
