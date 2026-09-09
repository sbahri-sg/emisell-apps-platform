# Personal developer workspace

The developer entry point is `/development`. Its top-level navigation is Apps,
Stores and Catalogs. Application-specific navigation remains Overview, Monitoring,
Logs, Versions and App settings. The public documentation entry point and admin
portal remain separate.

## Implemented

- Stores uses the existing authenticated developer `/account` response. Only
  merchant references asserted by the Emisell backend at merchant login are shown.
  Refreshing store access repeats that verification; opening a store is a normal
  seller navigation, not an installation or a permission grant.
- Catalogs is explicitly a **mock** requested for the initial interface. The
  reference-matched Configuration and Search preview panels expose source, query,
  region, attributes, listing, API-key placeholder and JSON/cURL/JS inspection.
  Search and request/response inspection run on synthetic fixtures. No catalog
  requests are sent. The displayed contract uses the existing `GET /v1/catalogs`
  merchant API (`page`, `limit`, `sort`, `order`, `search`; `data`, `meta`, `message`).
  Unsupported visual filters remain in a mock configuration envelope, not API
  parameters. Rename/delete actions affect the local example only; restoring the
  example or reloading recovers it. Image search is visibly unavailable.
- Admin review links are absent from the developer's top-level navigation. Legacy
  review/release workflows and backend policies have not been bypassed or deleted.

## Remaining backend work

The target is private applications installed in the developer's own authorized
stores, without public marketplace listing review. This UI change does **not**
enable that new installation policy. Existing release and assignment verification
still applies. A dedicated private distribution policy must validate current
merchant access and app ownership, validate the immutable version and runtime,
and hand off to seller consent before activating installation scopes. It must not
mark private apps as publicly listed or auto-grant scopes through developer SSO.

Replace Catalogs fixtures only through the existing merchant-authorized backend
endpoint. A developer session establishes identity; it is not a general-purpose
merchant resource token. Do not expose Core credentials or copy merchant cookies
into the portal.
