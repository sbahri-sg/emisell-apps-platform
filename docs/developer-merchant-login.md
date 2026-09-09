# Developer login with an Emisell merchant account

The developer portal uses the existing Emisell merchant login. Passwords and
merchant cookies never cross into Apps Platform. The merchant backend verifies
the session, active user, suspensions and per-store app-management privileges.
The developer must have at least one eligible store. Store owners qualify;
staff also require active membership and `manageAndInstallAppsAndChannels`.

## Flow

1. The portal creates a five-minute request and an HttpOnly, SameSite=Lax browser
   proof cookie. It redirects to the seller's `/auth/developer` page.
2. Seller verifies the account and eligible stores automatically. If needed, the existing
   merchant login opens in another tab; return and choose **Periksa akun kembali**.
3. With an active eligible merchant session, API-service automatically attests identity through
   `POST /api/v1/developer-login/approve`, using the existing Core platform key.
4. The browser returns with a one-use code. Apps Platform checks the request,
   proof cookie, intended portal origin and expiry before creating its own session.

No app is installed and no resource scopes are granted by this flow. Store links
are navigation only; Emisell continues enforcing store access independently.
The menu contains the verified store snapshot from the latest login. **Perbarui
daftar toko** repeats authentication; this is not a live membership subscription.
The landing page **Login** button opens `/development`. An unauthenticated visit
starts the merchant session check automatically. Failed callbacks show a retry
button rather than looping. Manual logout returns to documentation.

Developer sessions expire after **one hour without activity**, enforced on the
backend. Foreground user input renews a still-valid session through
`POST /api/v1/developer/activity`; ordinary requests, polling, and the read-only
`GET /api/v1/developer/activity` do not extend it. Expired sessions cannot renew.
Idle tabs check the shared deadline before returning to the landing page, so they
do not revoke a session being used in another tab. Merchant sessions are untouched:
returning to `/development` authenticates again if the merchant session is active.
Core-side account revocation is checked on the next merchant login;
instant cross-service session revocation is not implemented here.

## Fresh developer accounts

Manual developer creation, developer password login, and legacy account linking
have been removed. The first verified Core subject creates a fresh developer
profile automatically. Subsequent logins resolve that stable subject, not email.
The verified email is contact information; the private account uses a unique
internal identifier. Matching a retired account's email does not inherit its apps.
Old accounts, applications, and audit history are retained, not destructively
purged. Migration 0032 revokes sessions without a verified Core link.

Organizations remain an internal ownership boundary; developers do not configure
one in the new UI. API contact email is independently saved per app and does not
change account identity, credentials or release drafts.

## Configuration and rollout

- Apps Platform: `EMISELL_SELLER_ORIGIN` is the exact seller Dashboard origin.
  Local default: `http://localhost:3000`. Set explicitly in production.
- API-service: reuse `APP_PLATFORM_CORE_RPC_URL`, `APP_PLATFORM_CORE_KEY` and its
  seller origins (`CLIENT_URL` or `APP_PLATFORM_CORE_DASHBOARD_ORIGINS`).
- Only when the HTTP API origin differs from the RPC origin (local port 8088),
  set `APP_PLATFORM_DEVELOPER_API_URL=http://127.0.0.1:8087`. This uses the same
  Core key, which must be valid in that Apps Platform database.
- Seller Dashboard uses its existing `/api` rewrite and `NEXT_PUBLIC_SERVER_URL`.
  Deploy the `/auth/developer` page and the API-service module together.
- Apply migrations through 0032 before restarting Apps Platform. Existing linked
  developer sessions are capped at one hour; unlinked sessions are retired.
- Keep local and server deployments aligned. A local portal cannot complete a
  request through a backend still targeting a different Apps Platform database.

New authentication adapters do not add commerce resource endpoints. Products,
orders, shipping and other commerce requests continue using existing Emisell APIs.
