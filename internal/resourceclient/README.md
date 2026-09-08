# Private product adapter

Ported the bounded product transport from pre-migration commit `9b3fa98`, not the
old gateway's OAuth, storage, login or HTTP routing. Current module:
`emisell.app/platform/internal/resourceclient`.

The exported `Products.Read` delegates authorization to the current
`Lifecycle.WithResourceAccess`. Release/installation locks remain held through
the upstream read. No exported API accepts an arbitrary grant or pre-authorized
merchant tuple. Environment, origin and RSA signing key are server configuration.
Each call issues a fresh 45-second RS256 assertion for the existing api-service
resource contract. It disables proxies and redirects, limits timeout/body size,
validates response shape and permits plaintext only on sandbox loopback.

Reads now use **`GET /v1/products`** and **`GET /v1/products/:id`**, the existing
Emisell URLs, with `X-Emisell-App-Access: resource-v1`. The marker selects app
authentication; it does not replace the signature, installation/merchant binding
or scope. API-service keeps seller/public cookie traffic on its original path and
shares tenant-pinned product data access between both callers. App responses remain
limited to this documented projection; URL reuse does not expose Dashboard fields.

A successful response must acknowledge `X-Emisell-App-Access: resource-v1`.
Legacy/guest 200 responses, redirects and malformed acknowledgements are rejected.
There is no automatic fallback to a public or former internal URL. Deploy api-service
first: its old `/internal/app-platform/v1/products` compatibility alias still works
for an old Platform client, sharing the same policy/limits. Then deploy this client.
No production activation or widening of permissions is included in this change.

The matching api-service change is commit `a26dc958` on
`integration-app-platform`; it must be deployed before this adapter is rolled out.

This is the existing private REST product contract, **not** an implementation of
the distinct ProductService protobuf handoff (different projection/filter/cursor
semantics). No public token endpoint, route or production bootstrap is activated.
Do not advertise read_products as Active from the presence of this module alone.

Cross-repository check (from api-service):

    node scripts/test-app-platform-resources.mjs /absolute/path/to/emisell-app-platform

The runner now targets `./internal/resourceclient`, not deleted
`services/app-gateway`. It provisions disposable PostgreSQL and a matching Prisma
client. It tests Go signing → actual Node authentication/router → Prisma queries,
pagination and tenant isolation. This transport test uses a private synthetic
delegation; it does not claim production OAuth or full install-to-webhook coverage.
Current lifecycle consent/revoke tests remain separately in internal/bootstrap.

Remaining integration: trusted executable resource release/app-client source,
production readiness verifier, authenticated public app-access boundary, durable
producer intake/fan-out and worker delivery. No always-true verifier or legacy
authorization is installed as a shortcut. Emisell Kurir remains unchanged.

`bootstrap.ResourceClientSource` now composes a resource assignment source with
the existing signed-release binding verifier and `appclient.WithBoundReady`.
It checks organization/app/version/digest/client identity on every callback;
revoked/expired/unverified clients cannot reach the installation callback.
The caller must respect its lock order; the inner source must not already hold
the signed-client-release lock acquired by this wrapper (repository locks are
not reentrant). Tests cover mismatch, revocation, expiry, missing secrets, lock
lifetime and callback error propagation. Bootstrap does not yet mount this
wrapper: a trusted executable resource assignment source is still required.
