# Core connection: HTTPS domain or localhost

The Emisell backend may connect directly to an operator-configured HTTPS origin.
No SSH tunnel or caller IP enrollment is required. The existing private loopback
RPC listener remains supported for development. This is not a generic RPC proxy
and does not grant third-party apps unrestricted store access.

## Apps Platform server

Set `EMISELL_CORE_HTTP_ENABLED=true` in the deployment environment, then recreate
the API service and reload the matching reverse proxy configuration. The feature
defaults off. Both supplied Compose deployments pass this flag to the server.
The transport itself needs no new key. Apply current migrations before deployment;
migration `0028_core_merchant_references.sql` adds the reference-registration audit.

The HTTPS domain exposes `/api/v1/core/rpc/<procedure>` for connection checks,
verified merchant registration, test assignments, and the existing consent/installation lifecycle. It dispatches
to the same authenticated handlers, not a second implementation. Payment/shipping
execution, app-token issuance, internal resources, engine grants and metrics are
not exposed through this route. Their integration work remains separate.

The API listener must remain private behind the trusted TLS reverse proxy.
Do not publish port 8088. Nginx's dashboard-only configuration assumes an existing
HTTPS ingress and sets the forwarding protocol itself; do not expose its plain
HTTP upstream as a public alternative. Caddy terminates HTTPS directly. Normal
TLS certificate verification must remain enabled on the backend client.

Every call authenticates an active platform key from storage. Revoked/incorrect
keys, browser cookies/origins, arbitrary procedures, redirects at the client,
oversized bodies, and malformed protocol headers are rejected. Existing merchant
authorization, seller consent, scope checks, installation readiness and audit
remain unchanged. The endpoint has defensive body/concurrency/rate limits; it has
no caller-IP allowlist or commercial usage quota.

## Emisell backend environment

```dotenv
APP_PLATFORM_CORE_PREVIEW_ENABLED=true
APP_PLATFORM_CORE_RPC_URL=https://apps-platform.emisell.com
APP_PLATFORM_CORE_KEY=<active first-party platform key from admin>
APP_PLATFORM_CORE_DASHBOARD_ORIGINS=http://localhost:3000
```

For local Platform development, replace only the RPC URL:

```dotenv
APP_PLATFORM_CORE_RPC_URL=http://127.0.0.1:8088
```

Do not append the RPC prefix to the URL: the updated client selects the correct
path. Remote plain HTTP is rejected to protect the key. Dashboard origins must
be exact trusted seller URLs; no wildcard CORS. In production use HTTPS for both
the RPC URL and seller origins. Production can register merchant references and
read Apps metadata; development installation/profile flags remain prohibited.

Use exactly one key source: inline `APP_PLATFORM_CORE_KEY` OR the legacy
`APP_PLATFORM_CORE_KEY_FILE`. Never put this key in public/frontend environment
variables. Restart the backend after changing its environment, and run a checkout
that actually includes `/v1/app-platform/core/*` (not an older branch).

## Automatic merchant registration

The updated Emisell backend calls `ConnectionService/EnsureMerchant` after checking
the current seller session, active merchant, membership/ownership and Apps
permission in its primary database. Browser merchant/actor overrides are rejected.
Opening Settings → Apps therefore registers a missing reference automatically and
then lists apps; no manual registration or extra environment setting is needed.

The genuine merchant ID remains unchanged. Only that ID and the verified actor ID
are sent. New references receive a neutral display label; existing store labels
are not overwritten. The Platform rechecks the active first-party key inside the
transaction and records who registered a reference. Concurrent retries create at
most one merchant and one creation-audit row. No browser account, membership,
installation, app token, scope approval or resource grant is created.

`Check` alone is still read-only and never registers a merchant. Only first-party
Core may call `EnsureMerchant`; a third-party app credential cannot. The backend
continues to check the seller's current status on every request: an existing
reference never bypasses disabled/suspended/revoked seller access. Registration
failure is reported, not silently replaced by an empty app list. Deploy Platform
plus migration before enabling the updated backend, because old servers do not
implement this operation. Roll back the backend hook too when reverting Platform.

## Verification and rollback

Verify the authenticated connection check succeeds; an invalid key must fail.
Then verify the seller session can list its own installations and test apps.
Confirm the portal still loads, and engine/payment/metrics routes stay blocked.
Never send credentials to a redirect target or use an insecure TLS override.

Rollback: set `EMISELL_CORE_HTTP_ENABLED=false`, recreate only the API service,
and restore/reload the prior proxy config. Existing loopback RPC continues to
work. No merchant, app or credential records are removed by this change.
