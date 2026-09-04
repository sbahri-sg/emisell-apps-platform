# Extension Runtime Contract v1

Status: **generic external dispatcher draft, disabled**. One deliberately narrow exception is implemented separately: the default-off internal App Platform → API Kurir adapter for `shipping.rates.calculate`. It does not activate Payment, shipment, tracking, or arbitrary developer-provider traffic.

## Why this contract exists

The App Platform control plane already owns apps, immutable versions, scopes, installations, credentials, webhooks, catalog review and merchant consent. The remaining boundary is a general safe dispatcher from trusted Emisell services to selected Payment or external Shipping runtimes. Rate calculation uses the smaller API Kurir-specific bridge documented in [`shipping-rate-bridge.md`](./shipping-rate-bridge.md).

The design deliberately uses the existing `GET /v1/extension-catalog` response as its discovery source. It does not add another public API family. The generic `runtimeContract` remains `executionEnabled=false`. `shipping.rates.calculate` is reported as a default-off pilot because the internal API Kurir bridge exists; the other Payment/Shipping capabilities remain planned.

## Decisions from the reference systems

- [Shopware App System](https://github.com/shopware/shopware/tree/trunk/src/Core/Framework/App) informs the manifest, lifecycle and typed capability boundary. Emisell keeps one immutable version snapshot and avoids adding a database column for every new capability.
- [Saleor Apps](https://github.com/saleor/apps) informs the external-app model and synchronous Payment command pattern. Emisell remains the source of truth for transaction and order state.
- [GitHub App installation tokens](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app) inform short-lived, installation-bound runtime authorization. A runtime token cannot exceed the installed grant.
- [Nango](https://github.com/NangoHQ/nango) informs per-tenant connection and credential isolation. Provider credentials remain bound to one installation and one extension, encrypted at rest and resolved only by a trusted runtime.
- In-process plugin systems such as [Vendure](https://github.com/vendurehq/vendure) and [Medusa](https://github.com/medusajs/medusa) are suitable only for code operated by Emisell. Third-party provider code is never loaded into the App Gateway process.

## Ownership boundary

| Component | Owns |
| --- | --- |
| Emisell Backend | Merchant, checkout, order, payment, fulfillment and subscription business state |
| App Platform control plane | App, version manifest, scope, installation, extension selection, consent, credentials, webhooks and app billing |
| Extension dispatcher | Policy check, runtime token, trusted destination resolution, deadline, request/response validation and invocation evidence |
| Provider runtime | Provider-specific API call and normalized response; never authoritative Emisell business state |

Payment Gateway and Shipping Gateway become the first runtimes behind this contract. They are not moved into the control-plane process.

## Immutable manifest binding

Every invocation is resolved from an active installation pinned to an immutable `AppVersionSnapshot`. The trusted dispatcher derives:

- merchant ID from the authenticated Emisell request;
- installation ID from the merchant–app binding;
- extension ID and runtime destination from the installed version snapshot;
- version ID from the installation pin;
- environment from the installation;
- operation from the internal business command.

The provider cannot send any of these values to select a different tenant, version, destination or operation. A plain Merchant ID is an identifier, not authentication.

## Generic external-provider invocation flow · not implemented

1. Emisell Backend authenticates its caller and validates the business command.
2. App Platform resolves the active installation, installed version and selected extension.
3. Policy rejects inactive apps, suspended/uninstalled installations, disabled extensions, unsupported operations and unreviewed destinations.
4. App Platform creates a 60-second JWT restricted to one merchant, installation, extension, version, environment and operation.
5. The dispatcher sends one HTTPS JSON request with invocation ID, deadline and the required idempotency key.
6. The provider verifies token signature, audience, expiry, operation and identity binding before work.
7. The dispatcher validates the response schema and records safe invocation evidence. Secrets and sensitive payloads are not stored in ordinary logs.
8. Emisell Backend decides the authoritative transaction/order/fulfillment state transition.

There is no automatic platform retry in v1. A caller may repeat an uncertain mutation only with the same idempotency key after checking the recorded result.

## Canonical wire envelope

The exact payload schema is operation-specific. The outer envelope is stable:

```json
{
  "contractVersion": "v1",
  "invocationId": "0199b4e0-0000-7000-8000-000000000001",
  "operation": "shipping.rates.calculate",
  "context": {
    "merchantId": "merchant_from_trusted_context",
    "installationId": "installation_from_platform",
    "extensionId": "extension_from_installed_version",
    "versionId": "immutable_version_id",
    "environment": "sandbox"
  },
  "payload": {}
}
```

Successful synchronous response:

```json
{
  "contractVersion": "v1",
  "invocationId": "0199b4e0-0000-7000-8000-000000000001",
  "status": "succeeded",
  "providerReference": "provider_reference_without_secret",
  "result": {}
}
```

Rejected response:

```json
{
  "contractVersion": "v1",
  "invocationId": "0199b4e0-0000-7000-8000-000000000001",
  "status": "failed",
  "error": {
    "code": "operation_rejected",
    "message": "Safe developer-facing explanation",
    "retryable": false
  }
}
```

Required headers and per-operation timeout/idempotency rules are machine-readable under `runtimeContract` in the extension catalog. The catalog never contains a runtime URL, credential, merchant record or operational token.

## Payment v1

Initial commands are `payment.session.initialize`, `payment.session.process`, `payment.session.cancel` and `payment.refund.create`. All are synchronous, mutation commands and require idempotency. App Platform supplies the authorized transaction reference and amount; provider output cannot silently change them. Emisell Backend remains the payment-state owner.

Callbacks or provider webhooks are a separate authenticated ingress contract. A synchronous timeout must be treated as an unknown result until reconciled, not an automatic failure or reason to create a second charge.

## Shipping v1

- `shipping.rates.calculate`: optional synchronous command with a five-second deadline. API Kurir first checks an active local rate card, then an exact snapshot/cache. Only a miss/stale result can trigger its internal provider quote, quota ledger and fallback policy. The command itself does not require an idempotency key and never means “force one RajaOngkir hit.” An empty result means no supported service, never a zero-price service.
- `shipping.shipments.create`: synchronous command, 15-second deadline and required idempotency. Booking is separate from a rate quote.
- `shipping.tracking.read`: synchronous read, eight-second deadline. Tracking update webhooks will require their own event contract before activation.

`shipping.rates.calculate` is not mandatory for installation. Tracking-only, shipment-only or rate-card-only extensions can omit it. Dynamic provider quote remains an internal API Kurir operation rather than an App Platform capability.

Location mapping, money/currency validation, service allowlists, request coalescing, daily quota and provider release selection remain enforced by API Kurir. The existing API Kurir ownership must be migrated through an explicit reconciliation plan; credentials and active flags must not be duplicated silently.

## Implementation gates

The draft can move to pilot only after all of these exist:

1. dispatcher service with strict outbound destination policy, response-size limit and circuit breaker;
2. runtime JWT issuer/verifier with rotation, audience restriction and 60-second maximum lifetime;
3. operation-specific request/response schemas and contract tests;
4. idempotency result store and unknown-result reconciliation;
5. encrypted installation–extension credential resolution with revocation tests;
6. audit metrics without credential, token, payment detail or customer-address leakage;
7. sandbox test harness for success, rejection, timeout, malformed response, replay, cross-tenant access, suspend and uninstall;
8. Admin approval and merchant re-consent when a released version adds permissions or capabilities.

Until those gates pass, the UI must show the generic runtime as **Draft · execution off**, no Payment/shipment/tracking or arbitrary external-provider operation may be invoked, and saving a runtime URL cannot be described as a successful connection. The specialized rate bridge is configured by operators with a fixed API Kurir origin and follows the narrower controls in [`shipping-rate-bridge.md`](./shipping-rate-bridge.md); it never consumes a runtime URL from extension configuration.
