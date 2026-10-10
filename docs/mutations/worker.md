# Mutations: worker

The demonstrations of the controls whose patches sit in `worker/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A backfill run's start masks every stale subject stored unmasked again from the store, a batch at a time, with a masking event for each mask

- **Date · evidence:** 2026-10-08 · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), at RAPID_SEED=1, and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the run-start step writes the subjects it masks again from the store and records no masking event for their masks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (2):** the run-start step masks again only the first batch of subjects stored unmasked, so a mailbox holding more keeps the rest under the earlier scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (3):** the run-start step masks no subject stored unmasked again, so each keeps a mask an earlier scanner decided until a fetch the first pass never makes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (4):** the run-start step writes each subject it masks again with its masks applied and leaves it reading unmasked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`
- **Break (5):** the run-start step masks the subjects stored masked again from their masked text too, so a subject whose masks hid what the new scanner reads is never fetched again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore`

## A body the conversion refuses stays pending, never scanned, with a failed item

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a refusal is ignored, so the body is scanned as empty and recorded clean
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`, `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyTheConversionRefusesStaysPending`
- **Break (2):** a body the conversion refuses is scanned as its raw HTML and recorded scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`, `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyTheConversionRefusesStaysPending`

## A call fetching subjects again that leaves every one of them stale fails the run

- **Date · evidence:** 2026-10-08 · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), at RAPID_SEED=1, and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a call is progress only when it stores a subject, so a stale count another writer lowered during the call fails the run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFallingStaleCountIsProgress`
- **Break (2):** a call that stored its subjects under the pair in force is taken as having moved nothing, so every call fetching subjects again fails the run, a change of scanner that masks nothing included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`
- **Break (3):** any change of what a call recorded is taken as progress, so a stale count another writer raised during a call that stored nothing lets the pass go on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestARisingStaleCountIsNotProgress`
- **Break (4):** a call that leaves every subject it fetched stale is taken as progress, so the pass asks for the same subjects without end
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestARisingStaleCountIsNotProgress`

## A change of scanner reopens a pass whose stored work another scanner decided

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a pass that ended is skipped whether or not it is due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestBegin`, `TestBegin/a_pass_that_ended_and_is_due_again`, `TestBegin/a_pass_that_ended_and_is_due_again_with_a_run_left_running`
- **Break (2):** a reopened first pass runs while it stays recorded as ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (3):** a stored subject whose configuration revision differs and whose version matches does not make the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`
- **Break (4):** a stored subject whose scanner version differs and whose revision matches does not make the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`

## A concurrency limit per job kind bounds its runs at once

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and its break again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break:** a run never takes a slot of its job's limit, so the limit bounds nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobsWaitForASlotIsReported`, `TestALimitBoundsTheRunsOfAJobKindAtOnce`

## A credential the provider refuses is read again from its row before the refusal is reported

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a failed read of the refused credential is dropped and the refusal returned alone
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAFailedReadOfTheRefusedCredentialIsReported`
- **Break (2):** a refused call returns the refusal without reading the credential again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/a_body_fetch`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_call`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`
- **Break (3):** the hand-over reads the source the tick started with, so the refused credential is written over the operator's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/a_body_fetch`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_call`, `TestARotationAfterARereadIsWrittenBack`
- **Break (4):** a refused credential the row still holds is used for a second call
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`

## A credential the provider refuses is read again from the account's row before the refusal is reported, and a replaced one is used for the call it refused

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** an account whose row no longer holds a credential has the call made again with an empty one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`
- **Break (2):** a failed read of the refused credential is dropped, so the run reports the refusal alone
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedReadOfTheRefusedCredentialIsReported`
- **Break (3):** the new source and its re-read's adoption stamp are not made the account's holding, so the hand-over keeps handing over the refused source with the snapshot's stamp, and a rotation of the credential read again is never written back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARotationAfterARereadIsWrittenBack`
- **Break (4):** a refused call is reported at once, so the credential is never read again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`
- **Break (5):** the re-read's adoption stamp is made the account's holding but its new source is not, so the hand-over writes the refused credential over the one the operator stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARotationAfterARereadIsWrittenBack`
- **Break (6):** the comparison is inverted, so a replaced credential is reported as refused and the refused one is used again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`
- **Break (7):** the call is made again over the port the refused credential's source was built on, so the credential read again is never used
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`
- **Break (8):** a refused credential the account's row still holds is made a new source of and the call made again with it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`

## A cursor gap is recovered by re-enumerating from an hour before the last cursor was written, and counted

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the cursor gap rule fires while any gap was ever counted, so it never clears
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAlertingRules`
- **Break (2):** the cursor gap rule reads only the count's increase, so a gap the series' first sample counts never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAlertingRules`
- **Break (3):** a tick that finds a gap does not report it, so the gap series never moves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAFailedRecoveryIsTriedAgainByTheNextTick`, `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`
- **Break (4):** a recovery stores the current cursor before it lists the window, so a recovery that fails is never tried again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAFailedRecoveryIsTriedAgainByTheNextTick`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`
- **Break (5):** a gap's window starts an hour before now, whenever the last cursor was written
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/tick`:** `TestWindow`, `TestWindow/a_cursor_written_five_minutes_ago`, `TestWindow/a_cursor_written_long_ago`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (6):** a gap's window starts at the last cursor's write time, so mail the provider dated just before it is left out
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/tick`:** `TestWindow`, `TestWindow/a_cursor_written_five_minutes_ago`, `TestWindow/a_cursor_written_long_ago`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`

## A failed body fetch is retried, recorded gone or abandoned, or stops the run, by its class

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a message the provider no longer has stops the run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAFailedBodyFetch`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestOnFailure`, `TestOnFailure/gone`
- **Break (2):** a throttled body fetch stops the run at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAFailedBodyFetch`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestOnFailure`, `TestOnFailure/a_first_throttle`

## A failed job is asked again after a capped exponential backoff, which a success resets and a wake waits out

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** a success leaves the count of failures as it was, so the backoff never resets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestASuccessResetsTheBackoff`
- **Break (2):** a wake is answered during a backoff, so a failing job woken often runs at the rate it is woken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAWakeDuringABackoffWaitsForItsEnd`

## A first pass whose enumeration ended fetches again by identifier only the stale subjects, masks each from the subject the provider returns, and enumerates nothing when reopened

- **Date · evidence:** 2026-10-08 · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), at RAPID_SEED=1, and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a subject fetched again is masked from the text the index stores rather than from the subject the provider returns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestRefetch`
- **Break (2):** a pass reopened after its enumeration ended starts a fresh enumeration from the first page instead of fetching the stale subjects again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestAThrottledCallIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestUnder`, `TestUnder/a_pass_reopened_after_its_enumeration_ended`
- **Break (3):** the read of the subjects to fetch again ignores a configuration revision that alone differs, so a subject masked under the earlier revision is never fetched again and the pass ends with it stale
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (4):** the read of the subjects to fetch again ignores a scanner version that alone differs, so a subject masked under the earlier version is never fetched again and the pass ends with it stale
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`

## A gap recovery removes a stored message dated in its window once a complete listing left it out and the provider no longer returns it by its identifier

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a gap recovery treats a stored message dated before its window as left out of its listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (2):** a gap recovery removes nothing it found missing from its listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (3):** a gap recovery removes every stored message its listing left out, the ones the provider still returns by identifier included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/tick`:** `TestRemoval`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`

## A job's last success is a run that succeeded or a unit of work made durable, and its series follow every way a run ends

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (1) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (5) on 2026-10-10 at RAPID_SEED=1, added with the rule that a loop reports on its key only while it still holds it, with break (1) again, whose went-red list grew by that rule's test · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a job's last success is not emitted until its first success, so a job that never succeeds never ages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobsLatestSuccessStartsFromItsRecordedOne`, `TestARemovedLoopReportsNothingOnTheJobEnsuredInItsPlace`, `TestTheJobSeriesFollowEveryWayARunEnds`, `TestTheRulesReadOnlyEmittedSeries`
- **Break (2):** a run that answers it was not due counts as a success
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestANotDueAnswerChangesNeitherFailuresNorSuccess`, `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (3):** a removed job's series stay, reading the time of its last success
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (4):** a unit of work a run makes durable records no success
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAUnitMadeDurableIsASuccess`, `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (5):** a removed loop reports on the key whatever job holds it, so a run that makes a unit durable as it stops sets the latest success of the job ensured in its place
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestARemovedLoopReportsNothingOnTheJobEnsuredInItsPlace`

## A job's latest success starts from what its job kind's runs record

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (7) again on 2026-10-10 at RAPID_SEED=1, whose went-red list grew by the test of a removed loop's reports · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** backfill ensures its jobs without the latest success its runs record, so each starts at its ensure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachJobStartsFromItsRecordedLatestSuccess`
- **Break (2):** the latest recorded success reads only runs that succeeded and no page made durable, so a pass resumed across a restart counts from before its pages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachJobStartsFromItsRecordedLatestSuccess`
- **Break (3):** a backfill job with no work outstanding counts from its latest recorded success, so an account whose backfill ended long ago is late at every start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsSeed`, `TestBackfillsSeed/both_passes_ended`
- **Break (4):** a backfill job with runs and no success counts from its ensure, so a first pass in a crash loop never ages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsSeed`, `TestBackfillsSeed/a_first_pass_with_runs_and_no_success`
- **Break (5):** a delta sync job with ticks and no success counts from its ensure, so ticks failing in a crash loop never age
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/due`:** `TestDeltaSyncsSeed`, `TestDeltaSyncsSeed/ticks_and_no_success`
- **Break (6):** delta sync ensures its jobs without the latest success their ticks record, so each starts at its ensure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickJobStartsFromItsRecordedLatestSuccess`
- **Break (7):** the scheduler starts every job's latest success at its ensure, whatever its job kind's runs record
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobsLatestSuccessStartsFromItsRecordedOne`, `TestARemovedLoopReportsNothingOnTheJobEnsuredInItsPlace`

## A killed backfill run resumes from its last checkpointed page, with at most one page of rework

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a page made durable keeps the token it was asked with, so the next page asked for is the same one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAPageThatFailsToCommitLeavesNothing`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestARefusedTokenTheRunGotFailsTheRun`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestASubjectStoredUnmaskedIsTheProvidersSubject`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheCheckpointCarriesThePagesTheTotalImplies`, `TestTheModelAgreesWithPostgreSQL`, `TestThePassDoesNotEndWhileASubjectIsStale`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestAdvanceAndRestart`
- **Break (2):** a resumed run starts from the first page, dropping the checkpoint and counters of the run it resumes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestBegin`, `TestBegin/a_pass_not_ended_that_is_due_again`, `TestBegin/a_pass_that_ended_and_is_due_again_with_a_run_left_running`, `TestBegin/a_run_that_failed`, `TestBegin/a_run_that_stopped_without_recording_its_end`
- **Break (3):** a resumed run counts one page more than its checkpoint, so it reports a page it never made durable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
- **Break (4):** a run resuming a pass whose enumeration has ended asks for the first page again rather than fetching the stale subjects and finishing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestARisingStaleCountIsNotProgress`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestThePassDoesNotEndWhileASubjectIsStale`

## A killed second pass resumes from its last checkpointed page

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and break (2) again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after the crash test gained a sequence run against PostgreSQL in every run that resumes after a page that left a message waiting · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** the stored checkpoint loses the last message read, so a resumed run reads from the first waiting message
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** a resumed run counts one page more than its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAnAddedRuleRestrictsTheStoredClasses`, `TestTheTransitionMarksNothingTwice`

## A message the gate skipped whose subject is now masked returns to pending

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates break (1) against the statements it adds beside the one it breaks and demonstrates it again with the same tests red, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** every message the gate skipped returns to pending, whether or not its subject is masked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAGateSkipWhoseSubjectIsMaskedReturnsToPendingAtTheSecondPassesStart`
- **Break (2):** a gate skip decided without the subject's signal the message now carries stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAGateSkipWhoseSubjectIsMaskedReturnsToPendingAtTheSecondPassesStart`

## A newly connected account is backfilled within one reload with no manual step, and the run-start step is made once for each job's life

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** the reload adds jobs only at its first run, so an account connected later is never backfilled
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestANewAccountIsBackfilledWithNoManualStep`, `TestNoAccountIsAddedWhileThePolicyReloadFails`
- **Break (2):** the run-start step is never recorded as made, so it is made again at every ask of the job, the reconciliation ask included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestTheRunStartStepIsNotMadeAgainAtAJobsLaterAsks`
- **Break (3):** the run-start step is recorded as made before it is made, so a step that fails is not made again until the process restarts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedRunStartStepIsMadeAgainAtTheNextAsk`
- **Break (4):** the run-start step is made once for each account in the process rather than each job's life, so an account listed again never gets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestANewAccountIsBackfilledWithNoManualStep`
- **Break (5):** the run-start step is made once for the job kind's life rather than each job's, so an account served again after another account's step never gets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestANewAccountIsBackfilledWithNoManualStep`

## A page's rows, masking events, sender statistics and checkpoint become durable together or not at all

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a checkpoint that fails to record is ignored, so the page's rows commit without it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAPageThatFailsToCommitLeavesNothing`
- **Break (2):** the messages, masking events and sender statistics commit in a transaction of their own before the one recording the checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageThatFailsToCommitLeavesNothing`

## A panic in a job's run is recovered and recorded as the run's failure with its stack

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** a panic raised in a page of the first pass is recovered and returned, and the run is never recorded as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAPanicInARunIsRecordedAsTheRunsFailure`
- **Break (2):** a panic raised in a page of the first pass is not recovered, so it ends the process and the run's record says nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAPanicInARunIsRecordedAsTheRunsFailure`
- **Break (3):** a panic raised in a page of the second pass is recovered and returned, and the run is never recorded as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAPanicInTheSecondPassIsRecordedAsItsRunsFailure`
- **Break (4):** a panic raised in a page of the second pass is not recovered, so it ends the process and the run's record says nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAPanicInTheSecondPassIsRecordedAsItsRunsFailure`
- **Break (5):** a panic raised in a tick is recovered and returned, and the tick is never recorded as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAPanicInATickIsRecordedAsTheTicksFailure`
- **Break (6):** a panic raised in a tick is not recovered, so it ends the process and the tick's record says nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAPanicInATickIsRecordedAsTheTicksFailure`
- **Break (7):** a panic raised in the comparisons the second pass makes at its start is not recovered, so it ends the process and the recorded run stays running with no error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAPanicAtTheSecondPassesStartIsRecordedAsItsRunsFailure`

## A panic in one job is its run's failure and stops nothing else

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** schedule.Go starts its goroutine with no recovery, so a panic in a goroutine a run started stops the worker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAPanicIsItsRunsFailureAndStopsNothingElse`, `TestAPanicIsItsRunsFailureAndStopsNothingElse/in_a_goroutine_it_started`
- **Break (2):** a run is called with no recovery, so its panic stops the worker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAPanicIsItsRunsFailureAndStopsNothingElse`, `TestAPanicIsItsRunsFailureAndStopsNothingElse/in_the_run`, `TestTheJobSeriesFollowEveryWayARunEnds`

## A policy edit reaches a running second pass within one reload and one page

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** every page makes the comparisons again whatever the policy, so a reload with the same rules rewrites the stored classes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAPolicyEditReachesARunningSecondPassWithinAPage`
- **Break (2):** a page never makes the comparisons again, so an edit reaches the stored classes only at the next run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAPolicyEditReachesARunningSecondPassWithinAPage`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`

## A reload re-seals what was sealed to an old key, an OAuth client's secret included, whether or not any account is listed

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** the re-seal and the key-scan series move from the reload to each account's tick, so with no account listed nothing is re-sealed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAReloadReSealsWhatItOpensWithAnOldKey`, `TestAReloadWithNoAccountReSealsAClientSecretOnAnOldKey`
- **Break (2):** a reload that lists no account re-seals nothing and sets no key-scan series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAReloadWithNoAccountReSealsAClientSecretOnAnOldKey`

## A removed rule returns its sender's messages to pending scan, and the pass starts over

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** the comparison delists the domains the policy still restricts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestAStoredDomainClassifiesAsItsAddress`, `TestDelisted`, `TestDelisted/a_policy_that_never_loaded`, `TestDelisted/every_rule_in_place`, `TestDelisted/every_rule_removed`, `TestDelisted/the_bank's_rule_removed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestTheTransitionMarksNothingTwice`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickScansADelistedSendersMessages`
- **Break (2):** a run that marked messages keeps its checkpoint, so marked messages before it wait until the pass ends
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestTheTransitionMarksNothingTwice`
- **Break (3):** the transition marks no domain
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`, `TestTheTransitionMarksNothingTwice`
- **Break (4):** the comparison reads only the domains stored restricted, so a message skipped as restricted under a rule added after it was stored is never found
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`

## A reopened first pass always leads to a run of the second pass

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates breaks (1) and (2) against the run-start step that also returns overturned gate skips and demonstrates the row again, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the run-start step records the first pass as not ended in place of the second
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (2):** the run-start step reopens the second pass only when it returned a verdict or a skip to pending or masked a subject again, not when a subject stored masked leaves the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAReopenedFirstPassLeadsToASecondPass`

## A rotated refresh token is handed to the account snapshot library after every page, a page that failed included, and a failed write-back ends the pass in an error

- **Date · evidence:** 2026-10-02 · [pull request #254](https://github.com/ppat/mediated-mailbox-mcp/pull/254), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a page that fails ends the pass before the hand-over, so a rotation made before the failure is lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedPageIsStillHandedOver`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEverySeriesTheKindEmitsCarriesItsJobKind`
- **Break (2):** the account's token is handed over only once the pass ends, not after every page
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`, `TestEachPageIsAUnitOfWork`
- **Break (3):** a hand-over that fails is dropped, so the pass ends without an error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`
- **Break (4):** the hand-over hands over the source the run started with, so a source a re-read replaced is never handed over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARotationAfterARereadIsWrittenBack`
- **Break (5):** no refresh token is handed over, so a rotation never reaches the database
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedWriteBackEndsTheRunInError`, `TestARotationAfterARereadIsWrittenBack`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestTheSourcesRefreshTokenIsHandedOver`
- **Break (6):** the run never ends a unit of work over an account's session, so no source's token reaches the database
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`, `TestAFailedHandOverFailsTheSecondPassAtItsEnd`, `TestAFailedPageIsStillHandedOver`, `TestAFailedWriteBackEndsTheRunInError`, `TestARotationAfterARereadIsWrittenBack`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEachPageIsAUnitOfWork`, `TestEachSecondPassPageIsAUnitOfWork`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (7):** a failed write-back is dropped, so the run ends as if the rotation had landed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedWriteBackEndsTheRunInError`

## A run resuming an enumeration not yet ended that was made under another scanner starts it over from the first page

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a run resuming an enumeration not yet ended starts it over whatever scanner it was made under
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassStartedOverCountsNothingTwice`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestUnder`, `TestUnder/a_run_resumed_under_the_same_scanner`
- **Break (2):** a run starting an enumeration over under another scanner keeps the page count of the enumeration it abandoned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestUnder`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`
- **Break (3):** a run resumes an enumeration not yet ended that was made under another scanner from its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestUnder`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`

## A run serves only a connected account whose provider it has an adapter for and that connects through an OAuth client

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the connected accounts are skipped and the ones that are not connected are served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestADroppedAccountsRunIsCancelled`, `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestAFailedSnapshotReadKeepsThePreviousSnapshot`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestAPanicInARunIsRecordedAsTheRunsFailure`, `TestAPanicInTheSecondPassIsRecordedAsItsRunsFailure`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestARunTakesItsAccountsFromTheDatabase`, `TestAnAccountWithoutAClientIsSkipped`, `TestEachPageTakesTheActivePolicy`, `TestEverySeriesTheKindEmitsCarriesItsJobKind`, `TestNoAccountIsAddedWhileThePolicyReloadFails`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (2):** an account of a provider backfill has no adapter for is served through the Gmail adapter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARunTakesItsAccountsFromTheDatabase`
- **Break (3):** an account that is not connected is served with an empty refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestADroppedAccountsRunIsCancelled`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestARunTakesItsAccountsFromTheDatabase`, `TestAnAccountWithoutAClientIsSkipped`, `TestNoAccountIsAddedWhileThePolicyReloadFails`

## A scan verdict records its content flags, content rules, scanner version and configuration revision

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** a one-time code is stored as a login link
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestBothPartsOfABodyAreScanned`
- **Break (2):** a verdict is stored without the configuration revision it was made under
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestAnAddedRuleRestrictsTheStoredClasses`

## A second pass resuming a stopped pass while the run-start step's mark is set starts over from the first waiting message

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates break (3) against the comment it changes beside the statement it breaks and demonstrates it again with the tests it adds red too, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** a second pass run resumes a stopped pass from its checkpoint whatever the mark, so what a run's start returned to pending before it waits for good
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestOver`, `TestOver/a_resumed_run_with_the_mark`
- **Break (2):** the second pass run that starts leaves the mark set, so every later run resuming the pass starts it over again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (3):** the run-start step reopens the second pass without marking it to start over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`

## A sender's first scan hit reaches the gate's decision on its next message, and no other sender's

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the gate reads only the prior hits stored before the page, so a hit earlier on the page does not count
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestGate`, `TestGate/a_hit_earlier_on_the_page`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`
- **Break (2):** a hit on the page counts as a prior hit of every sender on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestASendersFirstHitReachesItsNextMessage`

## A served account's token source is built from the OAuth client the account connects through and the account's own refresh token

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the client's secret is passed as its identifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (2):** every account is served with the OAuth client of the snapshot's first account rather than its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (3):** the client's identifier is passed as its secret
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (4):** the client's secret is passed as the account's refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedWriteBackEndsTheRunInError`, `TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestARunTakesItsAccountsFromTheDatabase`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheSourcesRefreshTokenIsHandedOver`
- **Break (5):** backfill's re-read after a refusal keeps the client the source was built with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`
- **Break (6):** delta sync passes the client's secret as its identifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`
- **Break (7):** delta sync serves every account with the OAuth client of the snapshot's first account rather than its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`
- **Break (8):** delta sync passes the client's identifier as its secret
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`
- **Break (9):** delta sync passes the client's secret as the account's refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/a_body_fetch`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_call`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`, `TestATickTakesItsAccountsFromTheDatabase`, `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`, `TestEachTickHandsItsAccountOverAndRecordsItsAttempt`
- **Break (10):** delta sync's re-read after a refusal keeps the client the source was built with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`

## A stale subject is masked again under the scanner in force, each mask recorded under it

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a masking event is recorded with no scanner version or revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheModelAgreesWithPostgreSQL`
- **Break (2):** the first pass masks a stored subject again only when its scanner version differs, so a changed configuration revision leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (3):** the first pass masks a stored subject again only when its configuration revision differs, so a changed scanner version leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`

## A stopped worker and a job whose last success has aged are loud

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** the absence rule fires at the first evaluation that finds no worker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAlertingRules`
- **Break (2):** the rule on a job's last success compares it with a fixed fifteen minutes, not the bound the job exports
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAlertingRules`

## A stored gate skip the gate no longer decides as the same skip returns to pending at backfill's run-start step

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on and renamed the control from "A stored gate skip the gate no longer decides as the same skip returns to pending at every backfill run's start" · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a stored gate skip is decided again under the default thresholds rather than the ones the run holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/a_widened_high-volume_mark`, `TestOverturned/the_thresholds_the_skips_were_made_under`
- **Break (2):** every stored gate skip is overturned, the ones the gate still decides as the same skip included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/a_widened_high-volume_mark`, `TestOverturned/the_thresholds_the_skips_were_made_under`
- **Break (3):** a stored gate skip is overturned only when the gate would now scan it, so a skip under thresholds that cannot decide or of a sender now restricted stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/thresholds_that_cannot_decide`
- **Break (4):** the run-start step reads the gate skips and returns none to pending, so every skip made under earlier thresholds stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (5):** the read of the stored gate skips drops the sender's prior hits, so a skip whose sender has since gained a hit stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestASkipWhoseSenderGainedAPriorHitIsScanned`

## A stored subject whose message the provider no longer has is masked whole under the scanner in force

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a subject whose message the provider left out of its answer keeps its stored text under the scanner in force instead of being masked whole
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestRefetch`, `TestUnfound`
- **Break (2):** a call fetching subjects again masks no subject whose message the provider left out of its answer, leaving it masked under the earlier scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`

## A tick applies each change set in the transaction that advances the cursor past it

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and breaks 2, 3 and 4 again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322), and breaks 2, 3 and 4 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the cursor is stored in a transaction of its own before the change set's writes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (2):** a change set's application stores no cursor, so the next tick asks for the same changes again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (3):** a stored message's labels and flags are left as they were when the provider reports them changed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (4):** a message the provider no longer holds stays in the index
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestATickAppliesTheChangesSinceItsCursor`

## A tick hands its account over and records its latest authentication attempt when it ends

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on and moved its breaks (3) and (4), on the Gmail adapter's series, to the row "Every series a job kind emits carries its job kind" · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** no tick records its source's authentication attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickHandsItsAccountOverAndRecordsItsAttempt`
- **Break (2):** a tick that fails hands nothing over and records no attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickHandsItsAccountOverAndRecordsItsAttempt`

## A tick records as failed the account's tick and gap recovery a stopped process left running

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and break 2 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and breaks 1 and 3 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a recovery's start also records the tick that started it, still running, as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`
- **Break (2):** a tick's start records a stopped tick as failed and leaves a stopped recovery recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`
- **Break (3):** a tick's start leaves every run a stopped process left running recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`, `TestATickRecordsAStoppedRunAsFailed/a_tick_stopped_part_way`

## A tick spends from the account's budget in the sync class

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a tick's calls are leased in the batch class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickSpendsInTheSyncClass`
- **Break (2):** a tick's calls are leased in the interactive class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickSpendsInTheSyncClass`

## A timed job is asked on its interval's phase, and a job with no interval only when woken

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** a job with no interval is asked again at once after every run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobWithNoIntervalIsAskedOnlyWhenWoken`, `TestAUnitMadeDurableIsASuccess`
- **Break (2):** a run moves the phase to its own start, so after a run that outlasts the interval the phase never resumes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestATimedJobIsAskedOnTheIntervalsPhase`
- **Break (3):** an ask carries its own time as its time on the ticker's phase, so an ask at the end of a backoff or at a wake names a time off the phase
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestEachAskCarriesItsTimeOnThePhase`

## A verdict made under another scanner returns to pending, cleared, with its sender's prior hits counted again

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and break (1) again on 2026-10-02 with its patch regenerated for the same move, turning red `TestAChangeOfScannerMasksAndScansAgain`, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (1) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** backfill's run-start step returns no verdict to pending before the first pass, so stale verdicts keep releasing bodies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAFailedRunStartStepIsMadeAgainAtTheNextAsk`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (2):** backfill's run-start step returns stale verdicts to pending only after the first pass ends, so they keep releasing bodies while it runs or fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`
- **Break (3):** the prior hits of a sender whose verdicts returned to pending are not counted again, so a message flagged again counts twice
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (4):** a verdict whose configuration revision differs and whose version matches stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestADroppedAccountsRunIsCancelled`, `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestAPanicInARunIsRecordedAsTheRunsFailure`, `TestAPanicInTheSecondPassIsRecordedAsItsRunsFailure`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestARunTakesItsAccountsFromTheDatabase`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`, `TestEachPageTakesTheActivePolicy`, `TestEverySeriesTheKindEmitsCarriesItsJobKind`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheRunsHandOverReturnsAFailedRecording`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (5):** a verdict whose scanner version differs and whose revision matches stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
- **Break (6):** a verdict returned to pending keeps its content flags, content rules, scan time, version and revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (7):** a backfill run leaves a verdict made under another scanner in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`

## An account no longer served has its job removed, and its running run cancelled

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (1) again on 2026-10-10 at RAPID_SEED=1, with its patch regenerated after the code around it moved · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the reload never removes the job of an account it no longer lists, so its run keeps running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestADroppedAccountsRunIsCancelled`, `TestANewAccountIsBackfilledWithNoManualStep`, `TestNoAccountIsAddedWhileThePolicyReloadFails`
- **Break (2):** the reload never removes the job of an account it no longer lists, so its tick keeps running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestADroppedAccountsTickIsCancelled`, `TestEachReloadLoadsTheAccountSnapshot`, `TestNoAccountIsAddedWhileThePolicyReloadFails`

## An account with no cursor is reconciled over the first window inside its tick

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** an account's first reconciliation is reported as a cursor gap, so the gap series and its alert count a cursor that was never lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (2):** an account with no cursor takes the current cursor and lists only what is dated after it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickAppliesTheChangesSinceItsCursor`, `TestATickDecidesABoundedNumberAndTheNextGoesOn`, `TestATickRestrictsTheStoredClassOfAnAddedRulesSender`, `TestATickScansADelistedSendersMessages`, `TestATickSpendsInTheSyncClass`, `TestBeforeTheSecondPassHasEndedATickScansNothing`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`

## An account's series go once the work that sets them ends

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** the second pass's backlog series stays once the pass ends, reading its last value beside delta sync's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`, `TestTheKindsSeriesOfAnAccountGoWhenItsWorkEnds`
- **Break (2):** backfill's unclassified series of an account stays once its job is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestTheKindsSeriesOfAnAccountGoWhenItsWorkEnds`
- **Break (3):** backfill's backlog series of an account stays once its job is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestTheBacklogSeriesOfADroppedAccountGoesWithItsJob`
- **Break (4):** delta sync's tick series of an account stay once its job is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestADroppedAccountsTickSeriesGoWithItsJob`

## An account's tick runs only once its sync interval has passed since the last

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and breaks (1), (2) and (3) again on 2026-10-10 at RAPID_SEED=1, with their patches regenerated after the code around them moved · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** an account's first tick in the process is never due
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/due`:** `TestDeltaSyncsDueDecision`, `TestDeltaSyncsDueDecision/no_tick_yet_in_this_process`
- **Break (2):** a tick asked exactly one interval after the last is not due
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/due`:** `TestDeltaSyncsDueDecision`, `TestDeltaSyncsDueDecision/asked_one_interval_after_the_last_tick`
- **Break (3):** a tick is due whenever it is asked, before its interval has passed included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/due`:** `TestDeltaSyncsDueDecision`, `TestDeltaSyncsDueDecision/asked_a_moment_before_the_interval_has_passed`, `TestDeltaSyncsDueDecision/asked_at_once_again`, `TestDeltaSyncsDueDecision/asked_at_the_end_of_a_backoff_after_a_failed_tick`
- **Break (4):** an account's job ticks whenever it is asked, the end of a backoff included, whatever its due decision says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAJobAskedBeforeItsIntervalDoesNotTick`
- **Break (5):** a tick is recorded at the time it was asked rather than at its time on the ticker's phase, so after a tick at the end of a backoff the ticker's next tick reads not due and the account waits one interval more
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestATickAtTheEndOfABackoffKeepsThePhase`

## An overturned gate skip reopens the second pass, marked to start over

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the run-start step reopens the second pass only for a stale verdict, a subject it masked again or a stale subject, so a skip it returned to pending after the pass ended waits for good
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`
- **Break (2):** the run-start step reopens the second pass for overturned skips without marking it to start over, so a stopped pass resumes past the skips returned before its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`

## Applying the same changes twice leaves the index as applying them once

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** applying a message's labels adds them to the ones stored rather than setting them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnce`
- **Break (2):** a message the index already holds has its masks recorded again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnce`

## Backfill's due decision asks for the run-start step once for each job's life and for the passes while either has work

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and breaks (2), (3) and (4) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests they rest on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a first pass that has not ended is not due once the run-start step is made
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_first_pass_not_ended`
- **Break (2):** the passes are due only at the job's first run, whatever recorded state says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_first_pass_not_ended`, `TestBackfillsDueDecision/the_second_pass_marked_to_start_over`, `TestBackfillsDueDecision/the_second_pass_not_ended`, `TestBackfillsSeed`, `TestBackfillsSeed/a_first_pass_with_runs_and_no_success`, `TestBackfillsSeed/a_pass_not_ended_with_a_success_recorded`, `TestBackfillsSeed/the_second_pass_marked_to_start_over`
- **Break (3):** a second pass marked to start over is not due
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_second_pass_marked_to_start_over`, `TestBackfillsSeed`, `TestBackfillsSeed/the_second_pass_marked_to_start_over`
- **Break (4):** a second pass that has not ended is not due once the run-start step is made
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/the_second_pass_not_ended`, `TestBackfillsSeed`, `TestBackfillsSeed/a_pass_not_ended_with_a_success_recorded`
- **Break (5):** the run-start step is asked for at every run, not once for the job's life
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/both_passes_ended_and_started`, `TestBackfillsDueDecision/the_first_pass_not_ended`, `TestBackfillsDueDecision/the_second_pass_marked_to_start_over`, `TestBackfillsDueDecision/the_second_pass_not_ended`
- **Break (6):** the run-start step is never asked for, so a job never returns stale work to pending
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due`:** `TestBackfillsDueDecision`, `TestBackfillsDueDecision/a_new_account`, `TestBackfillsDueDecision/both_passes_ended,_not_yet_started_in_this_process`

## Backfill's second pass never asks for a restricted sender's body

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** the gate reads every sender as normal, so a restricted sender's body is scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestGate`, `TestGate/a_sender_the_policy_lists`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestAPolicyEditReachesARunningSecondPassWithinAPage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** every message is scanned whatever the gate decided, a restricted sender's included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestAPolicyEditReachesARunningSecondPassWithinAPage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage`

## Both parts of a body are scanned, and a flag in either flags the message

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** the message's flags are the HTML part's alone, though both parts are scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestScan`, `TestScan/a_body_with_no_HTML_part`, `TestScan/a_code_in_one_part_and_a_link_in_the_other`, `TestScan/a_code_in_the_text_part`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestASendersFirstHitReachesItsNextMessage`, `TestBothPartsOfABodyAreScanned`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** the text part is not scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestScan`, `TestScan/a_body_with_no_HTML_part`, `TestScan/a_code_in_one_part_and_a_link_in_the_other`, `TestScan/a_code_in_the_text_part`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestASendersFirstHitReachesItsNextMessage`, `TestBothPartsOfABodyAreScanned`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`

## Delta sync decides under backfill's gate thresholds and records backfill's scanner version and revision

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the worker hands delta sync a scanner built from a section of its own, so its verdicts and masks record another revision than backfill's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestBothJobKindsScanWithTheOneScanner`
- **Break (2):** backfill's second pass decides under thresholds of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARunDecidesUnderTheSharedThresholdsAndScanner`
- **Break (3):** delta sync's gate decides under thresholds of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestATickDecidesUnderBackfillsThresholdsAndScanner`

## Delta sync re-seals what it opens with an old key and scans every listed account and OAuth client

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the re-seal of a client secret writes nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAReloadReSealsWhatItOpensWithAnOldKey`, `TestAReloadWithNoAccountReSealsAClientSecretOnAnOldKey`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`
- **Break (2):** a client secret whose re-seal could not be written reads as done
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAReloadReSealsWhatItOpensWithAnOldKey`

## Delta sync refuses a configuration it cannot tick with

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the interval is refused when it is positive and taken when it is not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--sync.concurrency=0`, `TestATickThatCannotRunRefusesTheStart/--sync.decisions_per_tick=0`, `TestATickThatCannotRunRefusesTheStart/--sync.first_window=-1h`, `TestATickThatCannotRunRefusesTheStart/--sync.sync_interval=0s`, `TestAnEmptyListOfPrivateKeysRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/info`, `TestTheConfiguredLevelGovernsTheLog/no_level`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheEffectiveConfigurationIsLogged`, `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_first_of_two`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_only_private_key`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`
- **Break (2):** the interval, the first window and the bound on decisions are not validated
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--sync.decisions_per_tick=0`, `TestATickThatCannotRunRefusesTheStart/--sync.first_window=-1h`, `TestATickThatCannotRunRefusesTheStart/--sync.sync_interval=0s`

## Delta sync runs until stopped and serves every series between ticks

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (5) again on 2026-10-10 at RAPID_SEED=1, with its patch regenerated after the code around it moved · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a series whose account or client a later scan no longer holds stays reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/reseal`:** `TestTheSeriesFollowTheLastScan`
- **Break (2):** the worker hands delta sync a registry the metrics endpoint does not serve, so none of its series reach the scrape
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenTicks`
- **Break (3):** the rate limiter's series are registered on a registry the metrics endpoint does not serve
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenTicks`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEverySeriesTheKindEmitsCarriesItsJobKind`
- **Break (4):** a reload sets none of the key-scan series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenTicks`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestAReloadReSealsWhatItOpensWithAnOldKey`, `TestAReloadWithNoAccountReSealsAClientSecretOnAnOldKey`, `TestEverySeriesTheKindEmitsCarriesItsJobKind`
- **Break (5):** an account's gap series appears only at its first gap, so the rule's increase cannot see that gap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenTicks`

## Delta sync's re-seal of a client secret never puts back a value someone else replaced

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a re-seal that lost to a replaced secret keeps the secret it held instead of reading the stored one again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`
- **Break (2):** the re-seal's write replaces the stored secret whatever bytes it holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`

## Each flagged message adds one prior hit to its sender, durably

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and break (2) again on 2026-10-02 with its patch regenerated for the statistics' writes moved to `db/senders/statistics`, turning red the same tests, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** every scanned message counts as a hit, a clean one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestAdvance`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** no prior hit is added to the senders' statistics
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`

## Each job kind connects as its own role, through a pool of its own

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (2) again on 2026-10-10 at RAPID_SEED=1, with its patch swapping both roles as its description states · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the composition hands backfill delta sync's pool and delta sync backfill's, so each kind runs its statements under the other's role
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindIsHandedThePoolOfItsOwnRole`
- **Break (2):** the start opens backfill's pool as delta sync's role and delta sync's as backfill's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindIsHandedThePoolOfItsOwnRole`, `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_first_of_two`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_only_private_key`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`

## Each job kind's constructor takes only what the job kind needs

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** backfill's configuration gains delta sync's pool as a field
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindsConstructorTakesOnlyWhatItNeeds`
- **Break (2):** the jobs backfill adds and drops are reached through an interface that also stops the scheduler
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindsConstructorTakesOnlyWhatItNeeds`
- **Break (3):** delta sync's configuration gains backfill's pool as a field
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindsConstructorTakesOnlyWhatItNeeds`
- **Break (4):** the jobs delta sync adds and drops are reached through an interface that also stops the scheduler
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestEachJobKindsConstructorTakesOnlyWhatItNeeds`

## Each page of a run takes the active policy

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** the first pass takes the account's policy once at its start, so a policy edit reaches the pages it stores only at its next run, while the second pass still takes it at each page
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachPageOfTheFirstPassTakesTheActivePolicy`
- **Break (2):** the first pass's composition hands it the account's policy taken once at the run's start, so a policy edit reaches the pages it stores only at the next run, while the second pass is still handed the active one at each page
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachPageOfTheFirstPassTakesTheActivePolicy`
- **Break (3):** a run takes the account's policy once at its start, so a policy edit reaches it only at its next run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachPageOfTheFirstPassTakesTheActivePolicy`, `TestEachPageTakesTheActivePolicy`

## Each reload loads the account snapshot, and a failed read keeps the previous one

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker moved the account snapshot's load into each job kind's reload, which changed the code, tests and patches the row rests on and renamed the control from "A run takes its account snapshot once at the start, and a failed read stops it" · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the policy is loaded only for the accounts the kind serves rather than every listed one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARunTakesItsAccountsFromTheDatabase`
- **Break (2):** a reload whose read fails is ignored, so it goes on with the previous snapshot as if it were fresh and reports nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedSnapshotReadKeepsThePreviousSnapshot`

## Each reload loads the account snapshot, and a tick takes the active one

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker moved the account snapshot's load into each job kind's reload, which changed the code, tests and patches the row rests on and renamed the control from "Each tick takes the account snapshot again" · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (2) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** an account's job ticks the snapshot of the reload that added it, so a credential stored since is never ticked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestATickTakesTheActiveAccountSnapshot`
- **Break (2):** only the first reload loads the account snapshot, so an account connected afterwards is never ticked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestADroppedAccountsTickIsCancelled`, `TestADroppedAccountsTickSeriesGoWithItsJob`, `TestATickTakesTheActiveAccountSnapshot`, `TestEachReloadLoadsTheAccountSnapshot`, `TestNoAccountIsAddedWhileThePolicyReloadFails`
- **Break (3):** each tick loads the account snapshot itself rather than taking the one the last reload loaded, so the tick of an account disconnected since that reload fails on the account it no longer finds, rather than ticking until the next reload drops its job, and the warning for each account the load skips is written at every tick as well as every reload
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestATickTakesItsAccountsFromTheDatabase`, `TestEachReloadLoadsTheAccountSnapshot`

## Every backfill run is recorded with its checkpoint, counters, timeline and failed items

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and breaks 5 and 8 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a failure that is not the provider's is abandoned as the page's failure, recorded with no error class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAFailureNotTheProvidersFailsTheRunWithNoFailedPage`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestOnFailure`, `TestOnFailure/a_failure_that_is_not_the_provider's`, `TestOnFailure/an_attempt_nobody_described`
- **Break (2):** a throttled or failed page is asked for again whatever the attempts, so the run never fails on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestAThrottledCallIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestOnFailure`, `TestOnFailure/a_provider_failure_on_the_last_attempt`, `TestOnFailure/a_throttle_on_the_last_attempt`
- **Break (3):** a page the provider throttles fails the run on its first attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestOnFailure`, `TestOnFailure/a_first_throttle`, `TestOnFailure/a_throttle_before_the_last_attempt`, `TestOnFailure/a_throttle_on_a_resumed_token`
- **Break (4):** a page the run fails on after its last attempt records no failed item
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestARefusedCredentialFailsTheRunAtOnce`, `TestARefusedTokenTheRunGotFailsTheRun`
- **Break (5):** a page made durable records its checkpoint and no progress event
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunIsResumedByTheNextRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestARefusedTokenTheRunGotFailsTheRun`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`
- **Break (6):** a page is recorded as recovered in a transaction of its own as soon as it arrives, before its commit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAPageIsRecordedRecoveredOnlyOnceDurable`
- **Break (7):** a page that failed and then became durable records no failed item
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAThrottledPageIsRetriedAndRecorded`
- **Break (8):** a run resumed after it stopped without recording its end is left recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheModelAgreesWithPostgreSQL`

## Every scan gate decision over stored messages is recorded with its reason

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** a scan is recorded as a skip and a skip as a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyTheConversionRefusesStaysPending`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** a skip is not recorded in the decisions table
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`

## Every series a job kind emits carries its job kind

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, with breaks (3) and (4) moved from the row "Delta sync runs until stopped and serves every series between ticks", where they were breaks (3) and (4), demonstrated on 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and again on 2026-10-07 · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), on 2026-10-08 · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), on 2026-10-08 · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323) and on 2026-10-09 · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** each job kind is handed the process registry unlabelled, so its series carry no job kind and the two job kinds register one library's series under one name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenTicks`, `TestTheProbesServeTheRunsSeries`
- **Break (2):** backfill's Gmail adapter counts its requests on series registered on a registry the kind was not given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEverySeriesTheKindEmitsCarriesItsJobKind`
- **Break (3):** every Gmail adapter after the first counts on series of its own, so only the first adapter's requests reach the kind's registry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEverySeriesTheKindEmitsCarriesItsJobKind`
- **Break (4):** delta sync's Gmail adapter counts its requests on series registered on a registry the kind was not given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEverySeriesTheKindEmitsCarriesItsJobKind`

## Every subject is masked and every sender classified before the message reaches the index

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** every message reaches the index with its sender classified normal
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestAStoredDomainClassifiesAsItsAddress`, `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (2):** a masked subject carries no mask, so no masking event is recorded for it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (3):** a message reaches the index with its subject as the provider gave it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`

## No account is added while the policy reload fails, and a dropped account's job is still removed

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (1) again on 2026-10-10 at RAPID_SEED=1, with its patch regenerated after the code around it moved · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a reload whose policy load fails adds the jobs of the accounts newly listed and reports no error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestNoAccountIsAddedWhileThePolicyReloadFails`
- **Break (2):** a reload whose policy load fails returns before it removes the jobs of the accounts no longer listed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestNoAccountIsAddedWhileThePolicyReloadFails`
- **Break (3):** a reload whose policy load fails adds the jobs of the accounts newly listed and reports no error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestNoAccountIsAddedWhileThePolicyReloadFails`
- **Break (4):** a reload whose policy load fails returns before it removes the jobs of the accounts no longer listed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestNoAccountIsAddedWhileThePolicyReloadFails`

## No body text reaches the index or the workload's logs

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a scan records the body's text part among its content rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestNoBodyTextReachesTheIndexOrTheLogs`
- **Break (2):** a refused body's failed item records the start of the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestNoBodyTextReachesTheIndexOrTheLogs`

## No dependency added for a job imports unsafe or holds assembly, and the worker builds without cgo

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and renamed from "No dependency added for a job imports unsafe, and the worker builds without cgo" on 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (1) on 2026-10-10 at RAPID_SEED=1 with a patch that adds assembly to a package of the worker, in place of one adding a dependency whose red came through another package's import of unsafe. With that patch applied and the test's assembly clause removed, the test passes, shown by hand · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a package of the worker holds assembly and imports no unsafe, so code outside memory safety joins a job kind's build
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestNoDependencyAddedForAJobImportsUnsafe`, `TestNoDependencyAddedForAJobImportsUnsafe/amd64`, `TestNoDependencyAddedForAJobImportsUnsafe/arm64`
- **Break (2):** the worker's entry package imports a dependency that imports unsafe, which no job kind used before
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestNoDependencyAddedForAJobImportsUnsafe`, `TestNoDependencyAddedForAJobImportsUnsafe/amd64`, `TestNoDependencyAddedForAJobImportsUnsafe/arm64`
- **Break (3):** the worker's image builds its binary with cgo switched on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheWorkerBuildsWithoutCgo`

## Once backfill's second pass has ended, each tick decides and scans what waits, a bounded number from where the last stopped

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and break 10 again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** delta sync sets an account's backlog series before backfill's second pass has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (2):** a tick's backlog never reaches delta sync's backlog series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (3):** a tick reports no backlog after its scanning
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (4):** a tick decides every waiting message whatever its bound
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickDecidesABoundedNumberAndTheNextGoesOn`
- **Break (5):** a tick stopped by a throttle keeps its checkpoint past the rest of its read, so the next tick passes the throttled message over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`
- **Break (6):** a tick stopped by a throttle goes on deciding the rest of its read without the hits of the bodies it did not scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`
- **Break (7):** a tick never compares the stored restricted senders with its policy, so a delisted sender's messages stay skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickScansADelistedSendersMessages`
- **Break (8):** a tick never scans, so what waits stays waiting once backfill has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickDecidesABoundedNumberAndTheNextGoesOn`, `TestATickRestrictsTheStoredClassOfAnAddedRulesSender`, `TestATickScansADelistedSendersMessages`, `TestATickSpendsInTheSyncClass`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (9):** each tick reads from the first waiting message, forgetting where the last stopped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickDecidesABoundedNumberAndTheNextGoesOn`
- **Break (10):** a tick scans whether or not backfill's second pass has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestBeforeTheSecondPassHasEndedATickScansNothing`
- **Break (11):** a throttled body fetch leaves its message waiting and the tick goes on fetching
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`

## One run per job, with wakes coalesced, held in the process

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (3) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a job ensured again while its removal waits starts its loop at once, beside the removed loop still ending its run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobEnsuredWhileItIsRemovedNeverRunsTwiceAtOnce`, `TestEnsuringAndRemovingAJobRapidlyLeavesNoLoopRunning`
- **Break (2):** removing a job neither cancels nor waits for its loop, so the loop keeps asking the job after its removal
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestEnsuringAndRemovingAJobRapidlyLeavesNoLoopRunning`, `TestRemoveAndStopCancelTheirRunsAndWaitForThem`, `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (3):** the loop starts each run on a goroutine of its own and asks again without waiting for it, so runs of one job overlap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobsWaitForASlotIsReported`, `TestARunIsAskedAgainOnlyAfterItReturns`, `TestATimedJobIsAskedOnTheIntervalsPhase`, `TestTheJobSeriesFollowEveryWayARunEnds`, `TestWakesDuringARunCoalesceIntoOneRun`
- **Break (4):** every wake waits in a queue of its own, so wakes during a run are counted as further runs, not coalesced into one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobWithNoIntervalIsAskedOnlyWhenWoken`, `TestWakesDuringARunCoalesceIntoOneRun`

## Pass 1 sets its completion flag when it ends, and a pass that ended and is not due again does no work

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a pass that ended and is not due again starts a fresh run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassThatEndedIsSkipped`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1`:** `TestBegin`, `TestBegin/a_pass_that_ended`, `TestBegin/a_pass_that_ended_with_a_run_left_running`
- **Break (2):** the run that ends the pass records its end and leaves the account's completion flag unset
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestACheckpointWithoutAPageCountResumes`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestAPassThatEndedIsSkipped`, `TestARisingStaleCountIsNotProgress`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEmptyMailboxEndsThePass`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheModelAgreesWithPostgreSQL`, `TestThePassDoesNotEndWhileASubjectIsStale`

## Remove and Stop cancel a job's run and wait for it to return

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break:** removing a job cancels its run and returns before the run has returned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobEnsuredWhileItIsRemovedNeverRunsTwiceAtOnce`, `TestEnsuringAndRemovingAJobRapidlyLeavesNoLoopRunning`, `TestRemoveAndStopCancelTheirRunsAndWaitForThem`

## The first pass ends only while no stored subject is masked under another scanner

- **Date · evidence:** 2026-10-08 · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), at RAPID_SEED=1, and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the run that ends the enumeration ends the pass whether or not a stored subject is stale
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestThePassDoesNotEndWhileASubjectIsStale`
- **Break (2):** the check that no stored subject is stale reads under no configuration revision, so every subject reads stale and the pass never ends
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1`:** `TestACallThatLeavesEverySubjectStaleFailsTheRun`, `TestACheckpointWithoutAPageCountResumes`, `TestAFallingStaleCountIsProgress`, `TestAFetchWhoseNewScannerMasksNothingCompletes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestAPassThatEndedIsSkipped`, `TestARisingStaleCountIsNotProgress`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestASubjectStoredUnmaskedIsTheProvidersSubject`, `TestASubjectWhoseMessageIsGoneIsMaskedWholeAndRecordedGone`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheCheckpointCarriesThePagesTheTotalImplies`, `TestTheModelAgreesWithPostgreSQL`, `TestThePassDoesNotEndWhileASubjectIsStale`

## The latest authentication attempt is recorded at the end of every unit of work, a failed one included, and a failed recording ends the pass in an error

- **Date · evidence:** 2026-10-02 · [pull request #254](https://github.com/ppat/mediated-mailbox-mcp/pull/254), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the end of a unit of work never reads the source's attempt, so it records none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`, `TestAFailedHandOverFailsTheSecondPassAtItsEnd`, `TestAFailedPageIsStillHandedOver`, `TestAFailedRecordingIsReturned`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEachOutcomeIsRecordedAsReported`, `TestEachPageIsAUnitOfWork`, `TestEachSecondPassPageIsAUnitOfWork`, `TestOnlyALaterAttemptIsRecorded`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (2):** every attempt is recorded as failed, whatever outcome the adapter reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachOutcomeIsRecordedAsReported`
- **Break (3):** only an attempt that succeeded is recorded, so a refused or failed one never reaches the account's state row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedRecordingIsReturned`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEachOutcomeIsRecordedAsReported`, `TestOnlyALaterAttemptIsRecorded`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (4):** a recording that fails is dropped, so the pass ends as if the attempt had been recorded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`, `TestAFailedHandOverFailsTheSecondPassAtItsEnd`, `TestAFailedRecordingIsReturned`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (5):** the end of a unit of work records the attempt but drops a recording that fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAFailedHandOverFailsThePassAtItsEnd`, `TestAFailedHandOverFailsTheSecondPassAtItsEnd`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (6):** an attempt is recorded at the instant it is recorded rather than the instant it started
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEachOutcomeIsRecordedAsReported`, `TestOnlyALaterAttemptIsRecorded`

## The messages whose sender could not be classified are counted per account

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and break 1 again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (3) again on 2026-10-10 at RAPID_SEED=1, with its patch regenerated after the code around it moved · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (1) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** a page's unclassified senders never reach the account's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachPageIsAUnitOfWork`, `TestEverySeriesTheKindEmitsCarriesItsJobKind`, `TestTheKindsSeriesOfAnAccountGoWhenItsWorkEnds`
- **Break (2):** no message is marked as having an unclassified sender
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachPageIsAUnitOfWork`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (3):** a tick's unclassified senders never reach delta sync's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (4):** a reconciliation adds its unclassified senders to the index without counting them in the tick's result
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`

## The run-start step writes to no run's record

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break:** the run-start step records the stopped second pass run's progress as starting over, with a progress and a retry event, on that run's record
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`

## The scan backlog depth is emitted after each step of the second pass

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the backlog series is set under the run's identifier in place of the account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`, `TestTheBacklogSeriesOfADroppedAccountGoesWithItsJob`, `TestTheKindsSeriesOfAnAccountGoWhenItsWorkEnds`
- **Break (2):** the backlog series is never set
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEachSecondPassPageIsAUnitOfWork`, `TestTheBacklogSeriesOfADroppedAccountGoesWithItsJob`

## The scanner's section is validated before the start

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the scanner is built from its shipped defaults rather than from its section
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`
- **Break (2):** a scanner section the scanner refuses does not refuse the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`

## The second pass sets its completion flag when it ends

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-10 at RAPID_SEED=1, after a sequence the second pass's crash test runs against PostgreSQL in every run changed the tests the row rests on · [pull request #340](https://github.com/ppat/mediated-mailbox-mcp/pull/340)
- **Break (1):** the second pass ends without setting its completion flag
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyTheConversionRefusesStaysPending`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** the second pass sets the first pass's completion flag in place of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2`:** `TestABodyTheConversionRefusesStaysPending`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/resumed-after-a-page-that-left-a-message-waiting`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`

## The second pass waits for the first pass to end

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates both breaks against the comment it changes beside them and demonstrates them again with the same tests red, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the second pass starts whether or not the first has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestBegin`, `TestBegin/the_first_pass_has_not_ended`, `TestBegin/the_first_pass_has_not_ended_and_a_run_stopped`
- **Break (2):** the second pass is skipped once the first has ended, and runs only before it has
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2`:** `TestBegin`, `TestBegin/a_run_that_failed`, `TestBegin/no_run_yet`, `TestBegin/the_first_pass_has_not_ended`, `TestBegin/the_first_pass_has_not_ended_and_a_run_stopped`

## The time a job waits for a slot of its kind's limit does not count toward its lateness

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** the late rule counts the waits that ended, so a job that waited for a slot fires sooner after its wait than its bound
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAlertingRules`
- **Break (2):** the late rule counts the wait in progress, so a job waiting for a slot ages while it waits
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAlertingRules`
- **Break (3):** the late rule counts the time a job waits for a slot, so a job queued behind its kind's long runs fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAlertingRules`
- **Break (4):** the scheduler reports no wait for a slot, so a job queued behind its kind's runs ages as if it were late
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobsWaitForASlotIsReported`
- **Break (5):** a success leaves the time waited before it counted, so the job's lateness after the success loses that time
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule`:** `TestAJobsWaitForASlotIsReported`

## The worker refuses a configuration it cannot run its job kinds with

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1
- **Break (1):** a job kind's empty user or password file is refused by the composed section's validation, under the shared section's path rather than the job kind's own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--database.backfill.user=`, `TestATickThatCannotRunRefusesTheStart/--database.sync.user=`, `TestATickThatCannotRunRefusesTheStart/backfill_password_file`, `TestATickThatCannotRunRefusesTheStart/sync_password_file`
- **Break (2):** each job kind's pool size, each job kind's concurrency and the reload interval are not validated
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--backfill.concurrency=0`, `TestATickThatCannotRunRefusesTheStart/--database.backfill.pool_size=0`, `TestATickThatCannotRunRefusesTheStart/--database.sync.pool_size=0`, `TestATickThatCannotRunRefusesTheStart/--reload_interval=0s`, `TestATickThatCannotRunRefusesTheStart/--sync.concurrency=0`

## The worker serves the health probe and backfill's reload-failure series on its metrics endpoint

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-10 at RAPID_SEED=1, after the worker merged backfill and delta sync into one deployable, which moved the code, tests or patches the row rests on and renamed the control from "Backfill serves the health probe and the reload-failure series on its metrics endpoint" · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328)
- **Break (1):** the health probe is not served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeEverySeriesBetweenTicks`, `TestTheProbesServeTheRunsSeries`
- **Break (2):** backfill's policy loader registers its reload-failure series on a registry of its own, off the metrics endpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheProbesServeTheRunsSeries`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestEverySeriesTheKindEmitsCarriesItsJobKind`
