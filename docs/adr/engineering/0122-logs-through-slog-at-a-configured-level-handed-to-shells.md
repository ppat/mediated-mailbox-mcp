# 0122. Every deployable logs through log/slog as JSON to standard output, at the level its configuration sets, through a logger its entry package hands its shells

**Status:** Accepted ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable)

## Context

[ADR-0051](./0051-environment-contract.md) settles that every deployable writes structured logs to
standard output and that their collection, shipping and retention are the platform's.
[ADR-0078](./0078-configuration-layers-through-an-owned-library.md) settles how a value reaches a
deployable and how a value outside its designed range is refused.
[ADR-0040](./0040-pure-core-decisions-as-values.md) settles that nothing is ambient, so every
dependency arrives as a parameter, and that a pure core has no side effects.
[ADR-0028](../operability/0028-trust-anchor-hardening.md) asks that significant events be logged at
the level that matches them. [ADR-0042](./0042-implementation-stack.md) leaves each major library to
its own decision. What is left is the library the deployables log through, how a level is chosen,
and how a logger reaches the code that logs.

The choice looks like picking a logging library and mostly is not one. Every candidate writes JSON
at a level. What separates them is whether the logger can travel as one value through code that
already takes it, the MCP SDK's server among it, and whether code the project does not own, the
standard library's HTTP server and client among it, logs to the same place at the same level.

| Requirement | What it demands | From |
| --- | --- | --- |
| Structured to standard output | Each record one JSON object on standard output | [ADR-0051](./0051-environment-contract.md) |
| A configured level | One configuration value sets the lowest level written, with a default, refused at start when it names no level | [ADR-0078](./0078-configuration-layers-through-an-owned-library.md), this record for the value |
| A value, never ambient | The logger reaches every shell that logs as a parameter, and a logger for one job is derived from it with attributes added | [ADR-0040](./0040-pure-core-decisions-as-values.md), and [ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)'s worker, whose job loggers carry the job kind and the account |
| One level for everything the process writes | A record written by code the project does not own follows the same level and lands in the same stream | This record |
| Interoperates with what takes a logger | The MCP SDK's server takes a `*slog.Logger`, and `net/http`'s server takes a `*log.Logger` for its own errors | [ADR-0086](./0086-mcp-root-on-the-official-go-sdk.md), the standard library |
| The process default refusable | A lint ban can refuse project code that reads a process-wide default logger | [ADR-0071](./0071-static-enforcement-toolchain.md), as [ADR-0076](./0076-metrics-emitted-through-client-golang.md) refuses the default metrics registry |
| A small dependency footprint | Modules and binary size added to the processes that hold full-mailbox credentials | [ADR-0078](./0078-configuration-layers-through-an-owned-library.md)'s reading of [ADR-0042](./0042-implementation-stack.md) |

Structured output, the configured level and the value were gates, because each is a requirement
another record or the outcome sets. Footprint ordered what passed them, by the reading ADR-0078
gives ADR-0042. Interoperation and the refusable default broke ties. Throughput was left out of the
grading, because no shell logs in a loop tighter than one line per page of provider results.

## Decision

- **`log/slog`, the standard library's structured logger.** Its JSON handler writes one object per
  record, its level can be a `*slog.LevelVar` set after the handler is built, and `Logger.With`
  derives a logger carrying added attributes over the same handler and level.
- **One package builds the logger and reads the level, `process/logging`**, so every deployable
  writes the same format and accepts the same level names. A shared copy is a narrow library under
  [ADR-0050](./0050-shared-code-pure-or-narrow.md), and the process family holds it because writing
  logs to standard output is part of running as a process under ADR-0051.
- **The level is the configuration value `log_level`**, read by every deployable through the
  configuration library. It takes `debug`, `info`, `warn` or `error`, spelled exactly so, and
  defaults to `info`. Any other text is refused at start by the deployable's validation, as every
  value outside its designed range is under ADR-0078, so a configuration without the key starts as
  before. The default is `info` because routine progress, backfill's line per page and the UI's
  line per request ([docs/UI.md section 18.2](../../UI.md#182-the-uis-own-observability)) among it,
  is written at `info`.
- **What each level carries.** Errors, and the decisions an operator acts on, at `warn` or above.
  Routine progress at `info`. Detail at `debug`.
- **`main.go` builds the logger, and the entry package sets its level.** A deployable's `main.go`
  builds a `*slog.LevelVar`, builds the logger over it on standard output, makes that logger the
  process default, and passes the logger and the level variable to the entry's `Run`. The logger
  exists before the configuration does, so a start that fails before its level is read still
  reports its error as JSON on standard output.
- **The effective configuration is written before the level applies.** Once the configuration
  loads, `Run` writes each value with its source at `info` while the level variable still holds
  `info`, then reads `log_level`, refusing a level that names none, and only then sets the
  variable. ADR-0078 logs the effective configuration at start with each value's source, and a
  refusal names only the key, so the source of a refused value reaches the operator through those
  lines. Written after the level applies, they would vanish under `warn` or `error`, and with them
  the source of every refused value, `log_level`'s own included. The cost is the configuration's
  lines at every start, whatever the level.
- **The process default is for code the project does not own.** Making the logger the default also
  routes the standard `log` package into it, so a line the standard library or a dependency writes
  through either default is JSON on standard output under the configured level. Project code never
  reads a default. The entry package hands the logger to every shell that logs, as a parameter, and
  a deployable's shell that builds an `http.Server` gives it an error log made from that logger.
- **The MCP SDK's server logs through the handed logger, its routine records at `debug`.** With no
  session kept, the server connects one session per request and writes "server connecting" and
  "server session connected" at `info` for each, which is detail. The mediator's MCP root hands
  the server a logger over the handed logger's handler that lowers every record at `info` or above
  and below `warn` to `debug`, and passes its warnings and errors at their own levels. The
  streamable HTTP handler writes warnings and errors and takes the handed logger as it is.
- **A logger per job is `logger.With(...)` over the handed logger.** It shares the handler and the
  level, so a job's records follow the configured level without the job reading configuration.
- **A pure core does not log.** It imports no logging package and calls no built-in print function,
  as ADR-0040's pure core does no I/O.

The table gives what the library's ordinary path does that this project forbids.

| Construction | Harm | What stops it |
| --- | --- | --- |
| `slog.Default`, and `slog.Debug`, `slog.Info`, `slog.Warn`, `slog.Error`, their `Context` forms, `slog.Log` and `slog.LogAttrs`, which log through the default | A shell's logging becomes ambient, so what a shell logs through is not visible in its parameters, and a test cannot hand it a logger | A `forbidigo` ban over every Go file, proven by a violation file |
| The `log` package's `Print`, `Fatal` and `Panic` functions in their three forms, `log.Output`, `log.Default` and `log.Writer` | The same, through the older default | The same ban |
| The built-in `print` and `println` | A line written to standard error as plain text, outside the handler and its level, and the one way a pure core could write output without an import | The same ban |
| An `http.Server` built with no error log | The server writes its own errors through the `log` default rather than the handed logger | Review. The default still routes those lines to the handler, so a miss changes their level, never their stream |
| A logging import in a pure core | A pure core with a side effect | The pure-core import lists of ADR-0071, which admit no logging package, proven by a violation file |

The setters, `slog.SetDefault` and the `log` package's `SetOutput`, `SetFlags` and `SetPrefix`,
stay allowed, because `main.go` sets the default for code the project does not own, and a setter
gives project code no logger to log through.

| Requirement | Met by |
| --- | --- |
| Structured to standard output | `slog.NewJSONHandler` over standard output, built in `process/logging` |
| A configured level | `log_level`, read by `process/logging`'s level parser, set on the `*slog.LevelVar` the handler reads |
| A value, never ambient | `*slog.Logger` parameters from the entry package down, and the ban on reading a default |
| One level for everything the process writes | The logger made the process default in `main.go`, which also routes the `log` package, the server error logs made from the handed logger, and the MCP SDK's server and handler logs |
| Interoperates with what takes a logger | The handed `*slog.Logger` to the MCP SDK's streamable handler and, with its routine records lowered to `debug`, to its server, and `slog.NewLogLogger` over its handler for `http.Server.ErrorLog` |
| The process default refusable | The `forbidigo` ban |
| A small dependency footprint | The standard library alone |

What an implementer would otherwise pay to discover is the following.

- `slog.SetDefault` with a handler of the project's own also points the `log` package's output at
  that handler, at the `info` level. Without the call, `log` writes plain text to standard error, so
  removing it silently moves the standard library's lines out of the stream.
- The MCP SDK's server and its streamable HTTP handler each discard their log when given no logger,
  so neither reads the process default. Each is given its logger in its own options, the server's
  in `ServerOptions.Logger` and the handler's in `StreamableHTTPOptions.Logger`.
- `slog.Level`'s own text parser accepts any case and offsets such as `info+2`. The level parser
  accepts the four names only, so a value the operator mistyped is refused rather than read as a
  level between two others.

## Alternatives considered

Grades run 4 (meets it as shipped), 3 (meets it with configuration or a small part the project
owns), 2 (meets it only with a bridge or a named gotcha) and 1 (cannot meet it). Every cell rests on
a scratch program built for each candidate, writing a warning and suppressing an info record through
a logger derived with two attributes, apart from the refusable-default column, read from each
library's source.

| Candidate | Structured to stdout | Configured level | A value, derived per job | Footprint | Interoperates | Default refusable |
| --- | --- | --- | --- | --- | --- | --- |
| **log/slog** | 4 | 4 | 4 | 4 | 4 | 4 |
| zerolog | 4 | 4 | 4 | 3 | 2 | 3 |
| zap | 3 | 4 | 4 | 2 | 2 | 3 |

The configured level and the value separated nobody, since every candidate carries a level
variable and a derived logger. Footprint and interoperation separated the field. zap's
structured-output cell is a 3 because its production configuration writes to standard error until
told otherwise. Both challengers carry a 3 on the refusable default because each keeps a global
logger of its own, `zap.L()` and zerolog's `log.Logger`, that a ban would name beside slog's. slog
has no cell below 4, and each challenger carries a 2.

| | log/slog | zerolog | zap |
| --- | --- | --- | --- |
| Static binary of the scratch program | 2,691,232 bytes | 2,797,728 bytes | 4,886,688 bytes |
| Packages from outside the standard library | 0 | 5 | 10 |
| What the MCP SDK and `http.Server` need beside it | nothing | a bridge to `*slog.Logger` and `*log.Logger` | the same, through `zapslog` |

Whose worst cell is best decides it, and slog's worst is a 4. Each challenger's one strength,
encoding speed, can be had without choosing it, because slog's front end takes any `slog.Handler`
and both publish one, while each challenger's costs, a second API in every shell and a bridge for
the SDK, cannot be confined. Regretting slog costs a new handler in `process/logging` and nothing in
the shells. Regretting a challenger costs every shell's signature.

- **log/slog.** For it, the standard library, already the type every shell and the MCP SDK take,
  and a handler interface that keeps the other libraries available as back ends. Against it, its
  JSON handler allocates more per record than zerolog's encoder, and its default bridges the `log`
  package at a fixed level, so someone has to set the default for code the project does not own.
- **zerolog.** For it, the fastest encoder and nearly slog's footprint. Against it, the shells, the
  MCP SDK and `http.Server` all take slog's or `log`'s types, so it sits beside slog through a
  bridge rather than instead of it.
- **zap.** For it, a long production record and a sampling option. Against it, the largest
  footprint measured, standard error as its production default, and the same bridge as zerolog.
- **`main.go` passes standard output, and `Run` builds the logger once the level is known.** For
  it, the level is a value read once and never changed, with no variable shared between `main.go`
  and `Run`. Against it, a line written through the process default by code the project does not
  own would sit at a level fixed before the configuration loads, or as plain text on standard
  error if no default were set, so one level would not govern everything the process writes.
- **The configuration read in `main.go`, ahead of a `Run` that takes it.** For it, `main.go` would
  build the logger at its final level. Against it, every deployable's `main.go` would load and
  validate configuration, which [CLAUDE.md](../../../CLAUDE.md#inside-a-component) keeps in the
  entry package, the composition root.
- **A `log` section with a `level` key.** For it, room for later logging values under one prefix.
  Against it, no other logging value is needed, and the flat key matches the other single values,
  `listen` and `provider_timeout`.
- **The SDK server's own log handed at a fixed `warn`.** For it, no handler of the project's own.
  Against it, the server's routine records would be dropped even when the operator asks for
  `debug`, and the server's level would not follow the configured one.
- **The SDK server's own log left discarded.** For it, no line per request. Against it, the
  server's errors, a failed connection or an invalid tool among them, would reach no log.
- **The effective configuration written at the configured level.** For it, `warn` would hold no
  routine line at all. Against it, a start refused under `warn` or `error` would not name the
  layer that set the refused value, which ADR-0078 requires to reach the operator.
- **Accepting `slog.Level`'s own spellings.** For it, no parser to own. Against it, `INFO+2` and
  `Warn` would be accepted, which ADR-0078's strictness refuses for every other value.

## Consequences

- Every deployable's `main.go` builds the logger and its level variable and makes the logger the
  process default, and its entry package's `Run` takes both and sets the level. A future entry
  package, the worker's included, takes the same two.
- The worker's job loggers are `logger.With` over the logger its entry package receives, carrying
  the job kind and the account, so adding them changes no hand-off.
- The default-logger ban joins the bans `forbidigo` carries under
  [ADR-0071](./0071-static-enforcement-toolchain.md), proven by a violation file, and the pure-core
  import lists' refusal of a logging import is proven by violation files at a depth below the
  repository root's `core/`.
- Leaving slog costs a handler in `process/logging`. The key, its four names, the hand-off and the
  ban survive any exit that keeps slog's front end. Leaving the front end costs every shell's
  logger parameter.
- What would re-argue it is a per-record cost in a hot path that slog's JSON handler cannot meet,
  which a faster handler behind the same front end answers first.
- Assumptions about other components. The platform captures standard output
  ([ADR-0051](./0051-environment-contract.md)). The MCP SDK takes a `*slog.Logger`, and the standard
  library's `log` default stays routed through `slog.SetDefault`.
