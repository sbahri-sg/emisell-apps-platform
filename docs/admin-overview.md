# Admin overview

`GET /api/v1/admin/overview` requires an authenticated admin-portal session.
Developer and merchant sessions cannot access this endpoint. The response contains
aggregate counts only, without customer data, tokens, or credentials.

- `publishedApps`: distinct applications currently published in the catalog.
- `developers`: enabled developer portal accounts (not organizations).
- `activeInstallations`: installations currently in the active state.
- `pendingReviews`: all submitted reviews, independent of the review list limit.
- `history`: 30 daily UTC installation-event counts, including zero days and reinstalls.
- `webhookPending`, `webhookDead`: current delivery queue counts, not worker uptime.
- `portalSessions`: unexpired portal sessions belonging to enabled accounts; not embedded app sessions.
- `checkedAt`: snapshot completion time.

The page requests this snapshot on mount and manual refresh only. Failed refreshes
retain the previous snapshot with an error and timestamp; they do not display zero
or claim healthy services. Queries have a five-second timeout. No schema changes
or external provider requests are needed. API/database connectivity reflects only
the successful snapshot request, not historical availability.
