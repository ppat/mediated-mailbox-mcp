# 0119. The worker's jobs are scheduled by an in-process scheduler that stores nothing, from recorded state read on a cadence, and nothing messages through PostgreSQL

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox), [O2](../../../USE_CASES.md#o2--observable)

## Context

One worker process runs every background job kind
([ADR-0117](./0117-one-background-worker-runs-every-job-kind.md)). Something inside it has to decide
when each job runs, keep one job from running twice, stop jobs, and make a stuck or stopped job
loud. Each job kind needs to run on its own condition.

| Job kind | Runs when | Must not | Runs for |
| --- | --- | --- | --- |
| Delta sync | every sync interval, five minutes by default, and at start | overlap itself for an account | seconds, minutes after a burst |
| Backfill | an account has a pass not ended, after the worker starts with another scanner or other thresholds, and when an account is newly listed | run twice on one account | hours |
| Reorg apply and rollback | a plan's status reads approved, a rollback is requested, or a plan reads applying after a crash | run twice on one plan, or start without the operator's approval | minutes |
| Heuristics | daily, or as its unit decides | — | seconds |

The records already make correctness independent of when a run happens. Every run that masks,
scans or classifies compares what the index stores with what it runs with, by effect, and acts on
the difference ([ADR-0037](../redaction/0037-delisting-transition.md),
[ADR-0120](../redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md),
[ADR-0121](../redaction/0121-the-run-start-step-decides-each-gate-skip-again.md),
[ADR-0113](../redaction/0113-an-added-rule-reaches-the-stored-classes-by-its-effect.md)).
Checkpoints commit with the work they cover ([ADR-0017](../data/0017-two-pass-backfill.md),
[ADR-0045](../engineering/0045-crash-injection-testing.md)), and a body is released only by the
mediator's decision at fetch time ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md),
[ADR-0099](../engineering/0099-a-body-request-loads-the-policy-before-it-decides.md)).

A backfill run that loads the policy once at its start and holds it for every page of both passes,
for hours, lets a restriction the operator adds during the run reach neither its second pass's
check nor the stored classes until the next run. The second pass can then fetch and scan the body
of a sender the operator just restricted, which
[ADR-0008](../redaction/0008-restricted-senders-are-never-scanned.md) and
[ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md) intend never to happen.

This record also decides that nothing messages through PostgreSQL.

## Decision

**The scheduler decides when to ask. Each job kind's due decision decides whether there is work.**

- **One scheduler inside the worker, storing nothing.** It holds in memory which jobs exist, when
  each last ran, which wakes are pending, each job's backoff, and which comparisons this process has
  made, and loses all of it on a restart, which is right, because each is rebuilt from recorded
  state. It owns at most the event loop.
- **Each job kind has a pure due decision in its own `core`**
  ([ADR-0040](../engineering/0040-pure-core-decisions-as-values.md)). It takes recorded state and
  the time as values, namely completion flags, latest runs, plan status, accounts and policy, and
  returns the work there is, or none. The comparisons by effect the runs already make decide what
  the work is. A lost, duplicated or mistimed wake only shifts timing and never releases anything.

**The wakes.**

| Wake | What it runs |
| --- | --- |
| The worker starting, or an account appearing in a job kind's reload | The full run-start step, then whatever pass is not ended. Backfill's run-start comparisons are due once per process and account, because their inputs, the scanner and the thresholds, change only across a restart |
| Each job's own timer, and a slow reconciliation timer every job kind has | The due decision only. For backfill that is two one-row reads. A lost wake costs at most one reconciliation interval |
| A policy change during a running second pass | The delisting and added-rule comparisons again inside the running run, at its next page boundary, where "changed" means the rules differ by value, not that a reload built a new snapshot ([ADR-0037](../redaction/0037-delisting-transition.md), [ADR-0113](../redaction/0113-an-added-rule-reaches-the-stored-classes-by-its-effect.md)) |
| Pass 1 ending | Pass 2 |

A wake for a job that is running, or already waiting to run, coalesces into one further run. A job
ensured again while its removal waits for its loop starts once that loop has returned, and a removed
job's loop reports nothing on the key once the job is ensured again, so the new job's series are its
own. A job's own timer asks on its interval's phase, as a ticker ticks, and each ask carries the
latest time on the ticker's phase at or before the ask. Delta sync measures its interval from that
time for its last tick, so a tick at the end of a backoff keeps the phase. Nothing wakes the worker
from outside it. It serves the health probe and the metrics endpoint and nothing else
([ADR-0051](../engineering/0051-environment-contract.md)).

**Units of work take the active snapshots.** Each job kind's unit of work takes its kind's active
account snapshot and policy snapshot at its entry
([ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md),
[ADR-0090](./0090-accounts-reach-deployables-as-reloaded-snapshots.md)). The unit is a tick for
delta sync, a page of each pass for backfill, and for apply the whole plan for policy and a batch
for the accounts. So a policy edit reaches the second pass's restricted-sender check within one
reload and one page, and the stored classes within the same bound through the comparisons inside
the run.

**Control of runs.**

- **One run at a time** per job kind and account, and per plan, held in the process. With one copy
  of the worker running, that in-process exclusion is the whole claim.
- **Apply starts from APPROVED by a compare-and-set on the plan's status**, so no two runs start one
  plan whatever the platform does
  ([ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)). A rollback request is a plan
  status, written under the UI's existing grant
  ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)).
- **A concurrency limit per job kind**, which bounds the bodies held in memory at once.
- **A connection pool per job kind, under its own role**
  ([ADR-0118](../data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)), so one job's
  transactions never starve another of connections.
- **Capped exponential backoff on failure, reset on success.** A wake during a job's backoff waits
  for the backoff to end. A failing job never retries in a tight loop and never holds up another.
- **A panic is recovered within the run that raised it**, recorded as that run's failure with its
  stack, and stops nothing else. Go cannot recover a panic in a goroutine the run did not start
  itself, so job code starts no goroutine outside a helper that recovers. The goroutines analyser
  holds the worker's code outside its scheduler to this, refusing a `go` statement and any use of
  a function that starts a goroutine running what it is given, the standard library's and those of
  `golang.org/x/sync`. Left to review are such a function reached through an interface value or
  reflection, a dependency that calls a function or value it was given on a goroutine of its own,
  such as a metrics collector gathered concurrently, a connection pool's hook or a reader that
  `os/exec` copies, and a function of any other module that starts a goroutine.
- **Stopping.** Shutdown and an account dropped from a reload cancel the run's context, and the run
  stops at its next unit boundary with its checkpoint durable. A later interrupt is a cancelled
  context. A later pause is a decision recorded through the UI that the due decision reads.
- **A stuck run is not killed.** Its running time and its job's last success make it visible, and
  the timeouts its own calls carry bound it.

**Loaders.** Each job kind that holds credentials has its own account loader and policy loader,
under its own role and on its own pool, reloaded about every minute, and writes back the credentials
it rotated itself, as each deployable does
([ADR-0082](./0082-rotation-writeback-to-the-database.md),
[ADR-0089](./0089-sealed-values-written-by-compare-and-set.md)). Two job kinds that both rotate a
credential reconcile through the stored bytes by compare-and-set. Each job kind's loader adds and
drops its own jobs, from a reload that is itself a job of the kind, so the reload is asked, backed
off and watched as every job is. A reload whose policy load fails adds no job and still drops the
jobs of the accounts it no longer lists, because an account whose rules were never read would be
decided as if every sender were restricted. The heuristics run holds no credential loader. It reads the list of account
identifiers under its own role if the heuristics unit decides it reads `accounts`, which is not
decided here and is tracked in [ROADMAP.md's open decisions](../../../ROADMAP.md#open-decisions)
([ADR-0091](../data/0091-accounts-listed-apart-from-their-state.md)). The loader design changes no
grant.

**Observability.** Series per job kind and a time of last success per job, on the registry the
worker builds ([ADR-0076](../engineering/0076-metrics-emitted-through-client-golang.md)). The shared
library series carry the job kind, and the scheduler gives each job its identity as a constructor
parameter, never through ambient context. Logs carry the job kind and the account. An alert fires
when the worker stops, and one fires for each job of every job kind whose last success grows too
old, which for delta sync is how [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox)'s "a chronically
stuck sync job looks healthy" is caught ([ADR-0077](./0077-conditions-raised-as-alerting-rules.md)).

- **A job's success is a run that succeeds or a unit of work a run makes durable**, such as a page
  of backfill. A pass that runs for hours therefore stays silent while its pages land, and a run
  stuck on one call ages from its last page. A run whose due decision finds nothing to do succeeds,
  so a job that has caught up keeps succeeding. A run asked before it is due, such as a tick asked at
  the end of a backoff before its interval has passed, is neither a success nor a failure. Not due
  means only that the job's own interval or condition for asking again has not come, never that the
  job found no work, which is a success.
- **Each job exports its own bound** beside its last success, and the alerting rule on a job's last
  success compares the two per job, so a job kind added later brings its bound and needs no rule of
  its own.
- **Waiting for a slot is not lateness.** The time a job waits for a slot of its kind's limit, while
  every slot is held, does not count toward its age. Each job exports the waits that ended since its
  latest success, which a success clears, and when the wait in progress began, and the alerting rule
  subtracts both from the job's age. A run that holds a slot without a success still ages. Only the
  process that saw a wait knows it, so a wait from before a restart counts as lateness after it. The
  operator chose not to record waits across a restart.
- **A job's latest success starts from what its job kind's runs record.** At its ensure, a job's
  latest success is its kind's latest recorded success for the account, under the meaning of success
  above, so a page made durable counts. A restart therefore leaves a job that does not succeed
  ageing, and a worker killed again and again is caught by the alerting rule on a job's last
  success.
  - Delta sync takes the later of its latest succeeded tick's end and its latest recorded progress.
    With ticks and no success, it takes its earliest tick's start, and with no tick, the ensure.
  - Backfill with work outstanding, a pass not ended or the second marked to start over, takes the
    later of its latest progress event and its latest succeeded run's end. With neither, it takes
    its earliest run's start, so a first pass in a crash loop ages from its first attempt, and with
    no run, the ensure.
  - Backfill with no work outstanding takes the ensure, since it has nothing that can fall behind. A
    worker in a crash loop is still loud, through the backfill job of any account with work
    outstanding, whose latest success stops moving, and through every account's delta sync job,
    whose ticks stop.
  - A job kind's reload job takes the ensure.
  - A failed read of what the runs record is logged as a warning, and the job counts from the ensure.

  After real downtime, delta sync's jobs fire the rule until their first ticks land, seconds after
  the start, since the accounts really were late.
- **An account's series go once the work that sets them ends.** Backfill removes an account's scan
  backlog series when its second pass ends, after which delta sync's tick emits it
  ([ADR-0104](../redaction/0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)),
  and its backlog and unclassified series when its job is dropped. Delta sync removes its tick
  series when its job is dropped. Between the second pass's end and delta sync's next tick, the
  account has no backlog series, for at most one sync interval.

**The line between reading recorded state and messaging through PostgreSQL.** The test is to remove
the reader and ask whether the row still means something.

- A recorded decision passes it, whether an APPROVED status, a connected account, a policy rule, a
  rollback request or a pause. Each is a decision a person or component made, meaningful whoever
  reads it, and reading recorded decisions on a cadence is allowed.
- A row whose only purpose is to tell a component to act fails it, whether a `NOTIFY`, an outbox
  row, a signal row or a queued job. That is messaging, and it is refused.

**Replicas and other build shapes.** The worker runs as one copy. If it ever runs as several
replicas, only the claim step changes. Its candidates are a lease row in PostgreSQL, with an
expiry renewed while the job runs, or an expiring key in a platform-supplied Redis or Dragonfly. A
session advisory lock is not one, since it fails behind a transaction-mode pooler, and a
transaction-scoped lock lasts one transaction. A single binary that also holds the mediator and
the UI composes the same scheduler, from the worker's entry package, and needs no external store.
It lacks the platform's guarantee of a
single copy, so a guard against two copies is that build shape's choice.

**No framework.** The scheduler is hand-written on Go's standard library, behind four calls,
`Ensure`, `Remove`, `Wake` and `Stop`, and coalescing and one run per job are pinned by mutation
patches. Its fallback, taken only if production shows missed or doubled runs, is
`k8s.io/client-go/util/workqueue`. Scheduler timing tests use `testing/synctest`, kept free of
network calls, and a test against real PostgreSQL keeps its own real-time bound. Due decisions take
the time as a value. A job is a plain, typed Go function or value built in the hand-written
composition root, so the compiler checks its signature. Nothing is resolved by name, registry,
reflection or annotation ([ADR-0040](../engineering/0040-pure-core-decisions-as-values.md)).

## Alternatives considered

- **A signal sent between deployables when an account, a credential or the policy changes.** The
  case for it: a change arrives at once, and backfill, then a process that ran and exited, needed
  something to start it for a new account. Not chosen. Every need it guarded came from backfill not
  running when the change happened, which the worker removes, and the reloads already read each
  change within a minute. A signal cannot replace the policy load before each body request either,
  because a request must see every edit committed before it arrived.
- **Interrupting a running second pass on a policy change and running it again.** The case for it:
  it reuses the stop every run already supports, and the comparisons run at the new run's start.
  Not chosen. A stopped run is recorded failed, so every policy edit during a second pass would show
  a failed run, and each would pay the run-start step again. Making the comparisons inside the
  running run reuses the restart inside a run the delisting comparison already has, records one
  run, and costs only the comparisons.
- **A store for job state outside the process**, a platform-supplied Redis or Dragonfly. The case
  for it: claims and schedules that survive a restart and cross replicas. Not chosen. Under one copy
  it has nothing to hold, since the schedule is right to lose and the platform's single copy is the
  claim, and it would be another service for a homelab deployer to supply, secure and back up. It
  remains a candidate for the claim step if replicas are built.
- **PostgreSQL as the job channel**, `LISTEN` and `NOTIFY`, a signals or outbox table, or a job
  queue table such as River's. No case was tabled for it. Refused by the line above. `LISTEN` is
  also dropped by a transaction-mode pooler, which
  [ADR-0066](../data/0066-data-access-generated-from-sql.md) allows, and River's migrations create a
  database function, which [ADR-0060](../engineering/0060-no-code-in-the-database.md) refuses.
- **The platform's own triggers**, a Kubernetes Job per run or a CronJob. The case for it: the
  platform does the scheduling. Not chosen for a long-running worker, since the application would
  have to create Jobs, which [ADR-0051](../engineering/0051-environment-contract.md) forbids.
- **`k8s.io/client-go/util/workqueue`.** The case for it: it carries coalescing per key and one run
  per key natively, has been run by every Kubernetes controller for years, takes an injectable
  clock and runs under `synctest`. Not chosen first, because it adds fifteen packages, two
  package-level globals, klog as the default logger and four modules on Kubernetes' release cadence
  to the trust anchor, while per-key cancellation, periodic re-adds, panic recovery and a backoff
  every wake respects stay glue the project writes anyway. It is the fallback.
- **gocron.** The case for it: a mature library with a singleton mode, panic recovery, monitors, and
  a locker ready for replicas. Not chosen, because its singleton mode drops an on-demand run
  silently or queues one per wake, and it calls jobs through reflection.
- **asynq.** The case for it: retries with backoff, uniqueness, per-queue concurrency, and work that
  survives a restart and is shared across replicas. Not chosen, because it dispatches by a string
  type name with a byte payload, it would store a second account of whether a job is due beside the
  recorded state, every scheduler test would need a Redis, and it imports `unsafe`.
- **Temporal.** The case for it: durable timers, signals and replay at scale. Not chosen, because its
  server is a new pod and its engine owns control flow.
- **A clock passed in, the project's existing convention**, in place of `testing/synctest`. The case
  for it: the convention the project already uses elsewhere. Not chosen for the scheduler, because
  `synctest` tests its timing with no clock parameter in the package. Due decisions still take the
  time as a value.
- **A job's success is the end of a run that succeeded, and nothing during a run.** The case for
  it: one meaning of success for every job kind, read where the scheduler already sees each run end.
  Not chosen, because a backfill pass runs for hours, so the alert on a job's last success would
  either fire on a healthy pass or need a bound longer than any pass, which would hide a stuck run
  for as long.
- **A job decides whether it is due before it takes a slot of its kind's limit**, so only a run with
  work takes one. The case for it: an idle job keeps succeeding while other jobs of its kind hold
  every slot, so it never ages while it waits. Not chosen by the operator, who kept the slot taken
  after a job's wake and given back when its run ends, and took the time spent waiting out of the
  job's age instead.
- **An alert on the worker's restarts, from the process series client_golang's process collector
  emits.** The case for it: it is the smallest change that makes a worker restarting again and
  again loud. Not chosen by the operator, who chose to start each job's latest success from what
  its job kind's runs record, so the rule on a job's last success catches a crash loop itself.

## Consequences

- **Correctness never depends on the scheduler.** A scheduler bug costs liveness, which the per-job
  alerts make loud, so reviewing the scheduler is about liveness and resource bounds, and the
  comparisons by effect stay the controls they are.
- **A new account is backfilled with no manual step**, within one reload of its appearing, and
  backfill runs continuously as a reconciler per account.
- **Approval latency becomes the apply check's interval**, a seconds-scale value this record does
  not fix, at the cost of one indexed read per account per check.
- **One process restarts every job together** on a configuration change or a rollout, and each job
  resumes from its checkpoint.
- **The hand-written scheduler is concurrency code the project owns**, the class of code where timer
  resets, a wake racing a drain and goroutine leaks on removal live. Its mutation patches and a race
  test of adding and removing one job pin what can be named. A feature such as priorities or jitter
  arrives only with its consumer, so the package does not grow into a framework nobody chose.
- **Run history grows** by about 105,000 delta sync runs a year per account at the default interval,
  and how long it is kept is not decided here and is tracked in
  [ROADMAP.md's open decisions](../../../ROADMAP.md#open-decisions).
- Assumptions about other components. Every run reads recorded state and is idempotent or
  conditional, so the in-process exclusion is one layer of several. The day a run's correctness
  depends on the scheduler never dropping or doubling it, this choice is re-read. The platform runs
  one copy of the worker at a time, a rollout included
  ([ADR-0103](./0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)). A
  configuration change restarts the worker
  ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)), and a change of the scan
  gate's thresholds ships in a release, so the worker's start is when the run-start comparisons are
  due.
- The rules above that are enforced or tested are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
