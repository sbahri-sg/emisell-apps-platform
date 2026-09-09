# Release readiness — 9 September 2026

The merchant developer-login, CLI SSO, developer portal, contact/credentials,
and private-product application changes are committed for coordinated release.
Do not infer production runtime readiness from local installation tests.

## Verified

- Dashboard merchant integration: focused login/consent tests pass; additional
  proxy and shipping navigation tests pass in the merged deployment checkout.
- API-service: 210 Apps Platform unit/contract tests pass in a checkout based
  on the latest `origin/merge/develop`; existing resource routes are preserved.
- Apps Platform: dashboard tests, typecheck and build pass. Go bootstrap tests
  pass excluding `TestManagedShippingAssignments`, which needs a separate
  isolated API-Kurir engine executable/database. `go vet ./...` passes.
- Full Go testing initially found two outdated rejection fixtures using the
  now-supported `read_orders`; those fixtures now test unsupported `write_products`.
- Dashboard-wide lint still reports existing findings in admin overview,
  navigation and the older visual QA script; no claim of a clean global lint run.

## Production blockers found before cutover

1. `cmd/server/main.go` requires a persistent application-credential encryption
   key at startup. Existing server deployment does not yet supply it. Generate
   a production key securely once, back it up, and retain it across deployments.
2. Private app creation requires a resource signing key, but `readUIResourceKey`
   accepts only a development configuration. The current bootstrap passes this
   key through the opt-in reviewed-UI composition, not a production authoring
   composition. Do not bypass the production guard or copy local signing files.
3. Private product install/access requires the reviewed resource runtime and
   current Core resource authorization. `reviewed-ui.json` and
   `resource-runtime.json` explicitly reject production. Configure and test a
   production trust/transport composition separately before claiming installs work.
4. The running API-service has no Apps Platform URL/key configuration. The
   trusted seller origin, matching production Core credential, and Dashboard
   public origin must be configured on the server. Local demo credentials are
   not a production replacement.
5. Back up PostgreSQL and existing signing keys before migrations 0029–0035.
   Migration 0032 retires unlinked legacy developer sessions, and 0033 creates
   active-version snapshots only for untouched, never-submitted drafts.

## Release ordering after resolving blockers

1. Verify production configuration and backup recovery; build exact committed images.
2. Apply migrations explicitly, then deploy Apps Platform backend and portal.
3. Deploy API-service from `merge/develop`, preserving current unrelated changes.
4. Deploy merchant Dashboard from `develop`, with matching public/backend origins.
5. Verify public documentation, login redirect, CLI authorization, merchant
   selection, and consent read paths. Do not install an app for a real merchant
   merely to test deployment.

Prepared integration branches remain separate from deployment branches. No
production migration, credential change, or container cutover was performed
while these blockers were unresolved.
