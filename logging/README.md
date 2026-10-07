# logging

A narrow, named shared library, published as `mediated-mailbox-logging`. Shared code is pure, or it
is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Every deployable takes the level it logs at as one `log` section of its configuration, and logs
structured lines to standard output
([ADR-0119](../docs/adr/engineering/0119-each-deployable-logs-at-a-configured-level-through-the-logger-its-root-builds.md),
[ADR-0051](../docs/adr/engineering/0051-environment-contract.md)). This library is that section and
the logger built from it. The section's type and its validation are pure and sit in `core/`. The
shell builds the logger, JSON lines to the writer the composition root names at the configured
level, and a refusal of the level names the layer that set it, because the refusal comes before a
deployable logs its effective configuration
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).

The case for one library over per-deployable glue is drift that nothing would report. Each of four
composition roots, six once the reorg workload and the Heuristics Job are built, would otherwise
spell the four levels, map them to the log handler's levels and build the handler. A copy that
accepted `warning` or `INFO`, or took the log library's own parse, which ignores case and accepts an
offset such as `INFO+2`, would accept a level the others refuse. A copy that wrote text rather than
JSON, or to standard error, would break the environment contract, and one that mapped a level one
step off would hide warnings or flood the log. Each starts and runs normally. Written once, with a
test and a mutation patch for each rule, they hold in every deployable.

The library reads no environment and sets no process-wide state. Setting the process default for
the code the project does not own is the composition root's, with the logger this library returns.
