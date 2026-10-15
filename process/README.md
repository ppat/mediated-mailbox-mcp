# process

A family, one narrow, named shared library holding several packages of one concept, published as
`mediated-mailbox-process`. Shared code is pure, or it is a library like this one that argues its
own case ([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its
case. The conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Its concept is the [process](../DESIGN.md#glossary), what every deployable needs to run as a
process under the environment contract
([ADR-0051](../docs/adr/engineering/0051-environment-contract.md)), which is its configuration
layered from defaults, a file, the environment and flags, its database connection and what it
measures of that connection, what it measures of its runtime, its operations endpoints, its logs,
the release version it reports, and the host that runs it over the components it hosts. The decision it hides is
how a process meets that contract, which changes only when the contract or the platform under it
does, and never with what the process runs.

| Package | Holds |
| --- | --- |
| `settings/` | The layering of defaults, one optional configuration file, environment variables and flags into each deployable's configuration |
| `dbconnect/` | A deployable's database section of its configuration, in `dbconnect/core`, and the connection built from it |
| `dbmetrics/` | The opening of a deployable's measured pool, the latency of every statement a connection runs, by the statement's name, and the statistics of each connection pool |
| `runtimemetrics/` | The registration of the Go runtime's and the process's series, once per process |
| `probes/` | The operations endpoints of every process that runs until it is told to stop, the health and readiness probes and the metrics endpoint |
| `host/` | The host, which runs one process over the components a composition root hands it ([ADR-0138](../docs/adr/engineering/0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)) |
| `logging/` | The logger every deployable writes its JSON records to standard output through, and the reading of its `log_level` |
| `version/` | The release version every binary reports through `--version` and every deployable's start banner shows |

Each package is admitted to each component by its own entry in that component's import list, and
each has a list of its own in `.golangci.yaml`. A package added here without a list of its own falls
under the list over files outside every component, which admits only the standard library.

**What does not belong.** A concern's own configuration section stays with its concern
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), so the credential section
is the execution context family's and the scanner's section is `core/scan`'s, while this family
holds only the layering mechanism and the database section, whose whole concern is running as a
process. Nothing here reads an account, a policy or a message, and nothing here runs a statement.
Readiness is a component's, because each component answers it from state only it holds, so a
component hands the host its readiness checks and the host serves `/readyz` over them. No component
is imported here, and the host knows a component only by the parts it is handed. The execution
context a process's components share is the execution context family's, which the composition root
builds once and whose reload it hands the host as a task, so the host holds no account, no policy
and no credential.

## The configuration library, `settings`

Every deployable, and the migrator, takes its configuration the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).
Defaults, one optional YAML file, environment variables under `MEDIATED_MAILBOX_` and command-line
flags layer per value in that order, each value named once by its YAML key path, and every mistake
in any layer is refused at start. This package is that mechanism, and nothing else. Each concern's
configuration type, its defaults and its validation stay with the concern, and the host hands the
package the arguments, the environment and the root type a composition root hands it. A root type is
composed from sections, a section embedded inline sitting beside the embedding section's own values,
so a root keeps the process's keys and its component's at the top of its tree.

The case for one package over per-deployable glue is the refusals. Configuration that is misspelled,
duplicated, case-changed, of the wrong type or aimed at a key that does not exist has to stop the
process, because a setting that silently falls back to its default is the failure the fail-closed
pillar forbids. The refusals are the rules glue gets wrong. Copies in each deployable would drift,
and one copy that forgot to refuse unknown keys or to match environment names exactly would fail
open. Written once, with a test per refusal and a mutation patch for each refusal the package's own
code carries, they hold in every deployable.

The package is passed the arguments and the environment, and reads the one file they name. It returns the decoded configuration, each value's source and a revision
per concern, or an error naming the file and line, the environment variable or the flag at fault.
It holds no package-level state, and it holds no secret, since a value concerning a secret names a
mounted file.

## The database connection, `dbconnect`

Every process that connects to the database, every deployable and the migrator, takes its
connection settings as one `database` section of its configuration, and connects the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). This
package is that section and that connection. The section's type and its validation are pure and sit
in `dbconnect/core`. The shell renders every connection setting into the connection string, caps the
pool at the section's pool size, reads the password from the mounted password file
([ADR-0079](../docs/adr/operability/0079-secrets-arrive-as-mounted-files.md)) and refuses a start
while `PGPASSWORD` or `PGSSLPASSWORD` is set. Each deployable supplies its own defaults, the user
being its own runtime role, and the API server and the UI take the pool size the shell gives as the
driver's own default, as does the migrator, whose user is the migration role
([ADR-0139](../docs/adr/data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md)). The
worker names the server once and gives each job kind a user of its own, its job kind's runtime
role, with a password file and a pool size of its own
([ADR-0118](../docs/adr/data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)).

The case for one package over per-deployable glue is the database driver's fallbacks. pgx reads its
`PG*` environment variables on every parse, with no option to stop it. A copy that left a setting
out of the connection string, an empty `sslrootcert` included, would take that setting from the
environment. A copy that did not refuse `PGPASSWORD` would let a stray one win over the mounted
password file, and one that trimmed the file's text differently would connect with a different
password. A copy that validated less would let the driver substitute a default the operator never
chose, such as its socket directory for an empty host. Each of these fails silently. Written once,
with a test and a mutation patch for each rule, they hold in every deployable that connects.

The package connects as no role of its own and runs no statement. It returns the connection's
configuration, and the host opens the pool.

## The database measurements, `dbmetrics`

Every deployable that connects measures its statements and its pools the same way
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
This package holds a pgx query tracer, a collector over a pool, which reads the pool's statistics
when it is scraped, and the function that opens a pool from its configuration with both, which the
host calls to open every pool a composition root asks for. Both register on the registerer they
are handed, which carries the component's name and, in the worker, the job kind.

| Series | Kind and labels | Value |
| --- | --- | --- |
| `mediated_mailbox_db_statement_duration_seconds` | Histogram, `statement` | Each statement's latency, from its start until its result or its rows are closed |
| `mediated_mailbox_db_pool_acquires_total` | Counter | The connections acquired from the pool |
| `mediated_mailbox_db_pool_waited_acquires_total` | Counter | The acquires that found no idle connection and waited, for one to be released or for a new one to be built |
| `mediated_mailbox_db_pool_acquire_wait_seconds_total` | Counter | The time those acquires waited, a new connection's connect time included |
| `mediated_mailbox_db_pool_new_connections_total` | Counter | The connections the pool built, which tell construction waits from waits at the ceiling |
| `mediated_mailbox_db_pool_canceled_acquires_total` | Counter | The acquires whose caller gave up waiting |
| `mediated_mailbox_db_pool_acquired_connections` | Gauge | The connections held now |
| `mediated_mailbox_db_pool_max_connections` | Gauge | The pool's ceiling |

The case for one package over per-deployable glue is the statement label. It is the name in the
`-- name:` comment the statement's SQL starts with, `begin`, `commit` or `rollback` for a statement
without one that starts with that keyword, and `unnamed` for every other, so the label never takes
a value from the SQL's text and its values stay a closed set. A copy that labelled by the text, or
that read the name differently, would grow a series per distinct statement text or split one
statement's series in two, and either fails silently. Written once, with a test per rule, the label
means the same in every deployable. Opening the pool here, rather than in each root, keeps every
pool traced and collected alike, which a root that set the tracer and forgot the collector would
not be.

## The runtime measurements, `runtimemetrics`

Every deployable registers the Go collector's default set with four runtime metrics added to it,
the live heap, the garbage collector's and the process's CPU time and the scheduling latency, and
the process collector, once on its process's registry
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
The host calls this package to do it, on the process's own registry, not a component's or a job
kind's, so these series carry neither a component nor a job kind.

The case for one package over per-deployable glue is the set of runtime metrics. It is named by one
expression, and a copy in each root whose expression drifted would serve a different set from one
deployable with every gate green. Written once, with the test that pins which runtime metrics are
added and which are left out, the set is the same in every deployable.

## The operations endpoints, `probes`

Every process that runs until it is told to stop serves its health and readiness probes and its
metrics endpoint on a listener of its own while it runs
([ADR-0051](../docs/adr/engineering/0051-environment-contract.md)). `/healthz` answers 200 while
the process runs, `/readyz` answers 200 only while the readiness it is handed holds and 503 with a
line saying why otherwise, and `/metrics` serves the registry it is handed, so every series a
process registers reaches its scrape, between units of work as during them. Every answer is plain
text for a platform's healthcheck and scraper. The endpoints the platform reads are the contract a
copy could quietly break, a path renamed or a registry other than the process's own served. Written
once, the endpoints are the same in every process that serves them. The host serves them.

## The host, `host`

The host runs one process over the components a composition root hands it
([ADR-0138](../docs/adr/engineering/0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)).
It refuses the environment, loads the configuration and logs it, sets the level, writes the start
banner, builds the one registry and the runtime series, opens every pool a root asks for, assembles
each component with a logger and a registerer carrying its name, serves the application server and
the operations endpoints under one set of connection limits, starts and recovers each component's
background tasks and every goroutine a component asks its starter for while it runs, and stops in
order, joining every task and goroutine before the pools close. Its sections of the configuration
are `host.Process`, every process's, `host.Operations`, a process's that runs until it is told to
stop, and `host.Serving`, a process's whose components serve requests. `New`, `Add` and `Serve`
are its lifecycle without the configuration, which a test or a harness that composes a component
itself serves through. `RunOnce` runs a process that does one piece of work and exits, as the
migrator does, with the same start, registry and pools, and no servers or tasks, since nothing
probes, scrapes or calls a process that exits on its own ([ADR-0139](../docs/adr/data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md)).

The case for one package over the same work written in each root is that each copy drifts on its
own. A copy can answer its probes its own way or answer no readiness, load the key pair twice,
start a goroutine it neither recovers nor joins, and a harness composing a component by hand can run
it without the limits its root serves it under. Written once, with a test per responsibility and a
mutation patch for each rule it carries, a probe, a limit, a stop order or a refusal holds in every
process, a process hosting several components among them.

## The logs, `logging`

Every deployable, and the migrator, writes its logs as JSON to standard output at the level its
`log_level` sets
([ADR-0122](../docs/adr/engineering/0122-logs-through-slog-at-a-configured-level-handed-to-shells.md)).
This package builds that logger over a level variable, reads the level's name, and makes the error
log a shell gives an `http.Server` from the logger it was handed. A deployable's `main.go`, and the
migrator's, builds the logger, and the host sets the level once the configuration has loaded.

The case for one package over per-deployable glue is that the format and the level names are the
contract. A copy that wrote text rather than JSON, wrote to standard error, or accepted `warning`
or `INFO` where the others refuse it would make one deployable's logs read differently from the
rest, or start where the others refuse. Written once, with a test per level name and a mutation
patch for the refusal, they hold in every deployable.

It holds no logger of its own and no package-level state, and it logs nothing.

## The release version, `version`

Every binary the project publishes reports the release it was built for through its `--version`
flag, every deployable names it in its start banner, and the API server's MCP server reports it as
its version
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md),
[ADR-0122](../docs/adr/engineering/0122-logs-through-slog-at-a-configured-level-handed-to-shells.md)).
This package holds that version as one constant. The release cut writes it. release-please's
generic updater, named in the `extra-files` of `release-please-config.json`, replaces the version
on the constant's marked line in the release pull request, the same pull request that writes the
chart's `version` and `appVersion` and the manifest release-please reads. The package's test fails
when the constant, the manifest and the chart disagree, so a misconfigured updater fails on that
pull request rather than in a release.

The case for one package over a constant per binary is lockstep
([ADR-0049](../docs/adr/engineering/0049-image-per-component-lockstep.md)). One constant is one
line the release cut writes, and copies in each binary would be four lines one updater entry could
miss. A binary built between releases, a `latest` image among them, reports the version of the
newest release at or before the commit it was built from, and its image's revision label names that
commit ([ADR-0052](../docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)).

It holds one constant, and does no I/O.
