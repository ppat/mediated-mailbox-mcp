# 0076. Metrics are emitted through Prometheus's client_golang, on a registry each process builds

**Status:** Accepted ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

[ADR-0051](./0051-environment-contract.md) settles that every deployable emits Prometheus-format
metrics on a plain HTTP endpoint, and [ADR-0042](./0042-implementation-stack.md) leaves each major
library to its own decision. What is left is how the metrics are produced in Go. The first
consumers are the rate limiter's gauges and the count of provider request cost that
[ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md)'s alerting rules read, and
every later deployable emits through the same choice.

The choice looks like a test-tooling question and is not one. The alerting rules are tested with
Prometheus's rule unit tests, which feed rules synthetic series and never read an endpoint, so no
library changes whether those tests pass. What the library decides is whether the endpoint the
platform scrapes carries the names, labels and escaping the rules expect. A wrong encoding fails
silently. The scrape either rejects the text or reads a different series, and the paging alert of
[ADR-0024](../operability/0024-conservative-target-aimd.md) then watches nothing.

| Requirement | What it demands | From |
| --- | --- | --- |
| Prometheus text over HTTP | An `http.Handler` serving the text exposition format | [ADR-0051](./0051-environment-contract.md) |
| Counters and gauges with labels | Values dimensioned by account, priority class and provider, set from concurrent code | [ADR-0024](../operability/0024-conservative-target-aimd.md), [ADR-0025](../operability/0025-priority-classes-and-leases.md) |
| Exposition exactly as written | The metric name and labels the code declares are the ones scraped, with label values escaped and each family's `HELP` and `TYPE` lines present | This record, because the alerting rules match on names and labels |
| No package-level state in use | Each process builds its metrics and hands them where they are needed, so tests do not share state | This record, following [ADR-0040](./0040-pure-core-decisions-as-values.md)'s values-in style for the shell |
| Read back in memory | A test reads the emitted values without a listener or a stand-in | [ADR-0043](./0043-no-mocking.md) |
| A small dependency footprint | Modules and binary size added to every image that emits | This record |

Recording a metric changes state the call does not return, so no metrics library is admitted to a
pure core under [ADR-0071](./0071-static-enforcement-toolchain.md)'s conditions, whichever is
chosen. Metrics are shell code everywhere.

Exposition exactly as written ordered the field, with labels beside it, because a silent encoding
fault defeats the one alert that must page. Footprint broke ties. Maintenance cadence and
vulnerability history were read and not graded, because every candidate is maintained and none
carries an advisory on the code path used here.

## Decision

- **`github.com/prometheus/client_golang`**, the Go client the Prometheus project itself maintains,
  alongside the exposition format and `promtool`. Its vector types dimension a value by labels, its
  handler serves a registry, and its encoder escapes label values and writes `HELP` and `TYPE`
  lines. Measured in a scratch program, a label value holding a quote and a newline came out
  escaped as the format requires.
- **Each process builds one registry with `prometheus.NewRegistry()`** and passes it to whatever
  registers metrics. The handler serves that registry through `promhttp.HandlerFor`.

What the library's ordinary path does that this project forbids:

| Construction | Harm | What stops it |
| --- | --- | --- |
| The package-level default registry and its `MustRegister` and `Handler` helpers | Metrics registered in one test or package leak into every other, and a process serves metrics it never chose | A `forbidigo` ban on the default registry and the helpers that use it, proven by a violation file |
| `promauto` constructors without a registry | They register into the default registry | The same ban |
| The opt-in HTTP instrumentation middleware | It labels by request method, and an unbounded method set was the library's one advisory | Not used. Request counting happens where the provider call is made, which [ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md) requires anyway |

| Requirement | Met by |
| --- | --- |
| Prometheus text over HTTP | `promhttp.HandlerFor` over the process's registry |
| Counters and gauges with labels | `CounterVec` and `GaugeVec` |
| Exposition exactly as written | client_golang's encoder, which names and escapes exactly what was declared |
| No package-level state in use | A registry per process, and the ban above |
| Read back in memory | `Registry.Gather` and the library's `testutil` package, with no listener |
| A small dependency footprint | Accepted at the size measured below, with the reason under Alternatives |

What an implementer would otherwise pay to discover:

- The encoder lives in `github.com/prometheus/common/expfmt`, not in client_golang.
- The module graph reports about 35 modules, but a built program compiles 62 packages from 8
  modules. The rest are the modules' own test and tool dependencies, which never ship.
- `google.golang.org/protobuf` comes in through the metric data model and is already in this
  module's graph.

## Alternatives considered

Grades run 3 (meets it as shipped), 2 (meets it with configuration or a small wrapper the project
owns), 1 (meets it only by writing the mechanism) and 0 (cannot meet it).

| Requirement | client_golang | OpenTelemetry SDK and Prometheus exporter | VictoriaMetrics/metrics | Written by hand |
| --- | --- | --- | --- | --- |
| Prometheus text over HTTP | 3 | 2 | 2 | 1 |
| Counters and gauges with labels | 3 | 3 | 1 | 1 |
| Exposition exactly as written | 3 | 2 | 1 | 1 |
| No package-level state in use | 3 | 3 | 2 | 3 |
| Read back in memory | 3 | 2 | 3 | 2 |
| A small dependency footprint | 2 | 1 | 3 | 3 |

Package-level state separated nobody at the top, since every library can be used without its
defaults. Footprint and exposition were measured in scratch programs for all three libraries.
Labels and in-memory reading were measured for client_golang and VictoriaMetrics/metrics and read
from documentation for the OpenTelemetry exporter. client_golang is the only candidate with no
cell below 2.

| | client_golang | OpenTelemetry | VictoriaMetrics/metrics | By hand |
| --- | --- | --- | --- | --- |
| Modules a built program needs | 8 | 20, client_golang among them | 3 | 0 |
| Packages compiled | 62 | 88 | 3 external | 0 |
| A trivial static binary, against 5.6 MB with the standard library alone | 11 MB | 9.6 MB, a linker artifact of that program, since it compiles more packages | 6.0 MB | 5.6 MB |
| Name as scraped | as declared | adds unit and `_total` suffixes, a `target_info` series and scope labels unless each is switched off | as declared | as written |
| `HELP` text | yes | yes | none, the metadata writer takes no description | as written |

Whose worst cell is best decides it. client_golang's worst is the footprint, a cost paid in binary
size. Every other candidate's worst is a way for the scraped series to differ from what the rules
expect, which is the failure the paging alert cannot survive. No other part of the design guards
the exposition, so the strength client_golang brings is one nothing else supplies. Leaving it later
costs the registration calls in each shell and the test reads, since the metric names and rules
survive any exit.

- **client_golang.** For it, the reference encoder, vector types and an in-memory gather, from the
  project that defines the format. Against it, about 5.4 MB and 62 packages in every image that
  emits, mostly the data model's generated protobuf code, and a default registry that has to be
  banned rather than merely avoided.
- **OpenTelemetry's SDK with its Prometheus exporter.** For it, a vendor-neutral instrument API
  that could later export elsewhere. Against it, the exporter's own
  [documentation](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/prometheus) says it
  "implements `prometheus.Collector`", so it runs on top of client_golang rather than instead of
  it, and its defaults rename what the rules match on. Its footprint grade rests on the 20 modules
  and 88 packages it compiles, not on binary size, where its trivial program measured smaller.
- **VictoriaMetrics/metrics.** For it, the smallest footprint and no advisories. Against it, its
  [documentation](https://github.com/VictoriaMetrics/metrics) states "By default, exposed metrics do not have `TYPE` or `HELP` meta
  information", its metadata writer takes no description, labels are written by the caller into
  the name string, and the switch that turns metadata on is one value for the whole process.
- **Writing the format by hand.** For it, no dependency at all, the stance the Gmail OAuth code
  took with a small and stable protocol. Against it, the project would own escaping, name
  validation, metadata, concurrent reads and a label registry for every deployable indefinitely,
  and each fault fails silently.

## Consequences

- Every deployable that emits metrics carries client_golang, about 5.4 MB of binary.
- The default registry and its helpers are banned by `forbidigo` under
  [ADR-0071](./0071-static-enforcement-toolchain.md), proven by a violation file, so no shared
  state crosses tests or packages.
- Leaving the library costs the registration and gather calls in the shells. The metric names, the
  labels and [ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md)'s rules survive
  any exit unchanged.
- What would re-argue it is a need to export somewhere other than a Prometheus scrape, which would
  bring the OpenTelemetry exporter back, still on top of client_golang.
