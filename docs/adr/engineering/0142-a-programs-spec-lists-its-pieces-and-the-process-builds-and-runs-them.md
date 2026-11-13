# 0142. A program's spec lists what the program is made of, and the process builds each piece through that piece's own function, keeps one of each thing it shares, and runs them

**Status:** Proposed (supersedes
[ADR-0138](./0138-one-host-runs-each-process-and-each-component-provides-its-parts.md), and in
part [ADR-0040](./0040-pure-core-decisions-as-values.md)) ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes),
[O2](../../../USE_CASES.md#o2--observable),
[O6](../../../USE_CASES.md#o6--deployable)

## Context

Each program, the API server, the UI, the worker and the migrator, has a composition root that
chooses its pieces, the pools they use and the order they start, written by hand
([ADR-0040](./0040-pure-core-decisions-as-values.md)), and hands them to the host, which runs the
process ([ADR-0138](./0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)).
The code works and is hard to read, change and grow. One idea is spread over many packages and
types. The roots hold behaviour, the API server's holding its bearer check and the body
operation's use of the account session. Each program builds the things it shares its own way and
at its own depth. The schema check is copied into three roots, the scanner is built in two, and
`backfill.go`, `deltasync.go` and `ui/internal/api/server.go` each build again what a root also
builds.

A process holds things with two lifetimes. Some live as long as the process and are shared by
everything it runs, such as its pool. Others live as long as one job or one operation, such as the
adapters and limiters each job kind calls its provider through, so that every series a job
produces is its own ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)).
A design that builds the two in one place, or joins them into one object, loses that attribution.

## Decision

- **The spec and the process are two things.** The spec defines all the pieces that are assembled
  to build a program's running process, and does not do the assembling or the building. The process
  builds and runs them.

  | | The spec | The process |
  | --- | --- | --- |
  | Is | What one program is made of | One running program |
  | Holds | Its name, which is the program's directory and its `component` label. Its start check. Its endpoints, its jobs, or its one task, each with the function that builds it | The loaded configuration and the database pool. The logger, the metrics registry, the servers, the scheduler and the readiness reports. The Session, all the user state the process holds. One of anything else an owner package keeps in it |
  | Does | Nothing. It lists | Starts. Loads the configuration. Calls each function the spec lists. Serves. Runs the jobs. Answers the probes from what was reported. Stops in order |
  | Hands on | How each endpoint, job or task is built, to that piece's own package | What an endpoint or a job does, to its package. User state, to the Session. Calls to a provider, to the provider library through each account's connection. The shape of the configuration, to the configuration types in `core/config` |
  | Knows nothing of | The process it will run in, or what else runs there | Accounts, providers, policies, or any one program |

- **A spec is a struct of the pieces' structs.** It is `process.Spec[C]`, typed on the program's
  configuration type `C`, and holds the program's name, its start check `Init`, a list of
  endpoints, a list of jobs, and a task for a program that does one thing and exits. A program's
  `main.go` hands `process.Run` its arguments, its environment and its one spec, since only a
  `main.go` reads the environment
  ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)).

  | Program | What its spec lists |
  | --- | --- |
  | The API server | Endpoints at `/api/` and `/mcp`, and an `Init` that refuses `MCPGODEBUG` ([ADR-0086](./0086-mcp-root-on-the-official-go-sdk.md)) |
  | The UI | Endpoints at `/ui/api/` and `/ui/`, and the redirect at `/` |
  | The worker | Jobs, backfill and delta sync |
  | The migrator | One task, the migration ([ADR-0139](../data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md)) |

- **`Init` checks and refuses, and builds nothing.** It is given what `process.Run` was given and
  the loaded configuration, and returns only an error. Before the configuration is read, the
  process refuses the database driver's password variables, `PGPASSWORD` and `PGSSLPASSWORD`, for
  every program ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md),
  [ADR-0139](../data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md)).
- **Every endpoint, job and task is built the same way, by one function of its own package.** The
  function runs while the process is being built. It is given the build's context, which it may
  use to open what it needs, such as the pool, the program's whole configuration, and what the
  process offers the one piece being built, `process.Capabilities`. It returns what runs later, a
  handler for an endpoint, a plan for the scheduler for a job, and the work itself for a task. A
  piece reads from the configuration what it needs. The code that acts on a setting is fenced, by
  the import lists ([ADR-0071](./0071-static-enforcement-toolchain.md)), not the setting.
- **What the process offers a piece.** A logger and a metrics registerer carrying the piece's
  labels, `component` for the program and `job_kind` for a job, with the same label names on every
  piece so that two may register one series name. A report of its readiness as ready, not ready or
  failed, with a reason. A goroutine the process waits for. A context that ends when the process
  starts to stop. It also carries, unexported, the process it belongs to, which is how an owner
  package's function finds its slot.
- **Everything is built before anything runs.** The process calls every build function while it is
  being built, and starts the scheduler and the servers after. A build that fails stops the start
  before anything serves, a pool whose schema the binary cannot run on among them
  ([ADR-0140](../data/0140-each-migration-declares-the-oldest-chain-that-runs-on-it.md)).
- **A thing is built by its owner, as deep as its owner sits, and lives as long as its owner.** Some
  things are built at the top of the process very early and passed down, and others are built
  deeper. Which applies is decided once for each kind of thing, and that kind is built the same
  way in every program. The two lifetimes stay apart.

  | Lives as long as | What |
  | --- | --- |
  | The process, one for all its endpoints and jobs | The pool, the scheduler, the Session, the scanner, the API server's registry of operations, and the UI's checks and their key |
  | One job, or the API server's body operation | Its caller, its own equipment for calling a provider, which holds its adapter clients, its limiter, its provider metrics, its class and its timeout |

- **The process keeps one of each thing it shares.** A thing the whole process shares has one owner
  package, one function that gets it, and one slot in the process, `process.OnePerProcess[T]`,
  which the owner package declares once. The first call builds the thing under the slot's lock and
  every later call returns the same one. The process closes what each slot holds, last built
  first, once the servers have drained and the jobs have ended. Nothing is looked up by type, name
  or reflection.
- **A shared thing is built from the process, never from whoever asked first.** What it registers,
  logs and reports does not depend on which endpoint or job asked for it first. The owners of the
  pool, the scheduler, the Session, the connections each account holds to its provider and the
  scanner build each with the process's own logger and registerer, labelled with the program's
  name and no job, and report its readiness under its own name. A shared thing reads the parts of
  the configuration every program has under the same keys, and reads the revision of its settings
  from the process where it needs one. A thing only one program's endpoints ask for, the API
  server's registry and the UI's checks, is built with the labels of whichever of them asks, since
  all of them carry that program's labels. Go cannot stop a build function from using a variable
  of whoever asked, so this is held by review.
- **The compiler refuses a program a shared thing it has no settings for.** An owner's function
  requires a configuration type that has the part it reads, through an interface such as
  `config.HasDatabase`, `config.HasAccounts` or `config.HasConnections`. The UI's configuration
  type has no accounts part, so the UI cannot ask for the Session or the connections.
- **Readiness is reported.** Each piece and each shared thing reports its own state, and the probes
  answer from what was reported and run nothing. `/readyz` answers ready only while the process
  serves and every report is ready, and names the first that is not.
- **The process does what every process does, around the pieces.**

  | Responsibility | What the process does |
  | --- | --- |
  | Configuration | Loads one tree through the configuration library, answers `--help` and `--version`, logs every effective value, sets the log level, and runs the checks of the program's configuration type |
  | Banner | Writes the start banner to standard error |
  | Metrics | Builds the process's one registry, registers the runtime series once, and hands each piece a registerer carrying its labels |
  | Operations server | Serves `/healthz`, `/readyz` and `/metrics` on the probe listener as plain text |
  | Application server | Serves on one listener, with TLS when the configuration names a key pair ([ADR-0127](../operability/0127-tls-when-given-a-key-pair-plain-http-otherwise.md)), under the connection limits of [ADR-0130](../operability/0130-the-api-server-bounds-what-one-request-can-make-it-hold.md), mounting each endpoint at its pattern |
  | Jobs | Runs the spec's jobs on the process's scheduler, which recovers a panic in each run as that run's failure ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) |
  | Goroutines | Runs a goroutine a piece asks for while it runs, recovers its panic as a failure that stops the process in order, and waits for it before the shared things close. Once the process waits for its goroutines it starts no new one and says so to the caller |
  | Stop | When the process is told to stop, a server or a job fails, or a goroutine a piece started panics, it answers not ready, ends the context pieces watch for draining, drains the application server's requests for ten seconds, ends the jobs' context and waits for every job and every goroutine, stops the operations server and closes what each slot holds |
  | A task | A program whose spec holds a task is started as every process is, and runs the task once. It serves no operations endpoints and no application server and runs no scheduler, since nothing probes, scrapes or calls a process that exits on its own. It returns the task's error once every goroutine the task started has returned and the shared things are closed |

- **The UI's event stream ends when draining begins**, through the context that ends when the
  process starts to stop, so a stopping process drains its other requests rather than waiting out
  an answer that never ends.
- **A test hands a process a shared thing it made through one option, and that option is a test's
  one seam.** A test or a harness runs a program through its spec and the process, as its
  `main.go` does, so what it runs is what the program runs. Where it substitutes a shared thing,
  such as one built over the provider fake, it hands it to the process through that option. The
  option's form is decided where the process is built.
- **Wiring stays explicit.** A spec lists its pieces in ordinary code, and each shared thing is
  built by a call to its owner's function. Nothing resolves an implementation by registry,
  reflection, annotation or naming convention at run time, which is
  [ADR-0040](./0040-pure-core-decisions-as-values.md)'s ban and stays its decision. This record
  replaces ADR-0040's statements that each deployable has one hand-written composition root that
  constructs its object graph, and that a deployable's choice of pieces, which pool each uses and
  the order they start are its own, written by hand in its composition root.
- **A restriction a record puts on a program binds the code area it names however that code is
  packaged.** Where a record names a program for a rule about its code, its import list, its pool,
  its goroutines or its series, the rule holds for that code in any process that runs it.

## Alternatives considered

- **A composition root per program that chooses and builds its pieces and hands them to a host**,
  [ADR-0138](./0138-one-host-runs-each-process-and-each-component-provides-its-parts.md) with
  [ADR-0040](./0040-pure-core-decisions-as-values.md)'s roots. The case for it is that a host does
  what every process does once, and each root keeps its choice of pieces in its own code with
  nothing hidden. It loses because the roots came to hold behaviour, the same checks and builds were
  copied between roots, and each program built the same kind of thing its own way and at its own
  depth, which made the code hard to read, change and grow.
- **A spec that is one struct holding every part and setting of a program.** No case was tabled for
  it. The operator ruled that a spec is a struct of structs.
- **Building the two lifetimes in one place, such as one caller the whole process shares.** No case
  was tabled for it beyond its being the shape earlier rounds of this design took. It loses
  [ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)'s decision that each
  job kind calls through adapters and limiters of its own, so its series are its own.
- **An application framework that owns the lifecycle and the wiring.** The case for it is a
  lifecycle written and tested elsewhere, with hooks for start and stop. It loses because
  [ADR-0040](./0040-pure-core-decisions-as-values.md) refuses a framework that owns the object
  graph, and the process owns only what every process does, while the spec and the owner packages
  keep the choice of pieces in ordinary code.
- **A piece that takes a mux and registers its routes, or starts its own loops.** The case for it is
  fewer values passed back. It loses because the routes analyser holds the UI's routes to its
  recording mux, and a loop a piece starts is one the process can neither recover nor join.
- **The `component` label applied by each piece.** The case for it is that each piece's series name
  their owner without the process. It loses because a piece then knows how it is packaged, and a
  series would take its shape from whoever wrote the piece rather than from one rule.

## Consequences

- Each program's root becomes its spec, and what the roots held goes to the package that owns it.
  Each kind of thing has one owner and is built the same way in every program.
- [ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)'s fence of an entry
  constructor that takes only what a job needs no longer holds, since every build function takes
  the same arguments. A job's import list is the fence on what it can reach, as the operator ruled
  on 14 November 2026.
- The Session is a shared thing the process holds one of, built by its owner package. How the
  accounts and their policy are loaded and reloaded stays
  [ADR-0090](../operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)'s and
  [ADR-0099](./0099-a-body-request-loads-the-policy-before-it-decides.md)'s.
- No goroutine of a piece's code loses a panic or outlives the shared things. The API server's and
  the UI's code start one only through the process, which the `go vet` goroutines analyser holds
  them to, as it holds the worker's job code to the scheduler
  ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
- Within one registry a series name keeps one set of label names, so every piece carries the same
  label names.
- The probes carry no content security policy. They are plain text for a platform's healthcheck and
  scraper, and the UI's policy stays on every response of its own
  ([ADR-0062](../operability/0062-ui-content-security-policy.md)).
- One binary for several programs is not built. If it is, it is a program like the others with a
  spec of its own, which imports the other programs' specs and joins their endpoints, their jobs
  and their `Init` functions into its own lists. Each program's endpoints and jobs are typed on
  that program's configuration, so the one binary's configuration type has a function per program
  that gives that program its settings, and joining a list applies it. How its metrics are
  labelled and how its keys are laid out are decided when it is built.
- Assumptions about other components. Each build function reads what it needs from the values it
  is handed and nothing ambient. The configuration library loads a program's configuration type
  ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)). Go's HTTP server applies
  its read limit only to reading a request, and `Shutdown` waits for the requests in flight
  without cancelling them.
- Each rule above that a test can break is a control, with its violation injection in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
