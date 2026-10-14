# 0138. One host runs each process over the components its composition root hands it, and each component provides its parts without starting them

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes),
[O2](../../../USE_CASES.md#o2--observable),
[O6](../../../USE_CASES.md#o6--deployable)

## Context

Every process does the same work around its components. It refuses the environment, loads the
configuration, logs it, sets the level, writes the banner, builds a registry and the runtime
series, opens its pools, opens its listeners, loads its TLS key pair, serves its probes and stops.
Written once per deployable, each copy is a place a rule can be left out. A copy that leaves out
readiness leaves its platform unable to tell when the process can serve, and one that starts a
goroutine without recovering and joining it can lose a panic or close a pool under it. A harness or
test that composes a component by hand runs something other than what its root runs, missing the
operation timing or the connection limits the root applies. Some refusals belong to the whole
process, as `MCPGODEBUG` does, which the MCP SDK reads for every component in the process
([ADR-0086](./0086-mcp-root-on-the-official-go-sdk.md)). A configuration type that mixes the
process's keys with a component's ties the component to the process it runs in.

[ADR-0040](./0040-pure-core-decisions-as-values.md) keeps each root's choice of pieces, pools, roles
and order its own, and makes logic several roots repeat a library each root calls.
[ADR-0050](./0050-shared-code-pure-or-narrow.md) puts repeated assembly in its concept's family.
[ADR-0051](./0051-environment-contract.md) keeps a component ignorant of its platform, and the same
un-braiding asks that it be ignorant of what else shares its process.

## Decision

- **The host, a package of the process family, `process/host`, does what every process does.** A
  composition root hands it the root type of its configuration, the process's sections of it and
  its components, and the host runs the process.

  | Responsibility | What the host does |
  | --- | --- |
  | Environment | Refuses the database driver's password variables and every variable a component refuses, before the configuration is read |
  | Configuration | Loads one tree through the configuration library, answers `--help` and `--version` on the standard output it is handed, logs every effective value, sets the level, and validates the process's sections, then the root's |
  | Banner | Writes the start banner to the standard error it is handed |
  | Metrics | Builds the process's one registry, registers the runtime series once, and hands each component a registerer that labels every series it registers with `component`, the component's directory |
  | Logs | Hands each component the process's logger carrying `component` |
  | Pools | Opens every pool a root asks for through the database connection and measurement packages, and closes them once every task has returned |
  | Operations server | Serves `/healthz`, `/readyz` and `/metrics` on the probe listener as plain text. `/readyz` answers 200 only while the process serves and every readiness check of every component passes, and names the first that fails |
  | Application server | Serves on one listener, with TLS when the configuration names a key pair, loaded before anything is served, mounting each component's handler at the patterns its root gives |
  | Tasks | Starts each component's background tasks once it serves, and a goroutine a component asks for while it runs. It recovers a panic on either as that component's failure, and waits for every one before the pools close |
  | Stop | When the process is told to stop, a server or a task fails, or a goroutine a component started through the starter panics, answers not ready, ends what draining components listen for, drains the application server's requests for ten seconds, ends the tasks' context and waits for every task and every started goroutine, stops the operations server and closes the pools |

- **The process's sections are the host's.** `host.Process` holds `probe_listen` and `log_level`,
  which every process has, and `host.Serving` holds `listen`, `tls_cert` and `tls_key`, which a
  process has when a component it hosts serves requests. A root embeds them inline in its root
  type, so their keys sit at the top of its tree
  ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)). The sections every
  deployable shares, `database`, `credential` and `scanner`, stay their concerns' and are declared by
  each root.
- **Each component exports from its entry package its section and its assembly.** The section is
  the component's own keys with their defaults and their pure validation. The assembly takes what
  the component needs as values, its pools, its keys, its scanner, a registerer, a logger, a client
  and a clock, registers its series, starts nothing of its own and returns its parts, a handler,
  readiness checks and background tasks. An assembly may read what it needs to be built, and fails
  when that cannot be read, so a process that cannot serve never starts. It starts no goroutine of
  its own and registers no route on a mux it is handed. Its handler is a value the host mounts. A goroutine a
  component needs while it runs, such as the API server's policy load that the requests waiting on
  it share, is started through the starter the host hands the assembly. The starter runs it under
  the tasks' context, recovers its panic as the component's failure, which stops the process in
  order, and waits for it before the pools close. Once the host waits for its tasks, the starter
  starts nothing and says so to its caller.
- **Each deployable's root is one call.** Its `Run` hands the host its root type's defaults, its
  validation and its components, each with the patterns its handler is mounted at and an assembly
  that builds what the component needs from the loaded configuration and calls the component's own.
  A root that composes several components is the same call with several.
- **What each component provides.**

  | Component | Mounted at | Readiness | Tasks | Refuses |
  | --- | --- | --- | --- | --- |
  | The API server | `/api/` and `/mcp` | Its bearer token file holds a token, after its first reload of the accounts and their policy, which its assembly runs | The reload of its account snapshot on its interval | `MCPGODEBUG` |
  | The UI | `/` | The database answers a ping as the UI's role | None | Nothing of its own |
  | The worker | Nothing | Each job kind's pool answers a ping, and each job kind's reload has succeeded once | The scheduler, which recovers a panic in each run as that run's failure ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) | Nothing of its own |

- **The UI's event stream ends when draining begins**, through the draining context the host hands
  the UI's assembly, so a stopping process drains its other requests rather than waiting out an
  answer that never ends. A request's own context ends once the tasks are told to stop, after the
  drain.
- **Tests and harnesses compose through the same seam.** A test or a harness that needs a
  component's internals calls the component's assembly with what it substitutes, such as the
  provider fake, and serves it through the host's lifecycle, so what it runs is what the root runs.
- **A restriction a record puts on a process or a deployable binds the code area it names however
  that code is packaged.** Where a record names a deployable for a rule about its code, its import
  list, its pools and roles, its goroutines or its series, the rule holds for that component's code
  in any process that hosts it.

## Alternatives considered

- **Each root writing the process's work itself.** The case for it is that each root reads top to
  bottom with nothing hidden. It loses because each copy drifts on its own, in readiness, key-pair
  loading, task recovery and the harnesses, and every rule a copy encodes is one a further root
  would have to copy again.
- **An application framework that owns the lifecycle and the wiring.** The case for it is a
  lifecycle written and tested elsewhere, with hooks for start and stop. It loses because
  [ADR-0040](./0040-pure-core-decisions-as-values.md) refuses a framework that owns the object graph,
  and the host owns only what every process does, leaving each root's choice of pieces in its own
  code.
- **A component assembly that takes a mux and registers its routes, or starts its own loops.** The
  case for it is fewer values passed back. It loses because the routes analyser holds the UI's routes
  to its recording mux, and a loop a component starts is one the host can neither recover nor join.
- **The `component` label applied by each component.** The case for it is that each component's
  series name their owner without a host. It loses because a component then knows how it is
  packaged, and a series would take its shape from whoever wrote the component rather than from one
  rule.

## Consequences

- One rule set runs every process. A probe, a limit, a stop order or a refusal changes in one
  place, and a new root, a process hosting several components among them, gets them by calling the
  host.
- Every series a component registers carries `component`, and the worker's job kinds keep
  `job_kind` beneath it ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)).
  Within one registry a series name keeps one set of label names, so a root that hosts the worker
  beside a component that registers the same shared library's series must give both the same set,
  which a process hosting one component meets by construction.
- No goroutine of a component's code loses a panic or outlives the pools. The API server and the
  UI start one only through the host's starter, which the `go vet` goroutines analyser holds their
  code to, as it holds the worker's job code to the scheduler's helper
  ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
- The worker answers readiness, so the chart can probe it.
- The probes carry no content security policy. They are plain text for a platform's healthcheck
  and scraper, and the UI's policy stays on every response of its own
  ([ADR-0062](../operability/0062-ui-content-security-policy.md)).
- The UI's server takes the API server's connection limits, a read limit of 30 seconds among them,
  which bounds reading a request and never an answer, so an event stream is not cut
  ([ADR-0130](../operability/0130-the-api-server-bounds-what-one-request-can-make-it-hold.md)).
- The browser bundle is embedded by a package of its own beside it, `ui/browser`, which any root
  serving the UI imports.
- Assumptions about other components. Each component's assembly reads what it needs from the values
  it is handed and nothing ambient. The settings library reads an inline section's values beside the
  section that embeds it ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)).
  Go's HTTP server applies its read limit only to reading a request, and `Shutdown` waits for the
  requests in flight without cancelling them.
- Each rule above that a test can break is a control, with its violation injection in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
