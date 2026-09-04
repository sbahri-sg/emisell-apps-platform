# App Platform → API Kurir shipping rate bridge

Status: **implemented, sandbox-ready, disabled by default**. The bridge is an internal Emisell Backend integration. It is not a public Partner API, does not call a developer-supplied URL, and does not make `shipping.rates.calculate` an OAuth scope.

The authenticated Admin UI renders this contract as an end-to-end gateway map at `/admin/docs?contract=emisell&view=gateway`. Its sequence, hops, ownership and sandbox checklist come from `x-emisell-gateway-integrations` in the generated App Platform OpenAPI rather than a separate manually maintained endpoint list.

## Purpose and boundary

Emisell Backend can ask App Platform to calculate rates for the authenticated merchant through:

```http
POST /v1/integrations/emisell/shipping/rates/calculate
Authorization: Bearer <short-lived Emisell Backend assertion>
Content-Type: application/json

{
  "origin": "442",
  "destination": "1354",
  "weight": 1200
}
```

Merchant ID and environment come only from the verified Emisell Backend assertion. The request deliberately has no `merchantId`, `courier`, `provider`, `credentialId`, `installationId`, `extensionId`, or runtime URL. Unknown fields are rejected.

The assertion must include permission `shipping.rates.calculate`. This permission is an internal service permission, not a merchant-consent scope and not an installation token permission.

## Selection and isolation

Before contacting API Kurir, App Platform resolves the merchant's installed app state and requires exactly one eligible shipping extension:

1. installation is active and belongs to the authenticated merchant and environment;
2. app is active;
3. organization has access to the environment;
4. installation is pinned to an immutable version snapshot;
5. one shipping extension in that snapshot declares:

```json
{
  "capabilities": ["shipping.rates.calculate"]
}
```

No eligible extension returns `409 shipping_extension_unavailable`. More than one returns `409 shipping_extension_ambiguous`; App Platform never chooses one arbitrarily. Suspending or uninstalling the app stops future calculations before any upstream request.

## API Kurir call

The private adapter calls the existing API Kurir Main Service contract:

```http
POST /api/v1/calculate/district/domestic-cost
Content-Type: application/x-www-form-urlencoded
key: <server-only API Kurir service key>
X-Emisell-Merchant-ID: <authenticated merchant>
X-Emisell-Execution-Mode: sandbox
X-Request-ID: <correlation id>

origin=442&destination=1354&weight=1200&include_group=true
```

App Platform does not send `courier` or select a provider credential. API Kurir remains the owner of merchant service configuration, provider choice, credential, location rules, request coalescing, quota ledger, active rate card and exact snapshot/cache. Only API Kurir may decide that a miss or stale result needs an optional provider quote. Therefore one App Platform calculation does not mean one RajaOngkir hit.

The adapter has a maximum five-second deadline, disables proxy use and redirects, limits the response to 1 MiB, validates the normalized response, rejects duplicate services, and never forwards private upstream error messages.

## Response

```json
{
  "meta": {
    "message": "Success",
    "code": 200,
    "status": "success"
  },
  "data": [
    {
      "name": "J&T Express",
      "code": "jnt",
      "logo": "https://assets.example.invalid/jnt.svg",
      "service": "EZ",
      "canonicalService": "regular",
      "serviceGroup": "regular",
      "serviceType": "regular",
      "description": "Regular service",
      "cost": 18000,
      "etd": "2-3 day"
    }
  ]
}
```

An empty `data` array means no supported service. It never means zero-price or free shipping.

## Safe errors

| HTTP | Code | Meaning |
| --- | --- | --- |
| 400 | `invalid_merchant_context`, `invalid_rate_request` | Signed context or input is invalid |
| 401/403 | `unauthorized`, `shipping_rate_forbidden` | Assertion invalid or permission absent |
| 409 | `shipping_extension_unavailable`, `shipping_extension_ambiguous`, `shipping_disabled` | Extension/install or merchant shipping state cannot be used |
| 422 | `rate_not_available` | No eligible rate |
| 429 | `shipping_rate_limited` | Temporary limit; honor bounded `Retry-After` |
| 502 | `shipping_provider_unavailable` | API Kurir/provider authentication or upstream failed |
| 503 | `shipping_runtime_disabled`, `shipping_runtime_unavailable`, `provider_quota_exhausted` | Bridge off/unavailable or daily provider quota exhausted |

There is no automatic retry. A failure is never converted to a fake empty success.

## Configuration

The App Gateway process uses server-only variables:

```text
API_KURIR_RATES_ENABLED=false
API_KURIR_BASE_URL=
API_KURIR_SERVICE_KEY=
API_KURIR_RATES_TIMEOUT=5s
```

The bridge remains off unless explicitly enabled with a valid origin and service key. Plain HTTP is accepted only by the development process; production requires HTTPS. Never expose the key through `NEXT_PUBLIC_*`, frontend code, generated documentation, screenshots, or logs.

The generated Admin Postman request also skips execution unless a private environment sets `enable_api_kurir_rate_bridge=true`. That switch only removes the documentation guard; it does not enable the backend feature.

## Verification and rollout

The automated tests use local `httptest` servers and synthetic merchants. They verify strict request projection, headers, error mapping, malformed responses, installation/version isolation, ambiguity, suspension, permissions, and unknown-field rejection. They do not contact API Kurir, RajaOngkir, a real merchant database, or consume provider quota.

Before live activation:

1. make `api-service` mint the dedicated permission only after authenticating the checkout/merchant context;
2. provision the API Kurir origin and service key through the deployment secret manager;
3. configure one sandbox merchant with exactly one eligible installed shipping extension;
4. run a bounded sandbox test against API Kurir and confirm cache/rate-card and quota metrics;
5. validate disable, suspend, uninstall, timeout, malformed-response and quota-exhaustion behavior;
6. review egress allowlisting, secret rotation, monitoring and production entitlement;
7. enable production separately—sandbox success does not authorize live traffic.

Shipment creation, labels, pickup, tracking, payment execution, arbitrary external provider dispatch, and a public developer shipping runtime remain outside this slice.
