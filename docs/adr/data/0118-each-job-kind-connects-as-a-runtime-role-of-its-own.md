# 0118. Each deployable, and inside the worker each job kind, connects to the database as a runtime role of its own, and inside one process the roles bound a buggy job, not a compromised process

**Status:** Proposed (supersedes [ADR-0075](./0075-one-runtime-role-per-deployable.md)) ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

[ADR-0075](./0075-one-runtime-role-per-deployable.md) gave every deployable a runtime role of its
own, so that what a compromised deployable can do in the database is bounded by its own role's
grants. It named six roles, for the mediator, backfill, delta sync, the reorganization workload,
the heuristics job and the UI, beside the migration role of
[ADR-0048](./0048-forward-only-migrations.md), and held each to three lines: its writes are its own
statements', it holds no grant on a sealed credential or an OAuth client secret unless it opens
them, and it holds no grant on a column a record forbids it. It assumed the deployment platform
gives each deployable the credential of its own role.

Backfill, delta sync, reorg apply and rollback, and the heuristics run are job kinds of one
worker process ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)). A
process holding several roles' credentials gives each role's bound a different meaning. A
compromised process holds every credential it was given, so its database reach is the union of its
roles. A buggy job, a statement that writes what it should not, still reaches only what the role
its code runs under is granted. Bugs are likelier than compromises. The four batch roles' names
already name job kinds, so keeping them needs no rename.

The grant check of [ADR-0066](./0066-data-access-generated-from-sql.md) plans every statement of a
subsection under the role of each component whose import list admits it, and which role each list
connects as is held in one place in `db/check`.

## Decision

- **Every deployable connects as a runtime role of its own, and inside the worker every job kind
  does.** The roles are `mediated_mailbox_mediate` and `mediated_mailbox_ui` for the mediator and the
  UI, and `mediated_mailbox_backfill`, `mediated_mailbox_sync`, `mediated_mailbox_organize` and
  `mediated_mailbox_propose` for the worker's job kinds backfill, delta sync, reorg apply and
  rollback, and the heuristics run, beside the migration role `mediated_mailbox_migrate`. A job kind
  added to the worker brings a role of its own.
- **Each job kind's code runs under its own role, through its own connection pool**, and each job
  kind's import list maps to its role in the grant check, as each deployable's does.
- **Each role's grants hold three lines.** Its writes are exactly what its own statements write. It
  holds no grant on a sealed credential or an OAuth client secret unless its code opens them. It
  holds no grant on a column a record forbids it to act on, which is the stored sender class for
  the mediator ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)). Reads beyond those
  lines are acceptable.
- **What the roles bound, stated plainly.** Between processes, the mediator, the worker and the
  UI, a role bounds what a compromised process can do in the database. Inside the worker, the
  roles bound what a buggy job can do in the database. They do not bound a compromised worker,
  which holds every role's credential, so its reach is the union of its job kinds' roles.

## Alternatives considered

- **One role for the worker, holding the union of its job kinds' grants.** The case for it: one
  credential to provision, a new role name and nothing else. Not chosen, because it gives up the
  first line for every job kind. A heuristics bug could update a stored credential, and a backfill
  bug delete messages.
- **A role per credential reach**, one for the job kinds that call a provider and one for the
  heuristics run. The case for it: two credentials, with the heuristics run still held away from
  stored credentials. Not chosen, because backfill, delta sync and apply would share reach.
- **Group the roles by what the job kinds write**, backfill and delta sync sharing one since they
  write the same tables. The case for it is one credential fewer. A bug in delta sync would reach
  everything backfill can write, which it mostly reaches already, the two differing in 13 column
  grants, and the shared role would need a name of its own. Not chosen, in favour of a role for each
  job kind held to the three lines.
- **Grants of exactly what each component's own statements need.** For it, the tightest bound on
  reads. Against it, it narrows how the data-access library can be organized and named, splitting a
  table's statements into a subsection per reader, for little benefit in the risk it gates. Not
  chosen, in favour of the three lines.

## Consequences

- A deployment provisions a runtime credential per role beside the migration role's, and delivers
  the worker the credentials of every role its job kinds run as. Each job kind added brings a
  credential to provision.
- A shared library, such as the rate limiter, runs its statements under the role of each deployable
  or job kind that uses it, so each of those roles holds the grants the library's statements need.
- [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) states the UI's grant.
- A role whose code calls a provider reads its accounts, their sealed credentials and the
  installation's OAuth clients, and updates the credential column of an account's state row when it
  writes back a rotation ([ADR-0082](../operability/0082-rotation-writeback-to-the-database.md)).
  Delta sync's role also updates each OAuth client's sealed secret, because delta sync re-seals it
  when a key is replaced ([ADR-0092](../operability/0092-key-replacement-by-keyring-and-re-seal.md)).
  The heuristics run's role holds no grant on a sealed credential.
- A role whose statements touch the operation log also reads the plan columns that
  [ADR-0016](./0016-schema.md)'s policy on the log looks up, because PostgreSQL runs a policy's
  lookup with the querying role's privileges. Those columns are part of what its statements need.
- The bug bound inside the worker holds only while each job kind's code receives its own pool. The
  composition root wires that, and review holds it, since the grant check sees which statements a
  list admits, not which pool reaches the code.
- Assumptions about other components. The roles are created once per cluster before the chain runs,
  and the chain's grant statements name them ([ADR-0048](./0048-forward-only-migrations.md)). The
  grant check shows a role's grants are enough for the statements its list admits
  ([ADR-0066](./0066-data-access-generated-from-sql.md)). The decision holds only while the
  deployment platform gives each process the credentials of the roles it, or its job kinds, run as.
- That a role's grants stay within the three lines has no automated check. Its disposition is in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
