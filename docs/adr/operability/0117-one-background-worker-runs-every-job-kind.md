# 0117. One background worker runs every job kind, in one process, with each job kind's code and observability kept apart inside it

**Status:** Accepted (supersedes [ADR-0022](./0022-four-workloads.md)) ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox)

## Context

The background work is several kinds of job, backfill, delta sync, reorg apply and rollback, and
the heuristics run, with many more to come, among them Google Drive's jobs and possibly the send of
an approved email. They differ on runtime, trigger, reversibility and whether they write to the
provider. Reorg apply is the only path that changes the mailbox at the provider, and its approval
and rollback machinery must not burden the read-only paths. The case for a process per job kind
is [ADR-0022](./0022-four-workloads.md)'s, which this record supersedes. It rests on those
differences, with reorg apply as the settling row, and refuses one process carrying them all.

A process per job kind would guard several things. Read guard by guard, most of them do not depend
on the process boundary, as the table shows.

| What a process per job kind guards | What carries it inside one process |
| --- | --- |
| Each job starts on its own condition | Conditions are independent of where the code runs |
| Bulk mutation is reversible and checkpointed | The op log and the checkpoint in the database ([ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)) |
| Approval is in no client's vocabulary | The UI's direct database write and the client surface's registry ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md), [ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)) |
| Read-only paths carry no approval machinery | Apply's code is its own package that the read jobs never call. The boundary would add only that no other binary links it and no other process holds it |
| Every job is observable from recorded state | The run tables, which do not care which process writes them |
| A process that calls no provider holds no credential | Only the process boundary, for the heuristics run. Inside one process it becomes code isolation per job kind, below |
| One job's crash, leak or memory spike leaves the others running | Only the process boundary |

Every process that calls a provider holds the same `gmail.modify` grant and links the Gmail adapter,
whose label and mutation calls are implemented, so a process boundary separates the code path that
calls them, not the capability.

A process per job kind also costs.

- Backfill and delta sync share most of their code and their account handling, so each process
  would repeat the other's wiring, and each copy of the account-handling rules is a place one can
  drift. While backfill and delta sync were separate processes, three programs carried identical
  copies of those rules, and one copy drifted.
- The scanner's section and the scan gate's thresholds must agree between the processes that scan,
  or each reopens the other's work
  ([ADR-0120](../redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md),
  [ADR-0121](../redaction/0121-the-run-start-step-decides-each-gate-skip-again.md)). Across
  processes the scanner's section is held equal only by the deployment giving each the same value,
  and the thresholds only by each process's code passing
  [ADR-0093](../redaction/0093-composite-scan-gate.md)'s defaults.
- Backfill as a process that runs to its end starts only when something starts it, so a newly
  connected account waits for a backfill started by hand.
- Each job kind added would add a deployable, an image and chart objects.

## Decision

- **One background worker runs every kind of background job.** One deployable, the worker, runs
  backfill, delta sync, reorg apply and rollback, the heuristics run, and the background jobs
  growth adds later, scheduled jobs and triggered ones side by side. A job kind is one kind of
  background work, the unit `job_runs.workload` records ([ADR-0016](../data/0016-schema.md)). How
  the worker starts, schedules and stops its jobs is
  [ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)'s.
- **The job kinds keep their differences as facts of the job kind**, not of a process, as the
  table shows.

  | | Backfill | Delta sync | Reorg apply and rollback | Heuristics |
  | --- | --- | --- | --- | --- |
  | Runtime | minutes–hours | seconds | minutes | seconds |
  | Trigger | an account with a pass not ended, the worker's start after a change of scanner or of the scan gate's thresholds, and an account newly connected ([ADR-0120](../redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md), [ADR-0121](../redaction/0121-the-run-start-step-decides-each-gate-skip-again.md), [ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) | a tick every sync interval, five minutes by default ([ADR-0103](./0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)) | a plan the operator approved, or a rollback the operator requested | daily, or as its unit decides |
  | Reversible | n/a (read-only) | n/a | **must be** | n/a |
  | Writes provider | no | no | **yes, bulk** | no |

- **One process, with a connection pool per job kind.** Each job kind's code runs through its own
  pool, so one job kind cannot starve another of connections
  ([ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)). Every pool connects
  as the one application user the API server, the UI and the worker share
  ([ADR-0143](../data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)),
  so what a buggy job can do in the database is bounded by its code, below.
- **The heuristics run is a job kind inside the worker.** The guard that a process which calls no
  provider holds no credential, in the table above, is carried by code isolation per job kind, not
  by a process. This is a trade, stated as one. The heuristics run reads text any sender chooses,
  display names, domains and subjects, and inside the worker that text reaches code running in a
  process that holds the private key and every opened credential. A parsing or inference bug
  reachable from that text then pays off in credentials, where in a process of its own it would
  pay off in nothing. The anchor already parses attacker-chosen input on a larger scale,
  converting and scanning hostile bodies, so the added surface is the heuristics run's own
  tokenizer and any model runtime it brings.
- **Code isolation per job kind is checkable, not conventional.** Each job kind has these four.
  - Its own import list, so a job kind that needs no credential, no provider and no connection to
    a provider imports none of them. A shared thing reaches its code only through an import of its
    owner's package, which the list must admit
    ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)). The Session, which every job
    kind may hold, imports no credential and no provider code and reaches the connections only
    through an interface that loads them and says whether an account has one, so no credential
    crosses it ([ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).
  - The raw SQL analyser over the worker's code, so a job kind's own code runs only the statements
    its admitted data-access subsections hold. The process family's code and the rate limiter's,
    which a job kind calls and the analyser does not read, run no statement written by hand, which
    review holds ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md),
    [ADR-0143](../data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)).
  - A `Build` whose parameters the spec's type fixes, which are the build's context, the process's
    capabilities and the worker's configuration. The compiler holds them, so no job kind can be
    handed anything else. A job kind returns its plan rather than being handed the scheduler
    ([ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). The
    configuration it receives is the worker's whole tree, the key files' paths among its settings,
    and a setting confers nothing without the code that acts on it. The code that opens a key is
    admitted in the worker only to the connections' package, which the list of a job kind that
    calls no provider refuses. Code that reads the key file directly needs neither an import nor a
    parameter, so it is left to review.
  - No cgo in the worker's build, and no `unsafe` in this project's own code or in any dependency
    added for a job. The standard library and the dependencies the module already holds import
    `unsafe`, so the rule applies to what gets added.

  Memory safety is what makes code isolation hold, which is why the last line is part of it. The
  unsafe analyser also refuses this project's code any use of `reflect.NewAt`, `reflect.SliceAt`,
  `(reflect.Value).UnsafePointer` and `(reflect.Value).SetPointer`, which hand out or take an unsafe
  pointer with no import of `unsafe`. It leaves to review a use of those through an interface value
  or reflect's own method lookup, and writing the process's own memory through the operating system,
  such as through `/proc/self/mem` at an address `(reflect.Value).Pointer` gives, which needs
  neither. The worker's test counts a dependency holding assembly or a `.syso` object with those
  that import `unsafe`. A model runtime that needs cgo runs outside the worker, as
  [ADR-0042](../engineering/0042-implementation-stack.md) already allows for an embedding service.
- **Observability per job kind inside the worker.** Every shared metric series a job kind produces
  stays attributable to it, the library series for request cost, leases and throttles included, so
  each job kind calls its provider through a caller of its own, its own adapter clients, limiter,
  provider metrics, priority class and timeout. Each job kind builds its caller in its own `Build`,
  on the registerer the process hands it, which carries the job kind's label, and calls through it
  over the connections the worker's one Session holds. The reload of the accounts and their
  policy is the Session's, one per process, and its series is the worker's
  ([ADR-0090](./0090-accounts-reach-deployables-as-reloaded-snapshots.md)). Logs carry the job kind
  and the account. Liveness is reported per job. The worker reports ready once each job kind's pool
  answers, the Session has loaded the accounts, their connections and their policy once, and each
  job kind's first listing of what it runs for has succeeded
  ([ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). Something
  alerts when the worker stops
  ([ADR-0077](./0077-conditions-raised-as-alerting-rules.md)). A panic in one job does not take down
  the others.
- **Code and configuration.** The worker holds shared code and code isolated per job kind. It reads
  one configuration tree with shared sections, such as the scanner and the credential keys, and a
  section per job kind, and one `database` section names the one user every job kind's pool
  connects as, so no value is spelled twice
  ([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)).
- **Every job records its runs.** Each run writes a `job_runs` row with its state, checkpoint,
  counters, and last error, timeline events (start, progress at every checkpoint, backoff, retry,
  failure, resume, finish), and one row per item that failed with its error class, attempts, and
  disposition, in the tables [ADR-0016](../data/0016-schema.md) holds. A delta-sync tick and a
  cursor-gap recovery are runs too. None of these rows ever carries message content. This is what
  lets the operator watch a run and inspect a failed one from recorded state alone, the bound
  [ADR-0034](./0034-system-status-operation.md) sets.
- **Each job kind stays individually killable, resumable and observable.** A job is resumed from
  its checkpoint whatever stops it, its runs are recorded per job kind, and its series and logs are
  its own. A kill of the process stops every job at once, and each resumes as it would alone.

## Alternatives considered

- **A process per job kind**, as [ADR-0022](./0022-four-workloads.md) decided for four workloads.
  The case for it is that each job fails alone, the heuristics run sits outside the
  credential-holding processes, and each image holds only its own job's code. Not chosen, because
  most of what it guards is carried by the database and the code rather than the process, the
  guards that are the process's are carried inside the worker by the isolation above,
  and a process per job kind multiplies deployables, images and chart objects with every job kind
  growth brings. It would not bound a compromised job's reach in the database either, since every
  process would connect as the same application user
  ([ADR-0143](../data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)).
- **One container whose supervisor runs each job kind as a child process.** The case for it: a
  panic or a leak stays in its child, and each child keeps its own memory short of the container's
  limit. Not chosen. A supervisor and its children are new machinery to build, restart and observe,
  a child shares the container's filesystem and can read the key file, and an out-of-memory kill
  still depends on which process the kernel picks.
- **The database users the job kinds connect as.** One user for the API server, the UI and the
  worker, and the alternatives to it, a user per job kind, per credential reach or per group of job
  kinds, are weighed in
  [ADR-0143](../data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md).
- **Several worker replicas.** The case for it: throughput across accounts and a shorter outage on
  a node loss. Not chosen. Each account has one rate budget
  ([ADR-0025](./0025-priority-classes-and-leases.md)), so replicas add throughput only across
  accounts, and they would need a claim per job that a killed replica releases. What they would
  change is [ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)'s.

## Consequences

- **What the worker gives up, and what replaces it.** One job's out-of-memory kill or a deadlock
  that fails the liveness probe stops every job, so the failure of an unrelated job can make
  [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox)'s staleness bound reachable. A
  panic is recovered in the run that raised it, each job kind has its own pool, and the alert on a
  stopped worker and the alert on a job whose last success has aged make it loud, a kill repeated
  at every restart included, since a job's last success starts from what its runs record
  ([ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md),
  [ADR-0077](./0077-conditions-raised-as-alerting-rules.md)).
- **A configuration change restarts every job.** The chart restarts a deployable's pods when its
  configuration changes ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)), so a
  change to any job kind's setting restarts them all, and each resumes from its checkpoint.
- **One memory limit** is sized for the jobs that run at once, against the limit each process would
  have with a process per job kind, and a concurrency limit per job kind bounds the bodies held in
  memory ([ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
- **The worker's image holds the code of every job kind**, so exactness of image contents holds for
  the worker as a whole ([ADR-0049](../engineering/0049-image-per-component-lockstep.md)), and which
  process can open a stored credential stays a property of which processes receive the private key's
  file, and within the worker of each job kind's code
  ([ADR-0079](./0079-secrets-arrive-as-mounted-files.md),
  [ADR-0081](./0081-credentials-sealed-to-a-public-key.md)).
- **The trust anchor gains the heuristics run's code and loses processes.** The processes holding
  credentials are the API server, the worker and the UI, against five with a process per job kind.
  Each still yields the full mailbox, so fewer of them lowers the places an attacker can enter
  without raising what any one yields
  ([ADR-0028](./0028-trust-anchor-hardening.md)).
- **What the scanner and the thresholds must agree on is read once.** Backfill's and delta sync's
  scanner section comes from one configuration load, both scan with the process's one scanner
  built from it ([ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)),
  and their gate thresholds are one value in one process, so both agree by construction rather
  than by two processes being given the same.
- **Cross-job coordination of the rate budget stays.** The API server still spends from each
  account's budget, so leases stay in the database
  ([ADR-0025](./0025-priority-classes-and-leases.md)).
- **Undoing it is a packaging change.** Each job kind keeps its own entry in the worker's spec and
  its own `Build`, so moving one out of the worker later moves its entry into a spec of its own.
  What cannot be undone is what a window records, which is series written without a job label, and
  a credential exposed while the heuristics run sat inside the anchor, whose remedy is revoke and
  reissue.
- Assumptions about other components. The platform runs one copy of the worker at a time, a rollout
  included, keeps it running and restarts it when it stops
  ([ADR-0103](./0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)). The chart
  runs it as a Deployment of exactly one replica whose rollout stops the old pod before it starts
  the new one, which holds for a rollout. Outside a rollout Kubernetes does not hold it. Any deletion
  of the pod, an eviction for a node drain, a scale-down by a cluster autoscaler, a preemption or a
  deletion by hand, has the replacement created at once, so two workers can run together for up to
  the pod's termination grace period, and a lost node's pod can keep running until its node stops
  it. The harm that overlap can do is bounded as
  [ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md) states, since every run
  reads recorded state and is idempotent or conditional, so the in-process exclusion is one layer of
  several. It delivers the application user's credential and the private key's file to the worker
  ([ADR-0143](../data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)).
  A job kind's `Build` is handed the build's context, the process's capabilities and the worker's
  configuration, and nothing else, since the spec's type fixes its parameters
  ([ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). Through those
  it may ask the owners for its own job kind's pool, which `process/database` keeps, the worker's
  one Session, the connections and the scanner, each through an import of its owner's package that
  its import list admits. What it can ask for is bounded by that list, so a job kind that calls no
  provider cannot reach the connections. A test holds that each job kind's code runs on a pool of
  its own.
- The rules above that are enforced or checked are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
