# Shipping rate provider — local reference fixture

A small Go receiver showing the hosted `/rates` shape consumed by API Kurir. **Synthetic data only; not a courier engine, approved connector package, App Platform dispatcher or booking service.** No database, outbound HTTP, provider credential or merchant record is used.

## Test without running another stack

From `emisell-app-platform`:

```sh
npm run example:shipping:test
```

Requires Go >=1.24. Tests create an ephemeral local HTTP server, send the request example from `docs/shipping-provider.openapi.json`, compare the response and decode it like API Kurir's hosted consumer. They also check invalid keys, live/missing mode, browser requests, tenant overrides, input types, oversized bodies, real locations and non-fixture weight. They never connect to API Kurir or a real courier.

## Optional local server

Set `SHIPPING_EXAMPLE_KEY` to a newly generated random value of at least 32 characters using a private terminal environment or secret tooling. Do not reuse a merchant/provider/Emisell service credential, commit it or paste it into documentation. The example has no default key and never prints it.

Then, from this directory:

```sh
go run .
```

The server binds only to `127.0.0.1:3016`. Stop it with Ctrl+C. Do not expose it through a public proxy or register its URL as a live provider.

Send a server-side `POST /rates`, using `Content-Type: application/json`, the private example key in the `key` header and `X-Emisell-Execution-Mode: sandbox`. Live/missing mode, Cookie and Origin are rejected. This mode is an example guard, not an App Platform installation setting.

```json
{
  "origin": { "district_id": "fixture-origin" },
  "destination": { "district_id": "fixture-destination" },
  "weight_grams": 1000,
  "courier_codes": ["fixture"],
  "service_groups": ["regular"],
  "price": "lowest"
}
```

Only this synthetic route, weight and courier are implemented. An omitted courier filter is allowed. An accepted service-group filter excluding regular returns an empty quotes array, **not a free shipping rate**. Other real locations/couriers are rejected.

Success contains `data.quotes` with an explicitly synthetic 10000 IDR quote, service `FIXTURE_REG`, and `meta.fixture=true`. The amount is fixture data, not a pricing formula. No quote lock, booking ID or guaranteed delivery date is created.

## Trust boundary

- The reference follows `api-kurir/internal/providers/hosted/rates.go`, not the older generic starter's `data: []` example.
- In an actual connector, district IDs are provider-mapped location IDs. API Kurir's merchant calculate API has a different payload and authorization boundary.
- API Kurir currently ignores hosted quote currency on ingestion; this reference profile is restricted to IDR. Production integration needs explicit response validation.
- API Kurir's `rates:read`, this catalog's optional `shipping.rates.calculate`, the connector's provider quote, and App Platform OAuth scopes are different concepts. None grants the others automatically.
- App Platform live dispatch and credential/activation transition are not implemented here. See [`app-extension-model.md`](../../docs/app-extension-model.md).
- Developer docs/OpenAPI/Postman are readable after developer authentication. Shipping reference Postman requests always skip execution. Use the local test command to exercise this fixture.
