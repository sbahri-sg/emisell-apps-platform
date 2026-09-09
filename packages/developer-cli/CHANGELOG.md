# 0.5.0 — 2026-09-09

- Merchant browser SSO with explicit CLI confirmation, proof-bound five-minute requests and one-hour idle sessions. Requires Apps Platform migration 0035/server support; password login removed.
- `stores list`, `auth status`, `app config link`, `app install`, and opt-in `app dev --connect`; remembered account/project navigation, current Core store switch and browser consent. No automatic install or cookie import.
- `apps init` defaults to a private `read_products` app document; preserves legacy shipping/review commands and existing projects.
- CLI login remains distinct from app runtime OAuth, credentials and grants. No tunnel, App URL deployment, production release or npm publication performed by these commands.

# 0.4.0 — 2026-09-08

- Add reviewed `read_inventory` / `read_locations` demo pages using existing product URLs with `view=inventory` and existing location URLs. Preserve active-location balances, exclude addresses/phones, and require explicit signed scopes and seller consent. No production activation.
- Replace the HTML embedded/products generators with a single React Router + TypeScript + Vite app template.
- Include connection and read-only product pages, search, cursor pagination, hot reload, typecheck and build scripts.
- Add app build, multi-stage non-root Docker packaging, isolated UI Compose preview, explicit production web runtime, health/readiness and bounded shutdown. Production store access remains fail-closed until a real backend adapter is supplied; no live server is activated.
- Preserve backend identity/current-access checks, loopback/Origin boundaries and confidential-client handling. No production adapter, seller consent, deployment or npm publication is implied.
- Remove legacy template choices; existing projects are never deleted or overwritten. Use CLI 0.3.1 explicitly for legacy projects.

# 0.3.1

- Focus public documentation on Emisell features and usage.
- No command, runtime behavior, or merchant permission changes.

# 0.3.0

- Interactive `app init` for name, new directory, bundled template and trusted seller origin; explicit flags for CI.
- `app dev` discovers the local project from cwd/subdirectories; `--path` and legacy `--dir` are supported.
- `app info` projects safe local metadata; `app doctor` validates config/assets without network, secrets or backend execution. Neither claims merchant access.
- Command-specific help, `--name`/`-n`, `--path`/`-p`, `--json`/`-j`, and `auth login/logout` aliases.
- Existing commands and backend paths remain compatible; occupied ports are reported without stopping another process.
- No browser OAuth, tunnel, server registration, production release or automatic seller consent.

# 0.2.0

- Embedded starter with pinned UI Kit and bridge.
- Explicit local HTTPS origin and endpoint ownership challenge support.
- Backend identity verification and local Core identity adapter.
- UI release create/list/show and testing assignment request commands.
- UI creation submits for review; it does not grant business resource access.
- Product-reader template (`app init --template products`) with server-side name/SKU search, cursor pagination, empty/error/loading states and no polling.
- Explicit local product backend; private client-secret file, live authorization on every read, bounded responses and no redirects.
- Testing requests accept `--release-kind ui_resource` (default remains `ui`).
- Generated guide covers review, client/launch verification, assignment, seller consent, install, product reads and stopping testing.

The local resource runtime supports consent/install/read_products after the
required approvals. CLI commands cannot sign, approve or consent for a seller.
No production OAuth resource adapter, order/write access or webhook worker is
included. Server deployment is separate; installing this package does not enable
resource routes or production access.

Validation: CLI tests, local Core contract test and npm pack dry run. No .local
credentials, tests, or deployment configuration are included in the package.
