# 0139. The migration runner is goose, linked as a library into the migrator, which configures and connects like every process

**Status:** Accepted (supersedes [ADR-0067](./0067-migration-runner-goose.md)) ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

[ADR-0048](./0048-forward-only-migrations.md) fixes the shape of migrations and does not name the
tool that applies them. The shape is hand-written SQL files reviewed as code, forward-only, applied
from an empty database on every test run, and run as their own step under a role that owns the
schema. That shape removes most of what a migration tool sells, so what is left to choose on is
which tool imposes least on a project that wants almost nothing from it.

[ADR-0067](./0067-migration-runner-goose.md) chose goose and ran it as a command. The command took
its connection as one string, so the migrator was configured differently from every other process.
Every other process takes a `database` section through the configuration library, with the
password in a mounted file holding the password alone, refuses to start while `PGPASSWORD` or
`PGSSLPASSWORD` is set, logs JSON at its `log_level` and answers `--version`
([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md),
[ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md),
[ADR-0122](../engineering/0122-logs-through-slog-at-a-configured-level-handed-to-shells.md)). The
command read a connection string whose password came from a libpq password file, a line
`host:port:database:user:password`, and took no lock.

No record decided that the migrator should be configured differently. It fell out of the command
form, and it broke a real deployment. A password file line binds the password to one host name,
which the driver matches literally. CloudNativePG writes such a line into the Secret it generates
for the database's owner with the Service's short name only, while the server certificate it
issues names the short name and the fully qualified forms. So a deployment that addresses the
database by its fully qualified name, the convention of the estate this system first deploys to,
failed at the migration with an authentication error while every other process connected.

ADR-0067 also left a concurrent run open. The command takes no lock when it applies a chain, and
[ADR-0129](./0129-the-chart-runs-the-migration-step-as-a-pre-install-and-pre-upgrade-hook-job.md)
records that two runs of the migration step can still overlap, a guard against which "is a change
to the migrator, which would take that lock".

ADR-0067's reasons for the command form, each checked against the library form.

| ADR-0067's reason | Whether it holds for the library form |
| --- | --- |
| The command form keeps goose's tree, every database it supports, out of this project's binaries | The root package `github.com/pressly/goose/v3` imports no database driver. Linking it with its PostgreSQL session locker adds to a binary only goose's own packages and three small modules, `mfridman/interpolate`, `sethvargo/go-retry` and `go.uber.org/multierr`, beside `golang.org/x/sync`, which the driver's pool already links. The exclusion tags that kept the other databases out of the command are not needed |
| A `tool` directive takes part in version selection and raises shared dependency versions inside the project's binaries | It holds, and a library requirement does the same. Adding goose 3.29.0 raised `prometheus/procfs`, which every deployable links, `segmentio/asm`, which the API server links through the MCP SDK, `jackc/puddle`, `golang.org/x/net`, `golang.org/x/sync`, `golang.org/x/sys`, `golang.org/x/text` and `golang.org/x/tools`. Each moved to a newer release, every one but `golang.org/x/tools` to its newest, which the rule that every dependency is at its latest version asks for anyway ([CLAUDE.md](../../../CLAUDE.md#tools-and-versions)) |
| No requirement orders the field, and the choice is reversible by changing a command | It holds for the choice of tool, which this record keeps. Nothing in ADR-0067 argued for the migrator being configured differently |
| The step is distroless with no shell, so it cannot assemble a string from files (ADR-0129) | It holds, and it is why a fix in the chart alone could not use a key holding the password alone |

### The requirements

The requirements ADR-0067 graded the field on still hold, and its grid and reading carry over
unchanged.

| Requirement | What it demands | Source |
| --- | --- | --- |
| Hand-authored as the default path | The tool's own documentation treats hand-written SQL as the ordinary way to use it, not as a secondary mode beside generating migrations from a declarative schema | [ADR-0048](./0048-forward-only-migrations.md) |
| Up-only without ceremony | A migration that cannot be reversed is expressed by writing nothing, rather than by leaving something empty or relying on a file's absence | ADR-0067's refinement of [ADR-0048](./0048-forward-only-migrations.md)'s forward-only rule |
| No database-resident code | The tool installs no function, trigger or procedure. An ordinary version table is fine | [ADR-0060](../engineering/0060-no-code-in-the-database.md) |
| The generator reads its layout | The code generator takes the same files as its schema input, so one artifact feeds both the database and the code | [ADR-0066](./0066-data-access-generated-from-sql.md) |
| Runner binary footprint | What the distributed binary carries into a step that runs beside a system using one database engine | [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture, applied to a step that runs outside the runtime |
| Long-term fit | Governance, and whether the capabilities relied on stay available | The project outlives any tool it picks |

## Decision

- **goose applies the migration chain, linked as a library into the migrator.** The grid under
  Alternatives is ADR-0067's, and goose is preferred on the same two grounds: its pairing with the
  code generator is documented by both projects, and its footprint gotcha is removable. Linked as a
  library the gotcha is gone rather than removed, since the root package imports no driver.
- **The migration chain stays plain SQL in `db/migrations/`, and the migrator's binary embeds it.**
  The Go package `db/migrations` embeds every SQL file of its directory. The code generator reads
  only the SQL files there as its schema, and goose reads only the embedded SQL files.
- **One package applies a chain, `db/migrate`, and goose is admitted to it alone.** It applies
  every migration the database does not yet hold through goose's provider, with goose's PostgreSQL
  session locker and goose's own global registry of Go migrations switched off, logging through
  the logger it is handed. The session locker takes a PostgreSQL advisory lock on a connection of
  its own before it reads what is applied, so a second run at the same time waits for the first
  and then finds nothing pending.
- **The migrator is a Go component with a composition root shaped like every deployable's,**
  `migrator/main.go` and `migrator/app`, and the host of
  [ADR-0138](../engineering/0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)
  runs it, as it runs every process. Its root hands the host its root type, `log_level` in the
  process's section and a `database` section, and its one piece of work, and the host runs it as a
  process that runs once. The host starts it as every process starts. It refuses the start while
  `PGPASSWORD` or `PGSSLPASSWORD` is set, before the configuration is read, reads the configuration
  through the configuration library, answers `--help` and `--version`, logs every effective value
  before it sets the level, validates, and writes the start banner to standard error. The work
  connects through the pool the host opens from the database section, with the password from the
  mounted file the section names, and applies the chain the binary embeds through `db/migrate`. The
  section's `user` defaults to the migration role, `mediated_mailbox_migrator`. The process serves
  no operations endpoints and no application server, since nothing probes, scrapes or calls a Job
  that exits on its own.
- **The integration runs apply the chain through `db/migrate`**, the embedded chain for the
  template every test database is created from, so every integration run applies the chain through
  the code that ships.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| Hand-authored as the default path | Its documentation's ordinary path is a directory of SQL files, and it offers no schema-diffing mode to drift toward |
| A concurrent index build is available if one is ever wanted | A per-file `NO TRANSACTION` directive. Not graded, because every candidate offers the capability |
| Up-only without ceremony | One file carrying one annotation, where the annotation marking the reverse direction may simply be absent |
| No database-resident code | One ordinary version table, and the lock is PostgreSQL's built-in advisory lock, which installs nothing |
| The generator reads its layout | Documented by sqlc and by goose. The Go file beside the chain is not SQL, and the generator reads only the SQL files |
| Runner binary footprint | Native in the library form. The binary links goose's own packages and three small modules, and no driver of another database |
| Long-term fit | An organisation with a lead maintainer and secondary contributors, and no capability relied on here sits behind a licence |

## Alternatives considered

ADR-0067 graded six runners on a four-level scale, and the grades stand. **4** means the candidate
carries the requirement natively. **3** means it is carried with bounded discipline or a named
gotcha. **2** means it is carried only against the tool's own grain. **1** means it cannot honestly
satisfy it. The grades are of each tool as distributed, so goose's footprint cell is the command's.

| Requirement | goose | golang-migrate | tern | dbmate | atlas | sql-migrate |
| --- | --- | --- | --- | --- | --- | --- |
| Hand-authored as the default path | 4 | 4 | 4 | 4 | 2 | 4 |
| Up-only without ceremony | 4 | 3 | 4 | 3 | 4 | 4 |
| No database-resident code | 4 | 4 | 4 | 4 | 3 | 4 |
| The generator reads its layout | 4 | 4 | 3 | 4 | 2 | 3 |
| Runner binary footprint | 3 | 3 | 4 | 4 | 1 | 1 |
| Long-term fit | 3 | 3 | 3 | 3 | 2 | 2 |

Four candidates have no cell below 3, and three of them, goose, tern and dbmate, are level. The
grid does not break that tie. goose is preferred because its pairing with the generator is
documented by both projects, where every other candidate's is documented by one side only, and
because its gotcha is removable where dbmate's is fixed in its file format. A reader who weighs
dependency surface in the step that holds schema-owning rights above both grounds lands on tern,
and nothing here refutes that reading. atlas is rejected because its headline workflow generates
migrations by diffing a declarative schema, the shape ADR-0048 rejects by name, and that mode is
moving behind a login. sql-migrate is rejected because its binary links a C SQLite build into a
system using only PostgreSQL. Applying the chain with a hand-written runner had no case tabled for
it, since every candidate provides the version table and the ordering.

How the migrator takes its configuration had alternatives of its own, each of which kept goose as a
command.

- **goose as a command, as ADR-0067 decided.** The case for it is that the migrator holds none of
  this project's Go code and swapping runners is swapping a command. It loses because the command
  takes its connection as one string, so the migrator stays configured differently from every
  other process, its password in a file format that binds a host name, with no refusal of the
  driver's password variables, no JSON logs and no lock.
- **The short Service name as the host.** The case for it is that it needs no change at all. It
  loses because the deployment then addresses one process differently from the others, against
  its own convention, and the migrator stays configured differently.
- **A password file the user builds, its host field a `*` or the fully qualified name.** The case
  for it is that the chart is unchanged. It loses because every deployment carries a second Secret
  for one process, so the difference moves to the user rather than going away.
- **An init container with a shell writing the password file.** The case for it is that only the
  chart changes. It loses because it adds an image with a shell, outside the lockstep set of
  [ADR-0049](../engineering/0049-image-per-component-lockstep.md), to the step that owns the
  schema, against [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture, and a
  second place that formats credentials.
- **The password in the connection string from the Secret.** The case for it is that only the
  chart changes. Rejected by [ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md),
  which forbids a password in the environment.
- **A Helm lookup of the Secret to render the password file.** The case for it is that only the
  chart changes. It loses because it writes the password into the release's manifest, and it
  fails under `helm template` and Argo CD.
- **A `migrate` subcommand of the command-line program, run from the migrator's image.** The case
  for it is that the program is a Go entry that already exists. It loses because the image would
  then carry the key-generation subcommand too, against ADR-0049's exact contents, and the
  program is the operator's tool rather than a step.
- **The migrator's root doing the start itself, outside the host.** The case for it is that the
  host's other work, the probes, the metrics endpoint, the application server and the background
  tasks, has no use in a Job that runs once, so the root could call the configuration library, the
  database connection package and the logger directly in the host's order. It loses because the
  start would then be written twice, and a copy drifts on its own, which is the case
  [ADR-0138](../engineering/0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)
  makes for the host. The host instead runs a process that runs once with the same start and no
  servers.
- **The migrator run by the host as a process that runs until it is stopped.** The case for it is
  no new way to run a process. It loses because the Job would serve probes and a metrics endpoint
  nothing reads, for a process that has to end once its work is done.
- **The migrator's configuration in a file from a ConfigMap that a hook creates first.** The case
  for it is that the migrator would read a file exactly as the others do. It loses because it adds
  one more hook object with its own ordering and deletion, and the Job would read an object the
  chart renders, against ADR-0129. The environment layer is one ADR-0078 gives every process, and
  the configuration library makes the variable names the file's keys by construction.

## Consequences

- **The migrator's configuration is every process's.** A deployment gives it the server, the
  database name, the TLS mode, the CA file and a password file holding the password alone, the
  same as the API server, the UI and the worker. CloudNativePG's generated Secret for the
  database's owner works through its `password` key, and the host may be named in any form the
  server's certificate carries.
- **Two runs at once apply each migration once.** The second waits for the advisory lock. goose's
  session locker retries the lock every five seconds for five minutes, so a run that waits longer,
  behind a migration that takes longer, fails, and the Job's retries of ADR-0129 run it again.
- **The migrator's image holds one static binary,** built like every Go image of ADR-0049, and the
  chain is inside it.
- **A goose release moves the project's dependency graph.** goose's requirements take part in the
  module's version selection, so a goose update can move a module the deployables link. Renovate
  tidies the module after a Go dependency update, and the lint workflow fails on a module that is
  not tidy ([CLAUDE.md](../../../CLAUDE.md#tools-and-versions)).
- **No goose command is pinned.** Developers and CI apply the chain through `db/migrate`.
- **Changing the runner is changing `db/migrate`**, because the migration files outlive any runner
  and the generator reads the layout of every candidate this record keeps in play.
- **Assumptions about other components.** Something outside the application grants the
  schema-owning role to the migration step and withholds it from the runtime roles. The deployment
  mechanism runs the step before rolling the deployables. The extensions the schema needs are
  trusted, so the chain's first migration creates them under the migration role, which
  [ADR-0048](./0048-forward-only-migrations.md) decides. The migrator connects through pgx's
  `database/sql` adapter over the pool the host opens as `process/dbconnect` configures it, and
  goose runs each migration on the connection that holds the lock.
- Each rule above that a test can break is a control, with its violation injection in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
