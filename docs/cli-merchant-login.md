# CLI merchant login and store handoff — 9 September 2026

## Decision

CLI 0.5 replaces developer password login with the existing merchant SSO identity boundary. This intentionally supersedes the old CLI password guidance. It does not replace application OAuth, export browser cookies, or grant resources. Existing account IDs/apps/installations and browser logins remain intact.

The supplied `/Users/sbahri/Downloads/example-shopify-app-main` is reference-only and was not modified or executed. Its React Router app uses server authentication/session storage, while `shopify app dev` belongs to the CLI and its configuration binds client/application URL and scopes. Emisell adopts that separation, not Shopify tokens, GraphQL mutations or the sample's product/metaobject write scopes.

## Protocol

- CLI starts through `/api/v1/developer-login/cli/start`, retaining the random verifier in memory only. The browser receives only the request ID in the existing Core `/auth/developer` URL. Start/poll require the exact configured portal Host and Origin.
- Core authenticates its own merchant session, uses its server-only key to approve the existing SSO request, and returns the existing `/finish` URL. A CLI-kind request is redirected to a confirmation form rather than using browser-login cookies.
- The form validates the Core-issued code and exact Origin. A deliberate click confirms CLI login, not merchant consent. HTML escapes profile data, disables framing, and uses no-store/no-referrer. No session is returned to this browser page.
- Poll requires the terminal's independent verifier, matching origin, unexpired CLI-kind request, Core assertion and browser confirmation. Atomic single-use consumption reuses current enabled-account checks, stable Core subject linking and login audit. Browser-kind requests cannot be polled and CLI-kind requests cannot use browser Finish.
- CLI stores its own session in a private 0700 directory/0600 file outside the project. No password or verifier is saved. Each explicit authenticated CLI command verifies the current session and records activity; no background dev heartbeat renews it. Expired/revoked sessions require login again. CLI logout revokes only the CLI session; merchant browser logout remains separate.
- Five-minute login expiry, bounded polling, no automatic mutation retries. If the final response is lost, login again; do not resurrect a consumed request. Shared existing login rate limiting applies.

## Installation and development

`stores list` displays the last merchant-login profile, not a current authorization decision. `app config link` and `app install` select an owned app with an active version and a profile store. Store selection is remembered per session account/origin/project outside source. Backend-provided install URLs must match configured seller origin and the selected app/version. No URL from project metadata is used for consent.

Dashboard `/auth/stores` receives a `store` preference only. It highlights matching authorized list entries. The merchant confirms a store and the existing Core switch endpoint establishes the actual merchant session before navigating to consent. The switch response, never the hint, determines the consent destination. If the store is absent, refresh/login/search; CLI never treats cached membership as authority. Choosing a store is not Install consent.

`app dev --connect` opens that handoff then starts the existing local preview; it does not wait for/claim installation completion or create a tunnel, UI launch binding, grant, secret or production release. Offline `app dev` remains compatible. `apps init` now produces exactly `read_products` for a headless private app; UI scaffold creation and app registration are distinct.

## Rollout

Back up the local database, apply additive migration 0035, deploy/restart the matching Apps Platform backend, then use CLI 0.5. Core SSO URL contract is unchanged. Dashboard preference support is additive; an older Dashboard still allows manual store selection. CLI 0.4 password login cannot authenticate merchant-only accounts: upgrade the CLI, never restore the old password path as fallback.

This task does not publish npm, deploy production, run the Shopify sample, migrate its database, install apps on a merchant's behalf, or add new Core product endpoints. Keep the migration if rolling back; pending CLI requests expire after five minutes. Existing browser requests default to kind `browser`.

## Verification

CLI mock/HTTP tests cover login aliases, verifier/session redaction, private storage, idle renewal, revoked sessions, owned app/store selection, stale choices, hostile redirect rejection, no install mutations, local preview compatibility and default documents. PostgreSQL/HTTP contract tests cover actual CLI browser confirmation, SSO proof/origin binding, expiry, disabled owners, single-use replay and browser/CLI separation. Dashboard tests cover preferred store propagation through login and ensure authenticated switch output controls the destination.
