# 0096. A change of scanner re-opens backfill, whose passes re-mask and re-scan what an earlier scanner decided

**Status:** Accepted ·
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
([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)). Neither record
says how the operation is started or run.

The verdict is half of what the scanner decides. Subject masking runs on every message with the same
scanner, and the index stores only the masked subject
([ADR-0003](./0003-subject-masking.md), [ADR-0016](../data/0016-schema.md)), so a subject cannot be
masked again from what is stored, only from the provider. Backfill's first pass masks a subject as
it adds the message and leaves a message it already holds as it is
([ADR-0017](../data/0017-two-pass-backfill.md)), and the masked subject carries no record of the
scanner that masked it. The scan gate scans a message whose subject was masked
([ADR-0093](./0093-composite-scan-gate.md)), so a skip decided before a better scanner masked the
subject was decided without a signal the message now carries.

A new version ships between production points with no manual step, so the read path must re-scan on
a change without one ([ROADMAP.md](../../../ROADMAP.md)). The application does not know what invokes
its batch jobs, which are exit codes plus idempotent resume
([ADR-0051](../engineering/0051-environment-contract.md)). The delisting transition met the same
shape of problem, stored state that a change elsewhere made wrong, by having the workload compare
what the index stores with what it runs with at the start of its runs
([ADR-0037](./0037-delisting-transition.md)).

## Decision

**The masked subject carries the scanner it was masked under.** Every stored subject records the
scanner version and the configuration revision of the masking that produced it, beside the pair a
verdict records, and every masking event records the same pair, so a reader can tell the masks of
the subject as it now stands from those of a masking it replaced. A subject or a verdict is stale
when its pair differs from the scanner a workload runs with, a missing pair included.

**A stale verdict returns to pending as soon as a run sees it.** Every backfill run, at its start
and before the first pass, compares the index with the scanner it runs with, in one transaction. A
scanned message whose verdict carries another pair returns to pending, its verdict cleared, the
content flags, content rules, scan time, version and revision. The prior hits of every sender with a
message returned to pending are counted again from the flagged verdicts the index then holds, so a
re-scan that flags the message again counts it once. When it returned any message to pending, or a
stored subject carries another pair so the first pass is due again, the same transaction records the
second pass as not ended and marks it to start over, a flag on the account's state. It writes to no
run's record. The next run of the second pass, when it resumes a stopped one with the mark set,
starts over from the first message waiting for a scan, so a message returned to pending before the
checkpoint is read, and the run that starts clears the mark in the transaction that records its
start. The mark is set by what the run-start step did, never inferred from which scanner is in
force, so a change reverted before any second pass ran under it still starts the pass over. Pending
is the existing state, which denies the body
([ADR-0093](./0093-composite-scan-gate.md), [ADR-0002](./0002-fetch-time-re-evaluation.md)), and no
state is added. The verdict is denied from that moment, whatever the first pass's state, so a first
pass that runs for hours or fails on every run leaves no stale verdict releasing a body.

**Backfill observes a change by its effect, at the start of each run.** Backfill's first pass
counts as ended for an account only while every stored subject carries the pair of the scanner it
runs with, and when it is found due again the run that starts it clears its completion flag. The
second pass is recorded as not ended by the run-start comparison above, whenever it returned a
verdict to pending or found the first pass due. A pass recorded as not ended runs again, a fresh pass
when its latest run succeeded ([ADR-0017](../data/0017-two-pass-backfill.md)). So a reopened first
pass always leads to a run of the second, which also scans the mail the first pass added, and every
skip decided without its subject's signal follows a re-mask that reopened both. No record of the
change is kept or read. A change made in any layer, by a release or while nothing ran, is seen by
the next run.

**The first pass re-masks as it enumerates.** It enumerates the whole mailbox again. A message the
index already holds whose subject is stale has its subject masked again from the subject the
provider returns, under the scanner the pass runs with, with a masking event for each mask, and no
other column of the row changes. The pass's checkpoint records the pair its enumeration runs under,
and a run resuming an enumeration made under another pair starts the enumeration over from the first
page. That start over drops the estimate of the pages the enumeration takes with its token, as every
start over does ([ADR-0095](../provider/0095-enumeration-total-on-every-page.md)), so the estimate
always comes from the enumeration in progress.

**A message the provider no longer has is masked whole.** When an enumeration made under one pair
from its first page ends, a message still carrying a stale subject was not in the mailbox the
enumeration traversed. Its stored subject is masked whole under the pair the pass runs with, as a
scanner that cannot decide masks it ([ADR-0003](./0003-subject-masking.md)), with the masking event
of a whole subject, and the run records the message as a failed item of class `gone`. Its subject
is then current, so it never makes the pass due again.

**A skip decided without its subject's signal returns to pending at the second pass's start.** At the
start of each of its runs, after the delisting transition, the second pass returns to pending each
message the gate skipped whose subject is now masked, since the skip was decided without that signal.
Whether a subject is now masked is known only once the first pass has masked it again, which is why
this comparison waits for the second pass, which waits for the first. It needs no restart of its
own, because a subject is masked again only by a first pass the run-start step found due, which
marked the second pass to start over, so the run that follows starts from the first message waiting
for a scan.

**What starts backfill after a change is the deployment's.** Every backfill run makes the comparisons
above, so one run after a change of the scanner's version or section does the work, and a run with
nothing stale does none. Running backfill after each such change, with no manual step, is the
deployment's ([ADR-0051](../engineering/0051-environment-contract.md)).

## Alternatives considered

- **Re-masking through the metadata fetch over the stored identifiers, in a pass of its own.** The
  case for it is that the provider names each message it no longer has exactly, and that it reads
  only the stale messages. Not chosen. A third pass would be a third name in the run vocabulary that
  the UI's jobs screen, its banner and the system status read pass by pass, and any change of
  scanner makes every subject stale, so the stale messages are the whole mailbox and enumeration
  reads no more. Enumeration is the first pass's own primitive
  ([ADR-0010](../provider/0010-one-provider-port.md)).
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
  it is one comparison in one place. Not chosen. The second pass waits for the first, which
  re-enumerates the whole mailbox and may fail on every run, so stale verdicts would keep releasing
  bodies for that long, which is the alternative above under another name. Unlike a skip's signal, a
  verdict's staleness needs nothing the first pass produces.
- **Deciding the restart by effect, a waiting message at or before the checkpoint.** The case for it
  is that it needs no stored mark and covers any path that returns a message to pending before the
  checkpoint. Not chosen. Messages stay pending before the checkpoint by design, a body the converter
  refuses ([ADR-0017](../data/0017-two-pass-backfill.md)), one the provider no longer has, one
  abandoned after its attempts, and one the gate cannot decide, so every resume after a crash would
  start over and fetch their bodies again, which breaks a resume's one page of rework.
- **Deciding the restart from the scanner pair the stopped run's checkpoint records.** The case for
  it is that it needs no column, as the first pass's enumeration does. Not chosen. A change reverted
  before any second pass runs under it brings the pair back to the stopped run's, so the run resumes
  past the verdicts the change returned to pending and they stay pending for good. The first pass is
  not exposed the same way, because its run under the new scanner records the new pair at its start.
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

- Any change of version or revision makes every subject and every verdict stale, so the operation
  costs both passes again. From the run that sees the change until the second pass scans them again,
  the bodies of the messages returned to pending are denied as pending their content scan, which is
  over-redaction for that time.
- One window remains open. A message the gate skipped, whose subject the new scanner masks, keeps its
  skip, and its body stays released as the skip allows, until the first pass has masked the subject
  again and either the second pass has started or a later backfill run's start has decided the skip
  again ([ADR-0098](./0098-every-backfill-run-decides-each-gate-skip-again.md)). A first pass that
  fails on every run before it masks the subject holds that window open.
- Superseded masking events stay as history. The masking events of a message whose pair equals its
  subject's are the masks of the subject as it now stands.
- An enumeration that leaves out a message the provider still holds, without an error, masks that
  message's subject whole. It is visible as the whole-subject masking event and the failed item of
  class `gone`. The subject then carries the pair in force, so it stays masked whole until the next
  change of scanner masks it again from the provider. That is the direction
  [ADR-0003](./0003-subject-masking.md) chooses, a subject hidden that need not be rather than a code
  left visible, and its cost falls on [C1](../../../USE_CASES.md#c1--metadata-always-visible)'s
  visibility of that one subject.
- A re-opened second pass also runs the delisting transition at its start
  ([ADR-0037](./0037-delisting-transition.md)).
- Assumptions about other components. Every workload that masks or scans runs the same scanner
  version and the same scanner section, so two workloads that disagree re-open each other's work on
  every run. Enumeration returns every message the mailbox holds, as the Provider Port's
  contract for it states in `core/mail/port.go`, which for Gmail includes the trash and spam, since
  the adapter lists with `includeSpamTrash` (`provider/gmail/doc.go`). Every workload that masks a subject records
  the pair it masked under. Something runs backfill after each change of the scanner's version or
  section ([ADR-0051](../engineering/0051-environment-contract.md)).
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
