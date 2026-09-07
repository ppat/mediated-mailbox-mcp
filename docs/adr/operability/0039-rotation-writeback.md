# 0039. Rotation write-back is delegated: the mediator updates one writable location and holds no secret-store credential

**Status:** Accepted (supersedes
[ADR-0013](./0013-credentials-and-rotation-writeback.md), jointly with
[ADR-0038](./0038-credentials-as-mounted-files.md)) ·
**Pillar:** [The mediator is the irreducible trust anchor](../../../DESIGN.md#the-mediator-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

A provider may rotate a credential it issued; when that happens, the new value must reach the
secret store — otherwise the next restart resumes from the stale credential and mailbox access
is gone, the most common way systems like this die quietly. The mediator's credential files
arrive from the store ([ADR-0038](./0038-credentials-as-mounted-files.md)); that path can carry
values back.

## Decision

- **The mediator writes a rotated credential to a single writable location it alone may
  update** — narrowly scoped, its write restriction enforced by platform policy the same way the
  credential files' read restriction is.
- **That location is synced back into the secret store**, and the refreshed value returns to the
  mediator's mounted files through the ordinary delivery path. The full loop: store → mounted
  files → provider rotates → mediator writes the location → the store syncs → the mounted files
  refresh.
- **The mediator holds no secret-store credential.** The store's access credential lives outside
  the mediator entirely; the mediator's write capability is the one location, nothing more. A
  credential removed from the trust anchor.
- **No write-back alerting is built into the application.** A failed sync is visible and alerted
  by standard monitoring, with no application code involved. The one failure invisible there —
  the mediator failing its own write — is loud in the mediator's logs by nature.
- **Accepted exposure, on record:** between rotation and the sync-then-refresh round trip, a
  restart would load the stale credential. The process keeps the new value in memory, so this
  bites only if a restart lands inside that window.

## Alternatives considered

- **The mediator writes directly to the secret store's API** (the superseded mechanism). No case
  was tabled for keeping it. Rejected: it requires the mediator to hold a store credential and
  client code — a credential and a dependency inside the trust anchor that the chosen loop
  removes entirely.
- **Credentials in the configuration repository (sealed or encrypted).** Rejected: write-back
  needs a mutable store the mediator can write; a repository-mediated loop would put a
  commit-and-reconcile cycle inside an authentication failure window.
- **No write-back — re-consent manually when rotation happens.** Rejected: the failure is silent
  and delayed (everything works until the next restart), which is precisely the shape of failure
  that gets discovered weeks later, mid-incident.
- **Custom write-back failure alerting inside the application.** Its case: guaranteeing that a
  failed write-back is detected. Displaced: the chosen loop provides that detection for free, as
  the Decision states.

## Consequences

- Rotation write-back stays the deliberately exercised day-one drill: force a rotation, restart,
  confirm access survives — now over this path. The drill, and the deliberate break-the-sync
  injection, are catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- If the path to the store is down, write-back waits; the process keeps the rotated value in
  memory meanwhile. Accepted.
- Assumptions about other components: values written to the location do reach the store; that
  path's health is monitored; and the mediator's write permission can be scoped to the single
  location.
