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
- **And every statement against an account-keyed table carries an account predicate.** The
  signature rule above guarantees the account reaches the function. It does not guarantee the
  account reaches the query, and a function that takes the account and omits the predicate
  compiles and passes review. The predicate rule closes that gap and is the second of three
  layers, between the signature the compiler checks and the row-level security
  [ADR-0016](./0016-schema.md) keeps behind both. The rule reaches every statement, insert and
  delete included, where an insert supplies the account as a column value rather than as a
  predicate. Three exceptions are stated. They are exceptions to this predicate rule and not to
  [ADR-0016](./0016-schema.md)'s keying rule, which states two of its own, because a table can be
  keyed on the account and still be reached without a predicate of its own. Base policy rows carry
  a null account and are inherited by every account, so the predicate there is the account or null
  ([ADR-0004](../classification/0004-sender-list-decides.md)). The accounts listing is
  deliberately unscoped ([docs/UI.md](../../UI.md#17-the-read-api)). And the operation log carries
  no account column at all, so it is reached through its plan rather than by a predicate of its
  own. **How the rule is checked is a mechanism question and belongs to
  [ADR-0066](./0066-data-access-generated-from-sql.md)**, which also records why a query set
  declared as files is checkable by a build step where statements assembled at run time are not.
- **The account-keyed table list is derived from the schema rather than maintained by hand.** Any
  table with an account column is account-keyed, with the exceptions above named explicitly and
  the list of exceptions itself checked against the schema. A hand-kept list goes stale the first
  time a migration adds a table and nobody remembers.
- **Every unit of data access runs in a transaction that has set the account and has verified it
  is set.** The verification is not ceremony. [ADR-0016](./0016-schema.md) records why the
  database cannot raise here and why the resulting deny is silent, and this is where the
  compensating assertion lives.
- **One data-access library, produced from the one schema, serves every component** — so drift
  between the schema, the queries, and the result types is a build failure everywhere at once
  rather than a runtime discovery in one workload. The library is impure shared code, deliberately
  outside the pure core; it is a narrow, single-concern library. Whether its accessors are
  generated or hand-written under the same discipline, and by which tool, was left to its own
  record and is decided by [ADR-0066](./0066-data-access-generated-from-sql.md).
- **Row-level security stays the independent third layer**, behind the signature rule and the
  predicate rule, exactly as [ADR-0016](./0016-schema.md) names it. Nothing here substitutes for
  it.

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
