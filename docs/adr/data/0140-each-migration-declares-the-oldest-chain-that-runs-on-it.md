# 0140. Each migration declares the oldest chain whose binaries run on the schema it leaves, and a binary refuses a schema it cannot run on

**Status:** Accepted ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

[ADR-0048](./0048-forward-only-migrations.md) makes migrations forward-only, with no down
migrations, and [ADR-0129](./0129-the-chart-runs-the-migration-step-as-a-pre-install-and-pre-upgrade-hook-job.md)
runs the migration step as a Helm `pre-install` and `pre-upgrade` hook. Two facts follow that
neither record names.

- **Older binaries already run on a newer schema in every upgrade.** The hook runs while the
  previous release's pods serve. The API server and the UI then roll gradually, so old and new pods
  overlap, and the worker keeps running its jobs until Helm replaces its Deployment after the hook.
- **A rollback runs no migration step.** Helm's rollback runs the hooks of the release it rolls
  back to, and that release's Job is not a `pre-rollback` hook, so the older release's pods start on
  the newer schema. Down migrations could not run there either, since the older release's image was
  built before the newer migrations existed.

Nothing checks the schema's version today. No binary reads what the database has applied, and an
older migrator run on a newer database applies nothing and reports success. What older binaries
tolerate depends on the change. An added column, table or index leaves their statements working,
since the generator expands every column list when it generates. A dropped or renamed column or
table, a revoked grant, or a new `NOT NULL` column without a default that an older insert omits
breaks them with an error. A new value or column whose meaning older code ignores breaks them
silently, which is the dangerous case, since it could weaken a disposition
[C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released) rests on.

Established projects do not run down migrations on a Helm rollback. GitLab and Temporal keep the
previous release working on the new schema. Coder and Gitea have an older binary refuse a schema a
newer release migrated. Beyond that, each falls back to restoring a backup.

## Decision

- **A compatibility floor lives in the database.** `schema_compatibility` holds one row, the oldest
  chain version whose binaries may run on the schema, starting at 0
  ([ADR-0016](./0016-schema.md)), a chain version as the
  [Glossary](../../../DESIGN.md#glossary) defines it. Every role a deployable connects as reads the floor and the newest migration goose records, and
  only a migration writes the floor.
- **Each migration after the baseline declares which binaries run on the schema it leaves.** A
  migration that every chain at or above the current floor runs on, because none of those chains'
  binaries depends on what it removes or changes, says so in the marker line
  `-- compatibility: every chain the floor admits runs on this migration`, and the floor stays. A
  migration that binaries older than a version E cannot run on raises the floor to E in the same
  file, so the
  raise is atomic with the change, as `UPDATE schema_compatibility SET oldest_chain_version =
  greatest(oldest_chain_version, E)`, which never lowers it. E is the oldest chain whose binaries do
  not depend on what the migration removes or changes:

  | Change | Declaration |
  | --- | --- |
  | Additive | The marker, and the floor stays |
  | A contract step, dropping what the code stopped using at migration E | The floor raised to E, so a binary that knows E runs and an older one refuses |
  | A change the author chooses not to stage | The floor raised to the migration's own version, so every older binary refuses |

  The check over the library's migration files refuses a migration after the baseline that holds
  neither declaration. It admits a statement that names the floor's table only when the statement is
  that one form with E from 1 up to the migration's own version, a read, or a grant or a revoke of
  `SELECT` alone, so a role that opens its first pool gains the read in a migration of its own and
  no runtime role gains a write. Every other statement naming the table is refused, whatever its
  kind. The baseline builds the schema from empty, so no older chain
  runs on what it builds and it declares nothing. Expand and contract stays the default, and a
  declared floor is the exit when a change cannot be staged.
- **E is a version, not a yes or no.** A rollback goes to the previously deployed release, which
  may be older than the previous release when one was skipped. A flag saying "the previous release
  runs on this" would let a binary two releases old start on a schema that dropped something it
  reads. With a version the check is exact for a binary of any age.
- **Every process that runs until it is told to stop checks the schema before it serves.** The
  binary knows K, its own chain version, read from the chain its build embeds rather than kept by
  hand. On each pool the process opens, before any component uses the pool and so before readiness,
  it reads the floor F and the database's newest applied migration V under the pool's own role, and
  refuses to start with a named error when K is below F, meaning a migration it cannot run on has
  been applied, or when V is below K, meaning the migration step has not run for its release. A
  newer schema whose floor K meets is accepted, which is what makes a rollback across compatible
  migrations serve. The host runs the check its composition root hands it on each pool it opens
  ([ADR-0138](../engineering/0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)),
  and refuses a root that hands none, so no deployable can leave it out. The comparison is one
  function beside the embedded chain, the statement a data-access subsection of its own
  ([ADR-0066](./0066-data-access-generated-from-sql.md)), and each root's check reads the one
  through the other. The migrator, which moves V, does not check.

## Alternatives considered

- **Forbid rollback and retry instead, as the chart's guide said.** The case for it is that nothing
  is built. Rejected: it departs from the ordinary remediation a deployment uses, retries a failed
  upgrade forever, and does not stop a rollback by hand. Older binaries run on the newer schema in
  every rolling upgrade anyway, so forbidding rollback does not remove the hazard.
- **Down migrations in a `pre-rollback` hook.** The case for it is the conventional rollback story.
  Rejected: Helm runs the older release's hooks, whose image has no down steps for migrations
  written after it. Storing each migration's down step in the database for the older release to
  replay works mechanically, but brings back what
  [ADR-0048](./0048-forward-only-migrations.md) rejected, an untested reverse path, the loss of what
  was written since the upgrade, and no honest reverse for the append-only system-of-record tables.
- **Refuse any newer schema, as Coder and Gitea do.** The case for it is the simplest fail-closed
  rule. Rejected: every rollback after any migration becomes an outage. It is the floor with the
  floor raised by every migration, so the floor keeps that choice available per migration and pays
  for it only when declared.
- **Expand and contract as an absolute rule, with no runtime check, as GitLab and Temporal do.** The
  case for it is no new mechanism at run time. Rejected: a semantic break has no backstop, and every
  redesign costs two releases. The floor keeps expand and contract as the default and adds a
  declared exit that fails closed.
- **A backup taken before each upgrade and restored on rollback.** The case for it is the exact old
  state. Rejected: it throws away what was written since, the audit rows and policy changes among
  it, which are the system of record, no deployment tool restores it automatically, and the chart
  would have to know the database operator, against
  [ADR-0051](../engineering/0051-environment-contract.md). Backups stay the platform's recovery.

## Consequences

| Upgrade crossed | What a rollback does |
| --- | --- |
| No migration | The older pods start and serve |
| Only migrations that left the floor at or below the older release's K | The older pods start on the newer schema and serve, and the next fixed release goes forward |
| A migration that raised the floor above the older release's K | The older pods refuse with the named error, so the rollback fails and the worker, which stops its old pod before it starts the new one, is down, while the API server and the UI keep their newer pods. Nothing is written by a binary that cannot run on the schema, and recovery is going forward |
| A migration step that failed partway | goose applies each file in its own transaction, so the database holds a prefix of the chain, and Helm never touched the deployables, so the older pods keep running |

- A release's binaries can be rolled back to only if the release holds this check. The first
  release carries it, so every later release can be rolled back to it.
- The floor turns a declared break into a refusal rather than wrong behaviour. A break the author
  did not declare, a dropped column the marker wrongly calls compatible, is left to the declaration
  the check forces into every migration's review, and to a check that the newest release's
  generated statements still plan, as their roles, on the new chain. A rollback end to end, from
  the newest release to the one under test and back, is proven by deploying both.
- A pod already running when a migration raises the floor keeps running until it is replaced, since
  the floor is read at start. A structural break there produces errors, the brief unavailability
  [ADR-0048](./0048-forward-only-migrations.md) accepts.
- The check needs the database at start. The UI and the worker, which waited for the database in
  their readiness, now fail their start when it does not answer, as the API server already did, and
  the platform restarts them.
- Assumptions about other components. goose records each applied migration's version in
  `goose_db_version`, before the chain runs, and the chain declares that table to the generator only
  so the statement can read it. The roles of the worker's job kinds that open no pool yet gain the
  read with their pools. A binary's chain is the chain the migrator of the same release applies,
  which lockstep versioning guarantees
  ([ADR-0049](../engineering/0049-image-per-component-lockstep.md)).
