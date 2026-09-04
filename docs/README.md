# Emisell App Platform specifications

Start with **Admin Console → Documentation** (`/admin/docs`). The normal reading surface now exposes only two canonical integration contracts: **Internal Emisell Gateway** for Backend Emisell ↔ App Platform and **Partner API** for an approved app backend. Dashboard CRUD, identity tooling, managed runtime details, pilots, and planned blueprints remain available only under the collapsed **Operator & roadmap references** section. OpenAPI/Postman downloads are generated after server-side operator verification. See [`admin-guide.md`](./admin-guide.md) for the walkthrough, security boundary, and deployment caveats.

The Admin **Gateway integration** view visualizes Backend Emisell → App Platform → API Kurir from generated OpenAPI metadata. It documents the two separate authentication boundaries without exposing service-key values or turning API Kurir internals into Partner API endpoints.

App developers can use **Developer Console → Documentation** (`/docs`) for eight learning chapters and one machine-readable **Partner API** reference. Developer Dashboard CRUD is taught through the UI rather than published as an integration API. Payment and Shipping remain capability chapters reviewed separately after approval; the default-off API Kurir rate bridge is internal and is not exposed as a callable Partner API. See [`developer-documentation.md`](./developer-documentation.md) for navigation, source maintenance, access boundaries and verification. Internal Admin/backend contracts are not included in that portal.

- [`app-extension-model.md`](./app-extension-model.md) separates listing categories, configuration families, capabilities and UI surfaces, including the reviewed live catalog and remaining API Kurir ownership boundary.
- [`extension-runtime-v1.md`](./extension-runtime-v1.md) records the disabled generic engine contract for trusted App Platform → Payment/Shipping provider calls: immutable version binding, short-lived runtime token, idempotency, deadlines and rollout gates.
- [`shipping-rate-bridge.md`](./shipping-rate-bridge.md) documents the implemented, default-off internal App Platform → API Kurir calculation bridge, extension isolation, server-only configuration, errors and safe rollout.
- [`Shipping rate provider example`](../examples/shipping-rate-provider/README.md) implements an isolated synthetic receiver matching the hosted rate wire shape. It does not install/activate an app or connect a real courier.
- [`shipping-provider.openapi.json`](./shipping-provider.openapi.json) is a developer-facing reference profile for that external provider example. Arbitrary developer-provider dispatch remains **planned**; generated Postman requests are always skipped.

- [`admin-developer-consoles.md`](./admin-developer-consoles.md) explains the Admin/Developer navigation and authorization split.
- [`integration-handoff.md`](./integration-handoff.md) documents the shared integration inspection, Admin review-before-publication, evidence limits, stale-version protection and isolated verification.
- [`api-guide.md`](./api-guide.md) is the general App Platform API guide.
- [`authentication.md`](./authentication.md) documents browser sessions and identity integration.
- [`app-billing.md`](./app-billing.md) documents Free/Paid plans, explicit merchant subscription consent, prorated first fees, consolidated Emisell invoice line items, payment reconciliation, and the remaining live-integration gates. Default-off; no automatic live billing.
- [`managed-extensions.md`](./managed-extensions.md) documents operator-owned managed credentials, installation–extension runtime isolation, API examples, secure rollout and the distinction from external developer apps. Disabled by default; no payment/shipping execution.

- [`data-model.md`](./data-model.md) defines control-plane ownership, entities, state transitions, RBAC, and security invariants.
- [`openapi.json`](./openapi.json) is the executable OpenAPI 3.1 contract for the first App Platform API surface.
- [`provider-api.md`](./provider-api.md) is the practical, security-first guide for approved third-party app providers.
- [`provider-openapi.json`](./provider-openapi.json) is the Provider API contract; gated Products routes are marked pilot, while unimplemented resource paths are omitted.
- [`emisell-resource-api.md`](./emisell-resource-api.md) defines the internal App Gateway → Emisell Backend product trust boundary.
- [`emisell-resource-openapi.json`](./emisell-resource-openapi.json) records `implemented_gated`: two product routes, disabled by default, not general availability.
- [`resource-backend-blueprint.md`](./resource-backend-blueprint.md) is the backend implementation handoff: Shopify SDK research, Emisell-specific decisions, source-model mappings, scope/isolation rules, and rollout checklist.
- [`emisell-resource-blueprint.openapi.json`](./emisell-resource-blueprint.openapi.json) defines 12 **planned** read expansion operations. It is separate from the running pilot/provider contracts and available through authenticated Admin documentation.
- [`resource-pilot.md`](./resource-pilot.md) explains setup in both repositories, keys, merchant allowlist, tests, errors and rollback. No real merchant database has been modified by the isolated contract tests.
- [`oauth-webhooks.md`](./oauth-webhooks.md) documents safe OAuth and signed webhook integration.
- [`merchant-installation.md`](./merchant-installation.md) documents the sandbox Merchant Install, Consent, and Connected Apps flow.
- [`development-test-installation.md`](./development-test-installation.md) documents merchant-bound pre-release test invitations and the provider OAuth handoff.
- [`Product Reader example`](../examples/product-reader/README.md) is a runnable Go developer app with a disposable two-database lab, browser consent walkthrough, encrypted token storage, and restart/uninstall tests.
- [`emisell-backend-integration.md`](./emisell-backend-integration.md) is the step-by-step guide for App Store catalog, secure Emisell Backend SSO, OAuth installation, and Connected Apps.
- [`developer-onboarding.md`](./developer-onboarding.md) defines the invite-only developer review, invitation, and sandbox activation flow.
- [`security.md`](./security.md) defines secret handling, trust boundaries, deployment controls, and known limitations.
- [`../lib/app-platform/domain.ts`](../lib/app-platform/domain.ts) contains persistence-agnostic domain types.
- [`../lib/app-platform/contracts.ts`](../lib/app-platform/contracts.ts) contains TypeScript request and response contracts used by clients and services.

Run `npm run validate:contracts` whenever an OpenAPI document changes. The validator checks all five contracts for JSON syntax, unique operation IDs, local references, path parameters, schema requirements, and implementation status. Generators also detect drift, including extension catalog examples exported from the Go registry.

Run `npm run test:docs` as well: it checks Go route coverage, caller grouping, generated OpenAPI/Postman references and authentication, and rejection of unauthorized documentation access.

## Intentional boundaries

- Only Internal Emisell Gateway and Partner API are presented as integration entry points. The existence of a dashboard or operational route does not make it a partner contract.
- No live credential or webhook secret is present in the repository.
- Payment and shipping runtimes remain separate services connected through `payment` and `shipping` extension records.
- The generic runtime contract shown in the extension catalog is design metadata with `executionEnabled=false`; it is not an executable App Gateway route. The specialized API Kurir rate pilot is a separate internal, default-off endpoint.
- OAuth access tokens are installation-scoped and intentionally separate from developer control-plane JWTs.
- Merchant-facing App Store, consent, and Connected Apps support sandbox and production identities. Production requires a separately configured Emisell Backend RS256 issuer, same-site HTTPS deployment, and operational security review.
- Developer onboarding is operator-only; organization owners cannot access the internal review queue.

Documentation examples use placeholders or intentionally public local-development values. Production secrets must come from a secret manager and must never be committed.

Docker serves only the raw **Partner API** contract at `http://localhost:8082`, bound to loopback. This separate Swagger viewer has no application login and is development-only; Internal Emisell Gateway and operator/roadmap material remain in authenticated Admin Documentation.
