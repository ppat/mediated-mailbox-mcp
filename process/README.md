# process

A family, one narrow, named shared library holding several packages of one concept, published as
`mediated-mailbox-process`. Shared code is pure, or it is a library like this one that argues its
own case ([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its
case. The conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Its concept is the [process](../DESIGN.md#glossary), what every deployable needs to run as a
process under the environment contract
([ADR-0051](../docs/adr/engineering/0051-environment-contract.md)), which is its configuration
layered from defaults, a file, the environment and flags, its database connection, its probe
and metrics endpoint, and its logs. The decision it hides is how a process meets that contract,
which changes only when the contract or the platform under it does, and never with what the process
runs.

| Package | Holds |
| --- | --- |
| `settings/` | The layering of defaults, one optional configuration file, environment variables and flags into each deployable's configuration |
| `dbconnect/` | A deployable's database section of its configuration, in `dbconnect/core`, and the connection built from it |
| `probes/` | The health probe and the metrics endpoint of a process that runs work rather than serving requests |
| `logging/` | The logger every deployable writes its JSON records to standard output through, and the reading of its `log_level` |

Each package is admitted to each component by its own entry in that component's import list, and
each has a list of its own in `.golangci.yaml`. A package added here without a list of its own falls
under the list over files outside every component, which admits only the standard library.

**What does not belong.** A concern's own configuration section stays with its concern
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), so the credential section
is the execution context family's and the scanner's section is `core/scan`'s, while this family
holds only the layering mechanism and the database section, whose whole concern is running as a
process. Nothing here reads an account, a policy or a message, and nothing here runs a statement.
The mediator's and the UI's probes stay with them, because each answers `/readyz` from state only
that deployable holds.

## The configuration library, `settings`

Every deployable takes its configuration the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).
Defaults, one optional YAML file, environment variables under `MEDIATED_MAILBOX_` and command-line
flags layer per value in that order, each value named once by its YAML key path, and every mistake
in any layer is refused at start. This package is that mechanism, and nothing else. Each concern's
configuration type, its defaults and its validation stay with the concern, and a deployable's
composition root hands the package its arguments, its environment and the root type.

The case for one package over per-deployable glue is the refusals. Configuration that is misspelled,
duplicated, case-changed, of the wrong type or aimed at a key that does not exist has to stop the
process, because a setting that silently falls back to its default is the failure the fail-closed
pillar forbids. The refusals are the rules glue gets wrong. Copies in each deployable would drift,
and one copy that forgot to refuse unknown keys or to match environment names exactly would fail
open. Written once, with a test per refusal and a mutation patch for each refusal the package's own
code carries, they hold in every deployable.

The composition root passes the package its arguments and its environment, and the package reads
the one file they name. It returns the decoded configuration, each value's source and a revision
per concern, or an error naming the file and line, the environment variable or the flag at fault.
It holds no package-level state, and it holds no secret, since a value concerning a secret names a
mounted file.

## The database connection, `dbconnect`

Every deployable that connects to the database takes its connection settings as one `database`
section of its configuration, and connects the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). This
package is that section and that connection. The section's type and its validation are pure and sit
in `dbconnect/core`. The shell renders every setting into the connection string, reads the password
from the mounted password file
([ADR-0079](../docs/adr/operability/0079-secrets-arrive-as-mounted-files.md)) and refuses a start
while `PGPASSWORD` or `PGSSLPASSWORD` is set. Each deployable supplies its own defaults, the user
being its own runtime role
([ADR-0075](../docs/adr/data/0075-one-runtime-role-per-deployable.md)).

The case for one package over per-deployable glue is the database driver's fallbacks. pgx reads its
`PG*` environment variables on every parse, with no option to stop it. A copy that left a setting
out of the connection string, an empty `sslrootcert` included, would take that setting from the
environment. A copy that did not refuse `PGPASSWORD` would let a stray one win over the mounted
password file, and one that trimmed the file's text differently would connect with a different
password. A copy that validated less would let the driver substitute a default the operator never
chose, such as its socket directory for an empty host. Each of these fails silently. Written once,
with a test and a mutation patch for each rule, they hold in every deployable that connects.

The package connects as no role of its own and runs no statement. It returns the connection's
configuration, and the deployable opens the pool.

## The probe and metrics listener, `probes`

Backfill and delta sync each serve a health probe and their metrics endpoint on a listener of their
own while they run ([ADR-0051](../docs/adr/engineering/0051-environment-contract.md),
[ADR-0103](../docs/adr/operability/0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)).
`/healthz` answers 200 while the process runs, and `/metrics` serves the registry the composition
root passes in, so every series a process registers reaches its scrape, between units of work as
during them. Both composition roots held the same listener, and the probe the platform reads is the
contract a copy could quietly break, a path renamed or a registry other than the process's own
served. Written once, the endpoints are the same in every process that runs work. The listener is
stopped by the function it returns, which shuts it down within five seconds.

## The logs, `logging`

Every deployable writes its logs as JSON to standard output at the level its `log_level` sets
([ADR-0122](../docs/adr/engineering/0122-logs-through-slog-at-a-configured-level-handed-to-shells.md)).
This package builds that logger over a level variable, reads the level's name, and makes the error
log a shell gives an `http.Server` from the logger it was handed. A deployable's `main.go` builds
the logger, and its entry package sets the level once the configuration has loaded.

The case for one package over per-deployable glue is that the format and the level names are the
contract. A copy that wrote text rather than JSON, wrote to standard error, or accepted `warning`
or `INFO` where the others refuse it would make one deployable's logs read differently from the
rest, or start where the others refuse. Written once, with a test per level name and a mutation
patch for the refusal, they hold in every deployable.

It holds no logger of its own and no package-level state, and it logs nothing.
