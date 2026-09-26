# 0066. Data access is generated from hand-written SQL by sqlc over pgx, and the dataset endpoint is enumerated rather than composed

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[P3](../../../USE_CASES.md#p3--multi-account),
[O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

[ADR-0047](./0047-schema-first-data-access.md) fixes the approach and defers the mechanism to its
own record. The approach is the schema as sole authority, per-query result types, an account
identifier on every signature and in every statement, and one data-access library produced from
the one schema. What it leaves open is whether the accessors are generated or hand-written, and by
what.

Most of what a data-access ecosystem sells is already designed out. No entity model, no identity
map, no lazy loading, no scaffolding over a broad create-read-update-delete surface, no migrations
generated from code ([ADR-0048](./0048-forward-only-migrations.md)), and no database-resident code
to call ([ADR-0060](../engineering/0060-no-code-in-the-database.md)). Reads are projections and
aggregates, and writes are a small set. What remained to choose on is narrower. How a statement's
result type comes into being, what can check a statement before it runs, what a component can be
prevented from linking, and what any of it drags into the processes holding full-mailbox
credentials.

One question looks larger than it is. The dataset endpoint of
[ADR-0057](../operability/0057-one-dataset-endpoint-behind-a-registry.md) varies its grouping,
filters, sort and level per request, which reads as a composition problem. It is not. The
registry's grouping axis is closed and small, between two and six dimensions across the five
analysis lenses [docs/UI.md](../../UI.md#85-the-analysis-lenses) declares. Filters parameterise as
a null-guarded array, which covers the equality, any-of and exclusion forms of
[docs/UI.md](../../UI.md#5-information-architecture-and-the-url)'s filter grammar in one parameter
shape. Sort parameterises as arms of a case expression over literal columns. What is left is an
enumerated set of static statements, and that shape is affordable only because of the corpus size
[ADR-0016](./0016-schema.md) assumes.

### The requirements

Each derives from a record that already binds, named beside it.

| Requirement | What it demands | Source |
| --- | --- | --- |
| Account predicate | Every statement against an account-keyed table carries an account predicate, and omitting one is catchable before the statement runs | [ADR-0047](./0047-schema-first-data-access.md) |
| Fenced composition | A dimension, filter or sort the registry does not declare cannot reach the database | [ADR-0057](../operability/0057-one-dataset-endpoint-behind-a-registry.md) |
| Drift is a build failure | Schema, statements and result types cannot disagree except as a failed build | [ADR-0047](./0047-schema-first-data-access.md) |
| Schema authority | The migration set is the only authority on stored shape, and nothing derives it from code | [ADR-0047](./0047-schema-first-data-access.md), [ADR-0048](./0048-forward-only-migrations.md) |
| Per-query result types | Each statement has its own result shape, with no type shared across statements and no field a body could occupy | [ADR-0047](./0047-schema-first-data-access.md) |
| Role separation | Several database roles hold different grants, and a component cannot name an accessor its role has no grant for | [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) and [ADR-0048](./0048-forward-only-migrations.md) establish that the roles differ. That a component cannot name another role's accessor is strengthened to here |
| Transaction-scoped account | Each transaction sets the account by an ordinary statement before reading, and this survives pooling and statement caching | [ADR-0016](./0016-schema.md), [ADR-0060](../engineering/0060-no-code-in-the-database.md) |
| Batched writes | Several hundred to a thousand rows per batch without leaving the typed layer | [ADR-0025](../operability/0025-priority-classes-and-leases.md) |
| Type coverage now | Case-insensitive text, text arrays, JSON, timestamps with zone, big serial | [ADR-0016](./0016-schema.md) |
| Trust-anchor footprint | What the choice adds to the processes holding full-mailbox credentials | [ADR-0028](../operability/0028-trust-anchor-hardening.md) |
| Long-term fit | Governance, release cadence, breaking-change record, and what abandonment costs | The project outlives any tool it picks |
| Vector column | [ADR-0016](./0016-schema.md) declares a nullable vector column and [ADR-0048](./0048-forward-only-migrations.md) creates its extension before the chain and records it in the chain's first migration, so the generator maps the type from the start. Only populating the column waits for the heuristics work | [ADR-0016](./0016-schema.md), [ADR-0004](../classification/0004-sender-list-decides.md) |
| Crash-harness fit | Data access callable with plain values, with transaction boundaries the harness controls | [ADR-0045](../engineering/0045-crash-injection-testing.md) |
| Idiom pull | Whether a tool's ordinary path leads toward what these records forbid | The records above, taken together |

**What orders the field is the account predicate.** It is the one requirement where the candidates
differ in kind rather than degree, and the failure it guards is silent. A statement declared as a
file is a whole statement carrying its own predicate, and a build step can read it. A statement
assembled at run time exists only after the assembler runs, so the same check becomes a test, and
its static form reads almost nothing and reports clean.

**Ties break on what abandonment costs.** Every serious candidate depends on few maintainers, so
the likelihood of abandonment barely separates them while its cost separates them completely.

**Two things are excluded from grading.** Partitioned schema, because nothing partitions. And
throughput and latency, because the corpus assumption puts them out of reach of this decision.

## Decision

- **Statements are hand-written SQL files. `sqlc` generates the accessors and the per-statement
  result types from them, emitting code against `pgx`'s native interface.** The generator reads the
  migration directory of [ADR-0048](./0048-forward-only-migrations.md) directly as its schema
  input, so one artifact feeds both the database and the code. It understands the file layout of
  every migration runner [ADR-0067](./0067-migration-runner-goose.md) keeps in play.
- **`pgx` v5 is the driver, used natively through its pool, at v5.9.2 or newer.** The minimum is
  stated because releases before it carry memory-safety and injection advisories, and it is the
  only third-party dependency this decision adds inside the trust anchor
  ([ADR-0028](../operability/0028-trust-anchor-hardening.md)).
- **The dataset endpoint is enumerated**, one statement per dataset and grouping dimension, plus a
  summary and a rows statement per dataset.
- **The generator's defaults produce what these records forbid, and configuration declines only part
  of it.** The configuration omits the table structs no statement uses. No setting stops the
  generator reusing a table's struct for a statement that selects all of that table's columns in
  table order, so the assertion that the generated table-struct file declares no types is the
  mechanism, and a statement it refuses reorders or aliases its columns.
- **Five constraints bind what a statement file may contain, and they are checked together.** The
  parse-tree pass the project writes checks all five. The generator's own rule surface cannot carry
  them, because it sees each statement after a star select has been expanded and named parameters
  rewritten, and it sees no column types, so it can neither find a star select nor tell a cast
  against a case-insensitive column from one against plain text. Three of the constraints are still
  matters of a statement's text and two of its structure, which the table records.

  | Constraint | Why | Kind |
  | --- | --- | --- |
  | No star select | The generator expands a star at generation time, so a result type would silently gain every column a future migration adds | Text |
  | No case expression over a parameter in a grouping position | It generates and runs, types the group key as an empty interface, and lets an undeclared dimension reach the database and return one row with a null key, turning a refusal into silent wrong data | Text |
  | No cast on a parameter compared against a case-insensitive column | The cast makes the same question return no rows, with no error, and the generated Go is identical either way | Text |
  | Every paged read's sort ends with the row's own identity | [ADR-0057](../operability/0057-one-dataset-endpoint-behind-a-registry.md) requires the total order and leaves the check here. Whether a sort's final term is a dataset's identity column is a question about the statement's shape rather than its text | Structural |
  | The account predicate is present | [ADR-0047](./0047-schema-first-data-access.md)'s rule. A text match cannot tell a predicate on the account from a mention of it in a projection | Structural |

  The generator's own per-query suppression annotation appears in no statement file, because a
  suppression is refused wherever a check stands in for a control
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)). It cannot reach the parse-tree
  pass, which is the project's.
- **Generated code sits in the one data-access directory, in subsections organized by concern rather
  than per role.** One generated package per subsection, all from the one schema. A statement may be
  needed by more than one role, so grouping by role would duplicate it. The role boundary is held by
  two checks instead. Each component's import list names exactly the subsections it may use
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)), and a test runs every statement
  of a subsection under the role of each component whose list admits it, planning each one with
  `EXPLAIN (GENERIC_PLAN)` so nothing executes, so a list admitting a statement the role's grants do
  not allow fails naming the list, the statement and the role. Which role each component connects as
  is [ADR-0075](./0075-one-runtime-role-per-deployable.md)'s, and the check reads that mapping from
  one place in `db/check`, failing a deployable's list that names subsections without a role and a
  role whose deployable's list reaches no subsection, directly or through a shared library. A shared
  library connects as no role of its own. Its statements run under the role of each deployable whose
  list admits the library, and the check plans them under each of those roles, reading which
  deployables admit it from their lists.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| Account predicate | The parse-tree pass over the declared statement files, run as a build step, reading the account-keyed table list [ADR-0047](./0047-schema-first-data-access.md) requires be derived from the schema |
| Fenced composition | No identifier reaches SQL from a string, and an undeclared dimension has no generated function. Values still reach SQL, so the sort value is validated against the sortable columns [docs/UI.md](../../UI.md#172-the-registry-entry) declares, on the path that already validates the dimension |
| Drift is a build failure | Three mechanisms catching different things. Generation fails, naming the column, when a statement does not typecheck against the schema. Regenerate-and-diff catches what typechecking cannot, meaning generated code hand-edited or predating a schema change. A test catches a generated file left behind after its statement file is deleted, which neither regenerating nor the diff notices. None of them catches a configuration change, because generation runs from the configuration as checked in |
| Schema authority | The generator reads the migration files, and nothing derives schema from code |
| Per-query result types | One result struct per statement. The generated table-struct file declaring no types is asserted, because configuration omits only unused table structs and the generator reuses a table's struct for a statement selecting all its columns in table order |
| Role separation | One generated package per subsection by concern, all from the one schema. Separate packages alone are a naming convenience rather than a boundary, since any import compiles cleanly, so each component's import list names its subsections and a test plans every admitted statement under the component's role |
| Transaction-scoped account | An ordinary `SET LOCAL` under the driver, whose isolation holds under forced generic plans and under the driver's statement caching |
| Batched writes | One insert selecting from unnested array parameters, fully typed |
| Type coverage now | All five map without custom overrides or type registration |
| Trust-anchor footprint | The generator is a build-time binary contributing no run-time modules. Generated code imports the driver, not the generator |
| Long-term fit | Abandonment costs a generator. The statement files are the authored artifact and the generated Go keeps compiling |
| Vector column | Supported, conditional on the native driver target this record chooses. Three open defects apply, one of them to the nullable vector column this schema declares |
| Crash-harness fit | Accessors are callable with plain values, and an emitted interface gives the harness something to wrap |
| Idiom pull | Every default that breaks a record here is enumerable, and each is closed by a check rather than by memory |

### Implementation notes

What an implementer would otherwise pay to discover, because building against the opposite
assumption produces a silent wrong answer rather than a failure. The row marked as such is known
from the generator's issue tracker rather than from having been run.

| What | Why it matters |
| --- | --- |
| Carrying the grouping dimension as a value into a case expression is a **rejected option** | It generates and runs, types the group key as an empty interface, and lets an undeclared dimension reach the database and return one row with a null key. It converts a refusal into silent wrong data |
| A null-guarded filter parameter needs a cast to the column's type, **known from the generator's issue tracker rather than from a run** | Without it the generator refuses to generate. The array form of the same idiom loses nullability inference, so the generated parameter is a plain slice and a nil slice is the absent case |
| The natural multi-argument `unnest` form is refused by the generator's analyzer | Write it as a common table expression with parallel single-argument `unnest` calls |
| `COPY` is refused by PostgreSQL on any table with row-level security, for every role the policy applies to | And it would not serve this design regardless. [ADR-0025](../operability/0025-priority-classes-and-leases.md) carries both halves of that argument |
| Case-insensitive text needs no type registration and works in every driver mode | But a text cast on a parameter compared against such a column makes the same question return no rows, with no error, and the generated Go is identical either way. Under a policy, a composite index over such a column degrades to one column plus a filter, because the comparison operator is not leakproof |
| The account setting's deny state is silent | Once a connection has set it, PostgreSQL resets it to the empty string between transactions rather than to unrecognised, so a statement omitting it returns no rows without error on a warm connection and raises on a fresh one. The shared transaction helper reads it back and fails the transaction when it is empty. [ADR-0016](./0016-schema.md) carries the schema half |
| A grant refusal and a policy exclusion behave oppositely | A grant the role lacks raises, naming the table and not the column, so it carries nothing the operator can act on. A policy excluding the row empties the statement, which succeeds having changed nothing |
| The plain execute annotation discards the command tag | Every statement whose predicate a policy can empty uses the row-counting annotation instead, and zero rows affected is a failure. Today that is the four decision writes behind [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)'s two decision verbs |
| If a connection pooler is placed in front of PostgreSQL | Statement mode is unusable because it forbids transaction blocks. Transaction and session modes both preserve isolation. The driver's prepared statements require the pooler to permit them, which is preferred over disabling them, because disabling them discards the cached plans the isolation result was measured under |

## Alternatives considered

Five candidates were graded on every requirement above, on a four-level scale. **4** means the
candidate carries the requirement natively. **3** means it is carried with bounded discipline or
one small maintained package. **2** means it is carried only by convention or with a named gotcha.
**1** means it cannot honestly satisfy it.
The chosen candidate is the first column. The fifth pairs the generator with a run-time query
builder. Its requirement grades are the builder of the third column, because the other builders in
the field are unreleased since 2023 or pre-release. Its footprint figures are squirrel's.

| Requirement | sqlc | Hand-written over pgx | go-jet | sqlx over database/sql | sqlc plus a builder |
| --- | --- | --- | --- | --- | --- |
| Account predicate | 4 | 3 | 2 | 2 | 3 |
| Fenced composition | 4 | 2 | 3 | 2 | 3 |
| Drift is a build failure | 4 | 3 | 3 | 1 | 3 |
| Schema authority | 4 | 4 | 4 | 4 | 4 |
| Per-query result types | 4 | 4 | 3 | 4 | 3 |
| Role separation | 4 | 3 | 3 | 3 | 4 |
| Transaction-scoped account | 4 | 4 | 3 | 3 | 3 |
| Batched writes | 4 | 4 | 4 | 4 | 4 |
| Type coverage now | 4 | 4 | 3 | 2 | 3 |
| Trust-anchor footprint | 4 | 4 | 2 | 2 | 3 |
| Long-term fit | 3 | 3 | 3 | 1 | 3 |
| Vector column | 3 | 4 | 2 | 3 | 3 |
| Crash-harness fit | 4 | 4 | 3 | 4 | 4 |
| Idiom pull | 3 | 2 | 2 | 2 | 2 |

Two requirements sort nobody. Schema authority is a tie at the ceiling, because hand-written
access passes it by having no schema awareness at all while both generators pass it by
construction, and it separates only the entity model that a settled record already excluded.
Batched writes joins it once bulk copy is off the table, because every candidate's remaining path
stays inside its typed layer, measured for four of the five and true by construction for the fifth,
where the statement and its result type are both written by hand.

Per-query result types looks like a third and is not. The builder emits per-table structs and
leaves per-query result types to be written by hand, and its ergonomic bulk path is expressed over
those same per-table structs, so reaching for it undoes the arrangement. That is a requirement
carried by discipline rather than by the tool, which is what its 3 records.

Four rows were exercised rather than read, and for four of the five candidates. The account
predicate, by writing the check and taking it from passing to failing. Role separation, by building
the generated packages and attempting a cross-role import. Trust-anchor footprint, by building
identical minimal programs and comparing them. And batched writes. The struct-scanning candidate
was not built at any point, so its column rests on documented behaviour throughout, as do the
remaining rows for every candidate. There are three rows where
the chosen candidate stands alone at the ceiling, the account predicate, fenced composition and
drift, and the account predicate is the one among them whose failure is silent.

The second table carries what the requirements do not, meaning the facts that decided between
candidates the requirements had sorted into the same tier.

| Candidate | What abandonment costs | Run-time modules it adds | What generation needs | Where its defaults lead |
| --- | --- | --- | --- | --- |
| sqlc | The generator. Statement files and generated code are ordinary artifacts that keep compiling | None. Its module set is identical to hand-written driver code, and stripped binaries are byte-identical | The migration files | A star select, and one package for everything. Both closed by a check |
| Hand-written over pgx | Nothing beyond the driver every candidate needs | None beyond the driver | Nothing | String building and a wide struct, closed by a check the project writes and owns |
| go-jet | The query layer. Every statement is a builder expression | The builder, plus a second complete PostgreSQL driver its module requirements pull in | A live database with the chain already applied, so the build acquires a database dependency | Per-table model structs, which its documentation corpus is almost entirely written in, plus string-taking escape hatches in its own interface |
| sqlx over database/sql | The scanning layer, which is small | The library | Nothing | A wide struct, and a wrapper on every text array |
| sqlc plus a builder | The builder half, confinable to the user interface | Three small modules, two of them last published in 2018 and 2015, measured on squirrel | The migration files | Both sets above, and a second way to write a statement |

**Reading the two tables.** The chosen candidate is the only one with no requirement below 3, and
the only one alone at the ceiling on the requirement that orders the field. That outweighs a higher
ceiling elsewhere, because the failure the ordering requirement guards is silent where the others
announce themselves.

Its wins are also the ones nothing else backs up. Its strength on schema authority is redundant,
since the schema is already the authority by other means, but its build-step check on the account
predicate guards a property with no other build-time layer, and its generation-time typecheck turns
a statement selecting a body column into a failed build, which is the schema's own absence enforced
by machine rather than by review.

Regretting it is cheap and regretting the builder is not. Leaving the generator costs the
generator, because the statement files are what was authored and the generated code is ordinary
checked-in Go. Leaving the builder means rewriting every statement. Both rest on few maintainers,
so the asymmetry lies entirely in the cost and not in the odds.

The builder's one real advantage is detachable while its costs are not. A dimension naming a column
that does not exist is a compile error under it, which the chosen candidate matches at generation
time for statements and does not match on the sort axis, where a check stands in. Its second driver
inside the trust anchor, its build-time database dependency and its silent downgrade of
case-insensitive columns are confinable nowhere.

- **Hand-written statements with hand-written result types over the same driver.**
  [ADR-0047](./0047-schema-first-data-access.md) kept this alive deliberately and it is the closest
  alternative. The case for it is stronger than it first appears. The driver's row-scanning helpers
  have absorbed the boilerplate generation used to be worth, and preparing every declared statement
  against the migrated schema reproduces most of the drift catch, including the body-column case,
  using the fresh database [ADR-0043](../engineering/0043-no-mocking.md) already requires on every
  test run. Rejected because the helpers match by run-time reflection, so a struct disagreeing with
  its statement is a run-time error rather than a failed build, and because it produces no result
  types, leaving the statement half of the drift guarantee without the type half. It stays the
  fallback, which is the same fact as the cheap exit above.
- **A typed query builder generated from the migrated database.** The case for it is composition
  the compiler checks, and a nested-struct mapping confined to one result set that is therefore not
  the identity map [ADR-0047](./0047-schema-first-data-access.md) rejects. Rejected on the second
  table's row, and on one thing that table does not carry. It cannot model case-insensitive
  columns, warning on every generation as it downgrades them.
- **Struct scanning over the standard library's database interface.** The case for it is
  familiarity and less to write than hand-rolled scanning. Rejected on maintenance and on types. It
  has had no release since April 2024 and carries no maintenance statement either way, which is a
  gap rather than a clean record, and under that interface every text array needs a wrapper, which
  this schema has thirteen of.
- **A generator for the static statements plus a run-time builder for the dataset endpoint.** The
  case for it is that it answers the composition question directly and confines the builder to the
  user interface, which [ADR-0021](../mutation/0021-approval-surface.md) placed outside the trust
  anchor when the case was made. Rejected because the Context shows the composition question does
  not arise, so it buys a second way to write a statement for no gain.
- **An entity-model tool.** [ADR-0047](./0047-schema-first-data-access.md) rejected this on four
  named collisions and nothing here disturbs them. What it would have offered is the scaffolding
  productivity that record concedes is real, migration generation
  [ADR-0048](./0048-forward-only-migrations.md) forbids by name, and relation traversal this schema
  barely needs. Its case has weakened since, because the generated-code candidates now produce
  per-statement types with a build-time drift check, which is most of what the scaffolding bought.

### Where generated code sits

- **Each role's generated package under its owning deployable's `internal/`.** The case for it is
  that the compiler holds the role boundary, so no import check is needed. Not chosen, because the
  shared transaction helper would be copied into each deployable or split into a library of its own,
  and a statement several roles need would be duplicated.

## Consequences

- **The enumerated endpoint is around sixty-four statements at the read surface, and the number is
  what to watch.** That figure is a floor, derived from the registry's declared dimensions rather
  than from written SQL, so it is not reliable enough to schedule against and its first job is to
  be replaced by an actual count. It excludes the bespoke endpoints, the decision writes and the
  whole write surface, which together put the whole read and write surface somewhere near a
  hundred and twenty-five on the same weak footing. Past roughly a hundred and fifty the pressure
  to replace the set with one parameterised composer becomes hard to resist, and taking that
  option would give up the property this decision was made for. It is a pressure to resist rather
  than a capability gap, because the grouping axis stays closed per dataset however many datasets
  exist.
- **This decision is re-argued if the corpus assumption moves.** The enumerated shape is judged
  affordable because a null-guarded filter and a case-expression sort are not expected to use an
  index and do not need to at the assumed size. That expectation is reasoning about query planning
  rather than a measurement, and it is the premise most worth testing first. An order of magnitude
  more brings back the composition question, and partitioning with it, which
  [ADR-0016](./0016-schema.md) already records.
- **The statement set is checked against the registry by count**, meaning a dataset or dimension
  the registry declares with no statement behind it, or the reverse. The filter, sort and
  result-type seams are not covered and are accepted.
- **The enumerated endpoint taxes
  [ADR-0057](../operability/0057-one-dataset-endpoint-behind-a-registry.md)'s cost of a new
  analysis view**, which now includes one statement per groupable dimension plus a summary and a
  rows statement. That record states the corrected form.
- **Assumptions about other components.** The migration set is reviewable SQL and is the artifact
  the generator reads. The registry declares every dataset, dimension, filter, sort and result
  type, and its refusal of an undeclared one is application code under any tool. The test substrate
  starts a real database with the chain applied, which
  [ADR-0068](../engineering/0068-test-substrate-containers-directly.md) decides.
- The requirement-to-mechanism table and the implementation notes state controls. Their injections
  are catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
