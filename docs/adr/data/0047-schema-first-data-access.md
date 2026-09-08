# 0047. The schema is the single authority, read through per-query result types — no entity model

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[P3](../../../USE_CASES.md#p3--multi-account)

## Context

The schema's most load-bearing property is an absence: no body, snippet, or excerpt column
exists anywhere, and [ADR-0016](./0016-schema.md)'s DDL comment makes a future migration adding
one a violation, not an extension. An absence is enforceable only while the DDL is the single
authority on stored shape — a second authority in memory, from which the schema could be
regenerated, is where such absences quietly stop binding. Meanwhile every table keys on
`account_id` and there is no implicit current account
([ADR-0026](../provider/0026-multi-account-contexts.md)), and decisions take everything as
parameters ([ADR-0040](../engineering/0040-pure-core-decisions-as-values.md)).

## Decision

- **The database schema — the migration set — is the single authority on stored shape.** No
  in-memory entity model exists: no construct that owns the schema, generates DDL from classes,
  holds an identity map or cache of rows, or lazily fetches relations at attribute access.
- **Reads go through per-query result types: each query has its own result shape.** A metadata
  query's row type therefore has no body field to populate — there is no field a body could
  hide in, which is the unconstructability pillar's own wording, landed on the data layer.
- **Every data-access signature requires the account identifier.** The repository-layer rule
  [ADR-0016](./0016-schema.md) states becomes a fact of the function signatures; omitting the
  account is a compile error, not a runtime surprise.
- **One data-access library, produced from the one schema, serves every component** — so drift
  between the schema, the queries, and the result types is a build failure everywhere at once
  rather than a runtime discovery in one workload. The library is impure shared code, deliberately
  outside the pure core; it is a narrow, single-concern library. Whether its accessors are
  generated or hand-written under the same discipline, and by which tool, is an
  implementation-time decision with its own record.
- **Row-level security stays the independent second layer**, exactly as
  [ADR-0016](./0016-schema.md) names it — nothing here substitutes for it.

## Alternatives considered

- **An object-relational entity model.** The case for it: real scaffolding productivity, the
  ecosystem default, and the honest defense that in-memory/relational mapping is genuinely hard
  and hand-rolling it has historically gone worse. Rejected on four collisions with landed
  decisions: it inverts the schema authority (a model attribute *creates* a column, so the
  violating migration ships as a routine generated diff no reviewer is prompted about); its
  idiomatic account scoping is an ambient session — the implicit current account
  [ADR-0026](../provider/0026-multi-account-contexts.md) forbids; lazy loading fires I/O at
  attribute access, which lands *after* the gate has run and makes "the last hop" undefinable;
  and its identity map or cache can return a previously materialized row without re-entering
  the decision path, bypassing fetch-time re-evaluation
  ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)). The partial-object shape is the
  fifth: one shared entity type would hand metadata paths a body field the design refuses to
  let exist.
- **Hand-written string SQL scattered through each component.** The case for it: no build step,
  no shared artifact. Rejected: drift between DDL and queries is then caught only at runtime,
  near-identical queries duplicate across the workloads, and each copy drifts separately — the
  dual-schema problem multiplied by the number of components.

## Consequences

- General-purpose scaffolding is forgone. If a broad create-read-update-delete surface over
  many tables is ever genuinely wanted, that trade gets re-argued then — it is not this
  system's shape today (reads are projections and aggregates; writes are a deliberately small
  set).
- Assumptions about other components: the migration set exists as reviewable SQL and is the
  artifact the data-access layer is produced from — the evolution rule is
  [ADR-0048](./0048-forward-only-migrations.md); every component takes its database access
  through the shared library rather than opening its own SQL surface.
