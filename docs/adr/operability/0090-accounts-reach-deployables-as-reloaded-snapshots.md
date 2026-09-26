# 0090. A deployable takes its accounts and their credentials as an account snapshot, loaded at start and reloaded on a schedule

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account), [O6](../../../USE_CASES.md#o6--deployable)

## Context

Accounts and their credentials live in the database and are created and repaired through the UI
while the deployables that call a provider are running
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)). Each of those
deployables serves every account in one process
([ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md)). A running
deployable has to see an account connected after it started, and a credential the operator
replaced. The UI cannot restart a deployable, because the application does not query its platform
([ADR-0051](../engineering/0051-environment-contract.md)), and connecting an account is a guided
flow rather than a set of manual steps. Policy already reaches running processes this way, as an
immutable snapshot that a process reloads and that a unit of work takes once
([ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md)).

## Decision

- **Each deployable that calls a provider holds its accounts, the installation's OAuth client for
  each of their providers that has one, and the opened credentials as one immutable account
  snapshot.** An account whose provider authenticates without an OAuth client loads without one.
  The shared library [`accountload/`](../../../accountload/README.md) builds the snapshot. It
  loads the snapshot at start. A process that runs until stopped, as the mediator does, reloads it
  on a schedule its configuration sets
  ([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)). A workload
  that runs and exits, as backfill does, takes it once at start. Delta sync does one or the other,
  as its form is built ([ADR-0022](./0022-four-workloads.md)), and either way it loads the snapshot
  within each five-minute run.
- **A unit of work takes the snapshot once** and never reads it again mid-flight.
- **A reload whose read fails keeps the previous snapshot and is logged.** A read that succeeds and
  lists no account serves none, so a fault that empties the table fails closed rather than keeping
  stale accounts.
- **An account that appears in a reload is served from that reload on, and one that disappears is
  dropped.**
- **A credential the provider refuses is read again from its row before the refusal is reported**,
  so a re-authorization reaches the deployable at the next call rather than the next reload.
- **A reload keeps a rotated credential whose write-back failed**
  ([ADR-0082](./0082-rotation-writeback-to-the-database.md)). When the stored bytes are still the
  ones the process last knew, the value it holds in memory is newer and stays. When they differ,
  someone else replaced the value, and the stored one wins
  ([ADR-0089](./0089-sealed-values-written-by-compare-and-set.md)).

## Alternatives considered

- **Read at start, restart to pick up.** For it, the simplest code. Against it, the UI cannot
  trigger the restart, so every connection needs a manual step.
- **LISTEN and NOTIFY as the way changes arrive.** For it, changes arrive at once. Against it, a
  transaction-mode connection pooler, which [ADR-0066](../data/0066-data-access-generated-from-sql.md)
  allows, does not support LISTEN, so the notice would be lost silently behind one. It can still be
  added later as a wake-up on top of the scheduled reload.
- **Read the account's rows on every unit of work.** For it, always current. Against it, a database
  read of every account on every call, where the scheduled reload bounds the delay and the re-read
  on refusal covers the case that matters.

## Consequences

- A new account starts being served within one reload interval, or at the next run of a workload
  that runs and exits, and a re-authorized credential at its next refused call.
- The policy loader and the mediator's set of served accounts take their account set from each
  snapshot, so they are built for a set that changes.
- Assumptions about other components. The database is reachable when a reload runs, and a reload
  that cannot reach it leaves the process on its previous snapshot.
