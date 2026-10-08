# Mutations: backfill

The demonstrations of the controls whose patches sit in `backfill/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A body the conversion refuses stays pending, never scanned, with a failed item

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a refusal is ignored, so the body is scanned as empty and recorded clean
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachSecondPassPageIsAUnitOfWork`, `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestABodyTheConversionRefusesStaysPending`
- **Break (2):** a body the conversion refuses is scanned as its raw HTML and recorded scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachSecondPassPageIsAUnitOfWork`, `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestABodyTheConversionRefusesStaysPending`

## A change of scanner reopens a pass whose stored work another scanner decided

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a pass that ended is skipped whether or not it is due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestBegin`, `TestBegin/a_pass_that_ended_and_is_due_again`, `TestBegin/a_pass_that_ended_and_is_due_again_with_a_run_left_running`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`
- **Break (2):** a reopened first pass runs while it stays recorded as ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (3):** a stored subject whose configuration revision differs and whose version matches does not make the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAFailedHandOverFailsThePassAtItsEnd`, `TestAFailedHandOverFailsTheSecondPassAtItsEnd`, `TestAFailedPageIsStillHandedOver`, `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`, `TestEachPageIsAUnitOfWork`, `TestEachSecondPassPageIsAUnitOfWork`, `TestNoBodyTextReachesTheIndexOrTheLogs`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (4):** a stored subject whose scanner version differs and whose revision matches does not make the first pass due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`

## A credential the provider refuses is read again from the account's row before the refusal is reported, and a replaced one is used for the call it refused

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a failed read of the refused credential is dropped, so the run reports the refusal alone
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedReadOfTheRefusedCredentialIsReported`
- **Break (2):** an account whose row no longer holds a credential has the call made again with an empty one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`
- **Break (3):** the comparison is inverted, so a replaced credential is reported as refused and the refused one is used again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`
- **Break (4):** the new source and its re-read's adoption stamp are not made the account's holding, so the hand-over keeps handing over the refused source with the snapshot's stamp, and a rotation of the credential read again is never written back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARotationAfterARereadIsWrittenBack`
- **Break (5):** the call is made again over the port the refused credential's source was built on, so the credential read again is never used
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`
- **Break (6):** a refused credential the account's row still holds is made a new source of and the call made again with it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`
- **Break (7):** the re-read's adoption stamp is made the account's holding but its new source is not, so the hand-over writes the refused credential over the one the operator stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARotationAfterARereadIsWrittenBack`
- **Break (8):** a refused call is reported at once, so the credential is never read again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`

## A failed body fetch is retried, recorded gone or abandoned, or stops the run, by its class

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a message the provider no longer has stops the run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestOnFailure`, `TestOnFailure/gone`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAFailedBodyFetch`
- **Break (2):** a throttled body fetch stops the run at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestOnFailure`, `TestOnFailure/a_first_throttle`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAFailedBodyFetch`

## A killed backfill run resumes from its last checkpointed page, with at most one page of rework

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a page made durable keeps the token it was asked with, so the next page asked for is the same one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestAdvanceAndRestart`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAPageThatFailsToCommitLeavesNothing`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestARefusedTokenTheRunGotFailsTheRun`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheCheckpointCarriesThePagesTheTotalImplies`, `TestTheModelAgreesWithPostgreSQL`
- **Break (2):** a resumed run starts from the first page, dropping the checkpoint and counters of the run it resumes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestBegin`, `TestBegin/a_pass_not_ended_that_is_due_again`, `TestBegin/a_pass_that_ended_and_is_due_again_with_a_run_left_running`, `TestBegin/a_run_that_failed`, `TestBegin/a_run_that_stopped_without_recording_its_end`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
- **Break (3):** a resumed run counts one page more than its checkpoint, so it reports a page it never made durable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
- **Break (4):** a run resuming an enumeration that has ended asks for the first page again rather than finishing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`

## A killed second pass resumes from its last checkpointed page

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223)
- **Break (1):** the stored checkpoint loses the last message read, so a resumed run reads from the first waiting message
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** a resumed run counts one page more than its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestTheTransitionMarksNothingTwice`

## A message the gate skipped whose subject is now masked returns to pending

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates break (1) against the statements it adds beside the one it breaks and demonstrates it again with the same tests red, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** every message the gate skipped returns to pending, whether or not its subject is masked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (2):** a gate skip decided without the subject's signal the message now carries stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`

## A page's rows, masking events, sender statistics and checkpoint become durable together or not at all

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a checkpoint that fails to record is ignored, so the page's rows commit without it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAPageThatFailsToCommitLeavesNothing`
- **Break (2):** the messages, masking events and sender statistics commit in a transaction of their own before the one recording the checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPageThatFailsToCommitLeavesNothing`

## A removed rule returns its sender's messages to pending scan, and the pass starts over

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the comparison delists the domains the policy still restricts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestTheTransitionMarksNothingTwice`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDelisted`, `TestDelisted/a_policy_that_never_loaded`, `TestDelisted/every_rule_in_place`, `TestDelisted/every_rule_removed`, `TestDelisted/the_bank's_rule_removed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickScansADelistedSendersMessages`
- **Break (2):** a run that marked messages keeps its checkpoint, so marked messages before it wait until the pass ends
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestTheTransitionMarksNothingTwice`
- **Break (3):** the transition marks no domain
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestTheTransitionMarksNothingTwice`
- **Break (4):** the comparison reads only the domains stored restricted, so a message skipped as restricted under a rule added after it was stored is never found
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`

## A reopened first pass always leads to a run of the second pass

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates breaks (1) and (2) against the run-start step that also returns overturned gate skips and demonstrates the row again, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the run-start step records the first pass as not ended in place of the second
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (2):** the run-start step reopens the second pass only when it returned a verdict to pending, not when the first pass is due again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAReopenedFirstPassLeadsToASecondPass`

## A rotated refresh token is handed to the account snapshot library after every page, a page that failed included, and a failed write-back ends the pass in an error

- **Date · evidence:** 2026-10-02 · [pull request #254](https://github.com/ppat/mediated-mailbox-mcp/pull/254), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a page that fails ends the pass before the hand-over, so a rotation made before the failure is lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedPageIsStillHandedOver`
- **Break (2):** the account's token is handed over only once the pass ends, not after every page
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachPageIsAUnitOfWork`
- **Break (3):** a hand-over that fails is dropped, so the pass ends without an error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedHandOverFailsThePassAtItsEnd`
- **Break (4):** the hand-over hands over the source the run started with, so a source a re-read replaced is never handed over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARotationAfterARereadIsWrittenBack`
- **Break (5):** no refresh token is handed over, so a rotation never reaches the database
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedWriteBackEndsTheRunInError`, `TestARotationAfterARereadIsWrittenBack`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestTheSourcesRefreshTokenIsHandedOver`
- **Break (6):** the run never ends a unit of work over an account's session, so no source's token reaches the database
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedWriteBackEndsTheRunInError`, `TestARotationAfterARereadIsWrittenBack`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (7):** a failed write-back is dropped, so the run ends as if the rotation had landed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedWriteBackEndsTheRunInError`

## A run resuming an enumeration made under another scanner starts it over from the first page

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a run resuming an enumeration starts it over whatever scanner it was made under
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestUnder`, `TestUnder/a_run_resumed_under_the_same_scanner`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`
- **Break (2):** a run starting an enumeration over under another scanner keeps the page count of the enumeration it abandoned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestUnder`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
- **Break (3):** a run resumes an enumeration made under another scanner from its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestUnder`, `TestUnder/a_run_resumed_under_another_scanner`, `TestUnder/a_run_resumed_under_another_version`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`

## A run serves only a connected account whose provider it has an adapter for and that connects through an OAuth client

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the connected accounts are skipped and the ones that are not connected are served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestARunTakesItsAccountsFromTheDatabase`, `TestAnAccountWithoutAClientIsSkipped`, `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (2):** an account of a provider backfill has no adapter for is served through the Gmail adapter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunTakesItsAccountsFromTheDatabase`
- **Break (3):** an account that is not connected is served with an empty refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunTakesItsAccountsFromTheDatabase`, `TestAnAccountWithoutAClientIsSkipped`

## A run takes its account snapshot once at the start, and a failed read stops it

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the policy is loaded only for the connected accounts rather than every listed one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunTakesItsAccountsFromTheDatabase`, `TestAnAccountWithoutAClientIsSkipped`, `TestTheProbesServeTheRunsSeries`
- **Break (2):** a load whose read fails is ignored, so the run goes on with no account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedSnapshotReadStopsTheRun`

## A scan verdict records its content flags, content rules, scanner version and configuration revision

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a one-time code is stored as a login link
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestBothPartsOfABodyAreScanned`
- **Break (2):** a verdict is stored without the configuration revision it was made under
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestAnAddedRuleRestrictsTheStoredClasses`

## A second pass resuming a stopped pass while the run-start step's mark is set starts over from the first waiting message

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates break (3) against the comment it changes beside the statement it breaks and demonstrates it again with the tests it adds red too, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a second pass run resumes a stopped pass from its checkpoint whatever the mark, so what a run's start returned to pending before it waits for good
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestOver`, `TestOver/a_resumed_run_with_the_mark`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`
- **Break (2):** the second pass run that starts leaves the mark set, so every later run resuming the pass starts it over again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
- **Break (3):** the run-start step reopens the second pass without marking it to start over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`

## A sender's first scan hit reaches the gate's decision on its next message, and no other sender's

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the gate reads only the prior hits stored before the page, so a hit earlier on the page does not count
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestGate`, `TestGate/a_hit_earlier_on_the_page`
- **Break (2):** a hit on the page counts as a prior hit of every sender on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestASendersFirstHitReachesItsNextMessage`

## A stale subject is masked again under the scanner in force, each mask recorded under it

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a masking event is recorded with no scanner version or revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheModelAgreesWithPostgreSQL`
- **Break (2):** the first pass masks a stored subject again only when its scanner version differs, so a changed configuration revision leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
- **Break (3):** the first pass masks a stored subject again only when its configuration revision differs, so a changed scanner version leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`

## A stored gate skip the gate no longer decides as the same skip returns to pending at every backfill run's start

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a stored gate skip is decided again under the default thresholds rather than the ones the run holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/a_widened_high-volume_mark`, `TestOverturned/the_thresholds_the_skips_were_made_under`
- **Break (2):** every stored gate skip is overturned, the ones the gate still decides as the same skip included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/a_widened_high-volume_mark`, `TestOverturned/the_thresholds_the_skips_were_made_under`
- **Break (3):** a stored gate skip is overturned only when the gate would now scan it, so a skip under thresholds that cannot decide or of a sender now restricted stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestOverturned`, `TestOverturned/a_sender_the_policy_now_lists`, `TestOverturned/thresholds_that_cannot_decide`
- **Break (4):** the run-start step reads the gate skips and returns none to pending, so every skip made under earlier thresholds stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (5):** the read of the stored gate skips drops the sender's prior hits, so a skip whose sender has since gained a hit stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestASkipWhoseSenderGainedAPriorHitIsScanned`

## A stored subject the enumeration did not find is masked whole under the scanner in force

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a subject the enumeration did not find keeps its stored text under the scanner in force instead of being masked whole
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestUnfound`
- **Break (2):** a stored subject whose configuration revision alone differs is not found stale when the enumeration ends, so a removed message keeps a subject masked under the earlier revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAFailedHandOverFailsThePassAtItsEnd`, `TestAFailedHandOverFailsTheSecondPassAtItsEnd`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`, `TestEachPageIsAUnitOfWork`, `TestEachSecondPassPageIsAUnitOfWork`, `TestNoBodyTextReachesTheIndexOrTheLogs`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (3):** a stored subject whose scanner version alone differs is not found stale when the enumeration ends, so a removed message keeps a subject masked under the earlier version
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
- **Break (4):** the run that ends the pass masks no subject the enumeration did not find, leaving it masked under the earlier scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`

## A verdict made under another scanner returns to pending, cleared, with its sender's prior hits counted again

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and break (1) again on 2026-10-02 with its patch regenerated for the same move, turning red `TestAChangeOfScannerMasksAndScansAgain`, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the prior hits of a sender whose verdicts returned to pending are not counted again, so a message flagged again counts twice
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (2):** a verdict whose configuration revision differs and whose version matches stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (3):** a verdict whose scanner version differs and whose revision matches stays in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerVersionAloneMasksAndScansAgain`
- **Break (4):** a verdict returned to pending keeps its content flags, content rules, scan time, version and revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`
- **Break (5):** a backfill run leaves a verdict made under another scanner in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`
- **Break (6):** a backfill run returns no verdict to pending before its first pass, so stale verdicts keep releasing bodies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAChangeOfScannerVersionAloneMasksAndScansAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`, `TestThresholdsThatCannotDecideReturnEverySkipToPending`
- **Break (7):** a backfill run returns stale verdicts to pending only after its first pass ends, so they keep releasing bodies while it runs or fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestAReopenedFirstPassLeadsToASecondPass`, `TestARevertedChangeOfScannerStillScansWhatItReturnedToPending`

## An overturned gate skip reopens the second pass, marked to start over

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the run-start step reopens the second pass only for a stale verdict or subject, so a skip it returned to pending after the pass ended waits for good
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`, `TestASkipWhoseSenderGainedAPriorHitIsScanned`
- **Break (2):** the run-start step reopens the second pass for overturned skips without marking it to start over, so a stopped pass resumes past the skips returned before its checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfThresholdsDecidesTheStoredSkipsAgain`, `TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned`

## Backfill serves the health probe and the reload-failure series on its metrics endpoint

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the health probe is not served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheProbesServeTheRunsSeries`
- **Break (2):** the policy loader registers its reload-failure series on a registry of its own, off the metrics endpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheProbesServeTheRunsSeries`

## Backfill's second pass never asks for a restricted sender's body

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the gate reads every sender as normal, so a restricted sender's body is scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestGate`, `TestGate/a_sender_the_policy_lists`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** every message is scanned whatever the gate decided, a restricted sender's included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`

## Both parts of a body are scanned, and a flag in either flags the message

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the message's flags are the HTML part's alone, though both parts are scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestASendersFirstHitReachesItsNextMessage`, `TestBothPartsOfABodyAreScanned`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestScan`, `TestScan/a_body_with_no_HTML_part`, `TestScan/a_code_in_one_part_and_a_link_in_the_other`, `TestScan/a_code_in_the_text_part`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** the text part is not scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestARemovedRuleReturnsItsSendersMessagesToPendingScan`, `TestASendersFirstHitReachesItsNextMessage`, `TestBothPartsOfABodyAreScanned`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestScan`, `TestScan/a_body_with_no_HTML_part`, `TestScan/a_code_in_one_part_and_a_link_in_the_other`, `TestScan/a_code_in_the_text_part`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickScansADelistedSendersMessages`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`

## Each flagged message adds one prior hit to its sender, durably

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and break (2) again on 2026-10-02 with its patch regenerated for the statistics' writes moved to `db/senders/statistics`, turning red the same tests, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** every scanned message counts as a hit, a clean one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestAdvance`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (2):** no prior hit is added to the senders' statistics
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`

## Every backfill run is recorded with its checkpoint, counters, timeline and failed items

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a failure that is not the provider's is abandoned as the page's failure, recorded with no error class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestOnFailure`, `TestOnFailure/a_failure_that_is_not_the_provider's`, `TestOnFailure/an_attempt_nobody_described`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAFailureNotTheProvidersFailsTheRunWithNoFailedPage`
- **Break (2):** a throttled or failed page is asked for again whatever the attempts, so the run never fails on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestOnFailure`, `TestOnFailure/a_provider_failure_on_the_last_attempt`, `TestOnFailure/a_throttle_on_the_last_attempt`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`
- **Break (3):** a page the provider throttles fails the run on its first attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestOnFailure`, `TestOnFailure/a_first_throttle`, `TestOnFailure/a_throttle_before_the_last_attempt`, `TestOnFailure/a_throttle_on_a_resumed_token`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPageIsRecordedRecoveredOnlyOnceDurable`, `TestAThrottledPageIsRetriedAndRecorded`
- **Break (4):** a page the run fails on after its last attempt records no failed item
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPageFailingEveryAttemptFailsTheRun`, `TestARefusedCredentialFailsTheRunAtOnce`, `TestARefusedTokenTheRunGotFailsTheRun`
- **Break (5):** a page made durable records its checkpoint and no progress event
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAKilledRunIsResumedByTheNextRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestARefusedTokenTheRunGotFailsTheRun`, `TestAThrottledPageIsRetriedAndRecorded`
- **Break (6):** a page is recorded as recovered in a transaction of its own as soon as it arrives, before its commit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPageIsRecordedRecoveredOnlyOnceDurable`
- **Break (7):** a page that failed and then became durable records no failed item
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAThrottledPageIsRetriedAndRecorded`
- **Break (8):** a run resumed after it stopped without recording its end is left recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassStartedOverCountsNothingTwice`, `TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheModelAgreesWithPostgreSQL`

## Every scan gate decision over stored messages is recorded with its reason

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a scan is recorded as a skip and a skip as a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestABodyTheConversionRefusesStaysPending`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** a skip is not recorded in the decisions table
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`, `TestASendersFirstHitReachesItsNextMessage`

## Every subject is masked and every sender classified before the message reaches the index

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** every message reaches the index with its sender classified normal
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (2):** a masked subject carries no mask, so no masking event is recorded for it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (3):** a message reaches the index with its subject as the provider gave it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`, `TestDecideFailsClosed`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`

## No body text reaches the index or the workload's logs

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a scan records the body's text part among its content rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAChangeOfScannerMasksAndScansAgain`, `TestNoBodyTextReachesTheIndexOrTheLogs`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestNoBodyTextReachesTheIndexOrTheLogs`
- **Break (2):** a refused body's failed item records the start of the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestNoBodyTextReachesTheIndexOrTheLogs`

## Pass 1 sets its completion flag when it ends, and a pass that ended and is not due again does no work

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a pass that ended and is not due again starts a fresh run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestBegin`, `TestBegin/a_pass_that_ended`, `TestBegin/a_pass_that_ended_with_a_run_left_running`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPassThatEndedIsSkipped`
- **Break (2):** the run that ends the pass records its end and leaves the account's completion flag unset
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassStartedOverCountsNothingTwice`, `TestAPassThatEndedIsSkipped`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEmptyMailboxEndsThePass`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheModelAgreesWithPostgreSQL`

## The effective configuration is logged at start, each value with its source

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the effective configuration is not logged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheEffectiveConfigurationIsLogged`
- **Break (2):** the password file's text is logged beside its path
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheEffectiveConfigurationIsLogged`
- **Break (3):** each value is logged without the layer that set it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheEffectiveConfigurationIsLogged`

## The latest authentication attempt is recorded at the end of every unit of work, a failed one included, and a failed recording ends the pass in an error

- **Date · evidence:** 2026-10-02 · [pull request #254](https://github.com/ppat/mediated-mailbox-mcp/pull/254), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the end of a unit of work never reads the source's attempt, so it records none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedRecordingIsReturned`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEachOutcomeIsRecordedAsReported`, `TestOnlyALaterAttemptIsRecorded`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (2):** every attempt is recorded as failed, whatever outcome the adapter reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachOutcomeIsRecordedAsReported`
- **Break (3):** only an attempt that succeeded is recorded, so a refused or failed one never reaches the account's state row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedRecordingIsReturned`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEachOutcomeIsRecordedAsReported`, `TestOnlyALaterAttemptIsRecorded`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (4):** a recording that fails is dropped, so the pass ends as if the attempt had been recorded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedRecordingIsReturned`, `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (5):** the end of a unit of work the run gives its passes records the attempt but drops a recording that fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheRunsHandOverReturnsAFailedRecording`
- **Break (6):** an attempt is recorded at the instant it is recorded rather than the instant it started
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestEachOutcomeIsRecordedAsReported`, `TestOnlyALaterAttemptIsRecorded`

## The run-start step writes to no run's record

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break:** the run-start step records the stopped second pass run's progress as starting over, with a progress and a retry event, on that run's record
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAStoppedSecondPassKeepsItsRecordAndStartsOver`

## The scan backlog depth is emitted after each step of the second pass

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the backlog series is set under the run's identifier in place of the account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachSecondPassPageIsAUnitOfWork`
- **Break (2):** the backlog series is never set
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachSecondPassPageIsAUnitOfWork`

## The scanner's section is validated before the start

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the scanner is built from its shipped defaults rather than from its section
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`
- **Break (2):** a scanner section the scanner refuses does not refuse the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`

## The second pass sets its completion flag when it ends

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the second pass ends without setting its completion flag
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestABodyTheConversionRefusesStaysPending`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`
- **Break (2):** the second pass sets the first pass's completion flag in place of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestABodyTheConversionRefusesStaysPending`, `TestAKilledSecondPassResumesFromItsCheckpoint`, `TestAKilledSecondPassResumesFromItsCheckpoint/rapid-draw`, `TestAKilledSecondPassResumesFromItsCheckpoint/sampled-mix`, `TestAPassDecidesAndRecordsEveryWaitingMessage`

## The second pass waits for the first pass to end

- **Date · evidence:** 2026-10-01 · [pull request #231](https://github.com/ppat/mediated-mailbox-mcp/pull/231), which regenerates both breaks against the comment it changes beside them and demonstrates them again with the same tests red, first demonstrated by [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223)
- **Break (1):** the second pass starts whether or not the first has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestBegin`, `TestBegin/the_first_pass_has_not_ended`, `TestBegin/the_first_pass_has_not_ended_and_a_run_stopped`
- **Break (2):** the second pass is skipped once the first has ended, and runs only before it has
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2`:** `TestBegin`, `TestBegin/a_run_that_failed`, `TestBegin/no_run_yet`, `TestBegin/the_first_pass_has_not_ended`, `TestBegin/the_first_pass_has_not_ended_and_a_run_stopped`
