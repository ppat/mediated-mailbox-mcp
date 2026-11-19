# Mutations: worker

The demonstrations of the controls whose patches sit in `worker/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A backfill run's start masks every stale subject stored unmasked again from the store, a batch at a time, with a masking event for each mask

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the run-start step masks the subjects stored masked again from their masked text too, so a subject whose masks hid what the new scanner reads is never fetched again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (2):** the run-start step writes the subjects it masks again from the store and records no masking event for their masks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (3):** the run-start step masks again only the first batch of subjects stored unmasked, so a mailbox holding more keeps the rest under the earlier scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (4):** the run-start step masks no subject stored unmasked again, so each keeps a mask an earlier scanner decided until a fetch the first pass never makes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (5):** the run-start step writes each subject it masks again with its masks applied and leaves it reading unmasked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`

## A body the port or the conversion refuses is recorded refused, never scanned, and not fetched again

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the second pass abandons a body the port refuses, leaving it waiting to be fetched again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core`:** `TestOnFailure`, `TestOnFailure/a_body_the_port_refused`
- **Break (2):** a tick leaves a body the port refuses waiting, so the next pass over the backlog fetches it again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick/core`:** `TestOnBodyFailure`
- **Break (3):** a rendering refusal is ignored, so the body is scanned as empty and recorded clean, by a tick and by the second pass
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickRecordsABodyTheConversionRefusesAsRefused`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`
- **Break (4):** the record of what a job kind did with a message records a refused message as nothing, so it stays pending and is fetched again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`
- **Break (5):** a body the conversion refuses is scanned as its raw HTML and recorded scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`

## A call fetching subjects again that leaves every one of them stale fails the run

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a call is progress only when it stores a subject, so a stale count another writer lowered during the call fails the run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFallingStaleCountIsProgress`
- **Break (2):** a call that stored its subjects under the pair in force is taken as having moved nothing, so every call fetching subjects again fails the run, a change of scanner that masks nothing included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
- **Break (3):** any change of what a call recorded is taken as progress, so a stale count another writer raised during a call that stored nothing lets the pass go on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestARisingStaleCountIsNotProgress`
- **Break (4):** a call that leaves every subject it fetched stale is taken as progress, so the pass asks for the same subjects without end
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestARisingStaleCountIsNotProgress`

## A change of scanner reopens a pass whose stored work another scanner decided

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a reopened first pass runs while it stays recorded as ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (2):** a stored subject whose configuration revision differs and whose version matches does not make the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAFirstPassRecordsAPageOnItsRetries`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (3):** a stored subject whose scanner version differs and whose revision matches does not make the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
- **Break (4):** a pass that ended is skipped whether or not it is due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAFirstPassRecordsAPageOnItsRetries`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core`:** `TestBegin`, `TestBegin/a_pass_that_ended_and_is_due_again`, `TestBegin/a_pass_that_ended_and_is_due_again_with_a_run_left_running`

## A cursor gap is recovered by re-enumerating from an hour before the last cursor was written, and counted

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the cursor gap rule fires while any gap was ever counted, so it never clears
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAlertingRules`
- **Break (2):** the cursor gap rule reads only the count's increase, so a gap the series' first sample counts never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAlertingRules`
- **Break (3):** a tick that finds a gap does not report it, so the gap series never moves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAFailedRecoveryIsTriedAgainByTheNextTick`, `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`
- **Break (4):** a recovery stores the current cursor before it lists the window, so a recovery that fails is never tried again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAFailedRecoveryIsTriedAgainByTheNextTick`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`
- **Break (5):** a gap's window starts an hour before now, whenever the last cursor was written
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick/core`:** `TestWindow`, `TestWindow/a_cursor_written_five_minutes_ago`, `TestWindow/a_cursor_written_long_ago`
- **Break (6):** a gap's window starts at the last cursor's write time, so mail the provider dated just before it is left out
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick/core`:** `TestWindow`, `TestWindow/a_cursor_written_five_minutes_ago`, `TestWindow/a_cursor_written_long_ago`

## A failed body fetch is retried, recorded gone or abandoned, or stops the run, by its class

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a message the provider no longer has stops the run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAFailedBodyFetch`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core`:** `TestOnFailure`, `TestOnFailure/gone`
- **Break (2):** a throttled body fetch stops the run at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAFailedBodyFetch`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core`:** `TestOnFailure`, `TestOnFailure/a_first_throttle`

## A first pass whose enumeration ended fetches again by identifier only the stale subjects, masks each from the subject the provider returns, and enumerates nothing when reopened

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a subject fetched again is masked from the text the index stores rather than from the subject the provider returns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestRefetch`
- **Break (2):** a pass reopened after its enumeration ended starts a fresh enumeration from the first page instead of fetching the stale subjects again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAThrottledCallIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestUnder`, `TestUnder/a_pass_reopened_after_its_enumeration_ended`
- **Break (3):** the read of the subjects to fetch again ignores a configuration revision that alone differs, so a subject masked under the earlier revision is never fetched again and the pass ends with it stale
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAFirstPassRecordsAPageOnItsRetries`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (4):** the read of the subjects to fetch again ignores a scanner version that alone differs, so a subject masked under the earlier version is never fetched again and the pass ends with it stale
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`

## A gap recovery removes a stored message dated in its window once a complete listing left it out and the provider no longer returns it by its identifier

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a gap recovery treats a stored message dated before its window as left out of its listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (2):** a gap recovery removes nothing it found missing from its listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (3):** a gap recovery removes every stored message its listing left out, the ones the provider still returns by identifier included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick/core`:** `TestRemoval`

## A killed backfill run resumes from its last checkpointed page, with at most one page of rework

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a page made durable keeps the token it was asked with, so the next page asked for is the same one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAPageThatFailsToCommitLeavesNothing`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestARefusedTokenTheRunGotFailsTheRun`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestASubjectStoredUnmaskedIsTheProvidersSubject`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestTheCheckpointCarriesThePagesTheTotalImplies`, `TestTheModelAgreesWithPostgreSQL`, `TestThePassDoesNotEndWhileASubjectIsStale`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestAdvanceAndRestart`
- **Break (2):** a resumed run counts one page more than its checkpoint, so it reports a page it never made durable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
- **Break (3):** a run resuming a pass whose enumeration has ended asks for the first page again rather than fetching the stale subjects and finishing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAFirstPassRecordsAPageOnItsRetries`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (4):** a resumed run starts from the first page, dropping the checkpoint and counters of the run it resumes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core`:** `TestBegin`, `TestBegin/a_pass_not_ended_that_is_due_again`, `TestBegin/a_pass_that_ended_and_is_due_again_with_a_run_left_running`, `TestBegin/a_run_that_failed`, `TestBegin/a_run_that_stopped_without_recording_its_end`

## A killed second pass resumes from its last checkpointed page

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the stored checkpoint loses the last message read, so a resumed run reads from the first waiting message
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** the stored checkpoint loses the send time of the last message read, so a resumed run reads from the Unix epoch, past every message in the walk, and leaves the rest waiting
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (3):** a page's position keeps the identifier of its last message and not its send time, so the next page reads from the Unix epoch, past every message in the walk, and the pass leaves the rest waiting
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestAPolicyEditReachesARunningSecondPassWithinAPage`, `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestTheSecondPassReadsTheNewestWaitingMessagesFirst`
- **Break (4):** a page reads from the position's own message on, so a message left waiting at a page's end is read again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`
- **Break (5):** a resumed run counts one page more than its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`, `TestTheTransitionMarksNothingTwice`

## A message the gate skipped whose subject is now masked returns to pending

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** every message the gate skipped returns to pending, whether or not its subject is masked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAGateSkipWhoseSubjectIsMaskedReturnsToPendingAtTheSecondPassesStart`
- **Break (2):** a gate skip decided without the subject's signal the message now carries stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAGateSkipWhoseSubjectIsMaskedReturnsToPendingAtTheSecondPassesStart`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`

## A newly connected account is backfilled within one reload with no manual step, and the run-start step is made once for each job's life

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the run-start step is never recorded as made, so it is made again at every run of the job
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheRunStartStepIsMadeOnceForEachJobsLife`
- **Break (2):** the run-start step is made once for each account in the process rather than each job's life, so an account listed again never gets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheRunStartStepIsMadeOnceForEachJobsLife`
- **Break (3):** the run-start step is made once for the job kind's life rather than each job's, so an account served after another account's step never gets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheRunStartStepIsMadeOnceForEachJobsLife`
- **Break (4):** the run-start step is recorded as made before it is made, so a step that fails is not made again for the job's life
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAFailedRunStartStepIsMadeAgainAtTheNextAsk`

## A page's rows, masking events, sender statistics and checkpoint become durable together or not at all

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a checkpoint that fails to record is ignored, so the page's rows commit without it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAPageThatFailsToCommitLeavesNothing`
- **Break (2):** the messages, masking events and sender statistics commit in a transaction of their own before the one recording the checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageThatFailsToCommitLeavesNothing`

## A panic in a job's run is recovered and recorded as the run's failure with its stack

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a panic raised in a page of the first pass is recovered and returned, and the run is never recorded as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAPanicInARunFailsThatRunAlone`
- **Break (2):** a panic raised in a page of the first pass is not recovered, so it ends the process and the run's record says nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAPanicInARunFailsThatRunAlone`
- **Break (3):** a panic raised in the comparison the second pass makes at its start is not recovered, so it ends the process and the recorded run stays running with no error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAPanicAtTheSecondPassesStartIsRecordedAsItsRunsFailure`
- **Break (4):** a panic raised in a page of the second pass is recovered and returned, and the run is never recorded as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAPanicInARunFailsThatRunAlone`
- **Break (5):** a panic raised in a page of the second pass is not recovered, so it ends the process and the run's record says nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAPanicInARunFailsThatRunAlone`
- **Break (6):** a panic raised in a tick is recovered and returned, and the tick is never recorded as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAPanicInARunFailsThatRunAlone`
- **Break (7):** a panic raised in a tick is not recovered, so it ends the process and the tick's record says nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAPanicInARunFailsThatRunAlone`

## A policy edit reaches a running second pass within one reload and one page

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** every page makes the delisting comparison again whatever the policy, so a reload with the same rules returns to pending a message skipped as restricted whose sender no rule lists
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAPolicyEditReachesARunningSecondPassWithinAPage`
- **Break (2):** a page never makes the delisting comparison again, so a lift reaches the messages skipped as restricted only at the next run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`

## A refusal returns to pending only when the bound it was refused under changes

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the delisting transition returns a refused message of a delisted sender to pending with its bound cleared, so a change of the sender's class fetches a refused body again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`
- **Break (2):** the run-start step compares the stored bounds with none, so every refusal returns to pending at every start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`
- **Break (3):** the run-start step returns no refusal to pending, so a body refused under another bound stays refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`

## A reload re-seals what was sealed to an old key, an OAuth client's secret included, whether or not any account is listed

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the re-seal and the key-scan series move from the listing to each account's tick, so with no account listed nothing is re-sealed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_no_account`
- **Break (2):** a listing that finds no account re-seals nothing and sets no key-scan series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_no_account`

## A removed rule returns its sender's messages to pending scan, and the pass starts over

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the comparison delists the domains the policy still restricts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestAStoredDomainClassifiesAsItsAddress`, `TestDelisted`, `TestDelisted/a_policy_that_never_loaded`, `TestDelisted/every_rule_in_place`, `TestDelisted/every_rule_removed`, `TestDelisted/the_bank's_rule_removed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`, `TestTheTransitionMarksNothingTwice`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsNoPageOnItsEventsOrFailedItems`, `TestATickScansADelistedSendersMessages`
- **Break (2):** a run that marked messages keeps its checkpoint, so marked messages before it wait until the pass ends
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`, `TestTheTransitionMarksNothingTwice`
- **Break (3):** the transition reads only the first domain of the messages skipped as restricted, so a second delisted sender's messages stay skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`
- **Break (4):** the transition reads and returns scanned messages as well as those skipped as restricted, so a verdict made before a rule restricted its sender is undone when the rule is lifted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestTheTransitionLeavesACleanScannedVerdict`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARuleAddedAndLiftedLeavesATicksVerdict`, `TestARuleAddedAndLiftedLeavesATicksVerdict/clean`, `TestARuleAddedAndLiftedLeavesATicksVerdict/flagged`, `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickDecidesABoundedNumberAndTheNextGoesOn`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`, `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (5):** the transition marks no domain
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestARefusalReturnsToPendingOnlyWhenItsBoundChanges`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`, `TestTheTransitionMarksNothingTwice`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`

## A reopened first pass always leads to a run of the second pass

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the run-start step records the first pass as not ended in place of the second
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (2):** the run-start step reopens the second pass only when it returned a verdict, a refusal or a skip to pending or masked a subject again, not when a subject stored masked leaves the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAReopenedFirstPassLeadsToASecondPass`

## A rotated refresh token is written back at the end of every page's unit of work, a page that failed included, and a failed write-back ends the pass in an error

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a page that fails ends the pass before the hand-over, so a rotation made before the failure is lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedPageIsStillHandedOver`
- **Break (2):** the account's unit of work is ended only once the first pass ends or fails, not after every page
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`, `TestEachPageIsAUnitOfWork`
- **Break (3):** a hand-over that fails is dropped, so the pass ends without an error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`
- **Break (4):** the end of a unit of work never ends the account's unit, so no rotated credential is written back and no attempt recorded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAFailedRecordingFailsTheJobKindsRun`, `TestAFailedRecordingFailsTheJobKindsRun/backfill`, `TestAFailedWriteBackFailsTheJobKindsRun`, `TestAFailedWriteBackFailsTheJobKindsRun/backfill`

## A run resuming an enumeration not yet ended that was made under another scanner continues it from its checkpoint under the scanner in force

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a run records the stale subjects it has to fetch only once its enumeration has ended, so it continues an enumeration not yet ended before it fetches the subjects stored masked under another scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestUnder`, `TestUnder/a_fresh_pass`, `TestUnder/a_pass_reopened_after_an_enumeration_that_did_not_end`, `TestUnder/a_pass_reopened_with_no_run_recorded`, `TestUnder/a_run_resumed_before_its_first_page`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`, `TestUnder/a_run_resumed_under_the_same_scanner`
- **Break (2):** a run continuing an enumeration not yet ended that was made under another scanner drops the estimate of the pages the enumeration takes, so the card shows no estimate until the next page
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAnEnumerationUnderWayContinuesUnderANewScanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestUnder`, `TestUnder/a_run_resumed_before_its_first_page`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`
- **Break (3):** a run continuing an enumeration not yet ended that was made under another scanner keeps that scanner in its checkpoint, so the pass reads what is stale, and decides whether it has ended, under a scanner no longer in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestUnder`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`
- **Break (4):** a run resuming an enumeration not yet ended that was made under another scanner starts it over from the first page, enumerating again the pages already made durable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestUnder`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`
- **Break (5):** a run fetches stale subjects again only once its enumeration has ended, so it makes the pages of an enumeration not yet ended durable while the subjects stored masked under another scanner wait
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAnEnumerationUnderWayContinuesUnderANewScanner`

## A scan verdict records its content flags, content rules, scanner version and configuration revision

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a one-time code is stored as a login link
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestTheReleasedPartOfABodyIsScanned`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`
- **Break (2):** a verdict is stored without the configuration revision it was made under
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`, `TestTheTransitionLeavesACleanScannedVerdict`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`

## A second pass resuming a stopped pass while the run-start step's mark is set starts over from the first waiting message

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a second pass run resumes a stopped pass from its checkpoint whatever the mark, so what a run's start returned to pending before it waits for good
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core`:** `TestOver`, `TestOver/a_resumed_run_with_the_mark`
- **Break (2):** the second pass run that starts leaves the mark set, so every later run resuming the pass starts it over again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (3):** the run-start step reopens the second pass without marking it to start over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`

## A sender's first scan hit reaches the gate's decision on its next message, and no other sender's

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the gate reads only the prior hits stored before the page, so a hit earlier on the page does not count
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestGate`, `TestGate/a_hit_earlier_on_the_page`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`
- **Break (2):** a hit on the page counts as a prior hit of every sender on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestASendersFirstHitReachesItsNextMessage`

## A stale subject is masked again under the scanner in force, each mask recorded under it

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the first pass masks a stored subject again only when its scanner version differs, so a changed configuration revision leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAFirstPassRecordsAPageOnItsRetries`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (2):** the first pass masks a stored subject again only when its configuration revision differs, so a changed scanner version leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
- **Break (3):** a masking event is recorded with no scanner version or revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestTheModelAgreesWithPostgreSQL`

## A stored gate skip the gate no longer decides as the same skip returns to pending at backfill's run-start step

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a stored gate skip is decided again under the default thresholds rather than the ones the run holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/start/core`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/a_widened_high-volume_mark`, `TestOverturned/the_thresholds_the_skips_were_made_under`
- **Break (2):** every stored gate skip is overturned, the ones the gate still decides as the same skip included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/start/core`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/a_widened_high-volume_mark`, `TestOverturned/the_thresholds_the_skips_were_made_under`
- **Break (3):** a stored gate skip is overturned only when the gate would now scan it, so a skip under thresholds that cannot decide or of a sender now restricted stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/start/core`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/thresholds_that_cannot_decide`
- **Break (4):** the run-start step reads the gate skips and returns none to pending, so every skip made under earlier thresholds stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`
- **Break (5):** the read of the stored gate skips drops the sender's prior hits, so a skip whose sender has since gained a hit stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestASkipWhoseSenderGainedAPriorHitIsScanned`

## A stored subject whose message the provider no longer has is masked whole under the scanner in force

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a subject whose message the provider left out of its answer keeps its stored text under the scanner in force instead of being masked whole
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestRefetch`, `TestUnfound`
- **Break (2):** a call fetching subjects again masks no subject whose message the provider left out of its answer, leaving it masked under the earlier scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`

## A tick applies each change set in the transaction that advances the cursor past it

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the cursor is stored in a transaction of its own before the change set's writes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (2):** a change set's application stores no cursor, so the next tick asks for the same changes again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (3):** a stored message's labels and flags are left as they were when the provider reports them changed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (4):** a message the provider no longer holds stays in the index
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestATickAppliesTheChangesSinceItsCursor`, `TestATickStoresTheAttachmentMediaOfWhatItAdds`

## A tick hands its account over and records its latest authentication attempt when it ends

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a tick drops the write-back's and the recording's failures its unit of work ends with, so the tick's run reports neither
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAFailedRecordingFailsTheJobKindsRun`, `TestAFailedRecordingFailsTheJobKindsRun/delta_sync`, `TestAFailedWriteBackFailsTheJobKindsRun`, `TestAFailedWriteBackFailsTheJobKindsRun/delta_sync`
- **Break (2):** a tick that fails ends no unit of work, so it writes nothing back and records no attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAFailedTickStillEndsItsUnitOfWork`

## A tick records as failed the account's tick and gap recovery a stopped process left running

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a recovery's start also records the tick that started it, still running, as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`
- **Break (2):** a tick's start records a stopped tick as failed and leaves a stopped recovery recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`
- **Break (3):** a tick's start leaves every run a stopped process left running recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`, `TestATickRecordsAStoppedRunAsFailed/a_tick_stopped_part_way`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`

## A tick's metadata fetch names no more identifiers in one call than the mailbox says fit

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** a tick asks for the metadata of every identifier in one call, whatever the mailbox says fits
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATicksMetadataCallsFitTheCallSize`

## A verdict made under another scanner returns to pending, cleared, with its sender's prior hits counted again

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a verdict whose configuration revision differs and whose version matches stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (2):** a verdict whose scanner version differs and whose revision matches stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
- **Break (3):** a verdict returned to pending keeps its content flags, content rules, scan time, version and revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`
- **Break (4):** the prior hits of a sender whose verdicts returned to pending are not counted again, so a message flagged again counts twice
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (5):** a backfill run leaves a verdict made under another scanner in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`
- **Break (6):** backfill's run-start step returns no verdict to pending before the first pass, so stale verdicts keep releasing bodies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (7):** backfill's run-start step returns stale verdicts to pending only after the first pass ends, so they keep releasing bodies while it runs or fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerContinuesAnEnumerationUnderWay`, `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`

## A worker start logs no error before its Session loads, and is ready only once the Session's first load has woken each job kind's listing

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** backfill's listing asked before the Session loads fails rather than answering not due, so a normal start logs an error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`
- **Break (2):** backfill's listing answers a Session that has not loaded yet with no account and no error, so the listing counts as succeeded and the worker reports ready before the Session loads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`
- **Break (3):** delta sync's listing asked before the Session loads fails rather than answering not due, so a normal start logs an error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`
- **Break (4):** delta sync's listing answers a Session that has not loaded yet with no account and no error, so the listing counts as succeeded and the worker reports ready before the Session loads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`

## An account with no cursor is reconciled over the first window inside its tick

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an account's first reconciliation is reported as a cursor gap, so the gap series and its alert count a cursor that was never lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (2):** an account with no cursor takes the current cursor and lists only what is dated after it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestARuleAddedAndLiftedLeavesATicksVerdict`, `TestARuleAddedAndLiftedLeavesATicksVerdict/clean`, `TestARuleAddedAndLiftedLeavesATicksVerdict/flagged`, `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickAppliesTheChangesSinceItsCursor`, `TestATickDecidesABoundedNumberAndTheNextGoesOn`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`, `TestATickScansADelistedSendersMessages`, `TestATickStoresTheAttachmentMediaOfWhatItAdds`, `TestBeforeTheSecondPassHasEndedATickScansNothing`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`

## An account's series go once the work that sets them ends

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the second pass's backlog series stays once the pass ends, reading its last value beside delta sync's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnAccountsSeriesGoWithItsWork`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`
- **Break (2):** backfill's backlog series of an account stays once its job is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestADroppedAccountsBacklogGoesWithItsJob`
- **Break (3):** backfill's plan names nothing to do when the listing drops an account, so its series stay
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestADroppedAccountsBacklogGoesWithItsJob`, `TestAnAccountsSeriesGoWithItsWork`
- **Break (4):** backfill's unclassified series of an account stays once its job is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnAccountsSeriesGoWithItsWork`
- **Break (5):** delta sync's plan names nothing to do when the listing drops an account, so its series stay
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnAccountsSeriesGoWithItsWork`
- **Break (6):** delta sync's tick series of an account stay once its job is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnAccountsSeriesGoWithItsWork`

## An account's tick runs only once its sync interval has passed since the last

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an account's first tick in the process is never due
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/core`:** `TestDeltaSyncsDueDecision`, `TestDeltaSyncsDueDecision/no_tick_yet_in_this_process`
- **Break (2):** a tick asked exactly one interval after the last is not due
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/core`:** `TestDeltaSyncsDueDecision`, `TestDeltaSyncsDueDecision/asked_one_interval_after_the_last_tick`
- **Break (3):** a tick is due whenever it is asked, before its interval has passed included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/core`:** `TestDeltaSyncsDueDecision`, `TestDeltaSyncsDueDecision/asked_a_moment_before_the_interval_has_passed`, `TestDeltaSyncsDueDecision/asked_at_once_again`, `TestDeltaSyncsDueDecision/asked_at_the_end_of_a_backoff_after_a_failed_tick`
- **Break (4):** an account's job ticks whenever it is asked, the end of a backoff included, whatever its due decision says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAJobAskedBeforeItsIntervalDoesNotTick`
- **Break (5):** a tick is recorded at the time it was asked rather than at its time on the ticker's phase, so after a tick at the end of a backoff the ticker's next tick reads not due and the account waits one interval more
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestATickAtTheEndOfABackoffKeepsThePhase`

## An overturned gate skip reopens the second pass, marked to start over

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the run-start step reopens the second pass only for a stale verdict, a refusal made under another bound, a subject it masked again or a stale subject, so a skip it returned to pending after the pass ended waits for good
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`
- **Break (2):** the run-start step reopens the second pass for overturned skips without marking it to start over, so a stopped pass resumes past the skips returned before its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`

## Applying the same changes twice leaves the index as applying them once

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** applying a message's labels adds them to the ones stored rather than setting them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnce`
- **Break (2):** a message the index already holds has its masks recorded again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnce`

## Backfill's due decision asks for the run-start step once for each job's life and for the passes while either has work

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a first pass that has not ended is not due once the run-start step is made
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_first_pass_not_ended`
- **Break (2):** the passes are due only at the job's first run, whatever recorded state says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_first_pass_not_ended`, `TestBackfillsDueDecision/the_second_pass_marked_to_start_over`, `TestBackfillsDueDecision/the_second_pass_not_ended`, `TestBackfillsSeed`, `TestBackfillsSeed/a_first_pass_with_runs_and_no_success`, `TestBackfillsSeed/a_pass_not_ended_with_a_success_recorded`, `TestBackfillsSeed/the_second_pass_marked_to_start_over`
- **Break (3):** a second pass marked to start over is not due
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_second_pass_marked_to_start_over`, `TestBackfillsSeed`, `TestBackfillsSeed/the_second_pass_marked_to_start_over`
- **Break (4):** a second pass that has not ended is not due once the run-start step is made
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_second_pass_not_ended`, `TestBackfillsSeed`, `TestBackfillsSeed/a_pass_not_ended_with_a_success_recorded`
- **Break (5):** the run-start step is asked for at every run, not once for the job's life
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/both_passes_ended_and_started`, `TestBackfillsDueDecision/the_first_pass_not_ended`, `TestBackfillsDueDecision/the_second_pass_marked_to_start_over`, `TestBackfillsDueDecision/the_second_pass_not_ended`
- **Break (6):** the run-start step is never asked for, so a job never returns stale work to pending
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/a_new_account`, `TestBackfillsDueDecision/both_passes_ended,_not_yet_started_in_this_process`

## Backfill's second pass never asks for a restricted sender's body

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the gate reads every sender as normal, so a restricted sender's body is scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestGate`, `TestGate/a_sender_the_policy_lists`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestAPolicyEditReachesARunningSecondPassWithinAPage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`, `TestTheTransitionMarksNothingTwice`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsNoPageOnItsEventsOrFailedItems`, `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** every message is scanned whatever the gate decided, a restricted sender's included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestAPolicyEditReachesARunningSecondPassWithinAPage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestASecondPassRecordsAPageOnItsEventsAndFailedItems`, `TestTheTransitionMarksNothingTwice`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`

## Backfill's second pass reads the messages waiting for a scan newest first

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the pass reads through delta sync's statement, in the order of the identifiers, so the send time orders nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestTheSecondPassReadsTheNewestWaitingMessagesFirst`
- **Break (2):** messages sent at the same instant are read lowest identifier first, against the keyset
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-every-page-among-messages-sent-at-one-instant`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestTheSecondPassReadsTheNewestWaitingMessagesFirst`

## Delta sync decides under backfill's gate thresholds and records backfill's scanner version and revision

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** backfill's second pass decides under thresholds of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARunDecidesUnderTheSharedThresholdsAndScanner`
- **Break (2):** delta sync's gate decides under thresholds of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestATickDecidesUnderBackfillsThresholdsAndScanner`

## Delta sync refuses a configuration it cannot tick with

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the interval is refused when it is positive and taken when it is not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheScannerPartIsChecked`, `TestTheWorkerConfigurationIsChecked`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--account_reload_interval=0s`, `TestATickThatCannotRunRefusesTheStart/--sync.concurrency=0`, `TestATickThatCannotRunRefusesTheStart/--sync.decisions_per_tick=0`, `TestATickThatCannotRunRefusesTheStart/--sync.first_window=-1h`, `TestATickThatCannotRunRefusesTheStart/--sync.sync_interval=0s`, `TestAVerdictsRevisionFollowsTheScannerSection`, `TestAnEmptyListOfPrivateKeysRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`, `TestTheChartRendersAFileTheWorkerStartsWith`, `TestTheEffectiveConfigurationIsLogged`, `TestTheScannerIsBuiltFromItsSection`, `TestTheScannerSectionBuildsTheSharedScanner`, `TestTheStartBannerGoesToStandardErrorAndStandardOutputStaysJSON`
- **Break (2):** the interval, the first window and the bound on decisions are not validated
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheWorkerConfigurationIsChecked`, `TestTheWorkerConfigurationIsChecked/a_negative_first_window`, `TestTheWorkerConfigurationIsChecked/a_sync_interval_of_zero`, `TestTheWorkerConfigurationIsChecked/no_decisions_a_tick`, `TestTheWorkerConfigurationIsChecked/the_worker's_own_before_the_credential_part`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--sync.decisions_per_tick=0`, `TestATickThatCannotRunRefusesTheStart/--sync.first_window=-1h`, `TestATickThatCannotRunRefusesTheStart/--sync.sync_interval=0s`

## Delta sync runs until stopped and serves every series between ticks

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a series whose account or client a later scan no longer holds stays reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestTheSeriesFollowTheLastScan`
- **Break (2):** a listing re-seals but sets none of the key-scan series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_an_account`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_no_account`, `TestTheProbesServeEverySeriesBetweenRuns`
- **Break (3):** an account's gap series appears only at its first gap, so the rule's increase cannot see that gap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnAccountsSeriesGoWithItsWork`, `TestTheProbesServeEverySeriesBetweenRuns`

## Each flagged message adds one prior hit to its sender, durably

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** every scanned message counts as a hit, a clean one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core`:** `TestAdvance`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** no prior hit is added to the senders' statistics
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`

## Each job kind spends from the account's budget in its own class, backfill in the batch class and delta sync in the sync class

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** backfill's caller leases in the sync class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenRuns`
- **Break (2):** delta sync's caller leases in the batch class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenRuns`
- **Break (3):** delta sync's caller leases in the interactive class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenRuns`

## Each message the first pass or a tick stores carries its attachment media

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the index's decision drops each message's attachment media
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPassStoresEachMessagesAttachmentMedia`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickStoresTheAttachmentMediaOfWhatItAdds`
- **Break (2):** no message added to the index has its attachment media stored, whether the first pass or a tick adds it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPassStoresEachMessagesAttachmentMedia`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickStoresTheAttachmentMediaOfWhatItAdds`
- **Break (3):** every message added to the index stores each medium's media type with an empty extension
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPassStoresEachMessagesAttachmentMedia`

## Each page of a run takes the active policy

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a run takes the account's policy once, when its passes' dependencies are built, so a policy edit reaches it only at its next run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachPageTakesThePolicyInForce`
- **Break (2):** an account's job takes its policy once, when the job is made, so a policy edit never reaches its ticks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachTickTakesThePolicyInForce`

## Each reload of the worker's Session wakes every job kind's listing

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** backfill's plan is woken by no reload of the Session, so the jobs of an account it newly lists wait for the listing's next ask
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestADroppedAccountsBacklogGoesWithItsJob`, `TestADroppedAccountsRunsAreCancelled`, `TestAFailedRecordingFailsTheJobKindsRun`, `TestAFailedRecordingFailsTheJobKindsRun/backfill`, `TestAFailedRunStartStepIsMadeAgainAtTheNextAsk`, `TestAFailedWriteBackFailsTheJobKindsRun`, `TestAFailedWriteBackFailsTheJobKindsRun/backfill`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestAPanicInARunFailsThatRunAlone`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`, `TestAnAccountsSeriesGoWithItsWork`, `TestEachJobKindsPoolMustAnswerForTheWorkerToBeReady`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords/backfill`, `TestEachPageTakesThePolicyInForce`, `TestEachSessionReloadWakesTheJobKinds`, `TestNoAccountIsListedBeforeItsPolicyLoads`, `TestNoBodyTextReachesTheIndexOrTheLogs`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheDriversVariablesNameNothingTheWorkerConnectsTo`, `TestTheProbesServeEverySeriesBetweenRuns`
- **Break (2):** delta sync's plan is woken by no reload of the Session, so the jobs of an account it newly lists wait for the listing's next ask
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestADroppedAccountsRunsAreCancelled`, `TestAFailedRecordingFailsTheJobKindsRun`, `TestAFailedRecordingFailsTheJobKindsRun/delta_sync`, `TestAFailedTickStillEndsItsUnitOfWork`, `TestAFailedWriteBackFailsTheJobKindsRun`, `TestAFailedWriteBackFailsTheJobKindsRun/delta_sync`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestAPanicInARunFailsThatRunAlone`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`, `TestATickRecordsABodyTheConversionRefusesAsRefused`, `TestATickScansABodyHeldApart`, `TestATicksMetadataCallsFitTheCallSize`, `TestAnAccountsSeriesGoWithItsWork`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_an_account`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_no_account`, `TestEachAccountIsServedThroughItsOwnClient`, `TestEachJobKindsPoolMustAnswerForTheWorkerToBeReady`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords/backfill`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords/delta_sync`, `TestEachSessionReloadWakesTheJobKinds`, `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`, `TestEachTickTakesThePolicyInForce`, `TestNoAccountIsListedBeforeItsPolicyLoads`, `TestNoBodyTextReachesTheIndexOrTheLogs`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheDriversVariablesNameNothingTheWorkerConnectsTo`, `TestTheProbesServeEverySeriesBetweenRuns`, `TestTicksHoldTheirTokenUntilTheCredentialIsReplaced`

## Every backfill run is recorded with its checkpoint, counters, timeline and failed items

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a failure that is not the provider's is abandoned as the page's failure, recorded with no error class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFailureNotTheProvidersFailsTheRunWithNoFailedPage`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestOnFailure`, `TestOnFailure/a_failure_that_is_not_the_provider's`, `TestOnFailure/an_attempt_nobody_described`
- **Break (2):** a throttled or failed page is asked for again whatever the attempts, so the run never fails on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestAThrottledCallIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestOnFailure`, `TestOnFailure/a_provider_failure_on_the_last_attempt`, `TestOnFailure/a_throttle_on_the_last_attempt`
- **Break (3):** a page the provider throttles fails the run on its first attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core`:** `TestOnFailure`, `TestOnFailure/a_first_throttle`, `TestOnFailure/a_throttle_before_the_last_attempt`, `TestOnFailure/a_throttle_on_a_resumed_token`
- **Break (4):** a page the run fails on after its last attempt records no failed item
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestARefusedCredentialFailsTheRunAtOnce`, `TestARefusedTokenTheRunGotFailsTheRun`
- **Break (5):** a page is recorded as recovered in a transaction of its own as soon as it arrives, before its commit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageIsRecordedRecoveredOnlyOnceDurable`
- **Break (6):** a page that failed and then became durable records no failed item
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAThrottledPageIsRetriedAndRecorded`
- **Break (7):** a backfill run's progress, recorded through runs.RecordProgress, records its checkpoint and no progress event
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunIsResumedByTheNextRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestARefusedTokenTheRunGotFailsTheRun`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
- **Break (8):** a run resumed after it stopped without recording its end is left recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestTheModelAgreesWithPostgreSQL`

## Every scan gate decision over stored messages is recorded with its reason

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a scan is recorded as a skip and a skip as a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** a skip is not recorded in the decisions table
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`

## Every subject is masked and every sender classified before the message reaches the index

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a masked subject carries no mask, so no masking event is recorded for it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (2):** a message reaches the index with its subject as the provider gave it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`

## No account is added while the policy reload fails, and a dropped account's job is still removed

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** backfill's listing names every account the Session holds in force, so an account whose policy never loaded gets a job
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`, `TestNoAccountIsListedBeforeItsPolicyLoads`, `TestTheRunStartStepIsMadeOnceForEachJobsLife`
- **Break (2):** delta sync's listing names every account the Session holds in force, so an account whose policy never loaded gets a job
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`, `TestNoAccountIsListedBeforeItsPolicyLoads`, `TestTheRunStartStepIsMadeOnceForEachJobsLife`

## No body text reaches the index or the workload's logs

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a scan records the rendered body among its content rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestNoBodyTextReachesTheIndexOrTheLogs`
- **Break (2):** each body the second pass fetches is logged at debug through the run's logger
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestNoBodyTextReachesTheIndexOrTheLogs`
- **Break (3):** a refused body's failed item records the start of the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestNoBodyTextReachesTheIndexOrTheLogs`

## No dependency added for a job imports unsafe or holds assembly, and the worker builds without cgo

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a package of the worker holds assembly and imports no unsafe, so code outside memory safety joins a job kind's build
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestNoDependencyAddedForAJobImportsUnsafe`, `TestNoDependencyAddedForAJobImportsUnsafe/amd64`, `TestNoDependencyAddedForAJobImportsUnsafe/arm64`
- **Break (2):** the worker's entry package imports a dependency that imports unsafe, which no job kind used before
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestNoDependencyAddedForAJobImportsUnsafe`, `TestNoDependencyAddedForAJobImportsUnsafe/amd64`, `TestNoDependencyAddedForAJobImportsUnsafe/arm64`
- **Break (3):** the worker's image builds its binary with cgo switched on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheWorkerBuildsWithoutCgo`

## Once backfill's second pass has ended, each tick decides and scans what waits, a bounded number from where the last stopped

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** delta sync sets an account's backlog series before backfill's second pass has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (2):** a tick's backlog never reaches delta sync's backlog series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (3):** a tick reports no backlog after its scanning
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (4):** a tick decides every waiting message whatever its bound
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickDecidesABoundedNumberAndTheNextGoesOn`
- **Break (5):** a tick stopped by a throttle keeps its checkpoint past the rest of its read, so the next tick passes the throttled message over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`
- **Break (6):** a tick never compares the senders of the messages skipped as restricted with its policy, so a delisted sender's messages stay skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsNoPageOnItsEventsOrFailedItems`, `TestATickScansADelistedSendersMessages`
- **Break (7):** a tick never scans, so what waits stays waiting once backfill has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARuleAddedAndLiftedLeavesATicksVerdict`, `TestARuleAddedAndLiftedLeavesATicksVerdict/clean`, `TestARuleAddedAndLiftedLeavesATicksVerdict/flagged`, `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickDecidesABoundedNumberAndTheNextGoesOn`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`, `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (8):** each tick reads from the first waiting message, forgetting where the last stopped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickDecidesABoundedNumberAndTheNextGoesOn`
- **Break (9):** a tick scans whether or not backfill's second pass has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestBeforeTheSecondPassHasEndedATickScansNothing`
- **Break (10):** a throttled body fetch leaves its message waiting and the tick goes on fetching
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickRecordsNoPageOnItsEventsOrFailedItems`
- **Break (11):** the gate-and-scan loop stopped by a throttle in a tick goes on deciding the rest of its read without the hits of the bodies it did not scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`

## Pass 1 sets its completion flag when it ends, and a pass that ended and is not due again does no work

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the run that ends the pass records its end and leaves the account's completion flag unset
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestAPassThatEndedIsSkipped`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEmptyMailboxEndsThePass`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestTheModelAgreesWithPostgreSQL`, `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (2):** a pass that ended and is not due again starts a fresh run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassThatEndedIsSkipped`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core`:** `TestBegin`, `TestBegin/a_pass_that_ended`, `TestBegin/a_pass_that_ended_with_a_run_left_running`

## The chart renders the worker a file it starts with, holding every worker key the chart admits

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the schema admits a key in backfill's own section that the worker does not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheChartAdmitsEveryWorkerKeyItDoesNotRender`
- **Break (2):** the chart leaves the worker's pool size out of its file though the values set one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheChartRendersAFileTheWorkerStartsWith`
- **Break (3):** the chart leaves the shared sync interval out of delta sync's section
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheChartRendersAFileTheWorkerStartsWith`

## The default scanner section's revision is the known answer checked in

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the configuration library's revision algorithm takes the last 16 bytes of the section's hash rather than the first
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheScannerSectionBuildsTheSharedScanner`
- **Break (2):** the configuration library's revision algorithm keeps only the first 8 bytes of the section's hash
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheScannerSectionBuildsTheSharedScanner`
- **Break (3):** a field is added to the scanner's configuration type with a default that keeps today's behaviour, a third tier's switch, off, that nothing reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheChartAdmitsEveryWorkerKeyItDoesNotRender`, `TestTheScannerSectionBuildsTheSharedScanner`
- **Break (4):** a field of the scanner's configuration type is given another YAML name, dense_length rendered as dense_run_length, its value unchanged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheChartAdmitsEveryWorkerKeyItDoesNotRender`, `TestTheChartRendersAFileTheWorkerStartsWith`, `TestTheScannerSectionBuildsTheSharedScanner`

## The first pass ends only while no stored subject is masked under another scanner

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the run that ends the enumeration ends the pass whether or not a stored subject is stale
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (2):** the check that no stored subject is stale reads under no configuration revision, so every subject reads stale and the pass never ends
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestACheckpointWithoutAPageCountResumes`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAFirstPassRecordsAPageOnItsRetries`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestAPassStoresEachMessagesAttachmentMedia`, `TestAPassThatEndedIsSkipped`, `TestARisingStaleCountIsNotProgress`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestASubjectStoredUnmaskedIsTheProvidersSubject`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationUnderWayContinuesUnderANewScanner`, `TestTheCheckpointCarriesThePagesTheTotalImplies`, `TestTheModelAgreesWithPostgreSQL`, `TestThePassDoesNotEndWhileASubjectIsStale`

## The latest authentication attempt is recorded at the end of every unit of work, a failed one included, and a failed recording ends the pass in an error

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the end of a unit of work records the attempt but drops a recording that fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAFailedRecordingFailsTheJobKindsRun`, `TestAFailedRecordingFailsTheJobKindsRun/backfill`

## The messages whose sender could not be classified are counted per account

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** no message is marked as having an unclassified sender
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachPageIsAUnitOfWork`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (2):** a page's unclassified senders never reach the account's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachPageIsAUnitOfWork`
- **Break (3):** a tick's unclassified senders never reach delta sync's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (4):** a reconciliation adds its unclassified senders to the index without counting them in the tick's result
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`

## The run-start step writes to no run's record

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the run-start step records the stopped second pass run's progress as starting over, with a progress and a retry event, on that run's record
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`

## The scan backlog depth is emitted after each step of the second pass

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the backlog series is set under the run's identifier in place of the account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`
- **Break (2):** the backlog series is never set
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`

## The scan reads the part release serves, in the form it is served

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** neither the second pass nor a tick scans anything of a body with no HTML part
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestASendersFirstHitReachesItsNextMessage`, `TestTheReleasedPartOfABodyIsScanned`, `TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARuleAddedAndLiftedLeavesATicksVerdict`, `TestARuleAddedAndLiftedLeavesATicksVerdict/flagged`, `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`

## The scanner's section is validated before the start

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the worker's configuration check leaves the scanner section unchecked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheScannerPartIsChecked`, `TestTheWorkerConfigurationIsChecked`, `TestTheWorkerConfigurationIsChecked/a_scanner_threshold_above_1`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`

## The second pass sets its completion flag when it ends

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the second pass ends without setting its completion flag
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** the second pass sets the first pass's completion flag in place of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`

## The second pass waits for the first pass to end

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the second pass starts whether or not the first has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core`:** `TestBegin`, `TestBegin/the_first_pass_has_not_ended`, `TestBegin/the_first_pass_has_not_ended_and_a_run_stopped`
- **Break (2):** the second pass is skipped once the first has ended, and runs only before it has
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core`:** `TestBegin`, `TestBegin/a_run_that_failed`, `TestBegin/no_run_yet`, `TestBegin/the_first_pass_has_not_ended`, `TestBegin/the_first_pass_has_not_ended_and_a_run_stopped`

## The worker answers ready only while each job kind's pool answers

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** backfill's Build starts no readiness report of its pool, so the worker stays ready while that pool cannot answer
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindsPoolMustAnswerForTheWorkerToBeReady`
- **Break (2):** delta sync's Build starts no readiness report of its pool, so the worker stays ready while that pool cannot answer
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindsPoolMustAnswerForTheWorkerToBeReady`

## The worker refuses a configuration it cannot run its job kinds with

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** each job kind's concurrency and the reload interval are not validated
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheWorkerConfigurationIsChecked`, `TestTheWorkerConfigurationIsChecked/a_reload_interval_of_zero`, `TestTheWorkerConfigurationIsChecked/no_backfill_concurrency`, `TestTheWorkerConfigurationIsChecked/no_sync_concurrency`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--backfill.concurrency=0`, `TestATickThatCannotRunRefusesTheStart/--reload_interval=0s`, `TestATickThatCannotRunRefusesTheStart/--sync.concurrency=0`

## The worker serves the health probe

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the health probe is not served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenRuns`

## The worker's spec names the worker and lists both of its job kinds

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the spec lists backfill alone, so delta sync never runs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestADroppedAccountsRunsAreCancelled`, `TestAFailedRecordingFailsTheJobKindsRun`, `TestAFailedRecordingFailsTheJobKindsRun/delta_sync`, `TestAFailedTickStillEndsItsUnitOfWork`, `TestAFailedWriteBackFailsTheJobKindsRun`, `TestAFailedWriteBackFailsTheJobKindsRun/delta_sync`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestAPanicInARunFailsThatRunAlone`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads`, `TestATickRecordsABodyTheConversionRefusesAsRefused`, `TestATickScansABodyHeldApart`, `TestATicksMetadataCallsFitTheCallSize`, `TestAnAccountsSeriesGoWithItsWork`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_an_account`, `TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey/with_no_account`, `TestEachAccountIsServedThroughItsOwnClient`, `TestEachJobKindsPoolMustAnswerForTheWorkerToBeReady`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords/backfill`, `TestEachJobKindsUnitOfWorkWritesBackAndRecords/delta_sync`, `TestEachSessionReloadWakesTheJobKinds`, `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`, `TestEachTickTakesThePolicyInForce`, `TestNoAccountIsListedBeforeItsPolicyLoads`, `TestNoBodyTextReachesTheIndexOrTheLogs`, `TestTheProbesServeEverySeriesBetweenRuns`, `TestTicksHoldTheirTokenUntilTheCredentialIsReplaced`
- **Break (2):** the spec names the program other than the worker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheStartBannerGoesToStandardErrorAndStandardOutputStaysJSON`
