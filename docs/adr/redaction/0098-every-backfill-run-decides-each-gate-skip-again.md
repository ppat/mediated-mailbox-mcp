# 0098. Every backfill run decides each stored gate skip again, so a change of the scan gate's thresholds reaches the skips made under the earlier ones

**Status:** Superseded — **Superseded by:**
[ADR-0121](./0121-the-run-start-step-decides-each-gate-skip-again.md) ·
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

The thresholds have no identity a stored decision could record. Backfill's composition root passes
ADR-0093's defaults, no configuration section carries them, and so a change of thresholds is a
release. The delisting transition and a change of scanner met the same shape of problem, stored
state that a change elsewhere made wrong, by comparing what the index stores with what the workload
runs with at the start of its runs ([ADR-0037](./0037-delisting-transition.md),
[ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)).

## Decision

**Every backfill run decides each stored gate skip again, at its start and before the first pass.**
In the transaction of the run-start step that returns stale verdicts to pending
([ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)), and after it,
the run reads every message of the account stored as `SKIPPED_GATE` with the gate's inputs, the
sender's volume and prior hits joined in that same read, so the read does not grow with the number
of messages per sender ([ADR-0094](./0094-scan-gate-decisions-are-not-memoized.md)). It decides
each again with the gate, under the thresholds the run holds, with the sender's class under the
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
and a change of scanner already use. A change made in a release or while nothing ran is seen by the
next run.

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

## Consequences

- Every backfill run reads every gate skip of each account once, bounded by the corpus. A run with
  nothing changed returns nothing to pending and reopens nothing.
- The same comparison also returns to pending a skip whose sender has since gained a prior hit, or
  whose subject is now masked, since the gate decides those as scans.
- Nothing flaps. The comparison reads each skip with the columns and the sender join the second
  pass's page read uses, after the run-start step has counted the senders' prior hits again, and
  decides it with the gate the second pass uses, under the same policy and thresholds. A skip the
  second pass records is therefore one the next run's comparison decides as the same skip, unless
  the thresholds, the policy or the stored inputs changed in between, and those are the changes it
  exists to catch. The message's age cannot overturn a skip, since the rules that read it apply only
  to a message without a `List-Id`, which the skip requires.
- From the run that returns a skip to pending until the second pass decides it, its body is denied
  as pending its content scan, which is over-redaction for that time.
- Assumptions about other components. Every workload that runs the gate runs it under the same
  thresholds as backfill, or each returns to pending the skips the other made. Something runs
  backfill after each change of thresholds, which is a release while no configuration section
  carries them ([ADR-0051](../engineering/0051-environment-contract.md)). If the thresholds come
  from configuration, a change to that configuration needs a backfill run after it the same way.
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
