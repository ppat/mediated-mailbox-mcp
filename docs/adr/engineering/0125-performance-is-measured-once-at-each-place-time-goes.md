# 0125. Performance is measured once at each place a request or a job spends its time, and each series kept names the tuning question it answers

**Status:** Accepted ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable)

## Context

[ADR-0076](./0076-metrics-emitted-through-client-golang.md) settles that metrics are emitted
through client_golang on a registry each process builds, and refuses the default registry, the
library's HTTP middleware and OpenTelemetry. [ADR-0051](./0051-environment-contract.md) settles
that the platform scrapes the endpoint and owns collection, shipping and retention, and that the
app does not know its platform. [ADR-0040](./0040-pure-core-decisions-as-values.md) and
[ADR-0071](./0071-static-enforcement-toolchain.md) keep every metric in shell code and out of a
pure core. [ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md) keeps
every series a job kind of the worker produces attributable to its job kind. What is left is which performance measurements
the code captures, where each is captured, and how each is exposed.

The series emitted before this record answer correctness and liveness questions, such as the rate
limiter's state, the provider cost against the hard cap, the scan backlog and each job's last
success. None of them says where time goes. No series measured Gmail's latency, the rate lease's
wait, conversion or scan cost, a statement's latency, a wait for a pooled connection, an operation
of the client surface, or the Go runtime.

When something in this system is slow, the time is in one of a handful of places.

| Place | Why time goes there |
| --- | --- |
| The database pool | Every unit of work acquires a connection, and the worker's pools are sized by `pool_size` while the mediator's and the UI's take pgx's default of the larger of 4 and the processor count |
| A database statement | Every operation, page and tick runs statements, and [db/tx](../../../db/tx/tx.go) adds four round trips to each transaction to set and read back the account |
| The policy load a body request waits on | [ADR-0099](./0099-a-body-request-loads-the-policy-before-it-decides.md) puts a coalesced load on the body request's path |
| The rate lease | [ADR-0025](../operability/0025-priority-classes-and-leases.md)'s limiter makes a caller wait for its class, asking again at least every half second |
| Gmail | Every body, page and change set is a provider call, bounded by `provider_timeout` on the body request |
| Conversion and scanning | Every body is converted to Markdown and scanned in memory |
| The Go runtime | CPU and memory, including garbage collection |

Backfill's second pass is serial within an account. Gmail allows 6,000 units per minute per user
and `messages.get` costs 20 units ([Gmail quota](https://developers.google.com/workspace/gmail/api/reference/quota)).
The adapter averages that to 100 units per second and the target is at most half of it, so one
account's body fetches top out near 2.5 per second, and a pass is rate-bound only while Gmail's
latency, conversion and scan together stay under about 400 ms per message. Whether the pass is
bound by the rate budget or by its own loop is the central tuning question of a backfill that runs
for hours, and only the lease wait, Gmail's latency, the conversion and scan cost and the pass's
throughput together answer it.

The practices this record draws on each exist to localize time, and are read here for that intent
rather than followed as lists.

| Practice | Its intent as it bears here | Source |
| --- | --- | --- |
| The four golden signals | Latency, traffic, errors and saturation, with latency read as a distribution, and a slow error worse than a fast one. Signals collected but read by no dashboard and no alert are candidates for removal | [Google SRE book, Monitoring Distributed Systems](https://sre.google/sre-book/monitoring-distributed-systems/) |
| RED | Rate, errors and duration at the edge of a request-driven service | [Grafana, The RED Method](https://grafana.com/blog/2018/08/02/the-red-method-how-to-instrument-your-services/) |
| USE | Utilization, saturation and errors per resource, a software pool included, saturation being work queued that the resource cannot yet serve | [Brendan Gregg, The USE Method](https://www.brendangregg.com/usemethod.html) |
| Prometheus instrumentation | Online serving counts queries, errors and latency. Offline processing tracks items in and out per stage. A library reaching outside the process tracks at least its query count, errors and latency. Labels are added for a concrete use | [Prometheus, Instrumentation](https://prometheus.io/docs/practices/instrumentation/) |
| Data pipelines | Efficiency measured per stage, and historical runtimes as an early sign of degradation | [Google SRE workbook, Data Processing Pipelines](https://sre.google/workbook/data-processing/) |
| A proxy in front of an upstream | The upstream's request time kept apart from time pending a local slot, so a slow upstream is told apart from local queuing | [Envoy cluster statistics](https://www.envoyproxy.io/docs/envoy/latest/configuration/upstream/cluster_manager/cluster_stats) |
| Histograms | Histograms aggregate across processes and summaries do not, and each bucket is a series | [Prometheus, Histograms](https://prometheus.io/docs/practices/histograms/) |
| Pool sizing | A small pool with callers waiting is healthy, and waits read against statement latency say whether the pool or the database is the limit | [HikariCP, About Pool Sizing](https://github.com/brettwooldridge/HikariCP/wiki/About-Pool-Sizing), [HikariCP, Dropwizard Metrics](https://github.com/brettwooldridge/HikariCP/wiki/Dropwizard-Metrics) |
| Pool exhaustion in practice | Healthy databases with requests failing at the connection ceiling, found late because nothing measured the pool | [Buttondown incident 0024](https://buttondown.com/blog/incident-0024), [Val Town postmortem](https://blog.val.town/blog/post-mortem-exhausted-host-connection-pool) |
| Go garbage collection | GC CPU scales with the live heap, a memory limit needs headroom above it, and RSS is what a container limit sees | [Go GC guide](https://go.dev/doc/gc-guide), [Uber, How We Saved 70K Cores](https://www.uber.com/blog/how-we-saved-70k-cores-across-30-mission-critical-services/), [runtime/metrics](https://pkg.go.dev/runtime/metrics) |

| Requirement | What it demands | From |
| --- | --- | --- |
| Each place time goes is measured once | One series family per place, captured at the one point every path through it passes | This record |
| A reading that moves something | Each series kept has a reading that sends the reader to a setting or to a piece of code | The SRE book's removal test, and the operator's request that nothing be overkill |
| Closed label sets | No label takes a value from a message, an SQL text, a path or any other unbounded set | [ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md), [ADR-0076](./0076-metrics-emitted-through-client-golang.md) |
| Nothing assumed of the platform | No measurement depends on a tracing backend, container metrics or a server-side extension | [ADR-0051](./0051-environment-contract.md) |
| Shell code only | Capture sits in shells and composition roots, and the mediator's service layer takes no metrics import | [ADR-0040](./0040-pure-core-decisions-as-values.md), [ADR-0071](./0071-static-enforcement-toolchain.md) |

The reading requirement ordered the candidates, because a series no reading acts on is the
overkill the operator ruled out. Closed labels and the platform gate any series whatever its
reading.

## Decision

Eleven measurements are kept, each at the one place its time is spent. Every series is a counter, a
gauge or a classic histogram on the process's registry. The worker's carry `job_kind` through the
registerer each job kind is handed, apart from the Go runtime's and the process's, which belong to
the process rather than to a job kind. The bucket boundaries are reasoned from the latencies the
Context names, since no production run exists yet.

### The client surface

**K1. Operation latency.** `mediated_mailbox_mediate_operation_duration_seconds`, a histogram
labelled by `operation`, the name of each operation the registry holds, with buckets at 0.005,
0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10 and 30 seconds.

- **Question.** Which client operation is slow, and did it change?
- **Reading.** Read against K2, K3, K4, K5 and K8. A slow `get_message_body` breaks down into the
  policy load, the lease wait, Gmail, conversion and statements. A slow listing whose statements
  are fast spends its time in the process.
- **Evidence.** RED's duration and the SRE book's latency signal. The UI's dataset reads already
  carry one, and the mediator carried none.
- **Capture.** The mediator's composition root wraps each operation's handler before the registry
  is built, so every call through either root is observed and the service layer is unchanged. It
  observes the handler's run, after the registry's account check.
- **No outcome label.** A denied body is a successful call, which the body counters already split,
  and a malformed call is refused before the handler runs.

**K2. Policy load duration.** `mediated_mailbox_policyload_reload_duration_seconds`, a histogram
with no label of its own, with buckets at 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1 and 5 seconds.

- **Question.** How much of a body request is the coalesced policy load it waits for, and does it
  grow with the rules? Is the worker's reload cheap?
- **Reading.** A large share of K1's body latency means per-request policy freshness is the cost,
  and the fix is in `executioncontext/policyload` or its statements, which K8 names.
- **Evidence.** The pipeline guidance on per-stage cost, and ADR-0099's placement of the load on
  the request path.
- **Capture.** `(*policyload.Loader).Reload`, the one load every process uses, beside its
  reload-failure gauge. It observes every reload its caller did not cancel, succeeded or failed,
  from its call to its outcome, so a wait for the turn of a reload already running counts, as it
  does for the body request waiting on it.

### Provider calls

**K3. Gmail request latency.** `mediated_mailbox_provider_request_duration_seconds`, a histogram
labelled by `provider`, `endpoint` and `outcome`, with buckets at 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
10 and 30 seconds. `endpoint` is the Gmail method a request calls, such as `messages.get`, set by
the constructor of each request, and `outcome` is `ok`, `throttled` or `failed`.

- **Question.** Is Gmail slow, and on which endpoint? Is `provider_timeout` far above the real
  tail? How often do calls fail or throttle, and are the failures slow?
- **Reading.** A `messages.get` median near 400 ms means the second pass is latency-bound rather
  than rate-bound. A tail near the timeout means real answers are being cut. Flat latency while
  operations slow rules Gmail out. Its `_count` by `outcome` is the request and error rate, so no
  separate counter exists.
- **Evidence.** Prometheus's rule for libraries that reach outside the process, Envoy's upstream
  request time and response classes, and the SRE book's separation of failed requests' latency.
- **Capture.** The adapter's one sending function, from sending the request to reading its body.
  The token is obtained before the timer starts.
- **No `account` label.** One API serves every account alike, and the per-account cost counter
  stays.

**K4. Rate lease wait.** `mediated_mailbox_ratelimit_lease_wait_seconds`, a histogram labelled by
`class`, `interactive`, `sync` or `batch`, with buckets at 0.001, 0.01, 0.1, 0.5, 1, 2, 5, 10, 30,
60 and 300 seconds.

- **Question.** How much of a body request is the budget's deliberate wait? Is backfill rate-bound
  as designed, or bound by something else? Does an agent's body fetch queue behind backfill?
- **Reading.** Batch waits near zero while the second pass is slow mean its serial loop is the
  limit, and the fix is in code. Large batch waits mean backfill runs at the designed rate, and only
  the target moves it ([ADR-0024](../operability/0024-conservative-target-aimd.md)). Interactive
  waits above zero mean the agent waits on the budget, and the levers are class priority and the
  target.
- **Evidence.** USE's saturation, and Envoy's split between time pending a local slot and the
  upstream's time.
- **Capture.** `(*lease.Limiter).Acquire`, from its entry to a granted lease. A refused or
  cancelled ask is not observed.
- **No `account` label**, for K3's reason.

### Body processing

**K5. Conversion and scan duration.** `mediated_mailbox_body_processing_duration_seconds`, a
histogram labelled by `step`, `convert` or `scan`, with buckets at 0.0005, 0.001, 0.005, 0.01, 0.05,
0.1, 0.5, 1 and 5 seconds.

- **Question.** What does each body cost in CPU on this side? In the second pass's serial loop the
  cost adds to every message, and on the client surface to every served body.
- **Reading.** A conversion or scan tail that is a real share of Gmail's median means CPU per
  message limits the pass or the body request, and a CPU profile finds where. Readings well under a
  millisecond rule both out.
- **Evidence.** The pipeline guidance on per-stage efficiency, and Prometheus's per-stage duration
  for offline processing. [ADR-0074](../redaction/0074-html-to-markdown-v2-converts-bodies.md)
  records that converters can blow up on nested input, and the chosen one's cost on real mail is
  unmeasured.
- **Capture.** In the worker, each conversion and each scan of the second pass and of delta sync's
  tick, the one series defined once for both job kinds. In the mediator, the conversion the body
  operation is handed, wrapped in the composition root. The mediator's serve-time pattern check
  runs inside the release decision's pure core over a scanner value, so observing it would need a
  hook in the service layer, and it is not observed. K1's body latency bounds it.
- **No body-derived label**, as everywhere ([ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md)).

### Throughput of the index's passes

**K6. Messages made durable.** `mediated_mailbox_index_messages_total`, a counter labelled by
`account` and `stage`. `indexed` counts the messages the first pass and a tick, its changes and its
reconciliation of a window alike, added to the index, and `scanned` the messages the second pass and a tick scanned and made durable.

- **Question.** How fast is each pass going, is the rate steady or decaying as the index grows, and
  how long until a pass ends? Does a tick keep up with what arrives?
- **Reading.** A second pass near 2.5 per second per account at the default target is rate-bound
  as designed, which K4 confirms. Well below that it is latency-bound, and K3, K5 and K8 say where.
  A first pass whose rate falls as the index grows points at a statement. Inflow above what the
  tick scans shows as the backlog rising, and the levers are `decisions_per_tick` and
  `sync_interval`.
- **Evidence.** Prometheus's items in and out per stage of offline processing, and the pipeline
  guidance on throughput. Before this record the second pass's rate could be read only from the
  backlog's slope, and the first pass's and the tick's not at all.
- **Capture.** Where each step's or tick's counts are already added to the unclassified series,
  defined once for both job kinds. `account` follows the worker's per-account series.

### Job runs

**K7. Job run duration, split by job.** The scheduler's
`mediated_mailbox_job_run_duration_seconds` is labelled by `job_kind` and `job`, `reload` for the
job keyed by the kind alone and `account` for an account's job, and observes only runs that
succeeded, failed or panicked. `mediated_mailbox_job_runs_finished_total` keeps counting every
outcome, `not_due` and `cancelled` included.

- **Question.** How long does a tick, a reload or a backfill run take, and is any creeping toward
  its interval?
- **Reading.** A tick's tail approaching `sync_interval` means freshness will slip, so raise the
  sync concurrency or find the slow stage. A reload's duration growing tracks the snapshot's and
  the policy's cost.
- **Evidence.** The pipeline guidance on historical runtimes. Labelled by the job kind alone and
  observing every outcome, the series mixed a reload's duration with a tick's and drew instant
  `not_due` runs into the distribution, so it could not answer its own question. No alerting rule
  reads it.
- **This also measures the worker's account and policy reloads**, which is why no separate timer
  of the snapshot load exists.

### The database

**K8. Statement latency.** `mediated_mailbox_db_statement_duration_seconds`, a histogram labelled
by `statement`, with buckets at 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1 and 5 seconds.

- **Question.** Which statement is slow, at the median or only in the tail, in which deployable or
  job kind? Did a schema or query change make it faster? What does the per-transaction account
  setup cost?
- **Reading.** A statement whose tail climbs with the corpus while others hold is an index or plan
  problem, fixed in its data-access subsection or a migration. Every statement fast while an
  operation is slow rules the database out. The `begin`, `commit` and transaction setup rows price
  the round trips each transaction adds.
- **Evidence.** Prometheus's rule for libraries that reach outside the process, the SRE book on
  distributions over means, pgx's [`QueryTracer`](https://pkg.go.dev/github.com/jackc/pgx/v5#QueryTracer)
  hook, and [sqlc-pgx-monitoring](https://github.com/amirsalarsafaei/sqlc-pgx-monitoring), a
  published tracer built for latency per sqlc name.
- **Capture.** A `pgx.QueryTracer` on every pool's connection configuration, in `process/dbmetrics`,
  set by each composition root. The label is the name in the `-- name:` comment the generated SQL
  starts with. `db/tx`'s own statements carry such comments too. A statement with none is labelled
  `begin`, `commit` or `rollback` by its first keyword, and `unnamed` otherwise, so the set is
  closed. Every statement name in the data-access library is unique, which `db/check` holds, so a
  label names one statement. A `Query`'s trace ends when its rows are closed, so its reading
  includes the generated code's row scanning.
- **No error label.** It would double the largest histogram, and errors are returned and logged.

**K9. Pool statistics.** Six series per pool, read from `pgxpool.Stat` when scraped.

| Series | Kind | From |
| --- | --- | --- |
| `mediated_mailbox_db_pool_acquires_total` | Counter | Every acquire that succeeded |
| `mediated_mailbox_db_pool_waited_acquires_total` | Counter | The acquires that waited for a connection because none was idle |
| `mediated_mailbox_db_pool_acquire_wait_seconds_total` | Counter | The time those acquires waited |
| `mediated_mailbox_db_pool_canceled_acquires_total` | Counter | The acquires whose caller gave up waiting |
| `mediated_mailbox_db_pool_acquired_connections` | Gauge | The connections held now |
| `mediated_mailbox_db_pool_max_connections` | Gauge | The pool's ceiling |

- **Question.** Is time lost waiting for a connection rather than in the database? Is the worker's
  `pool_size` right, and is pgx's processor-dependent default right for the mediator and the UI,
  which have no setting? Do callers give up waiting?
- **Reading.** A rising waited fraction with flat statement latency means the pool is the limit,
  so raise it or find code holding a connection across other work. Waits with rising statement
  latency mean the database is saturated, where a bigger pool hurts. Sustained waits in the mediator
  or the UI are the evidence for a pool size setting there.
- **Evidence.** USE, HikariCP's sizing and metrics, and the Buttondown and Val Town postmortems.
- **Capture.** A collector over the pool in `process/dbmetrics`, registered by each composition
  root, which reads the pool only when scraped and adds nothing on the path of a statement.

The tracer and the collector are written in this project rather than taken from a library.

| Requirement | Written here | sqlc-pgx-monitoring | [otelpgx](https://github.com/exaring/otelpgx) | [pgxpoolprometheus](https://github.com/IBM/pgxpoolprometheus) |
| --- | --- | --- | --- | --- |
| Emits through client_golang, with no OpenTelemetry layer | Yes | No, it emits through OpenTelemetry | No, it emits through OpenTelemetry | Yes |
| Names in seconds and counters ending `_total`, under the project's prefix | Yes | Set by OpenTelemetry's conventions | Set by OpenTelemetry's conventions | No, `pgxpool_acquire_count` and nanoseconds |
| A closed label of the sqlc name | Yes | Yes | Labels by operation and text unless configured | Not its concern |
| Only the series kept | Yes | Not its concern | Not its concern | Thirteen series |
| No new module in a shipped image | Yes | OpenTelemetry's SDK and exporter | OpenTelemetry's SDK and exporter | One module |

The two OpenTelemetry tracers reach a Prometheus scrape only through OpenTelemetry's exporter,
which [ADR-0076](./0076-metrics-emitted-through-client-golang.md) rejected because it runs on top
of client_golang and renames what it exports. pgxpoolprometheus's own collector declares
`pgxpool_acquire_count` as a counter and reports waits in nanoseconds, which a rename cannot fix
without forking it. The tracer and the collector are about 150 lines on pgx and client_golang,
which every deployable already carries.

### The Go runtime and the process

**K10. The Go collector**, its default set and four runtime metrics added to it,
`go_gc_heap_live_bytes`, `go_cpu_classes_gc_total_cpu_seconds_total`,
`go_cpu_classes_total_cpu_seconds_total` and `go_sched_latencies_seconds`.

- **Question.** What is each deployable's live heap at peak, which sizes its memory limit and
  `GOMEMLIMIT`? Is garbage collection eating CPU? Are goroutines leaking, in the UI's streams or the
  worker's jobs? Is the process starved of CPU?
- **Reading.** Peak live heap plus headroom sizes the limit, and the worker's checks ADR-0117's
  claim that its concurrency bounds its memory. The heap goal near the limit with GC CPU rising is
  thrashing. GC CPU over total CPU high under steady load points at allocation hot spots, found next
  with a heap profile, or a GOGC set too low. `go_goroutines` rising under flat load is a leak. A
  scheduling latency tail in tens of milliseconds means waiting for a CPU.
- **Evidence.** The Go GC guide and Uber's GC tuning, which tracked GC CPU and the live heap.
  client_golang [reverted](https://raw.githubusercontent.com/prometheus/client_golang/main/CHANGELOG.md)
  exposing every runtime metric by default, so the additions are named one by one.
- **Capture.** Each composition root registers it once on the process's registry. The default set
  carries `go_goroutines`, `go_threads` and the effective GOGC, memory limit and GOMAXPROCS, which
  the rest are read against.

**K11. The process collector**, its resident memory, CPU seconds, open and maximum file
descriptors and start time.

- **Question and reading.** Resident memory near the limit with a modest live heap is growth
  outside the heap, such as goroutine stacks. Open descriptors near the maximum are leaked sockets.
  A start time that moves is a restart or an out-of-memory kill.
- **Evidence.** The GC guide's advice to watch resident memory, and ADR-0051, under which container
  metrics cannot be assumed.
- **Capture.** Each composition root registers it once on the process's registry.

### Where each is registered

| Process | Registers |
| --- | --- |
| The mediator | K1, K2, K3, K4, K5's conversion step, K8 and K9 for its pool, K10 and K11 |
| The UI | K8 and K9 for its pool, K10 and K11, beside the dataset read latency it already emits |
| The worker | K10 and K11 once. Per job kind, through the registerer that adds `job_kind`, K2, K3, K4, K5, K6, and K8 and K9 for the kind's pool. K7 on the scheduler's series |

## Alternatives considered

The candidates left out, each with the case for it and why it lost.

| Path | Candidate | The case for it | Why it is left out |
| --- | --- | --- | --- |
| Client surface | A span per stage of each body request | It shows each request's split rather than an aggregate | It is tracing, the environment contract names no tracing backend, and ADR-0076 rejected OpenTelemetry. K1 with K2 to K5 and K8 gives the split in aggregate, which is what a tuning decision reads |
| Client surface | HTTP middleware per route | It comes with client_golang | ADR-0076 rejected it, and K1 measures at the operation, which means more than a route |
| Client surface | An outcome label on K1 | Errors read by latency | A denied body is a successful call that the body counters split, and a malformed call is refused fast and logged |
| Client surface | A count of MCP sessions | It is a count of what is held in memory | The MCP root is stateless, so nothing is held |
| Client surface | The mediator's account snapshot load time | It is a reload's cost | It runs on an interval off the request path, its statements are in K8, and no setting follows from it |
| Provider calls | The token refresh's latency | It is a provider call | It happens about once an hour per account |
| Provider calls | An `account` label on K3 and K4 | Per-account attribution | One API and one budget mechanism serve every account alike, and the per-account counters stay |
| Provider calls | Retry and backoff counters | They count trouble | Backfill records each retry and backoff as an event of its run, which the Jobs screen shows, and throttles are already counted |
| Body processing | A distribution of body sizes | It explains slow conversions | Memory per body is bounded by the adapter's response limit and measured for real by K10's live heap. A CPU profile explains a slow conversion better, and no setting follows from size |
| Body processing | Scan hits by pattern | It counts the scanner's work | It is a correctness concern, recorded already as masking events, and not a performance question |
| Backfill | A gauge of a pass's progress or its expected end | It answers "how long" directly | The UI shows a pass's page of its total from recorded state, and K6's rate with that remaining count gives the end. A gauge would copy recorded state |
| Backfill | Time to finish a pass, or each page's duration | A pass's cost | Each run's start and end are recorded and shown on the Jobs screen, K7 gives run duration, K6 the throughput, and K3, K4, K5 and K8 split a slow step |
| Delta sync | Index lag measured against Gmail's latest history identifier | Freshness measured directly | It needs one more provider call per tick. The last-success series with its late rule bounds the lag at a tick plus its interval, and K7 gives the tick's duration |
| Delta sync | Changes per tick as a histogram | Inflow per tick | K6 by stage gives inflow, and the backlog gauge shows whether scans keep up |
| UI | Latency of its other routes and of the stream's poll | Coverage of every route | Operator writes run at human rate, each poll's statements land in K8, and the subscriber gauge exists |
| Database | [pg_stat_statements](https://www.postgresql.org/docs/current/pgstatstatements.html) instead of K8 | It is the server's own view, per role and statement | It may not be loaded, which ADR-0051 forbids assuming, it sees server execution and not the round trip, row reading or pool wait, it keeps means and not a distribution, and mapping its text back to a statement is manual. It stays the deploying side's deeper tool |
| Database | A histogram of every pool acquire | Each wait's distribution | K9's counters give the waited fraction and the mean wait, which is what sizing reads |
| Database | Connection hold time | HikariCP measures it | Little's law over held connections and the acquire rate gives the mean where it matters, and at idle there is nothing to find |
| Database | Idle, total, constructing and new connection counts, and destroys by lifetime and idleness | They come with `pgxpool.Stat` | No setting here is tuned from them, and churn shows as errors |
| Database | An error label on K8 | Errors by statement | It doubles the largest histogram, and errors are returned and logged |
| Database | A timer around each transaction | A unit's whole cost | It is the sum of its statements, now including the setup rows, and no name is available where it runs |
| In memory | Counts of the account snapshot, the policy snapshot, the sessions and the scheduler's jobs | The operator's example of counts held in memory | Each is bounded by the account count, which is single digits. What can grow, the UI's streams and running jobs, is already gauged, and the code holds no cache |
| Go runtime | Every runtime metric | The full picture | client_golang reverted it as a default, and most are allocator internals with no setting here |
| Go runtime | Mutex wait time | Lock contention | No shell holds a hot shared mutex, and a mutex profile is the tool if one is suspected |
| Go runtime | `go_gc_duration_seconds` read as GC cost | It is in the default set | It is pause time, while the cost is concurrent marking's CPU, hence K10's ratio. It stays in the default set |
| Histograms | Native histograms beside the classic buckets | Finer resolution at less cost, and harmless to a text scrape | Scraping them is a per-job platform setting the app cannot know (ADR-0051), and [ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md)'s rule tests read classic series |
| Profiling | `net/http/pprof` on the probe listener, off by default | It is the most effective tool for CPU and allocation hot spots | It is an exposure decision on processes that hold full-mailbox credentials, not a measurement, and is the operator's to make in a record of its own |
| Statement names | Labelling a statement by its subsection as well as its name | Two subsections could give statements one name | The tracer sees only the statement's SQL, which carries no subsection. No two statements of the library share a name, and `db/check` refuses one that would, so the name alone names one statement |

## Consequences

- **Series count.** K8 is the one family that knowingly exceeds Prometheus's guideline of fewer
  than ten series per metric, at about 11 series for each statement a process runs, up to about
  1,000 in the UI and 1,400 in the worker, because attribution per statement is the point. Every
  other family stays within tens of series per process.
- **The bucket boundaries are reasoned, not measured.** Each is reconsidered once real runs of
  backfill and of the mediator exist.
- **What it assumes of the platform.** The platform scrapes each process's metrics endpoint and
  keeps what it scrapes ([ADR-0051](./0051-environment-contract.md)). The deploying side owns
  pg_stat_statements if it wants the server's view, and nothing here depends on it. Dashboards are
  configuration on series that exist.
- **What it assumes of other components.** sqlc keeps starting each generated statement with its
  `-- name:` comment, which the tracer reads. pgx keeps calling a `QueryTracer` around `Query`,
  `QueryRow` and `Exec`, and the code uses no batch, copy or prepare call that the tracer does not
  see. `pgxpool.Stat` keeps its acquire counters. A pgx major that moved any of these fails the
  build or the tracer's tests.
- **What it adds to other code.** Each Gmail request constructor names its endpoint. The
  scheduler's run duration series carries the job and observes fewer outcomes. Each composition
  root registers the runtime, process and database collectors, and the worker opens its pools with
  its registry in hand.
- **A panic in a collector** runs on the scrape's goroutine, which the worker's scheduler does not
  own and [ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)
  leaves to review. The tracer runs on the goroutine that runs the statement, so in the worker a
  panic in it is the run's, which the scheduler recovers.
- **What would re-argue it.** An export target other than a Prometheus scrape brings OpenTelemetry
  back, as ADR-0076 states. A platform that always provides container metrics makes K11
  redundant. A first real run whose readings no series answers, or a series nobody reads once the
  system runs, re-opens the kept set.
