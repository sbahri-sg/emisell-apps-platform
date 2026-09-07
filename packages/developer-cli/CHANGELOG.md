# 0.2.0 — release candidate (not published)

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
included. Published npm version remains 0.1.1; publishing requires approval.

Validation: CLI tests, local Core contract test and npm pack dry run. No .local
credentials, tests, or deployment configuration are included in the package.
