# 0117. One background worker runs every job kind, in one process, with each job kind's code, role and observability kept apart inside it

**Status:** Proposed (supersedes [ADR-0022](./0022-four-workloads.md)) ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox)

## Context

[ADR-0022](./0022-four-workloads.md) split the background work into four workloads, backfill,
delta sync, reorg apply and the heuristics run, because they differ on runtime, trigger,
reversibility and whether they write to the provider, and it named the settling row: reorg apply
is the only provider-mutating path, and its approval and rollback machinery would burden the
read-only paths. It rejected one process carrying all four.

Read guard by guard, the process boundary carried less of that reasoning than the record says.

| What the four-way split guards | What carries it |
| --- | --- |
| Each job starts on its own condition | Conditions are independent of where the code runs |
| Bulk mutation is reversible and checkpointed | The op log and the checkpoint in the database ([ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)) |
| Approval is in no client's vocabulary | The UI's direct database write and the client surface's registry ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md), [ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)) |
| Read-only paths carry no approval machinery | Apply's code is its own package that the read jobs never call. What the boundary adds is the binary linking it and the process holding it |
| Every job is observable from recorded state | The run tables, which do not care which process writes them |
| A process that calls no provider holds no credential | The process boundary, for the heuristics run |
| One job's crash, leak or memory spike leaves the others running | The process boundary |
| A compromised job's database reach is its own role's | One role's credential per process |

Every process that calls a provider already holds the same `gmail.modify` grant and links the Gmail
adapter, whose label and mutation calls are implemented, so what the boundary separates is the code
path that calls them, not the capability.

The split also costs. Every change to delta sync touched backfill. Their composition roots repeat
each other, and three roots carry identical copies of the account-handling rules, one of which
drifted. The scanner's section and the scan gate's thresholds must agree between two processes or
each reopens the other's work ([ADR-0096](../redaction/0096-a-scanner-change-reopens-backfill.md),
[ADR-0098](../redaction/0098-every-backfill-run-decides-each-gate-skip-again.md)). The scanner's
section is held equal only by the deployment giving both processes the same value, and the
thresholds only by both composition roots passing
[ADR-0093](../redaction/0093-composite-scan-gate.md)'s defaults. A newly connected account waits
for a backfill started by hand.

And growth is coming. Many background jobs will be added, among them Google Drive's jobs and
possibly the send of an approved email, and a process per job kind would multiply deployables,
images, chart objects and roles with each one. The operator ruled the topology on 2026-10-07.

## Decision

- **One background worker runs every kind of background job.** One deployable, the worker, runs
  backfill, delta sync, reorg apply and rollback, the heuristics run, and the background jobs
  growth adds later, scheduled jobs and triggered ones side by side. A job kind is one kind of
  background work, the unit `job_runs.workload` records ([ADR-0016](../data/0016-schema.md)). How
  the worker starts, schedules and stops its jobs is
  [ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)'s.
- **The job kinds keep their differences as facts of the job kind**, not of a process:

  | | Backfill | Delta sync | Reorg apply and rollback | Heuristics |
  | --- | --- | --- | --- | --- |
  | Runtime | minutes–hours | seconds | minutes | seconds |
  | Trigger | an account with a pass not ended, the worker's start after a change of scanner or of the scan gate's thresholds, and an account newly connected ([ADR-0096](../redaction/0096-a-scanner-change-reopens-backfill.md), [ADR-0098](../redaction/0098-every-backfill-run-decides-each-gate-skip-again.md), [ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) | a tick every sync interval, five minutes by default ([ADR-0103](./0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)) | a plan the operator approved, or a rollback the operator requested | daily, or as its unit decides |
  | Reversible | n/a (read-only) | n/a | **must be** | n/a |
  | Writes provider | no | no | **yes, bulk** | no |

- **One process, with one database role per job kind.** Each job kind's code runs under its own
  role, through its own connection pool
  ([ADR-0118](../data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)). The roles
  bound what a buggy job can do in the database. They do not bound a compromised process, which
  holds every role's credential.
- **The heuristics run is a job kind inside the worker.** The rule that a process which calls no
  provider holds no credential is reinterpreted as code isolation per job kind, not process
  isolation. This is a trade, stated as one. The heuristics run reads text any sender chooses,
  display names, domains and subjects, and inside the worker that text reaches code running in a
  process that holds the private key and every opened credential. A parsing or inference bug
  reachable from that text then pays off in credentials, where outside the worker it paid off in
  nothing. The anchor already parses attacker-chosen input on a larger scale, converting and
  scanning hostile bodies, so the added surface is the heuristics run's own tokenizer and any model
  runtime it brings.
- **Code isolation per job kind is checkable, not conventional.** For each job kind:
  - its own import list, so a job kind that needs no credential, no provider and no account session
    imports none of them ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md));
  - its own role's grants
    ([ADR-0118](../data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md));
  - an entry constructor that takes only what the job needs, pinned by the parameter checks the
    project already runs;
  - no cgo in the worker's build, and no `unsafe` in this project's own code or in any dependency
    added for a job. The standard library and today's dependencies already import `unsafe`, so the
    rule applies to what gets added.

  Memory safety is what makes code isolation hold, which is why the last line is part of it. A
  model runtime that needs cgo runs outside the worker, as
  [ADR-0042](../engineering/0042-implementation-stack.md) already allows for an embedding service.
- **Observability per job kind inside the worker.** Every shared metric series stays attributable
  to the job kind that produced it, the library series for request cost, leases, throttles and the
  policy reload included. Logs carry the job kind and the account. Liveness is reported per job.
  Something alerts when the worker stops
  ([ADR-0077](./0077-conditions-raised-as-alerting-rules.md)). A panic in one job does not take down
  the others.
- **Code and configuration.** The worker holds shared code and code isolated per job kind. It reads
  one configuration tree with shared sections, such as the scanner and the credential keys, and a
  section per job kind, and the per-job database users sit inside one `database` section, so no
  value is spelled twice
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

- **Four separate workloads**, as [ADR-0022](./0022-four-workloads.md) decided. The case for it:
  each job fails alone, the heuristics run sits outside the credential-holding processes, each
  image holds only its own job's code, and a compromised job reaches only its own role. Not chosen,
  because most of what it guards is carried by the database and the code rather than the process,
  the guards that are the process's are replaced inside the worker by the isolation above, and a
  process per job kind multiplies deployables, images, chart objects and roles with every job kind
  growth brings.
- **One process with one role holding every job kind's grants.** The case for it: one credential to
  provision and one pool. Not chosen, because a bug in any job could then write what only another
  job writes, a heuristics bug updating a stored credential and a backfill bug deleting messages,
  and bugs are likelier than compromises.
- **One container whose supervisor runs each job kind as a child process.** The case for it: a
  panic or a leak stays in its child, and each child keeps its own memory short of the container's
  limit. Not chosen. A supervisor and its children are new machinery to build, restart and observe,
  a child shares the container's filesystem and can read the key file, and an out-of-memory kill
  still depends on which process the kernel picks.
- **A role per credential reach**, one for the job kinds that call a provider and one for the
  heuristics run. The case for it: two credentials, with the heuristics run still held away from
  stored credentials. Not chosen, because backfill, delta sync and apply would then share reach,
  which the role per job kind keeps apart at no more than two extra credentials.
- **Several worker replicas.** The case for it: throughput across accounts and a shorter outage on
  a node loss. Not built. Each account has one rate budget
  ([ADR-0025](./0025-priority-classes-and-leases.md)), so replicas add throughput only across
  accounts, and they would need a claim per job that a killed replica releases. What they would
  change is [ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)'s.

## Consequences

- **What the worker gives up, and what replaces it.** One job's out-of-memory kill or a deadlock
  that fails the liveness probe stops every job, so the failure of an unrelated job can now make
  [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox)'s staleness bound reachable. A
  panic is recovered in the run that raised it, each job kind has its own pool, and the alert on a
  stopped worker and the alert on a job whose last success has aged make it loud
  ([ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md),
  [ADR-0077](./0077-conditions-raised-as-alerting-rules.md)).
- **A configuration change restarts every job.** The chart restarts a deployable's pods when its
  configuration changes ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)), so a
  change to any job kind's setting restarts them all, and each resumes from its checkpoint.
- **One memory limit** is sized for the jobs that run at once, against today's limit per process,
  and a concurrency limit per job kind bounds the bodies held in memory
  ([ADR-0119](./0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
- **The worker's image holds the code of every job kind**, so exactness of image contents holds for
  the worker as a whole ([ADR-0049](../engineering/0049-image-per-component-lockstep.md)), and which
  process can open a stored credential stays a property of which processes receive the private key's
  file, and within the worker of each job kind's code
  ([ADR-0079](./0079-secrets-arrive-as-mounted-files.md),
  [ADR-0081](./0081-credentials-sealed-to-a-public-key.md)).
- **The trust anchor gains the heuristics run's code and loses processes.** Once reorganization is
  built, the processes holding credentials are the mediator, the worker and the UI, against five
  with a process per job kind. Each still yields the full mailbox, so fewer of them lowers the
  places an attacker can enter without raising what any one yields
  ([ADR-0028](./0028-trust-anchor-hardening.md)).
- **What the scanner and the thresholds must agree on is read once.** Backfill's and delta sync's
  scanner section comes from one configuration load, and their gate thresholds are one value in
  one process, so both agree by construction rather than by two processes being given the same.
- **Cross-job coordination of the rate budget stays.** The mediator still spends from each account's
  budget, so leases stay in the database ([ADR-0025](./0025-priority-classes-and-leases.md)).
- **Undoing it is a packaging change.** Each job kind keeps its own entry, so moving one out of the
  worker later moves its wiring into a root of its own, and the role per job kind already exists.
  What cannot be undone is what a window records: series written without a job label, and a
  credential exposed while the heuristics run sat inside the anchor, whose remedy is revoke and
  reissue.
- Assumptions about other components. The platform runs one copy of the worker at a time, a rollout
  included, keeps it running and restarts it when it stops
  ([ADR-0103](./0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)). It delivers
  the credentials of every role the worker's job kinds run as, and the private key's file to the
  worker. A job kind's code receives only its own pool, which the composition root wires and review
  holds, since the grant check sees which statements a list admits and not which pool reaches the
  code ([ADR-0066](../data/0066-data-access-generated-from-sql.md)).
- The rules above that are enforced or checked are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
