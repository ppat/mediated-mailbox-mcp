# 0116. The chart runs backfill whenever its pod changes, under a Job named by a hash of the pod, and the operator starts it by hand from a suspended CronJob

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [O6](../../../USE_CASES.md#o6--deployable)

## Context

Backfill is a batch job: a run that exits, with an exit code and idempotent resume
([ADR-0051](../engineering/0051-environment-contract.md), [ADR-0022](./0022-four-workloads.md)).
[ADR-0022](./0022-four-workloads.md) starts it by hand once, and then again after each change of
scanner or of the scan gate's thresholds, started by the deployment with no manual step. Every
backfill run compares the index with the scanner and the thresholds it runs with, so one run after
a change does the work and a run with nothing stale does none
([ADR-0096](../redaction/0096-a-scanner-change-reopens-backfill.md),
[ADR-0098](../redaction/0098-every-backfill-run-decides-each-gate-skip-again.md)). A change of
scanner is a change of its version, which ships in a release, or of the scanner's section of the
configuration. The gate's thresholds ship in a release. So the deployment runs backfill after each
release and each change to the scanner's section, and nothing tells it which releases changed
either.

The first run that does any work is the one after the operator connects an account through the UI
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)), which no deployment
step marks. Kubernetes Jobs are immutable once created, so a Job whose pod template changes cannot
be updated in place, and an upgrade that tries fails.

## Decision

- **A Job runs backfill, named by a hash of its pod template.** The chart renders one Job whose
  name is backfill's name followed by the first ten hexadecimal digits of the SHA-256 of the
  rendered pod template. The template holds the image, which carries the release's version, and a
  checksum of backfill's own configuration file, which holds the scanner's section. A release or a change of
  the scanner's section changes the name, so Helm creates the new Job, which runs backfill, and
  deletes the old one. An install or upgrade that leaves the template as it was leaves the finished
  Job alone and runs nothing.
- **The scanner's section is one value shared by every deployable that masks or scans.** The chart
  renders it into the configuration file of the mediator, backfill and delta sync, so the three run
  the same section, as ADR-0096 assumes, and a change to it reaches backfill's pod template. A change
  that reaches only another deployable's file leaves backfill's file, and so its pod template, as it
  was.
- **A suspended CronJob, never scheduled, carries the same pod template**, so the operator starts
  backfill by hand with `kubectl create job --from=cronjob/<fullname>-backfill <name>`. That is the
  first start once an account is connected, and any later run the operator wants, such as after
  connecting another account.
- **The Job also runs at install.** A run with no account listed ends at once, successfully, as
  delta sync and the mediator serve no account until one is listed. Backfill's composition root
  logs that the run has nothing to do and exits 0 before it loads any policy, since the policy
  loader reads in an account's transaction.

## Alternatives considered

- **A Job named by the chart's version and a hash of the scanner's section alone.** For it, only the
  two changes ADR-0096 and ADR-0098 name rename the Job. Against it, any other change to the pod
  template, a database host or a resource request, would change a Job that keeps its name, and the
  upgrade would fail on the immutable template. Hashing the whole template trades that for an extra
  run after such a change, which finds nothing stale and does nothing. Not chosen.
- **A `post-install` and `post-upgrade` hook Job.** For it, the standard way to run something after
  each upgrade. Against it, it runs after every upgrade whatever changed, and Helm waits for a hook
  Job to complete, so an upgrade would wait for backfill's whole run, hours after a change of
  scanner, and fail at its timeout. Not chosen.
- **The Job rendered only on upgrade, never at install.** For it, the first run stays by hand, as
  ADR-0022 words it, with no run before an account exists. Against it, the first upgrade after
  install runs the Job whatever it changed, since the Job was not there before, and a reinstall over
  a database holding accounts runs nothing. Not chosen.
- **A scheduled CronJob.** For it, no dependence on how the deployment changes. Against it, it runs
  on its schedule rather than after a change, so a change waits for the next slot and most runs find
  nothing to do. Not chosen.
- **Every run by hand.** No case was tabled for it. Rejected, because ADR-0022 starts the runs after
  a change with no manual step.

## Consequences

- A deployment tool that waits for Jobs to complete holds an install or upgrade for backfill's whole
  run, which after a change of scanner takes hours. flux's helm-controller waits for Jobs unless its
  release's `disableWaitForJobs` is set, so a deployment through it sets that switch on install and
  on upgrade, as the chainsaw suite does.
- The first run that does any work is by hand only until an upgrade renames the Job. Once an
  account is connected, an upgrade that renames the Job, a release or a change of the scanner's
  section among them, starts backfill for that account with no manual step, and when the operator
  has not started it yet, that run is the account's first backfill. An install over a database that
  already lists accounts runs backfill for them at once the same way. ADR-0022's first start by hand
  holds for an account connected after the last such install or upgrade, and the operator may still
  start a run by hand at any time.
- A change that reaches backfill's pod template for any other reason, a database setting, a resource
  request or a key added to the keyring, runs backfill once more, which finds nothing stale and does
  nothing.
- The finished Job stays until a later upgrade renames it. A deployment that deletes it sees it
  created again, and backfill run again, by the next upgrade.
- Two backfill runs can work on one account at once. A run started by hand from the CronJob
  overlaps the Job an upgrade starts, and a renamed Job overlaps the old one while Helm deletes it in
  the background. The CronJob's `Forbid` concurrency covers only the runs it schedules itself, and it
  schedules none. A pass that starts while its latest run is still recorded as running resumes from
  that run's checkpoint and records it as failed (`Begin` in `backfill/internal/core/pass1`), though
  its process goes on working: the statement that records a run's progress is conditioned on the
  account and the run and not on the run's state, while the one that records its end leaves a run
  no longer running as it is (`db/jobruns/record/record.sql`). What keeps the index correct:
  - A first-pass page taken twice counts nothing twice, and a page's rows and its checkpoint are one
    transaction ([ADR-0017](../data/0017-two-pass-backfill.md)), which the
    [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) row for taking pass 1's pages again proves for a
    page taken again by one run. Two runs taking one page at the same moment rest on the same insert,
    which adds a message only when the account holds none under its identifier
    (`InsertMessage` in `db/messages/ingest`), and no verification row injects that.
  - A second-pass verdict or skip is written only to a message still pending its scan
    (`RecordVerdict` and `RecordSkip` in `db/messages/scan/scan.sql`), so a message both runs decide
    keeps the first decision. No verification row injects two runs scanning one message.
  - Both runs spend one account's budget through its leases, under its one hard cap
    ([ADR-0025](./0025-priority-classes-and-leases.md)), the verification rows for the lease issuer
    and the rate-state row's lock.

  The residual: the earlier run's record reads failed while its process still writes progress to it,
  so the UI's jobs screen shows a failed run that is still working, and the two runs fetch and scan
  the same messages, which doubles the provider calls and the time the budget spends on them.
- Assumptions about other components. The deployment tool applies the rendered objects as Helm
  does, creating an object whose name is new and deleting one no longer rendered. Backfill exits 0
  when a run ends without error and resumes a stopped run ([ADR-0051](../engineering/0051-environment-contract.md)).
- The rules above are controls, the Job renamed by a release or a change of the scanner's section
  and by nothing that reaches only another deployable, and a run with no account ending
  successfully. Their injections are catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
