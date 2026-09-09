# Initial active developer version

The personal-app portal selects the first configuration as **Active** in the
same transaction that creates an app and its credentials. No metadata review
is needed to create this initial active configuration.

`Draft.activeVersion` is a server-owned snapshot containing the document,
source revision and activation time. It is not derived from credential existence,
review approval, a mock label or the latest editable draft. GET/list APIs expose
the same persisted snapshot. PUT keeps it unchanged; retried creates return the
same snapshot and produce no duplicate activation audit.

This changes developer version selection only. It does not publish to the app
store, sign an executable release, install an app, grant scopes, select a shipping
provider or change any running merchant installation. Those boundaries stay
independent. The private installation pipeline and explicit activation of later
versions are separate follow-up work; existing release tools remain available.

Migration 0033 adds an optional snapshot and upgrades only revision-1 apps with
no submission history. Reviewed or edited legacy apps are not promoted. Each
upgrade is audited; existing signed bytes, reviews and credentials are unchanged.
An immutable-column trigger prevents draft updates replacing the initial
snapshot. Later version activation requires a version-history/pointer migration,
not dropping this protection or updating the old snapshot in place.

Rollout: back up PostgreSQL, apply migration, start the new backend, then refresh
the portal. Older readers can ignore the additive response field. Retain the
schema/snapshots and forward-fix rather than deleting version history. Do not
run production migrations implicitly.
