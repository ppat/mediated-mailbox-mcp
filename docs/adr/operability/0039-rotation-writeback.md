# 0039. Rotation write-back is delegated: a credential-holding deployable updates one writable location and holds no secret-store credential

**Status:** Superseded — **Superseded by:**
[ADR-0082](./0082-rotation-writeback-to-the-database.md) ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

A provider may rotate a credential it issued. When that happens, the new value must reach the secret
store, or the next restart resumes from the stale credential and mailbox access is gone, the most
common way systems like this die quietly. The credential files of every deployable that calls a
provider arrive from the store ([ADR-0038](./0038-credentials-as-mounted-files.md)). That path can
carry values back.

## Decision

- **A deployable that receives a rotated credential writes it to a single writable location that
  only the credential-holding deployables may update** — narrowly scoped, its write restriction
  enforced by platform policy the same way the credential files' read restriction is.
- **That location is synced back into the secret store**, and the refreshed value returns to every
  credential-holding deployable's mounted files through the ordinary delivery path. The full loop
  runs store → mounted files → provider rotates → the deployable that received the rotation writes
  the location → the store syncs → the mounted files refresh. How rotations arriving at two
  deployables close together are reconciled is open in
  [ROADMAP.md](../../../ROADMAP.md#open-decisions).
- **No deployable holds a secret-store credential.** The store's access credential lives outside
  every deployable entirely. A credential-holding deployable's write capability is the one location,
  nothing more. A credential removed from the trust anchor.
- **No write-back alerting is built into the application.** A failed sync is visible and alerted by
  standard monitoring, with no application code involved. The one failure invisible there, a
  deployable failing its own write, is loud in that deployable's logs by nature.
- **Accepted exposure, on record:** between rotation and the sync-then-refresh round trip, a
  restart would load the stale credential. The process keeps the new value in memory, so this
  bites only if a restart lands inside that window.

## Alternatives considered

- **A deployable writes directly to the secret store's API** (the superseded mechanism). No case was
  tabled for keeping it. Rejected: it requires each credential-holding deployable to hold a store
  credential and client code, which puts a credential and a dependency inside the trust anchor that
  the chosen loop removes entirely.
- **Credentials in the configuration repository (sealed or encrypted).** Rejected: write-back needs
  a mutable store the credential-holding deployables can write. A repository-mediated loop would put
  a commit-and-reconcile cycle inside an authentication failure window.
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
- Assumptions about other components: values written to the location do reach the store, that path's
  health is monitored, and each credential-holding deployable's write permission can be scoped to
  the single location.
