# 0090. A deployable takes its accounts as an account snapshot and their credentials into its connections, loaded at start and reloaded on a schedule

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

- **Each deployable that calls a provider holds its accounts, each paired with the OAuth client it
  connects through where its provider has one, as one immutable account snapshot, the list in
  force, and their opened credentials in its connections.** An account whose provider
  authenticates without an OAuth client loads without one. An account's token is made from its own
  client and from no other
  ([ADR-0106](../provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)).
  The process's [Session](../../../process/README.md#the-session-sessions), `process/sessions`,
  holds three members.

  - The snapshot, the accounts it has in force, read by `process/accounts`. A reload replaces it
    whole.
  - Each account's policy, in `process/policies`, reloaded with the snapshot.
  - The connections, `process/connections`, which hold each listed account's opened credential in
    a connection record of that account's own.

  A process that calls a provider holds one Session, which every endpoint and job kind it runs may
  ask for, built once from the process
  ([ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). Its statements
  run as the one application user the API server, the UI and the worker connect as
  ([ADR-0143](../data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)).
  It loads the snapshot at start. A process that runs until stopped reloads it in one reload, on a
  schedule its configuration sets
  ([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)),
  `account_reload_interval` in the API server and in the worker, whose `reload_interval` paces its
  job kinds' own listing
  ([ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)). Its first load runs
  at once and, while no load has read the accounts, their connections and their policy, again every
  five seconds, so a process whose first load fails after it has started, the database having
  answered its start's schema check
  ([ADR-0140](../data/0140-each-migration-declares-the-oldest-chain-that-runs-on-it.md)), serves
  soon after a load reads them. The first load does not block the start, and until a load has read
  them the Session reports not ready, and so does the process. A policy whose rows do not validate
  is read all the same, and holds up neither
  ([ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). A process
  whose database does not answer at start does not start, and the platform restarts it. Once a
  load has read them, a reload that fails is tried again at the next interval. Every unit of work
  takes the latest snapshot its process holds when it starts, whether the unit is a body request, a
  tick of delta sync or a page of backfill
  ([ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md),
  [ADR-0103](./0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)). A process
  that runs and exits takes the snapshot once at start.
- **A unit of work takes the snapshot once** and never reads it again mid-flight. A reload puts a
  new list in force for the units that start after it, and leaves a unit in flight on the list it
  took. An account's connection record is the connections' own, not part of the snapshot. It
  changes in place only when the connections adopt a value read from the account's row, at a
  reload or at the re-read after a refusal
  ([ADR-0106](../provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)),
  or write one back by compare-and-set
  ([ADR-0089](./0089-sealed-values-written-by-compare-and-set.md)), and every unit of work on that
  account follows those changes.
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

- A new account starts being served within one reload interval, or at the next run of a process
  that runs and exits, and a re-authorized credential at its next refused call.
- The Session's policies, the API server's set of served accounts and the worker's account jobs take
  their account set from each snapshot, so they are built for a set that changes.
- Assumptions about other components. The database is reachable when a reload runs, and a reload
  that cannot reach it leaves the process on its previous snapshot.
