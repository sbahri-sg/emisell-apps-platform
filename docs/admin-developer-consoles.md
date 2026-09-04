# Admin Console and Developer Console

Emisell App Platform uses one trusted identity system but exposes two intentionally separate product surfaces.

Merchant App Store, Install, and Connected Apps form a third, narrower surface.
`/merchant/app-store`, `/install`, and `/merchant/apps` use an independent merchant session and never inherit
Developer Console or Admin Console authority. See
[`merchant-installation.md`](./merchant-installation.md).

## Developer Console

The Developer Console starts at `/overview` and is scoped to the active organization membership stored in the server-side session.

| Route       | Purpose                                                                      |
| ----------- | ---------------------------------------------------------------------------- |
| `/overview` | Organization-scoped app and installation summary                             |
| `/apps`     | Custom apps, versions, extensions, scopes, credentials, and webhooks          |
| `/apps/<app-slug>/integration` | Read-only configuration inspection and integration handoff; no end-to-end success claim |
| `/stores`   | Installations belonging to the organization's apps                           |

The previous `/api-access`, `/team`, and `/settings` placeholders redirect to `/overview`. App-level API access remains available because it is backed by persisted credentials and scopes. Analytics stays hidden until a real telemetry pipeline is available.

Developer roles (`owner`, `admin`, `developer`, and `analyst`) grant capabilities only inside the active organization. Developer navigation never exposes internal review operations.

## Admin Console

The Admin Console starts at `/admin` and represents Emisell's internal control plane.

| Route                       | Purpose                                                                          |
| --------------------------- | -------------------------------------------------------------------------------- |
| `/admin`                    | Internal program metrics and recent developer requests                           |
| `/admin/developer-requests` | Candidate intake, review, approval, rejection, and one-time invitations          |
| `/admin/organizations`      | Read-only organization inventory backed by entitlement, app, and membership data |
| `/admin/app-catalog`        | Review app eligibility and explicitly draft, publish, feature, or hide listings   |
| `/admin/docs`               | Operator guide, caller-specific API reference, authentication, troubleshooting, guarded downloads |

The previous `/admin/access`, `/admin/audit`, and `/admin/settings` policy-summary pages redirect to `/admin`. They will return only when they have dedicated operator workflows and persisted APIs.

App catalog now includes **Review app**, sharing the configuration/evidence inspection with Developer Console. Publication carries the inspected app revision and active version; stale decisions return 409. Catalog pagination and refresh are API-backed. See [integration handoff](./integration-handoff.md) for the evidence limitations and remaining workflow gaps.

The old `/developer-requests` route redirects to `/admin/developer-requests` so existing bookmarks do not silently enter the Developer Console.

## Authorization boundary

The `platform_operator` identity claim is independent from organization membership and role. An organization owner is not an Emisell platform operator.

Frontend guards and hidden navigation are usability controls only. The security boundary remains the Go App Gateway:

- every `/v1/internal/developer-*` operation requires `platform_operator=true`;
- `GET /v1/internal/organizations` and its detail route require `platform_operator=true` and return only organizations with developer entitlements;
- catalog candidate and publication-decision routes require `platform_operator=true`; public catalog routes return only eligible published snapshots;
- tenant app operations continue to require the active organization's role capabilities;
- organization selection is loaded from a verified server-side session membership;
- new apps are custom/invite-only; public distribution is rejected by the API;
- credential and installation environments are checked against the organization's persisted entitlements;
- production access remains separate from developer admission and sandbox activation.

An authorized operator can switch between consoles from the account menu. A normal developer receives no Admin Console link and sees an access-restricted state if they navigate directly to `/admin`.

The Organizations surface is deliberately read-only in this phase. It supports search and status filtering, then exposes the persisted organization status, sandbox/production entitlement flags, current non-archived app count, configured app/webhook limits, and accepted memberships. Suspend/reactivate, entitlement mutation, operator management, and production approval are not part of this API.

The documentation content/download endpoint also verifies operator identity through the gateway on the frontend server before loading contracts. See [`admin-guide.md`](./admin-guide.md) for this boundary, cookie-routing requirements, and the separately served development Swagger viewer.
