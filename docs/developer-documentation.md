# Developer documentation portal

The Developer Console has a **Documentation** entry at `/docs`. It teaches the provider journey rather than presenting internal operator configuration as developer APIs. The existing `/admin/docs` remains the private operator/integration portal.

## Chapters

1. **Getting started** — invitation, workspace, app configuration and active version.
2. **Authentication & installation** — merchant-bound development request, OAuth state/PKCE, installation tokens and uninstall.
3. **Working with merchant data** — live scope catalog, consent, Products pilot semantics and errors.
4. **Webhooks** — live event catalog, signatures, durable event deduplication, test events and retries.
5. **App types & extensions** — categories versus configuration families, live capability catalog, and unsupported App Home/Admin/Checkout/Online Store UI surfaces.
6. **Build a shipping rate provider** — API Kurir hosted wire shape, synthetic HTTP example, the separate default-off internal rate pilot, and unresolved external-provider live gates.
7. **Testing & release** — manual acceptance checks, the isolated Product Reader example and the difference between version release and publication.
8. **Partner API reference** — searchable endpoints that the installed app backend may call, including cURL, parameters, schemas and guarded OpenAPI/Postman downloads.

Use `/docs?chapter=webhooks` for a chapter link. Use `/docs?chapter=api-reference&contract=provider&endpoint=getProviderInstallationContext#getProviderInstallationContext` for a specific operation. Invalid chapter IDs have a recovery state; unknown operation IDs have a notice.

## Sources and maintenance

- `lib/documentation/developer-guide.mjs` contains reviewed developer-facing prose, prerequisites, section links, checks and limitations. API links use operation IDs; tests verify them against the actual contracts.
- `app/docs/developer-documentation.tsx` renders the chapters and reference using the existing dashboard layout and design tokens. It does not import raw guide/contract sources or the Admin documentation renderer.
- Scope, webhook and extension catalogs are fetched from gateway endpoints. Loading, errors and empty results are distinct; the UI must not invent an available fallback catalog. The extension catalog is shared with the app Extensions page; schema examples are generated from the Go registry.
- `lib/documentation/contracts.mjs` generates reference/download content from OpenAPI. The developer endpoint passes an explicit audience allowlist to prevent other contract summaries from leaking into the navigation payload.
- Keep pilot/default-off and unsupported features explicit. Do not write runnable examples for target-only Resource API expansion or UI runtimes. Read the current Go handlers and contracts before changing tutorial claims.

The portal only displays examples. It has no credential-entry field or “Try it” execution. Example values are placeholders; live credentials never belong in documentation. Product pilot and billing safeguards remain in exported Postman requests.

## Access boundary

`GET /docs/content` verifies the developer identity with the configured gateway at `/v1/session` **before loading content**:

- A developer cookie takes precedence; the server forwards only that cookie, not Admin/Merchant cookies or a fallback bearer. Duplicate developer cookies are rejected.
- Without a developer cookie, a supplied developer bearer can be verified by the gateway. `X-Organization-Id` is forwarded only as bearer context; the gateway must verify its binding. Forwarded user/role/operator headers are not accepted as identity.
- A valid user, allowed developer role and matching active workspace are required. Expired identity, no active workspace, malformed responses and redirects fail closed.
- Only the `provider` contract, displayed as **Partner API**, is allowed. `developer`, `shipping-provider`, `admin`, `emisell`, `identity`, `resource`, `resource-blueprint` and `managed` requests fail even for an authenticated developer. Developer Dashboard actions remain UI guidance; Payment and Shipping execution contracts are capability-specific runtime material reviewed separately by Emisell.
- Guide, reference and downloads all return private/no-store responses. A developer identity does not grant access to `/admin/docs/content`.

The `/docs` HTML route is a generic shell; its content API is private. The guide and OpenAPI sources are loaded through server-only virtual modules. Vite blocks raw requests to `docs/**` and the guide source, including raw/import file URLs. Do not add these sources to `public/`, client imports or a static host. This does not replace production HTTPS, gateway authentication and deployment review.

## Verification

Run from the platform repository:

```sh
npm run validate:contracts
npm run test:docs
npm run typecheck
npm run lint
npm run build
```

Documentation tests cover chapter links, audience filtering, generated examples, gate preservation, cookie/bearer forwarding, forged identity, missing workspaces, expired sessions, redirects, malformed identity responses and loader failures. They do not prove production deployment or commerce UI runtime readiness.

For an interactive review with an approved developer account, open `/docs`, navigate all chapters, follow an operation link, search the reference and download each format. Verify keyboard navigation and narrow-screen chapter/table scrolling. No provider or merchant mutation is needed to read the portal.
