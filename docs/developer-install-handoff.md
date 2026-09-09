# Developer installation handoff

Overview's Install app reads the authenticated, ownership-checked
`GET /api/v1/developer/apps/{id}/install-url`. The backend uses the operator's
`EMISELL_SELLER_ORIGIN` (localhost:3000 in development) and stored active version
to navigate to `/auth/stores?app={appId}&version={activeVersion}`.
The portal does not load `/account` or render a merchant picker during install.
Dashboard Emisell reads its own session and existing merchant list. Missing
sessions go to `/auth/login` with a validated local return destination; password
and OTP login preserve the selection request. Selecting a store uses the existing
`/merchants/switch` endpoint before navigating to
`/store/{commonId}/app/grant?app={appId}&version={activeVersion}`.
No secret, merchant assertion, consent or installation mutation is sent by the
portal. Core must recheck the current seller session, store membership and app
management permission; the account list is navigation convenience only.

The new merchant route reuses the existing AppInstall consent lifecycle with a
compact grant-page presentation. Prepare persists only the intent ID in that
same route; reload reads it. Install is explicit and only verified active
installation plus active grant redirects to Installed apps. The old settings
install route is retained for existing links.

Limit: the initial active developer configuration introduced in migration 0033
is not yet an eligible private installation release. Newly created apps can
reach this consent route but cannot yet prepare/install successfully. No fallback
permission list, fabricated consent, or automatic review/assignment is introduced.
Private-release eligibility and the compatible Core adapter are required before
calling this an end-to-end personal-app installation flow.

Local verification (2026-09-09): portal frontend build/typecheck and 93 tests,
eight Dashboard navigation/login/consent tests, and PostgreSQL handoff ownership
tests pass. Portal and Dashboard picker return HTTP 200. Browser-authenticated
selection/consent was not exercised and no user app was installed.
The user selected the main API-service checkout on port 8000. That checkout's
`GET /v1/app-platform/core/connection` returns 404 and the Core install adapter
is not mounted there. The navigation change does not copy/merge integration
branch code into that checkout; that is a separate authorized rollout step.
