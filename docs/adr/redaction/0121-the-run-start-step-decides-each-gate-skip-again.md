# 0121. Backfill's run-start step, which the worker makes once per process and account, decides each stored gate skip again, so a change of the scan gate's thresholds reaches the skips made under the earlier ones

**Status:** Accepted (supersedes [ADR-0098](./0098-every-backfill-run-decides-each-gate-skip-again.md)) ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

The scan gate's thresholds are values the gate is given, tuned from the recorded skip rates, and
widening or narrowing the gate is a policy change with observable effect
([ADR-0093](./0093-composite-scan-gate.md)). A skip is the one state that releases a body no
scanner read, and the gate has one skip of its own, `high_volume_no_hits`, stored as
`SKIPPED_GATE`. A skip as restricted follows the sender's class, and the delisting transition
already decides it again when the class changes
([ADR-0037](./0037-delisting-transition.md)).

Backfill's second pass decides a message once, when it reads it waiting for a scan
([ADR-0017](../data/0017-two-pass-backfill.md)). A stored skip is decided again in one case only,
at the second pass's start for a subject now masked, after a change of scanner
([ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)). So a change of
thresholds that scans more re-decides no stored skip, and every message skipped under the earlier
thresholds stays released unscanned, in the residual the change meant to narrow.

[ADR-0098](./0098-every-backfill-run-decides-each-gate-skip-again.md) has every backfill run decide
each stored skip again at its start, which fits a backfill that runs and exits, one run per
process, where every run is a process start. As a job kind of the worker
([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)), backfill is run by a
scheduler that makes the run-start step once per process and account, at the worker's start and
for a newly listed account, and retries a failed run after its backoff without it
([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).

The thresholds have no identity a stored decision could record. Backfill's job kind passes
ADR-0093's defaults, no configuration section carries them, and so a change of thresholds is a
release. The delisting transition and a change of scanner met the same shape of problem, stored
state that a change elsewhere made wrong, by comparing what the index stores with what the job kind
runs with at the start of its runs ([ADR-0037](./0037-delisting-transition.md),
[ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)).

## Decision

**Backfill's run-start step decides each stored gate skip again, before the first pass.** The
worker makes the run-start step once per process and account, at its start and for a newly listed
account ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)). In
the transaction of the run-start step that returns stale verdicts to pending
([ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)), and after it,
the run reads every message of the account stored as `SKIPPED_GATE` with the gate's inputs, the
sender's volume and prior hits joined in that same read, so the read does not grow with the number
of messages per sender ([ADR-0094](./0094-scan-gate-decisions-are-not-memoized.md)). It decides
each again with the gate, under the thresholds the process holds, with the sender's class under the
policy in force and the message's age measured at the run's clock.

**A skip stands only while the gate decides it again as the same skip.** A message the gate decides
again as `high_volume_no_hits` keeps its skip. Every other stored skip returns to pending, which
denies its body ([ADR-0093](./0093-composite-scan-gate.md),
[ADR-0002](./0002-fetch-time-re-evaluation.md)). That covers a message the gate would now scan, one
under thresholds that cannot decide, which ADR-0093 leaves pending, and one whose sender the policy
now restricts, which the second pass then records as skipped as restricted. The recorded gate
decision keeps its earlier decision and reason until the second pass decides the message again, as
it does for every other message returned to pending.

**A skip returned to pending reopens the second pass.** When any skip returned to pending, the same
transaction records the second pass as not ended and marks it to start over, as the run-start step
does when it returns a verdict to pending, so a skip returned before a stopped pass's checkpoint is
read ([ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)). The run
writes to no run's record.

The deciding argument. Comparing the stored skips with the gate the run holds needs no stored record
of the thresholds, works whatever later delivers them, and is the pattern the delisting transition
and a change of scanner already use. A change of thresholds ships in a release, which restarts the
worker, so it is seen by the run-start step the worker makes at its start.

## Alternatives considered

- **Each gate decision records a revision of the thresholds it was made under, and a skip under
  another revision returns to pending**, as a verdict records its scanner. Its case is that only the
  skips a change touched are read, by a stored stamp. Not chosen. The thresholds have no identity
  to stamp until a configuration section carries them, so this would braid the re-decision to how
  the thresholds are delivered. It would also catch only a change of thresholds, and not a skip
  whose sender has since gained a prior hit.
- **Nothing decides a stored skip again, so a change of thresholds applies only to mail decided
  afterwards.** Its case is no work and no window in which returned bodies are denied. Not chosen,
  because it leaves the whole historical residual released, which is what the operator widens the
  gate to narrow.
- **The thresholds join the scanner's stamp, so a change of thresholds re-opens backfill through
  [ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)'s machinery.**
  Its case is reuse of machinery already built. Not chosen, because it re-masks and re-scans the
  whole mailbox for a change that touches only the skips.
- **Deciding the skips again at the second pass's start, beside the return of the skips whose
  subject is now masked.** Its case is one comparison of skips in one place. Not chosen, because a
  change of thresholds alone never reopens the second pass, so once backfill has ended the
  comparison would never run.
- **Returning a skip to pending only when the gate would now scan it.** Its case is fewer bodies
  denied. Not chosen, because it keeps a skip releasing its body under thresholds that cannot
  decide, where ADR-0093 leaves the message pending.
- **Making the step at the start of every run, a failed run's retry inside the worker included**, as
  [ADR-0098](./0098-every-backfill-run-decides-each-gate-skip-again.md) reads for a backfill whose
  every run is a process start. Its case is that inputs which change without a restart, a sender's
  new prior hit and the policy in force, are re-decided at each retry as they are at each restart
  after a failure. Not chosen, because under the wakes of
  [ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md), the end
  of a backoff runs only the due decision, without the run-start step.

## Consequences

- Every run-start step reads every gate skip of its account once, bounded by the corpus. A step
  with nothing changed returns nothing to pending and reopens nothing.
- **The step runs once per process and account, not once per run.** Every start of the process
  that runs backfill makes it, and so does an account newly listed while it runs. A failed run's
  retry inside the worker follows the backoff without the step. Inputs that change without a
  restart, a sender's new prior hit and the policy in force, are therefore re-decided at the next
  worker start. The policy in force also reaches the second pass through the comparisons a running
  run makes again when its policy changes
  ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md),
  [ADR-0037](./0037-delisting-transition.md),
  [ADR-0113](./0113-an-added-rule-reaches-the-stored-classes-by-its-effect.md)), and the scanner
  and the thresholds change only across a restart, so their guard holds whole.
- The same comparison also returns to pending a skip whose sender has since gained a prior hit, or
  whose subject is now masked, since the gate decides those as scans.
- Nothing flaps. The comparison reads each skip with the columns and the sender join the second
  pass's page read uses, after the run-start step has counted the senders' prior hits again, and
  decides it with the gate the second pass uses, under the same policy and thresholds. A skip the
  second pass records is therefore one the next step's comparison decides as the same skip, unless
  the thresholds, the policy or the stored inputs changed in between, and those are the changes it
  exists to catch. The message's age cannot overturn a skip, since the rules that read it apply only
  to a message without a `List-Id`, which the skip requires.
- From the run that returns a skip to pending until the second pass decides it, its body is denied
  as pending its content scan, which is over-redaction for that time.
- Assumptions about other components. Every job kind that runs the gate runs it under the same
  thresholds as backfill. In the worker the thresholds are one value in one process, which gives
  that by construction
  ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)). A change of
  thresholds, which ships in a release while no configuration section carries them, restarts the
  worker, and so does a change to its configuration if the thresholds come from there
  ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)).
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
