# 0119. Each deployable logs at the level its log section sets, through the one logger its composition root builds and hands to every shell

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable),
[C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

Every deployable writes structured logs to standard output, and the platform collects them
([ADR-0051](./0051-environment-contract.md)). No level was configurable. Each composition root set
the process's default logger, and some shell code read that default rather than a logger handed to
it, the mediator's two protocol roots among them. A process default is ambient, which
[ADR-0040](./0040-pure-core-decisions-as-values.md) refuses for every other dependency, and it is
shared by everything in the process, so two workloads merged into one process, or one binary
holding several, could not log at different levels or to different places.

Configuration reaches a deployable through the configuration library, strictly, with a value outside
its designed range refused at start by an error naming its source
([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)). The effective configuration
is logged at start, so the logger exists only once the configuration has loaded, and a start
refused before then still has to be logged.

The standard library's structured logger, `log/slog`, is already the one every deployable uses. Its
own text form of a level ignores case and accepts an offset, such as `INFO+2`, which is wider than
the four levels an operator needs to choose between.

## Decision

- **Every deployable's configuration has a `log` section holding one value, `level`.** Its YAML
  key is `log.level`, its environment variable `MEDIATED_MAILBOX_LOG__LEVEL` and its flag
  `--log.level`, by ADR-0078's one name per value. The value is exactly one of `debug`, `info`,
  `warn` and `error`, in lower case, and defaults to `info`. Any other text is refused at start,
  naming the file and line, the environment variable or the flag that set it.
- **The section, its validation and the logger built from it are one narrow shared library,
  `logging/`**, which argues its case in [its README](../../../logging/README.md) under
  [ADR-0050](./0050-shared-code-pure-or-narrow.md). The section's type and its validation are pure
  and sit in `logging/core`. The logger writes JSON lines to the writer the composition root names,
  standard output in a deployable.
- **Each composition root builds one logger, once its configuration has loaded, and hands it as a
  value to every shell that logs.** It logs the effective configuration through it. A start
  refused before the configuration loads, or by the level itself, is logged by the root through a
  logger at the default level writing the same JSON lines.
- **The root also sets the process default to that same logger, and only for the code the project
  does not own.** The standard library's HTTP server, the `log` package and any SDK left without a
  logger write through the process default, so setting it keeps their output structured, on
  standard output and at the configured level. The `log` package's lines arrive at info. Where an
  SDK takes a logger, it is handed the root's, as the MCP SDK's transport is.
- **Project code never reads the process default.** A `forbidigo` ban
  ([ADR-0071](./0071-static-enforcement-toolchain.md)) refuses `slog.Default` and `log/slog`'s
  package-level functions that log through it, `Debug`, `Info`, `Warn`, `Error`, their `Context`
  forms, `Log` and `LogAttrs`, and every use of the `log` package, whose lines are unstructured and
  whose loggers write wherever they were made to. The ban reaches every Go file, test files
  included, which take the logger they hand the code under test as a value. Setting the default,
  and building a logger, stay allowed.
- **A line's level says what the reader does with it.**

  | Level | The line records | Examples |
  | --- | --- | --- |
  | `error` | A failure an operator acts on, where the system stopped doing something it should | a run or a tick that failed, a reload or a write-back that failed, a call that failed inside the mediator, a start refused |
  | `warn` | A decision an operator may need to act on, where the system carries on | an account skipped because it is not connected, a call the provider failed |
  | `info` | Routine progress | the effective configuration, what is served, a pass or a tick starting and ending, a client's request the system refused, a UI request |
  | `debug` | Detail | a page made durable, a response or a probe answer that could not be written to a client that went away |

  An existing line is placed by this rule, and debug detail is added only where a gap is found.
- **Every test that searches a workload's logs for what must never be logged runs its logger at
  `debug`**, so a detail line carrying it is caught too.
- **A pure core never logs.** It returns its decision as a value and the shell holding it logs, as
  ADR-0040 decides. The import list over pure cores already refuses `log` and `log/slog`
  ([ADR-0071](./0071-static-enforcement-toolchain.md)), and a violation file proves it.

## Alternatives considered

- **A flat `log_level` value.** For it, one key shorter. Against it, it has no section type of its
  own, so the library could not own its validation and every root would spell it again.
- **A field typed as `log/slog`'s level.** For it, the standard library parses it. Against it, that
  parse ignores case and accepts offsets, so `INFO`, `Warn` and `INFO+2` would all start, a range
  wider than the four levels and wider than ADR-0078's strictness allows.
- **Each root keeping its own copy of the vocabulary and the handler.** For it, no new library.
  Against it, the copies drift with nothing to report it, as the library's README argues.
- **Leaving the process default unset.** For it, nothing in the process is ambient. Against it, the
  HTTP server's own errors, such as a failed TLS handshake, and anything an SDK logs on its own
  would go to standard error unstructured, against ADR-0051.
- **Shells reading the process default, set by the root.** For it, no logger to pass. Against it, it
  is the ambient dependency ADR-0040 refuses, and two workloads in one process would share one
  level and one destination.
- **Enforcing the hand-off by review.** No case was tabled for it. The ban is one configuration
  entry with a violation file, and review misses a call that reads correctly.

## Consequences

- Each composition root's configuration type gains the `log` section, pinned like every other
  section, and each deployable's `--help` lists it. The reorg workload and the Heuristics Job take
  the section, the hand-off and the level rule when their roots are built.
- At `warn` or `error` the effective configuration, which is logged at info, is left out of the
  log. A level refused at start names its own source, because it is refused before the effective
  configuration is logged.
- A shell's constructor or function takes the logger as a parameter, so a test hands it a logger
  writing where the test reads.
- The ban cannot see a logger built and written outside the library, for example a JSON handler on
  standard error built in a shell. That is left to review.
- Assumptions about other components: the platform collects standard output whatever the level
  (ADR-0051), and the code the project does not own logs through the process default or takes a
  logger it is handed.
