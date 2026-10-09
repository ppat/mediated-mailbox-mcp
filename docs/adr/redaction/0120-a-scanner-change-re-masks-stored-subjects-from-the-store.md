# 0120. A change of scanner re-opens backfill, which re-masks from the store every stored subject that was unmasked, fetches again only the subjects that were masked, and re-scans what an earlier scanner decided

**Status:** Accepted (supersedes [ADR-0096](./0096-a-scanner-change-reopens-backfill.md)) ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [C1](../../../USE_CASES.md#c1--metadata-always-visible)

## Context

Every scan verdict records the scanner version and the revision of the scanner's configuration it was
made under, so that when either changes the rows decided before are marked stale and reprocessed, a
planned operation rather than a migration
([ADR-0009](./0009-scanner-verdicts-carry-no-content.md),
[ADR-0005](../classification/0005-tiered-detection.md)). The revision is the configuration library's
revision of the scanner's section, so a change in any layer, the file, an environment variable or a
flag, changes it without any release
([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)).

The verdict is half of what the scanner decides. Subject masking runs on every message with the same
scanner, and the index stores only the subject as masked
([ADR-0003](./0003-subject-masking.md), [ADR-0016](../data/0016-schema.md)). Masking replaces only
the spans it matched, and a stored subject's `subject_masked` reads true exactly when a mask was
applied, so a stored subject that reads unmasked is the subject the provider returned, as fetched.
Such a subject can be masked again from what is stored. Only a subject that was masked lost what a
new scanner would read, and only it must be fetched from the provider again. Backfill's first pass
masks a subject as it adds the message and leaves a message it already holds as it is
([ADR-0017](../data/0017-two-pass-backfill.md)). The scan gate scans a message whose subject was
masked ([ADR-0093](./0093-composite-scan-gate.md)), so a skip decided before a better scanner masked
the subject was decided without a signal the message now carries.

[ADR-0096](./0096-a-scanner-change-reopens-backfill.md) re-masks every stale subject by enumerating
the whole mailbox again, on the premise that a subject cannot be masked again from what is stored.
That premise holds only for the masked minority. Under it each change of scanner costs about a day
of over-redaction, about twelve hours of it the re-enumeration, and the scanner is tuned against
real codes more than once, so the cost recurs. Measured for this decision on a seeded corpus where
1% of subjects are masked, re-masking the other 99% from the store took about 0.3 s of scanner
time and a 3.9 s update, and fetching the masked ones again by identifier took about 7 minutes,
and about 34 at 5%.

A new version ships between production points with no manual step, so the read path must re-scan on
a change without one ([ROADMAP.md](../../../ROADMAP.md)). The delisting transition met the same
shape of problem, stored state that a change elsewhere made wrong, by having the job kind compare
what the index stores with what it runs with at the start of its runs
([ADR-0037](./0037-delisting-transition.md)).

## Decision

**The masked subject carries the scanner it was masked under.** Every stored subject records the
scanner version and the configuration revision of the masking that produced it, beside the pair a
verdict records, and every masking event records the same pair, so a reader can tell the masks of
the subject as it now stands from those of a masking it replaced. A subject or a verdict is stale
when its pair differs from the scanner a run runs with, a missing pair included.

**In backfill's run-start step, stale verdicts return to pending and unmasked stale subjects are
masked again from the store.** The worker makes the run-start step once per process and account, at
its start and for a newly listed account
([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)). The step,
before the first pass, compares the index with the scanner the process runs with, in one
transaction. The scanner changes only across a restart, so a step per process start sees every
change of scanner.

- A scanned message whose verdict carries another pair returns to pending, its verdict cleared, the
  content flags, content rules, scan time, version and revision. The prior hits of every sender with
  a message returned to pending are counted again from the flagged verdicts the index then holds, so
  a re-scan that flags the message again counts it once.
- Every stale subject stored unmasked is masked again from the subject the store holds, under the
  scanner the run runs with, with a masking event for each mask and the pair of the scanner it ran
  under, and no other column of the row changes. The subjects are read, masked and written back a
  batch at a time, each batch in a few statements rather than one a message. This happens before
  the run decides its stored gate skips again
  ([ADR-0121](./0121-the-run-start-step-decides-each-gate-skip-again.md)), so those decisions see
  the subjects as now masked.
- When it returned any message to pending, masked any subject again, or left a stored subject stale
  so the first pass is due again, the same transaction records the second pass as not ended and marks
  it to start over, a flag on the account's state. It writes to no run's record. The next run of the
  second pass, when it resumes a stopped one with the mark set, starts over from the first message
  waiting for a scan, so a message returned to pending before the checkpoint is read, and the run
  that starts clears the mark in the transaction that records its start. The mark is set by what the
  run-start step did, never inferred from which scanner is in force, so a change reverted before any
  second pass ran under it still starts the pass over.

Pending is the existing state, which denies the body
([ADR-0093](./0093-composite-scan-gate.md), [ADR-0002](./0002-fetch-time-re-evaluation.md)), and no
state is added. The verdict is denied from that moment, whatever the first pass's state, so a first
pass that runs for long or fails on every run leaves no stale verdict releasing a body.

**Backfill observes a change by its effect, in its run-start step.** Backfill's first pass counts
as ended for an account only while every stored subject carries the pair of the scanner it runs
with, and when it is found due again the run that starts it clears its completion flag. After the
run-start step, only subjects stored masked can still be stale. The second pass is recorded as not
ended by the run-start step above. A pass recorded as not ended runs again, a fresh pass when its
latest run succeeded ([ADR-0017](../data/0017-two-pass-backfill.md)). So a reopened first pass always
leads to a run of the second, which also scans the mail the first pass added, and every skip decided
without its subject's signal follows a re-mask that reopened the second pass. No record of the change
is kept or read. A change made in any layer, by a release or while nothing ran, is seen by the next
run.

**The first pass fetches again only the subjects that were masked, by identifier.** A message the
index already holds whose subject is stored masked and stale has its metadata fetched again by its
identifier, inside the first pass, and its subject masked again from the subject the provider
returns, under the scanner the pass runs with, with a masking event for each mask, and no other
column of the row changes. No pass is added, so the run vocabulary the UI's jobs screen, its banner
and the system status read pass by pass stays as it is. The fetch comes once the pass's enumeration
has ended, a call at a time, and the pass ends only once no stored subject is stale. A call moves
the pass when it stores any subject it fetched under the pair in force, whether or not the new
scanner masks anything in it, or when it leaves fewer stored subjects stale than before it. A call
that moves neither way fails the run, since the next read would ask for the same subjects again
without end. A first pass reopened after its enumeration ended does not enumerate again. Its run
starts from the checkpoint of the enumeration that ended and only fetches. The pass's checkpoint
records the pair its enumeration runs under, and a run resuming an enumeration not yet ended that
was made under another pair starts the enumeration over from the first page. That start over drops
the estimate of the pages the enumeration takes with its token, as every start over does
([ADR-0095](../provider/0095-enumeration-total-on-every-page.md)), so the estimate always comes from
the enumeration in progress.

**A message the provider no longer has is masked whole.** When the provider reports a message it is
asked for by identifier gone, its stored subject is masked whole under the pair the pass runs with,
as a scanner that cannot decide masks it ([ADR-0003](./0003-subject-masking.md)), with the masking
event of a whole subject, and the run records the message as a failed item of class `gone`. Its
subject is then current, so it never makes the pass due again.

**A skip decided without its subject's signal returns to pending at the second pass's start.** At the
start of each of its runs, after the delisting transition, the second pass returns to pending each
message the gate skipped whose subject is now masked, since the skip was decided without that signal.
It needs no restart of its own, because a subject is masked again only by a run-start step or a first
pass the run-start step found due, which marked the second pass to start over, so the run that follows
starts from the first message waiting for a scan.

**What starts backfill after a change is the worker's start.** The run-start step makes the
comparisons above, so one step after a change of the scanner's version or section does the work,
and a step with nothing stale does none. The worker that runs backfill makes the run-start step at
its own start ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)),
and a change of the scanner's version ships in a release and a change of its section is a
configuration change, either of which restarts the worker
([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)).

## Alternatives considered

- **Re-masking every stale subject by enumerating the whole mailbox again in the first pass**, as
  [ADR-0096](./0096-a-scanner-change-reopens-backfill.md) decided. The case for it is that
  enumeration is the first pass's own primitive ([ADR-0010](../provider/0010-one-provider-port.md)),
  and that any change of scanner makes every subject stale, so the stale messages are the whole
  mailbox. Not chosen, because a subject stored unmasked can be masked again from the store, so
  enumeration fetches again the metadata of the whole mailbox to re-mask the few subjects that were
  masked, and holds every re-scan behind about twelve hours of quota-paced enumeration on each change
  of scanner.
- **Fetching the masked subjects again in a pass of its own.** The case for it is a pass that does
  one thing. Not chosen. A third pass would be a third name in the run vocabulary that the UI's jobs
  screen, its banner and the system status read pass by pass, so the fetch sits inside the first
  pass.
- **Leaving a gone message's subject stale, and ending the operation by recording the pair the last
  completed pass ran under.** The case for it is that no subject is changed on an inference. Not
  chosen. It keeps a subject masked under a scanner no longer in force, which is the under-masking
  direction [ADR-0003](./0003-subject-masking.md) refuses, and it adds a second account of what is
  stale beside the one the rows carry.
- **A stale verdict stays in force until it is re-scanned.** The case for it is that no body is
  denied while the re-scan runs. Not chosen. A verdict made under a scanner no longer in force is
  not the affirmative clearance by the scanner in force that release requires, and work that has not
  happened yet is a deny state ([DESIGN.md](../../../DESIGN.md#fail-closed-everywhere)).
- **Returning stale verdicts to pending at the second pass's start, beside the skips.** The case for
  it is one comparison in one place. Not chosen. The second pass waits for the first, which may fail
  on every run, so stale verdicts would keep releasing bodies for that long, which is the alternative
  above under another name. Unlike a skip's signal, a verdict's staleness needs nothing the first
  pass produces.
- **Deciding the restart by effect, a waiting message at or before the checkpoint.** The case for it
  is that it needs no stored mark and covers any path that returns a message to pending before the
  checkpoint. Not chosen. Messages stay pending before the checkpoint by design, a body the converter
  refuses ([ADR-0017](../data/0017-two-pass-backfill.md)), one the provider no longer has, one
  abandoned after its attempts, and one the gate cannot decide, so every resume after a crash would
  start over and fetch their bodies again, which breaks a resume's one page of rework.
- **Deciding the restart from the scanner pair the stopped run's checkpoint records.** The case for
  it is that it needs no column, as the first pass's enumeration does. Not chosen. A change reverted
  before any second pass runs under it brings the pair back to the stopped run's, so the run resumes
  past the verdicts the change returned to pending and they stay pending for good.
- **Keeping the old flags on a message returned to pending.** The case for it is that nothing the
  old scan found is forgotten before the new one runs. Not chosen. The Redaction Gate reads content
  flags on a message that is not scanned as an invalid stored state
  ([ADR-0002](./0002-fetch-time-re-evaluation.md)), so the denial would misreport a pending scan,
  and the body is denied while pending whatever the flags say.
- **Masking every subject whole the moment the scanner changes, until it is re-masked.** The case
  for it is that no subject is served under a scanner no longer in force. Not chosen. It withholds
  every subject in the mailbox for as long as the re-mask takes, which
  [C1](../../../USE_CASES.md#c1--metadata-always-visible) forbids.
- **Marking the rows stale when the change is made**, by a migration shipped with a new version or
  by whatever writes a new configuration. The case for it is that the marking happens at the moment
  of the change. Not chosen. Re-scanning is a planned operation and not a migration
  ([ADR-0009](./0009-scanner-verdicts-carry-no-content.md)), and a configuration change has no
  writer the application runs, which is the gap [ADR-0037](./0037-delisting-transition.md) found
  in its writer alternative.
- **Comparing each newly loaded configuration with the one before it.** The case for it is that no
  stored stamp is read. Not chosen, for the reason [ADR-0037](./0037-delisting-transition.md) gives,
  a process that starts after the change holds no earlier configuration.
- **Delta sync doing the re-mask.** No case was tabled beyond its running anyway. Not chosen,
  because its ticks last seconds and scanning is a backfill-scale concern
  ([ADR-0018](../data/0018-delta-sync-polls.md)).

## Consequences

- Any change of version or revision makes every subject and every verdict stale. The unmasked
  subjects are masked again at the run's start, the masked ones by fetches of their own, and the
  second pass runs again. From the run that sees the change until the second pass scans them again,
  the bodies of the messages returned to pending are denied as pending their content scan, which is
  over-redaction for that time. That time includes no re-enumeration of the mailbox.
- One window remains open. A message another job kind stored with its subject unmasked under
  another scanner pair after the run-start step, and which the gate skipped, keeps its skip while
  the new scanner would mask its subject, and its body stays released as the skip allows, until the
  second pass starts and returns to pending each skip whose subject is then masked. A subject stored
  masked is never skipped, since the gate scans every message whose subject is masked
  ([ADR-0093](./0093-composite-scan-gate.md)).
- Superseded masking events stay as history, and a re-mask inserts new masking events with new
  identifiers. The masking events of a message whose pair equals its subject's are the masks of the
  subject as it now stands.
- A re-opened second pass also runs the delisting transition at its start
  ([ADR-0037](./0037-delisting-transition.md)).
- Assumptions about other components. Every job kind that masks or scans runs the same scanner
  version and the same scanner section, which one configuration load in the worker gives by
  construction ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)). Ingest
  stores a subject in no form other than the provider's, so an unmasked stored subject is the
  subject as fetched, and a subject does not change at the provider after it is stored. A provider
  fetching messages by identifier reports a message it no longer has as gone by leaving it out of
  the result, as the Provider Port's contract for the metadata read states in `core/mail/port.go`
  ([ADR-0010](../provider/0010-one-provider-port.md)). Every job kind that masks a subject records
  the pair it masked under.
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
