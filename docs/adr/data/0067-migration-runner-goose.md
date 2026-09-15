# 0067. The migration runner is goose, invoked as a command

**Status:** Accepted ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

[ADR-0048](./0048-forward-only-migrations.md) fixes the shape of migrations and does not name the
tool that applies them. The shape is hand-written SQL files reviewed as code, forward-only, applied
from an empty database on every test run, and run as their own step under a role that owns the
schema.

That shape removes most of what a migration tool sells. Nothing generates a migration from a model,
there are no down migrations to manage, there is no expand-and-contract choreography because all
components version together, and there is no branching or squashing story to need because the chain
is short and the index it builds is rebuildable by re-backfill
([ADR-0048](./0048-forward-only-migrations.md)). What remained to choose on is which tool imposes
least on a project that wants almost nothing from it.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| Hand-authored as the default path | The tool's own documentation treats hand-written SQL as the ordinary way to use it, not as a secondary mode beside generating migrations from a declarative schema | [ADR-0048](./0048-forward-only-migrations.md) |
| Up-only without ceremony | A migration that cannot be reversed is expressed by writing nothing, rather than by leaving something empty or relying on a file's absence | This record's refinement of [ADR-0048](./0048-forward-only-migrations.md)'s forward-only rule |
| No database-resident code | The tool installs no function, trigger or procedure. An ordinary version table is fine | [ADR-0060](../engineering/0060-no-code-in-the-database.md) |
| The generator reads its layout | The code generator takes the same files as its schema input, so one artifact feeds both the database and the code | [ADR-0066](./0066-data-access-generated-from-sql.md) |
| Runner binary footprint | What the distributed binary carries into a step that runs beside a system using one database engine | [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture, applied to a step that runs outside the runtime |
| Long-term fit | Governance, and whether the capabilities relied on stay available | The project outlives any tool it picks |

One thing every candidate does, so it is not graded. All six run as a command taking a directory
and a connection string, which is what [ADR-0051](../engineering/0051-environment-contract.md)
needs.

**No requirement orders the field, and that is the finding.** Four of the six candidates come out
close enough that the grid does not separate them. The two that fall away do so for different
reasons, one on what its distributed binary carries and the other on the mode it would have to be
used in. So this record is decided by a stated preference rather than by a derivation, and the
reading below says which preference and on what.

This is a close decision and the Decision says so rather than dressing it up.

## Decision

- **`goose` applies the migration chain, invoked as a command rather than linked as a library.**
- **`tern` and `dbmate` are level with it on the grid below**, each carrying the same number of
  cells below the ceiling and trading which ones, and either would be a defensible choice.
  `golang-migrate` is one cell further back.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| Hand-authored as the default path | Its documentation's ordinary path is a directory of SQL files, and it offers no schema-diffing mode to drift toward |
| A concurrent index build is available if one is ever wanted | A per-file `NO TRANSACTION` directive. Not graded, because every candidate offers the capability, five by a directive of their own and one by arranging the file's contents |
| Up-only without ceremony | One file carrying one annotation, where the annotation marking the reverse direction may simply be absent |
| No database-resident code | One ordinary version table. Where it locks, it uses PostgreSQL's built-in advisory locks rather than anything it installs |
| The generator reads its layout | Documented by sqlc and by goose. Every other candidate's pairing is documented by one side only |
| Runner binary footprint | Carried with a named gotcha rather than natively. Its distributed binary carries every database it supports unless built with the tags that exclude them. What the command form does buy is that none of that tree reaches this project's own binaries |
| Long-term fit | An organisation with a lead maintainer and secondary contributors, and no capability relied on here sits behind a licence |

### Implementation notes

- **One known gap.** The version-table bootstrap always runs inside a transaction, and the per-file
  directive does not reach it. For PostgreSQL this is immaterial, because data definition there is
  transactional anyway.
- **The distributed binary is not minimal.** The tree the command form keeps out of this project's
  binaries is still inside the runner binary itself unless it is built with the tags that exclude
  other databases. That is a property of the step, not of anything this project ships, so the
  migration step's image builds goose from source with those tags.
- **Neither `go tool` nor a `tool` directive in the project's module suits the step.** `go tool`
  ignores build tags, so it cannot build the tagged runner, and a `tool` directive in the project's
  own module takes part in version selection, so goose's requirements raise shared dependency
  versions inside the project's binaries. Developers and CI run a pinned release binary instead
  ([CLAUDE.md](../../../CLAUDE.md#tools-and-versions)).

## Alternatives considered

Six runners were graded on a four-level scale. **4** means the candidate carries the requirement
natively. **3** means it is carried with bounded discipline or a named gotcha. **2** means it is
carried only against the tool's own grain. **1** means it cannot honestly satisfy it. The chosen
candidate is the first column.

| Requirement | goose | golang-migrate | tern | dbmate | atlas | sql-migrate |
| --- | --- | --- | --- | --- | --- | --- |
| Hand-authored as the default path | 4 | 4 | 4 | 4 | 2 | 4 |
| Up-only without ceremony | 4 | 3 | 4 | 3 | 4 | 4 |
| No database-resident code | 4 | 4 | 4 | 4 | 3 | 4 |
| The generator reads its layout | 4 | 4 | 3 | 4 | 2 | 3 |
| Runner binary footprint | 3 | 3 | 4 | 4 | 1 | 1 |
| Long-term fit | 3 | 3 | 3 | 3 | 2 | 2 |

Two candidates sit at 3 on up-only for reasons the requirement's name does not show. One expresses
an irreversible migration by the absence of a companion file, against documentation recommending
that file always be written. The other requires a marker line in every file, whose body is then
left empty. Neither is a refusal to support the shape, which is why both are 3 rather than lower.

The second table carries what the grid does not, meaning what each candidate's cells rest on where
that is not obvious from the requirement's name.

| Candidate | How up-only is expressed | What its distributed binary carries | What stays available |
| --- | --- | --- | --- |
| goose | One file, one annotation, and the reverse marker may be absent entirely | Every database it supports, unless built with exclusion tags | Everything relied on here |
| golang-migrate | A separate down file, whose absence is what makes a migration irreversible, against documentation that recommends always writing one | Unverified. Small if built for one database, unknown as distributed | Everything relied on here |
| tern | Omission, framed in its documentation as deleting the marker | The smallest tree of the six, with no C dependencies | Everything relied on here |
| dbmate | A marker line required in every file, whose body may be empty | A single static binary, with an optional external dump dependency that is avoidable | Everything relied on here |
| atlas | Cleanly, in a mode that is not its headline one | Cloud clients, an entity model, a query-language client and an authorisation library | Diffing commands already require a login where they touch commercial schema features. Its version-table statements were not read from source, so what it installs is documented rather than confirmed |
| sql-migrate | Omission. A file with no down section parses cleanly | A C SQLite build, linked unconditionally, into a system using only PostgreSQL | Everything relied on here. Thin maintenance |

**Reading the two tables.** Four candidates have no cell below 3, and three of those four are
level, each carrying two cells at 3 and trading which ones. The chosen candidate's are what its
distributed binary carries and its long-term fit. One alternative trades the generator's reading of
its layout instead, and the other trades how it expresses an irreversible migration. The fourth
carries three cells at 3 and is a step further back.

**The grid does not break that tie and this record does not pretend otherwise.** None of the three
distinguishing cells is a refusal to support what is needed. Each is a bounded gotcha, which is
what a 3 means here, and the grades behind them come from each tool's documentation rather than
from anything built.

**goose is preferred on two grounds the evidence carries.** Its pairing with the generator is
documented by both projects, sqlc's side and goose's, where every other candidate's pairing is
documented by one side only. And its gotcha is removable, because building the runner with the tags
that exclude the databases this project does not use turns that cell into a ceiling at the cost of
owning the build. dbmate's gotcha is fixed in its file format. tern's is already inert under
[ADR-0048](./0048-forward-only-migrations.md)'s hand-written shape, so on removability tern is
ahead rather than behind and only the first ground separates the two of them.

**What would land on a different answer.** A reader who weighs dependency surface in the step that
holds schema-owning rights above both of those grounds lands on tern, whose tree is the smallest of
the six and carries no C dependencies. Nothing here refutes that reading.

The two ruled out are ruled out for different reasons. One links a second database engine into a
step that will never use it, which is dependency surface bought for nothing. The other is ruled out
first on what it does, because generating migrations by diffing a declarative schema is the shape
[ADR-0048](./0048-forward-only-migrations.md) rejects by name, so this project would live in its
second-class mode. What it carries and where its licensing is going are the further reasons rather
than the first.

The preference is stated rather than derived because the choice is reversible by changing a
command. Migration files are plain SQL under every candidate, and the generator reads the layout of
every candidate this record keeps in play, so a wrong answer costs the time to swap one binary for
another.

- **`tern`.** The case for it is the smallest dependency tree of the six and the clearest framing
  of omitting a migration that cannot be reversed. Its shared authorship with the driver of
  [ADR-0066](./0066-data-access-generated-from-sql.md) was checked and is a mild positive rather
  than an integration, since it runs as a standalone command with no shared pool or type
  registration. Not chosen, on the preference stated in the reading rather than on a weakness. Its
  one cell below the ceiling is the generator's reading of its layout, where an unaddressed
  template-syntax edge case could in principle stop a file parsing, and that edge case needs a
  template feature [ADR-0048](./0048-forward-only-migrations.md)'s hand-written shape never uses.
- **`dbmate`.** The case for it is a single self-contained binary assuming no language runtime,
  which suits a migration step any deployment mechanism can invoke. Not chosen on how it expresses
  an irreversible migration, since its format requires the marker line in every file and leaves the
  body empty.
- **`golang-migrate`.** The case for it is the broadest deployment of any candidate, which is why
  it is the one whose real-world edge cases are documented at all. Not chosen on three cells rather
  than two. Its irreversible migrations rest on a file's absence against documentation recommending
  the opposite, and its distributed binary's contents were not confirmed. Separately, and not
  graded, its route to a concurrent index build is arranging a file's contents rather than a
  directive.
- **`atlas`.** The case for it is the most capable tooling in the field, including schema linting.
  Rejected on two mechanisms rather than on preference. Its headline workflow generates migrations
  by diffing a declarative schema, which is the shape [ADR-0048](./0048-forward-only-migrations.md)
  rejects by name, so choosing it means living in its second-class mode, whose compatibility with
  hand-written-only files was never confirmed. And that headline mode is moving behind a login.
- **`sql-migrate`.** The case for it is a long-standing, simple runner. Rejected because its
  distributed binary unconditionally links a C SQLite build into a system using only PostgreSQL,
  which is dependency surface bought for nothing.
- **Applying the chain with a hand-written runner.** No case was tabled for one. The version table
  and ordering logic are small, but every candidate already provides them and owning them would buy
  nothing this decision needs.

## Consequences

- **Changing the runner is changing a command**, because the migration files outlive any of them
  and the generator reads the layout of every candidate this record keeps in play.
- **The migration step's failure is a failed step before any process starts**, which is the right
  shape for a deployment. [ADR-0048](./0048-forward-only-migrations.md) already accepts a brief
  unavailability during a migration.
- **Assumptions about other components.** Something outside the application grants the schema-owning
  role to the migration step and withholds it from the runtime roles. The deployment mechanism runs
  the step before rolling the deployables, and any mechanism that can run a command can do it, which
  is what [ADR-0051](../engineering/0051-environment-contract.md) requires. The extensions the
  schema needs are created by the bootstrap before the chain and recorded in its first migration,
  which [ADR-0048](./0048-forward-only-migrations.md) decides.
- No new control. The constraints this record checked belong to
  [ADR-0048](./0048-forward-only-migrations.md) and
  [ADR-0060](../engineering/0060-no-code-in-the-database.md), whose catalogue rows already exist.
