# process

A family, one narrow, named shared library holding several packages of one concept, published as
`mediated-mailbox-process`. Shared code is pure, or it is a library like this one that argues its
own case ([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its
case. The conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

## What it is for

Its concept is the [process](../DESIGN.md#glossary), one running program, and what that process
holds. It does two jobs.

1. **It runs a program as a process.** Every program but the command-line program runs under the
   environment contract ([ADR-0051](../docs/adr/engineering/0051-environment-contract.md)). For
   that it needs its configuration, layered from defaults, a file, the environment and flags, its
   database pools and what it measures of them, what it measures of its runtime, its operations
   endpoints, its logs, the release version it reports, and the spec and the process that run it
   ([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).
2. **It holds what a process's endpoints and job kinds share**, one scheduler, one scanner and one
   [Session](../DESIGN.md#glossary), beside the database pools above. The Session is all the user
   state a process holds, which is the accounts, each account's policy and each account's
   connection to its provider.

It hides two decisions.

- **How a process meets the environment contract.** That changes only when the contract or the
  platform under it changes, never with what the process runs.
- **How a process builds, holds and reloads what its endpoints and job kinds share.** Each shared
  thing is built once, by one owner, in one way, whichever program asks. A read the process cannot
  trust never replaces what it holds. The credentials inside are opened, rotated and written back
  by one set of rules.

The Session's members sit together because they change together. The policies are loaded, held
and reloaded exactly as the accounts are, in the one reload the Session owns. The unit of work is
the one place the steps around an account's credential are written, which every program that calls
a provider would otherwise write for itself.

| Package | Holds |
| --- | --- |
| `process` itself | The spec and the process, `Spec`, `Endpoint`, `Job`, `Task`, `Capabilities`, `State` and `OnePerProcess`, `process.Run` and `process.RunContext`, the option a test substitutes a shared thing through, the revision of a top-level key read from the process, and an owner's upkeep on the scheduler, the build and the run, the servers, the operations endpoints, the runtime series, the stop order and the readiness reports ([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)) |
| `settings/` | The layering of defaults, one optional configuration file, environment variables and flags into the configuration type of each program that runs as a process, and the reading of a `config.Duration` |
| `database/` | The connection a pool is opened over, every pool, the latency of every statement a connection runs, by the statement's name, which its pure core `database/core` gives, the statistics of each pool, and the schema check, through `database.Get` and, for the migrator, `database.ForMigration`, and the readiness report of the process's one pool answering a ping |
| `schedule/` | The scheduler, which runs each job kind's plan, adding and dropping its jobs by key, and the helper job code starts a goroutine through ([ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) |
| `sessions/` | The Session, its three members, the reload, the order of the reload, its failure rules, and who is told after it |
| `accounts/` | The reading of the list of accounts |
| `policies/` | Each account's policy, the base policy included, the load concurrent body requests share, and the reload-failure alarm |
| `connections/` | Each account's connection to its provider, one record per account, the unit of work, the compare-and-set write-back of a rotated credential, and delta sync's re-seal |
| `scanner/` | The scanner, built once per process, and the body-processing histogram |
| `logging/` | The logger every program that runs as a process writes its JSON records to standard output through, and the reading of its `log_level` |
| `version/` | The release version every binary reports through `--version` and every process's start banner shows |
| `host/` | The host the API server runs on until it is on its spec ([ADR-0138](../docs/adr/engineering/0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)). It opens its pools through `database` and refuses the database driver's password variables through `process`, so neither is written twice |
| `probes/`, `runtimemetrics/` | The operations endpoints' handler and the registration of the runtime series, which `process` and `host/` share while both run programs |

Each package is admitted to each component by its own entry in that component's import list, and
each has a list of its own in `.golangci.yaml`, so joining this family never widens what any
component may import. A package added here without a list of its own falls under the list over
files outside every component, which admits only the standard library.

## What does not belong

- **The shape of the configuration.** The configuration type of each program that runs as a
  process, with its defaults and its checks, is `core/config`'s. This family holds the loader and
  no configuration type.
- **Any one program.** The `process` package itself knows nothing of accounts, providers, policies
  or any one program. No program is imported here, and the process knows an endpoint, a job kind or
  a task only by the function that builds it.
- **Reading a message.** Nothing here reads one.
- **The schema check's statement and comparison.** The statement is the data-access library's, and
  the comparison sits beside the migration chain. `database` runs the two on each pool.
- **Deciding readiness.** Readiness is reported, through `Report`, by what holds the state it
  answers from, so the probes answer from what was reported and run nothing.
- **A credential in the Session.** The Session's package imports no credential and no provider
  code. The connections attach to it as its third member, so code that holds the Session and not
  the connections cannot reach a credential.
- **Sealing and opening a credential.** Those are `credential/`'s.
- **Calling a provider and spending from the rate budget.** Those are the provider library's, which
  the connections reach through its calling side, `provider`. The rate limiter's leases are shared
  across processes, and it changes for different reasons, so it is the provider library's
  `provider/ratelimit`, never this family's.
- **Editing or importing the policy.** That is the UI's and `db/policyrules`', never `policies`',
  which only holds what was loaded.
- **Scanning.** That is `core/scan`'s, and `scanner` only builds the scanner.

The failure to watch for is the Session growing policy, scanning or leasing code, the connections
growing an adapter's or the rate limiter's code, or `policies` growing behaviour unrelated to
holding what was loaded. Each shows first as an entry its import list does not hold.

The statements of `accounts`, `policies` and `connections` run under the application user, in the
API server and the worker, which hold the Session
([ADR-0143](../docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)),
and the grant check plans each of them under it
([ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)). When a process reloads, and
what is put in force after a reload, are the Session's. Each member loads when asked, and tells a
read it cannot trust from one that found nothing.

## The configuration library, `settings`

Every deployable and the migrator take their configuration the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).
Defaults, one optional YAML file, environment variables under `MEDIATED_MAILBOX_` and command-line
flags layer per value in that order. Each value is named once, by its YAML key path, and every
mistake in any layer is refused at start. This package is that mechanism, and nothing else.

Each of their configuration types, its defaults and its checks are `core/config`'s. The process
hands this package the arguments, the environment and the configuration type its program's spec is
typed on. A program's type is composed of the shared parts it has and its own keys. A part embedded
inline sits beside the embedding part's own values, so a program keeps the process's keys and its
own at the top of its tree. A length of time is a `config.Duration`, a number of nanoseconds this
package fills from text such as `5m` and writes back as such text in the help, in the effective
configuration logged at start and in the rendering a section's revision is taken from.

The case for one package over per-deployable glue is the refusals. Configuration that is misspelled,
duplicated, case-changed, of the wrong type or aimed at a key that does not exist has to stop the
process, because a setting that silently falls back to its default is the failure the fail-closed
pillar forbids. The refusals are the rules glue gets wrong. Copies in each deployable would drift,
and one copy that forgot to refuse unknown keys or to match environment names exactly would fail
open. Written once, with a test per refusal and a mutation patch for each refusal the package's own
code carries, they hold in every program that runs as a process.

The package is passed the arguments and the environment, and reads the one file they name. It
returns the decoded configuration, each value's source and a revision per top-level key, or an
error naming the file and line, the environment variable or the flag at fault. It holds no
package-level state, and it holds no secret, since a value concerning a secret names a mounted
file.

## The database, `database`

Every process that connects to the database, every deployable and the migrator, takes its
connection settings as one `database` section of its configuration, `config.Database` in
`core/config`, and connects the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). This
package is the connection built from that section, the pools opened over it, their measurement and
the schema check.

| Function | What it returns |
| --- | --- |
| `database.Get` | A pool for the part that asks, opened the first time that pool is asked for and kept in the process. In a process that runs until it is told to stop, it runs the schema check on the pool before returning it, so a schema the process cannot run on stops the start before anything serves ([ADR-0140](../docs/adr/data/0140-each-migration-declares-the-oldest-chain-that-runs-on-it.md)) |
| `database.ReportReadiness` | Nothing but an error. It starts the readiness report of the process's one pool, the pool `database.Get` gives an endpoint, a task or a shared thing, so a program whose readiness rests on the database, the UI's, is ready only while it answers. The pool is pinged at once and then every five seconds, a ping that has not answered within two seconds counts as failed, and the report reads not ready from the moment it is asked for until a ping answers, and again from the first that fails, under the pools' own name, `database`. The ping runs on a goroutine of the process, which the process waits for before it closes the pools, and asked for again the report is not started twice. For a job kind's part it reports the job kind's own pool instead, under the job kind's name, so the worker is ready only while each job kind's pool answers and each pool's report is told apart from the others. The API server's pool does not run it, since the API server's readiness rests on its bearer token and the Session ([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)) |
| `database.ForMigration` | The migrator's pool, with no schema check. It takes only a configuration type with an unexported method that only `config.Migrator` has, so the compiler refuses a deployable's own configuration there. Left to review are a `Build` that makes a `config.Migrator` value of its own, by a literal, by its defaults or as a zero value, and, until `process/host` is deleted, `database.Open`, the opening the host uses with no schema check, `database.OpenPool`, which `database.Open` opens through and this package's own tests call, and a pool a deployable's code opens through the driver itself ([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)) |

A process holds one or more pools, and every one of them is this package's. In the worker each job
kind has a pool of its own, built for it and labelled with it, so one job kind cannot starve
another of connections
([ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
Every other pool is built from the process, with the process's own logger and registerer, as a
shared thing is, and its series carry no `job_kind`, in every process. Each pool registers its
series, the statement histogram and its statistics, as an unchecked collector, one whose
description yields nothing, so pools whose series carry different label names share one registry
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
A scrape still fails on two series of one name and one set of label values. The cost is the
registry's check against registering one collector twice, which it makes only for a checked
collector. So each pool `database.Get` or `database.ForMigration` gives is opened once, under the
pools' lock, and each pool the host opens through `database.Open` or `database.OpenPool` is opened
once by its root, until the host is deleted. `database.Get` reads from the
capabilities it is handed whether the part is a job kind, through `process.JobKind`, and keeps the
pools in a slot of the process, closed with what the other slots hold.

The connection renders every connection setting into the connection string, caps each pool at the
section's pool size, which in the worker sizes each job kind's pool alike
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)), and
reads the password from the mounted password file
([ADR-0079](../docs/adr/operability/0079-secrets-arrive-as-mounted-files.md)). The API server, the
UI and the worker connect as the application user, and the migrator as the migrator's user. Each
connects under the name its required `database.user` gives, which the deployment platform provides
([ADR-0143](../docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md),
[ADR-0139](../docs/adr/data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md)). The
worker names one database user, which every job kind's pool connects as. The API server, the UI
and the migrator take the pool size the connection gives as the driver's own default, and each of
the worker's job kind pools defaults to 4 connections
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).

The case for one package over per-deployable glue is the database driver's fallbacks. pgx reads its
`PG*` environment variables on every parse, with no option to stop it. A copy that left a setting
out of the connection string, an empty `sslrootcert` included, would take that setting from the
environment, and one that trimmed the password file's text differently would connect with a
different password. A copy that validated less would let the driver substitute a default the
operator never chose, such as its socket directory for an empty host. Each of these fails silently.
Written once, with a test and a mutation patch for each rule, they hold in every deployable that
connects. The refusal of `PGPASSWORD` and `PGSSLPASSWORD` is not this package's but the process's,
before the configuration is read
([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).

### The database measurements

Every deployable that connects measures its statements and its pools the same way
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
This package holds a pgx query tracer, and a collector over a pool that reads the pool's statistics
when it is scraped, and opens every pool with both. Both register on a registerer that carries the
component's name and, for a job kind's pool in the worker, the job kind.

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
without one that starts with that keyword, and `unnamed` for every other. So the label never takes
a value from the SQL's text, and its values stay a closed set. A copy that labelled by the text, or
that read the name differently, would grow a series per distinct statement text or split one
statement's series in two, and either fails silently. Written once, with a test per rule, the label
means the same in every deployable. Opening the pools here, rather than in each program, keeps every
pool traced and collected alike, which a program that set the tracer and forgot the collector would
not do.

## The spec and the process, `process`

This package holds the types
[ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md) defines,
`Spec`, `Endpoint`, `Job`, `Task`, `Capabilities`, `State` and `OnePerProcess`, and
`process.Run`, which runs one spec as a `main.go` does and exits with status 1 when the run fails. What a spec lists, what the process hands each part it builds,
and what the process does from start to stop are that record's. The process serves the application
server and the operations endpoints under one set of connection limits
([ADR-0130](../docs/adr/operability/0130-the-api-server-bounds-what-one-request-can-make-it-hold.md)).
The shared parts of the configuration it reads, `config.Process`, `config.Operations` and
`config.Serving`, are `core/config`'s.

A test or a harness runs a program through its spec and `process.RunContext`, which runs it as
`process.Run` does until a context it is given ends, with the writers it is given, and returns the
run's error rather than exiting. It substitutes a shared thing as
[ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md) states,
through the options it is handed. The one kind of option fills an owner's slot before anything
asks, and an owner exports one function returning it, built by its slot's `Substitute`.
`process.Run` takes no option, so a `main.go` cannot substitute.

Two more things a shared thing reads from the process rather than from whoever asked.
`process.Revision` gives the revision the configuration library computed for a top-level key, which
the scanner is built under. `process.Upkeep` has the process's scheduler run a job as the shared
thing's own upkeep, asked for from the build of its slot while the process is built, such as the
Session's reload. An upkeep job belongs to no job kind, so the scheduler's series observe none of it,
and a process whose scheduler runs only upkeep registers none of them
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
A program that does one piece of work runs no scheduler, so upkeep is refused there, as are a second
upkeep for one shared thing and one asked for once the process has been built. A job kind may not
share its name with an upkeep, since the two would be one job.

The case for one package over the same work written in each program is that each copy drifts on its
own. A copy can answer its probes its own way or answer no readiness, load the key pair twice, start
a goroutine it neither recovers nor joins, or build a shared thing its own way and at its own depth.
A harness that puts a program together by hand can run it without the limits the program serves
under. Written once, with a test per responsibility and a mutation patch for each rule it carries,
a probe, a limit, a stop order or a refusal holds in every process, including a program whose spec
joins several programs' specs.

### The operations endpoints

Every process that runs until it is told to stop serves its health and readiness probes and its
metrics endpoint on a listener of its own while it runs
([ADR-0051](../docs/adr/engineering/0051-environment-contract.md)).

| Path | Answers |
| --- | --- |
| `/healthz` | 200 while the process runs |
| `/readyz` | 200 only while the process serves and everything that reports is ready, and otherwise 503 with a line naming the first that is not |
| `/metrics` | The process's registry, so every series a process registers reaches its scrape, between units of work as during them |

The probes answer from what was reported and run nothing. Every answer is plain text for a
platform's healthcheck and scraper. The endpoints the platform reads are the contract a copy could
quietly break, by renaming a path or serving a registry other than the process's own. Written once,
the endpoints are the same in every process that serves them.

### The runtime measurements

Every deployable registers the Go collector's default set with four runtime metrics added to it,
the live heap, the garbage collector's and the process's CPU time and the scheduling latency, and
the process collector, once on its process's registry
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
The process registers them on its registry itself, which carries no label, rather than through a
registerer, which adds the program's name and for a job kind the job kind. So these series carry
neither a component nor a job kind.

The case for one package over per-deployable glue is the set of runtime metrics. It is named by one
expression, and a copy in each program whose expression drifted would serve a different set from
one deployable with every gate green. Written once, with the test that pins which runtime metrics
are added and which are left out, the set is the same in every deployable.

## The scheduler, `schedule`

The process holds one scheduler. It runs every job kind the spec lists, and an owner uses it for
its own upkeep through `process.Upkeep`, as the Session runs its reload on it
([ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md),
[ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).

A job kind's `Build` returns its plan, `schedule.Plan`.

```go
schedule.Plan{
    Keys    func(ctx context.Context) ([]string, error) // the accounts, or the plans, to run for now
    Recheck time.Duration                               // how often Keys is read again
    Backoff schedule.Backoff                            // the listing's backoff after a failure
    Late    time.Duration                               // how long after its latest success the listing is late
    Job     func(key string) (schedule.Job, error)      // the job for one key
    Dropped func(key string)                            // what follows when a key is no longer listed
    Woken   func(wake func())                           // handed what asks Keys again at once
}
```

A key is an account for backfill and delta sync, and a plan for apply and rollback. A job kind that
calls a provider lists the accounts the Session holds workable. The scheduler keeps one job per key
the plan lists, adds and drops jobs as the list changes, and reports the job kind not ready, as
`<job kind> listing`, until its first listing has succeeded. The listing is itself a job of the job
kind, asked at once, then every `Recheck` and whenever the function `Woken` hands out is called,
which a plan gives to what changes its keys, such as the Session's reload. A key the listing no
longer finds is told to `Dropped` only once its job has returned, and an empty key fails the
listing, since the listing's own job is keyed by the job kind alone. A plan with no `Keys` or no
`Job` is refused when the process hands it to the scheduler, which fails the start before anything
serves, with an error naming the job kind. While it runs the scheduler has no failure of its own,
since a job's failure fails its run and a listing that fails is asked again after its backoff. How a
job is woken, run, backed off and stopped is
[ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)'s.

The worker's job code starts a goroutine only through the scheduler's helper, `schedule.Go`, which
recovers a panic on it and returns it as the run's error, so it fails that run and stops nothing
else. The API server's and the UI's code, and a thing the process shares, start one only through
the process's `Go`, whose panic fails what started it and stops the process in order
([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). The
`go vet` goroutines analyser refuses, in the non-test code under `api/`, `ui/` and `worker/`, a
`go` statement and any use of a function that starts a goroutine running what it is given, so that
code starts one only through one of the two helpers. It refuses a call of neither, and a job kind's
`Build` is handed the process's `Go` too, so which helper code uses is held by review, as is a
goroutine a shared thing starts, since the analyser does not read this family's code.

The case for one package over each job kind's own code is that adding and dropping jobs as the list
changes, and reporting a first listing, would otherwise be written once per job kind, each copy
drifting on its own.

## The Session, `sessions`

A process holds one Session, which every endpoint and job kind of the process may ask for through
`sessions.Get`. Until the worker and the API server are each on its spec, they hold the execution
context of `executioncontext/` and reload it themselves, and `sessions`, `accounts`, `policies`,
`connections` and `scanner` are built and tested beside it and used by no program. The UI never holds it, since the UI's configuration type has no accounts part
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md),
[ADR-0081](../docs/adr/operability/0081-credentials-sealed-to-a-public-key.md),
[ADR-0084](../docs/adr/mutation/0084-ui-writes-decisions-and-account-setup.md),
[ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). It is
built from the process, with the process's own logger and registerer, and holds three members.

| Member | Held by |
| --- | --- |
| The accounts in force | [`accounts`](#the-accounts-accounts), which reads them |
| Each account's policy, the base policy included | [`policies`](#the-policies-policies) |
| Each account's connection to its provider | [`connections`](#the-connections-connections), which attach to the Session once, when they are built, and read each connected account's lowered rate target at each reload |

What a change in one member means for the others is the Session's to say.

- An account no longer listed loses its connection and its policy, and its jobs stop.
- No unit of work begins on an account the list in force does not hold.
- An account is workable only once it is listed, connected and its policy loaded, so no newly
  listed account is worked before its policy has loaded.
- A job kind that calls a provider works on the accounts the Session holds workable, and one that
  calls no provider on the accounts that have a policy.
- The workable accounts are a job kind's listing, so they answer as
  [ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)
  says a listing answers. Before a reload has put a list in force they are not due, an error
  wrapping the scheduler's, so a normal start logs no error. While the latest reload's read of the
  list or load of the connections failed they are that failure, until a later reload reads them. A
  reload whose policy load alone failed leaves them as the policy in force makes them.

Its reload is the Session's own upkeep, run on the process's scheduler on the schedule
[ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)
states, at the interval `account_reload_interval` sets in the API server and in the worker. It runs
at once when the process serves, and every five seconds, or the interval when it is shorter, while
no reload has loaded everything. Once one has, a reload that fails is logged by the Session and
answered to the scheduler as not due, so it is tried again at the next interval rather than after
the scheduler's backoff. Each reload its caller did not cancel is timed on one series per process
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
Its order is written once.

1. Read the list of accounts.
2. Load the connections for it.
3. Put the list in force.
4. Load the policy for it.
5. Tell those who asked, the scheduler's plans and the API server's registry of operations among
   them.

| What fails | What happens |
| --- | --- |
| The read of the list | What is in force stays. Nothing loads and no one is told, and the reload returns the error |
| The connections' read of the credentials or the clients | What is in force stays, the list included. Nothing more loads and no one is told, and the reload returns the error |
| One credential or client secret that does not open | That account has no connection, and the others load ([ADR-0091](../docs/adr/data/0091-accounts-listed-apart-from-their-state.md), [ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)) |
| One account's lowered rate target that cannot be read, or lies outside the range [ADR-0024](../docs/adr/operability/0024-conservative-target-aimd.md) allows | No unit of work begins on that account, so its work stops while every other account's goes on |
| The policy's load | The accounts and the connections are in force all the same, the policy keeps what it had, and the reload returns an error that says so ([ADR-0041](../docs/adr/engineering/0041-policy-as-immutable-snapshots.md)) |

The first reload does not hold up the start. Until a reload has loaded the list, the connections
and the policy, the Session reports not ready and the reload is tried again. So the API server and
the worker each start, and report not ready until one has. A later reload whose policy fails leaves
the Session ready. A body request has the policy loaded before it decides, through the load
concurrent body requests share, which is `policies`'
([ADR-0099](../docs/adr/engineering/0099-a-body-request-loads-the-policy-before-it-decides.md)).

The case for one package over per-program code is that the sequence of loading the accounts, their
credentials and their policy would otherwise be written once in each program that loads them, and
each copy is a place a rule can be left out
([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). Written
once, the order and the failure rules hold in every process that holds a Session.

Its package imports `accounts` and `policies`, and no credential and no provider code. It reaches
the connections only through a small interface, which loads the connections of the accounts it
names and says whether an account has one. The only way to the connections is their own package.
So a job kind that calls no provider, such as the heuristics run, holds the Session and cannot reach
a credential, since its import list refuses that package, as
[ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)
requires. A second attach is refused, and an analyser rule holds the callers of the attach to the
connections' package ([ADR-0071](../docs/adr/engineering/0071-static-enforcement-toolchain.md)).

## The accounts, `accounts`

Each process that holds a Session holds its accounts, each with its provider and the name of the
OAuth client it connects through where its provider has one
([ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)),
as one read of the `accounts` table gives them. This package is that reading, from `db/accounts`
([ADR-0091](../docs/adr/data/0091-accounts-listed-apart-from-their-state.md)), and it holds no
credential.

The case for one package over per-program code is a rule a copy could get wrong without anyone
noticing. A read that fails is reported as a failure, never as a list with no account, so the
Session keeps the list in force, while a read that succeeds with no account is the empty list,
which serves none. A copy that treated a failed read as an empty list, or an empty list as a
failure, would serve the wrong set.

## The connections, `connections`

Each process that calls a provider, the API server and the worker, holds each account's connection
to its provider as one record per account. None of a record is exported
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md),
[ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)).

| A record holds | Why |
| --- | --- |
| The stored credential | The bytes a write-back compares against |
| The opened credential | What the token is made from |
| The adoption stamp | Keeps a credential the operator replaced from being overwritten |
| The token made from the credential | What a unit of work calls the provider with |
| The account's lowered rate target | What the account's calls lease at |

A process gets the connections through `connections.Of`, given the Session, and they attach to it
as its third member. They make every token and mailbox through the provider library's list of
shipped adapters. Their one option for a test, `connections.WithAdapters`, hands a process another
list, such as the provider fake's, which only the provider library can build
([provider/README.md](../provider/README.md)), and fills the slot the connections keep the list
in before anything asks
([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). They open each stored credential with the keys they hold, and seal a rotated
one before writing it back
([ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md)). At each reload
they make a token, through the provider library's calling side, `provider`, for each account that
is new or whose credential changed. They keep a token across reloads while its own current
credential equals the stored one, so a refresh that was written back is not thrown away. Delta sync
re-seals what they open with a key that is not the current one, and reports the scan the key's
retirement waits on, through `Reseal`, which an analyser rule holds to delta sync's package
([ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md),
[ADR-0071](../docs/adr/engineering/0071-static-enforcement-toolchain.md)). The sealing and opening
themselves are [`credential/`'s](../credential/README.md#the-sealed-credentials-seal-and-open).

The case for one package over per-program code is rules a copy could get wrong without anyone
noticing.

- **A write-back never puts back a value someone else replaced.** Every write of a sealed value is
  a compare-and-set on the bytes the process last read or wrote. A reload keeps a rotated
  credential whose write-back failed only while the stored bytes are still those. A unit of work
  that began before the connections adopted a value someone else stored, by a reload or a re-read,
  never writes the older credential back. It carries no stamp of its own, and writes back under the
  stamp of the record's token, as [the unit of work](#the-unit-of-work) states. The process's own
  write-backs never stop one another, so overlapping rotations each land. A copy that wrote
  unconditionally, or that read the stamp apart from the token it writes back, would put a revoked
  grant back after the operator re-authorized the account. One that refused a write-back after its
  own earlier write-back would lose a live rotation.
- **An OAuth client is optional per provider, and required where the provider uses one.** An
  account whose provider authenticates without one loads without one, and nothing looks for a
  client that does not exist. Which providers authenticate through a client is one of the provider
  library's facts about each provider, so the connections stay blind to which they are. An account
  of such a provider that names no client, or whose client's secret did not open, loads as not
  connected. A copy that assumed every provider has a client would refuse an account that needs
  none. One that left the refusal to each program would serve such an account wherever a program
  forgot it.
- **An account reaches only the client it names.** Each account carries the client its row in
  `accounts` names, and the connections offer no lookup of a client by provider or by name. A copy
  that took a provider's first or only client would pair an account with a client its grant was
  never issued to, and would look right for as long as each provider had one client. A re-read
  after a refusal pairs the credential with the client the account names at the time of the
  re-read.
- **Only delta sync re-seals, and its scan never passes on silence.** It reports a series for every
  account and every stored client secret, and a missing series never reads as done.

Copies of these rules in each program would drift, and one that got any of them wrong would fail
silently. Written once, they hold in every program that calls a provider.

Each account's credential comes from `db/accountstate/credential`
([ADR-0091](../docs/adr/data/0091-accounts-listed-apart-from-their-state.md)). The re-seal of an
OAuth client's secret is written back by the connections through a compare-and-set, as every
sealed value is
([ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)). Only delta
sync re-seals, the one writer of a client secret beside the UI's setup
([ADR-0016](../docs/adr/data/0016-schema.md)).

### The unit of work

The API server, backfill and delta sync each call a provider for an account in units of work. A unit
is a body request, a page of a pass, or a tick, and each is begun through the connections with the
[caller](../DESIGN.md#provider-abstraction-and-accounts) that makes it. Each job kind, and the API
server's body operation, builds its caller in its own `Build`, through the connections' `Caller`,
given its own capabilities and its own pool, the one `database.Get` gives it, so its provider calls
are counted on its own series and its leases, write-backs and recorded attempts run on its own pool
([ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)).

| When | What happens |
| --- | --- |
| A unit begins | `conns.Begin` finds the account's record and hands the caller the account's connection as the provider library needs it. It refuses an account the list in force does not hold, one with no connection and one whose lowered target could not be read. The unit's mailbox is what the program calls |
| During the unit | Each call takes a lease, calls the provider, tells the limiter how it went, and tries once more on a refused credential |
| The unit ends | `End` writes a refreshed credential back under the adoption stamp of the record it began on ([ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)), and records the latest authentication attempt, both on the pool of the caller the unit began through ([ADR-0097](../docs/adr/operability/0097-authentication-outcome-reported-by-the-adapter-recorded-by-the-deployable.md)) |

`connections` writes these steps once, because they hide a decision of the same concept, how a
process holds an account's credential while it uses it. A copy in each program that calls a
provider would drift on its own, for example one that writes a credential back naming no adoption
stamp while every other copy names one.

The unit of work never branches on which caller holds it, and decides two things alike for every
caller.

- **How long a token lives.** Across units of work, while the stored credential is the one the
  token was made from or the one it rotated to. The connections hold across units, so an access
  token the provider issued serves the units after it, the API server's requests, backfill's pages
  and delta sync's ticks alike. A token per tick would cost a token refresh on every tick.
- **Which units run concurrently.** The API server runs concurrent units for one account, and in
  the worker backfill's and delta sync's do, so each record takes a lock around what it holds.
  Every path that writes a sealed value or adopts one read from a row takes turns with every other
  under one lock of the connections: a reload's read and adoption, a re-read after a refusal, a
  write-back from its stamp check through its compare-and-set and its re-read to the record's
  update, and the re-seal. So a later write-back compares against the earlier's bytes and both land,
  and no read takes the process's own write for someone else's. Beginning a unit of work and its
  calls read a record under its own lock. Beginning never waits on the connections' lock, and a
  call waits on it only for the re-read a refused call makes.

Every caller begins its units of work through the one connections of its process, so it shares
their records, while its calls go through its own caller.

These are the rules a copy could get wrong without anyone noticing.

- **A write-back names the stamp of the credential its token was made from.** A unit of work
  carries no stamp of its own. Its end reads the token and the adoption stamp together from the
  record it began on, under the record's lock, and writes the token's current credential back by
  compare-and-set under that stamp. The write-back lands only while the record's stamp is still the
  one read, and is discarded when the connections adopted another value in between. A copy that
  named no stamp, or read the stamp apart from the token, would let a unit that started before the
  operator re-authorized put the revoked grant back.
- **A refused credential is read again before the refusal stands, and the call is made once more
  only over a different credential**, paired with the client the account names at the time of the
  re-read. The re-read reads the account's row once more. When the row names another client or
  holds another credential, the re-read makes the token again from the pair
  ([ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)),
  changing the account's record in place, so another unit of work on the same account follows it. A
  re-read that finds the refused credential, or none, returns the refusal. A copy that retried over
  the same credential would spend a call the provider refuses, and one that kept the old client
  would make a token the new grant was never issued to.
- **A token and its stamp change only together.** When the token cannot be made over a re-read
  credential, the record keeps the token, the stored bytes and the stamp it had. The account's row
  then holds the re-read credential, which differs from the bytes the record holds, so a unit's
  write-back fails its compare-and-set and the refused credential is never written back.
- **Only a recorded attempt is recorded.** A token that made no attempt records nothing, and the
  statement keeps a later attempt another program recorded.

What the connections leave to others.

- When the accounts reload is the Session's
  ([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
- Which adapters ship is the provider library's list of shipped adapters, so no program names an
  adapter ([ADR-0010](../docs/adr/provider/0010-one-provider-port.md)). The connections make each
  token from plain values, the provider's name, the client's identifier and secret and the
  credential, so no adapter learns what an account is. An account whose provider no shipped adapter
  serves has no connection, and is logged in one place.
- Each call through a unit of work's mailbox leases inside the caller, at the account's lowered
  target, which the unit of work hands the caller's mailbox, so the connections never see the rate
  limiter's code.

Its import list admits the Session's package, the provider library's calling side, `provider`, the
credential family's packages and the data-access subsections its statements need. No adapter, no
rate limiter, no policy and no scanner belong in it.

## The policies, `policies`

Each process's Session holds the active policy as one immutable snapshot, and replaces it only with
an update that validates
([ADR-0041](../docs/adr/engineering/0041-policy-as-immutable-snapshots.md)). The API server,
backfill and delta sync decide under it, as the reorg job kind and the heuristics job kind do.

The validation, the composition of the base policy with an account's overlay and the atomic swap
are pure, and sit in `core/policy`. What is left is impure and the same in every process. This
package reads the base rows and each account's overlay rows from the policy tables, hands them to
the pure half, and raises the reload-failure alarm when a reload fails, whether in the read or in
the validation.

The reload that swaps a snapshot in also prepares each account's policy for the sender classifier,
through `classify.Prepare`, which converts every rule's suffixes once. The snapshot holds the
prepared policies, so they are swapped with it and never outlive it. A prepared policy also holds
the lookups its suffixes were converted with, which the classifier normalizes the sender with. A
classification then converts only the sender, however many suffixes the policy holds.

The package also holds the load concurrent body requests share, which a body request calls before
it decides
([ADR-0099](../docs/adr/engineering/0099-a-body-request-loads-the-policy-before-it-decides.md)),
and which runs on a goroutine started through the process's `Go`.

The case for one package over per-program glue is one rule the glue could get wrong. The pure half
accepts a policy with no rules as valid. A read of the tables that failed, or returned nothing
because of a fault, looks like that empty policy. Treated as one, it would classify the senders the
active policy lists as normal, and release content the previous snapshot withheld. So the package
tells a read it cannot trust apart from a policy that is empty, and a read it cannot trust never
replaces the active snapshot. Written once, that rule holds in every program that loads policy.
Written in each program, one copy that gets it wrong fails open.

The alarm is the other shared part. Each loading process emits the same series, and one alerting
rule reads it ([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)), so
the series is defined once, in this package.

| Series | Kind | Emitted by | Read by |
| --- | --- | --- | --- |
| `mediated_mailbox_policyload_reload_failed` | Gauge, 1 while the latest reload failed and 0 once one succeeds | Each process, once, from its Session's policies, on the process's own registerer, which labels it with `component` ([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)) | `MediatedMailboxPolicyReloadFailed` in `packaging/chart/rules/alerting-rules.yaml`, whose promtool tests sit in `policies/testdata/` |

The package also times one load per reload, every one its caller did not cancel, succeeded or
failed, as `mediated_mailbox_policyload_reload_duration_seconds`, a histogram on the same registry
that no rule reads
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).

The reload-failure series is a gauge rather than a counter because a process first loads policy as
it starts, usually before its first scrape, and a rule over a counter whose first sample is already
1 sees no increase.

Its statements are in `db/policyrules`. Which accounts it reads is the Session's, which loads the
policy for the list it has just put in force, so a process whose accounts change reads the new ones
at the next reload
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
It reads each account's own rules and the base rules in one statement. A reload whose accounts read
different base rules reads every account once more, and only when the second read's accounts
disagree too is the read one it cannot trust. So an edit to the base policy landing midway never
composes accounts from two versions of it, and raises the alarm only when edits keep landing
through both reads
([ADR-0114](../docs/adr/engineering/0114-a-torn-base-policy-read-is-read-again-before-it-fails-the-reload.md)).
A reload stopped because its caller cancelled it keeps the active policy and raises no alarm, and
one that ran out of time is a failed read like any other.

## The scanner, `scanner`

A process holds one scanner, which `scanner.Get` builds from its settings and their revision, so
every verdict, mask and serve-time check in the process carries one revision
([ADR-0120](../docs/adr/redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md),
[ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). Its
settings are `config.Scanner` in `core/config`, and what it decides is `core/scan`'s. This package
also defines the body-processing histogram once, and each job kind or operation that converts and
scans registers it on its own registerer
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
Written once, the scanner is built the same way in every program that scans, and the histogram
means the same in each.

## The logs, `logging`

Every deployable, and the migrator, writes its logs as JSON to standard output at the level its
`log_level` sets
([ADR-0122](../docs/adr/engineering/0122-logs-through-slog-at-a-configured-level-handed-to-shells.md)).
This package builds that logger over a level variable, reads the level's name, and makes the error
log a shell gives an `http.Server` from the logger it was handed. The process builds the logger
before the configuration exists, so a start that fails before its level is read still reports its
error as JSON, and sets the level once the configuration has loaded
([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).

The case for one package over per-deployable glue is that the format and the level names are the
contract. A copy that wrote text rather than JSON, wrote to standard error, or accepted `warning`
or `INFO` where the others refuse it would make one deployable's logs read differently from the
rest, or start where the others refuse. Written once, with a test per level name and a mutation
patch for the refusal, they hold in every deployable. The level names and the refusal of any other
text are the process part's check in `core/config`, which this package's parser calls before it
maps a name to its level, so a level is refused in one place.

It holds no logger of its own and no package-level state, and it logs nothing.

## The release version, `version`

Every binary the project publishes reports the release it was built for through its `--version`
flag, every process names it in its start banner, and the API server's MCP server reports it as
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
