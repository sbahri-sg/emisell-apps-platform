# App categories, capabilities and surfaces

## Implemented in this slice

One App remains the installation, version and permission boundary. No new installation model or database migration is introduced.

| Concept | Existing/new representation | What it does not do |
| --- | --- | --- |
| Listing category | Existing `payment`, `shipping`, `erp`, `marketing`, `operations`, `custom` | Does not grant scopes or select an execution engine |
| Configuration family | Existing `payment`, `shipping`, `custom`; UI labels custom as **General apps** | Does not mean a callable runtime exists |
| Capability | Reviewed registry ID, status, call direction, existing scopes/endpoints and limitations | Not an OAuth scope, permission grant or free-form dispatch target |
| Surface | Server-only integration available; App Home/Admin/Checkout/Online Store UI planned | Does not create a renderer or session bridge |
| Distribution | Existing app distribution; Apps table now labels this **Distribution** | Not a category or extension family |

Reviews, Loyalty, Accounting and similar names are use-case examples, not newly activated APIs. Multiple extensions may belong to one App. Do not infer a single exclusive app engine from its primary listing category.

`GET /v1/extension-catalog` serves public, non-tenant metadata, like scope/event catalogs. It contains no merchant data, connections, keys or runtime URLs. Source: `internal/application/extension_catalog.go` and typed domain structures. `validExtensionType` uses the same family registry. New entries require code review; this is not an operator-editable execution registry.

The same response now carries `runtimeContract`, a machine-readable **v1 draft** for platform-to-provider calls. It intentionally reuses this catalog instead of adding another public endpoint. The draft records short-lived token binding, required headers, operation IDs, deadlines, idempotency and no-retry policy. `executionEnabled=false`; metadata availability does not make a provider callable. See [`extension-runtime-v1.md`](./extension-runtime-v1.md).

The dashboard renders the live catalog in **App → Extensions** and **Documentation → App types & extensions**, with category/capability/surface views and family filtering. It removes the previous static “Available capabilities” claims for payment execution, labels and fulfillment. Add Extension loads its allowed family choices from the backend. Failure to load the catalog does not invent choices or availability.

`executionEnabled` denotes general availability, not a tenant decision. Pilot and planned entries are false. Every real invocation still needs the appropriate verified token, active installation/version, scope policy and merchant consent. Capability IDs and API Kurir's `rates:read` are rejected as App Platform OAuth scopes.

`merchant.profile.read` uses the existing Provider API. `products.read` stays a default-off pilot. Shipping quote, shipment, tracking and payment create/refund are **planned**. Configuration/version storage remains backward compatible; no existing credentials, installations, categories or snapshots are rewritten.

The draft Payment operations are initialize/process/cancel/refund. The draft Shipping operations are calculate rates/create shipment/read tracking. Rate calculation is optional and delegates resolution to API Kurir: local rate card and exact snapshot/cache first, provider quote only on miss/stale. It does not force a RajaOngkir hit. These operations map to existing planned capabilities and do not create routes in the App Gateway.

## Shipping contract inspection

Reviewed against local API Kurir source on 2026-09-03:

- `internal/providers/hosted/rates.go`: actual outbound `/rates` payload and `data.quotes` decoder.
- `internal/providers/hosted/client.go`: `key` authentication, execution mode forwarding, default 8-second transport timeout, response size boundary.
- `artifacts/rajaongkir-hosted-v1.0.10/openapi.yaml`: hosted request/quote schemas and separate shipping-cost versus shipping-delivery operations.
- `internal/httpapi/rates.go`: merchant-facing calculate API is a different boundary.
- `docs/merchant-shipping-providers.md`: credentials and activation currently owned by API Kurir; at most one effective active shipping provider per merchant.

Important mismatches that must not be hidden:

1. The generic partner starter's response example uses `data: []`. The running hosted rate consumer uses `data.quotes`. Our reference follows the consumer, not that starter example.
2. The hosted consumer currently ignores quote `currency`, while the calculate response declares IDR. Our example/reference is therefore narrowed to IDR; this is not evidence of upstream currency validation. A future adapter must reject unknown/non-IDR amounts before publication.
3. Hosted district IDs are provider mappings, not arbitrary Emisell location IDs. Dimensions are not in this rate request; do not copy fulfillment quote fields into shipping-cost requests.
4. Existing API Kurir mode can default to live. The new local example requires explicit sandbox and rejects live, missing mode, browser credentials and real route input. This is example safety, not a new App Platform merchant environment selector.
5. A rate quote is not a booking guarantee. Do not synthesize quote-lock IDs, shipment references or expiry dates absent from this contract.

`docs/shipping-provider.openapi.json` describes the narrowed reference. It is separate from executable App Gateway paths and delivered through authenticated Admin/Developer documentation. Planned Postman exports always skip requests. `examples/shipping-rate-provider` is a loopback-only fixture with no dependencies, database or upstream calls. It is **not** an approved API Kurir connector package.

## Live integration gate: credential and activation ownership

The desired long-term model is central control in App Platform. The current API Kurir implementation has its own credential resolution, active-provider choice, quota handling, location mapping and connector release approval. App Platform currently has managed credential resolution but no shipping operation dispatcher.

Do not provision duplicate credentials, mirror active flags without a reconciliation contract, or silently bypass API Kurir's policy. Before implementing live dispatch, agree and test:

- Which component is authoritative for credentials and activation during and after transition; how existing merchant records migrate and roll back.
- How an authenticated Emisell business request binds merchant, installed app/version and extension; which component authorizes that operation and owns business validation.
- How scopes/approval/consent and suspension/uninstall stop new invocations without changing immutable versions.
- Who maps locations, selects the approved connector release, enforces trusted destinations, handles quota/timeout, and validates returned monetary values and services.
- How provider keys are isolated and rotated without giving external developers Emisell service keys or managed runtime tokens.

No changes were made to API Kurir, Payment Gateway, their credentials or merchant state in this slice. No live shipping integration is claimed.

## Verification and maintenance

```sh
npm run sync:extension-catalog
npm run sync:shipping-contract
npm run validate:contracts
npm run test:docs
npm run example:shipping:test
npm run backend:check
npm run typecheck
npm run lint
npm run build
```

The catalog generator exports fresh reviewed Go data to both OpenAPI documents; it does not query a database or service. Backend tests assert no implicit grants and unchanged family semantics. Example HTTP tests read the exact request/response fixtures from the shipping OpenAPI and verify successful decoding plus rejection paths. These are local contract tests, not deployment, certification or real-merchant tests.
