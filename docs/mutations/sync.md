# Mutations: sync

The demonstrations of the controls whose patches sit in `sync/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A credential the provider refuses is read again from its row before the refusal is reported

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a failed read of the refused credential is dropped and the refusal returned alone
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestAFailedReadOfTheRefusedCredentialIsReported`
- **Break (2):** a refused call returns the refusal without reading the credential again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/a_body_fetch`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_call`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`
- **Break (3):** the hand-over reads the source the tick started with, so the refused credential is written over the operator's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/a_body_fetch`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_call`, `TestARotationAfterARereadIsWrittenBack`
- **Break (4):** a refused credential the row still holds is used for a second call
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`

## A cursor gap is recovered by re-enumerating from an hour before the last cursor was written, and counted

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the cursor gap rule fires while any gap was ever counted, so it never clears
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAlertingRules`
- **Break (2):** the cursor gap rule reads only the count's increase, so a gap the series' first sample counts never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAlertingRules`
- **Break (3):** a tick that finds a gap does not report it, so the gap series never moves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAFailedRecoveryIsTriedAgainByTheNextTick`, `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`
- **Break (4):** a recovery stores the current cursor before it lists the window, so a recovery that fails is never tried again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAFailedRecoveryIsTriedAgainByTheNextTick`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`
- **Break (5):** a gap's window starts an hour before now, whenever the last cursor was written
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/core/tick`:** `TestWindow`, `TestWindow/a_cursor_written_five_minutes_ago`, `TestWindow/a_cursor_written_long_ago`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (6):** a gap's window starts at the last cursor's write time, so mail the provider dated just before it is left out
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/core/tick`:** `TestWindow`, `TestWindow/a_cursor_written_five_minutes_ago`, `TestWindow/a_cursor_written_long_ago`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`

## A gap recovery removes a stored message dated in its window once a complete listing left it out and the provider no longer returns it by its identifier

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a gap recovery treats a stored message dated before its window as left out of its listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (2):** a gap recovery removes nothing it found missing from its listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`
- **Break (3):** a gap recovery removes every stored message its listing left out, the ones the provider still returns by identifier included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/core/tick`:** `TestRemoval`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`

## A tick applies each change set in the transaction that advances the cursor past it

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and breaks 2, 3 and 4 again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322)
- **Break (1):** the cursor is stored in a transaction of its own before the change set's writes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (2):** a change set's application stores no cursor, so the next tick asks for the same changes again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestTheCursorNeverRunsAheadOfTheIndex`
- **Break (3):** a stored message's labels and flags are left as they were when the provider reports them changed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (4):** a message the provider no longer holds stays in the index
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestATickAppliesTheChangesSinceItsCursor`

## A tick hands its account over and records its latest authentication attempt when it ends

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** no tick records its source's authentication attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickHandsItsAccountOverAndRecordsItsAttempt`
- **Break (2):** a tick that fails hands nothing over and records no attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickHandsItsAccountOverAndRecordsItsAttempt`

## A tick records as failed the account's tick and gap recovery a stopped process left running

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a recovery's start also records the tick that started it, still running, as failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`
- **Break (2):** a tick's start records a stopped tick as failed and leaves a stopped recovery recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`
- **Break (3):** a tick's start leaves every run a stopped process left running recorded as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickRecordsAStoppedRunAsFailed`, `TestATickRecordsAStoppedRunAsFailed/a_recovery_stopped_part_way`, `TestATickRecordsAStoppedRunAsFailed/a_tick_stopped_part_way`

## A tick spends from the account's budget in the sync class

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a tick's calls are leased in the batch class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickSpendsInTheSyncClass`
- **Break (2):** a tick's calls are leased in the interactive class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickSpendsInTheSyncClass`

## An account with no cursor is reconciled over the first window inside its tick

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322)
- **Break (1):** an account's first reconciliation is reported as a cursor gap, so the gap series and its alert count a cursor that was never lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (2):** an account with no cursor takes the current cursor and lists only what is dated after it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAGapIsRecoveredOverTheWindowSinceTheLastCursor`, `TestARecoveryRemovesWhatTheProviderNoLongerHolds`, `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickAppliesTheChangesSinceItsCursor`, `TestATickDecidesABoundedNumberAndTheNextGoesOn`, `TestATickRestrictsTheStoredClassOfAnAddedRulesSender`, `TestATickScansADelistedSendersMessages`, `TestATickSpendsInTheSyncClass`, `TestBeforeTheSecondPassHasEndedATickScansNothing`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`

## Applying the same changes twice leaves the index as applying them once

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** applying a message's labels adds them to the ones stored rather than setting them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnce`
- **Break (2):** a message the index already holds has its masks recorded again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnce`

## Delta sync re-seals what it opens with an old key and scans every listed account and OAuth client

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the re-seal of a client secret writes nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickReSealsWhatItOpensWithAnOldKey`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`
- **Break (2):** a client secret whose re-seal could not be written reads as done
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickReSealsWhatItOpensWithAnOldKey`

## Delta sync refuses a configuration it cannot tick with

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** the interval is refused when it is positive and taken when it is not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--decisions_per_tick=0`, `TestATickThatCannotRunRefusesTheStart/--first_window=-1h`, `TestATickThatCannotRunRefusesTheStart/--sync_interval=0s`, `TestAnInvalidScannerSectionRefusesTheStart`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/info`, `TestTheConfiguredLevelGovernsTheLog/no_level`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheEffectiveConfigurationIsLogged`, `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_only_private_key`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`
- **Break (2):** the interval, the first window and the bound on decisions are not validated
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickThatCannotRunRefusesTheStart`, `TestATickThatCannotRunRefusesTheStart/--decisions_per_tick=0`, `TestATickThatCannotRunRefusesTheStart/--first_window=-1h`, `TestATickThatCannotRunRefusesTheStart/--sync_interval=0s`

## Delta sync refuses to start unless its public key matches one of its private keys

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** the keyring holds only the first private key named, so a public key matching a later one refuses the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`
- **Break (2):** the keyring seals to the public key the first private key derives, so the mounted public key is never compared
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`

## Delta sync runs until stopped and serves every series between ticks

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** a series whose account or client a later scan no longer holds stays reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal`:** `TestTheSeriesFollowTheLastScan`
- **Break (2):** an account's gap series appears only at its first gap, so the rule's increase cannot see that gap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheProbesServeEverySeriesBetweenTicks`
- **Break (3):** every adapter after the first counts on series of its own, so only the first tick's requests reach the metrics endpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheProbesServeEverySeriesBetweenTicks`
- **Break (4):** the Gmail adapter's request cost and hard cap are registered on a registry the metrics endpoint does not serve
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheProbesServeEverySeriesBetweenTicks`
- **Break (5):** the rate limiter's series are registered on a registry the metrics endpoint does not serve
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheProbesServeEverySeriesBetweenTicks`
- **Break (6):** a tick sets none of the key-scan series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickReSealsWhatItOpensWithAnOldKey`, `TestTheProbesServeEverySeriesBetweenTicks`
- **Break (7):** the syncer registers its series on a registry the metrics endpoint does not serve
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheProbesServeEverySeriesBetweenTicks`

## Delta sync's re-seal of a client secret never puts back a value someone else replaced

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a re-seal that lost to a replaced secret keeps the secret it held instead of reading the stored one again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`
- **Break (2):** the re-seal's write replaces the stored secret whatever bytes it holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`

## Each tick takes the account snapshot again

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** a tick serves the snapshot it held before its load, so an account connected since is ticked a tick late
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestAFailedReadOfTheRefusedCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported`, `TestARefusalOfTheStoredCredentialIsReported/the_account_disconnected`, `TestARefusalOfTheStoredCredentialIsReported/the_stored_credential_refused`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/a_body_fetch`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_call`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`, `TestATickTakesItsAccountsFromTheDatabase`, `TestATickWhoseRebuildFailsLeavesTheReauthorization`, `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`, `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`, `TestEachTickHandsItsAccountOverAndRecordsItsAttempt`, `TestEachTickLoadsTheAccountSnapshot`, `TestNoBodyTextReachesTheIndexOrTheLogs`, `TestTheProbesServeEverySeriesBetweenTicks`
- **Break (2):** only the first tick loads the account snapshot, so an account connected afterwards is never ticked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickLoadsTheAccountSnapshot`

## Once backfill's second pass has ended, each tick decides and scans what waits, a bounded number from where the last stopped

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and break 10 again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322)
- **Break (1):** delta sync sets an account's backlog series before backfill's second pass has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (2):** a tick's backlog never reaches delta sync's backlog series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (3):** a tick reports no backlog after its scanning
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (4):** a tick decides every waiting message whatever its bound
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickDecidesABoundedNumberAndTheNextGoesOn`
- **Break (5):** a tick stopped by a throttle keeps its checkpoint past the rest of its read, so the next tick passes the throttled message over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`
- **Break (6):** a tick stopped by a throttle goes on deciding the rest of its read without the hits of the bodies it did not scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`
- **Break (7):** a tick never compares the stored restricted senders with its policy, so a delisted sender's messages stay skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickScansADelistedSendersMessages`
- **Break (8):** a tick never scans, so what waits stays waiting once backfill has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`, `TestATickDecidesABoundedNumberAndTheNextGoesOn`, `TestATickRestrictsTheStoredClassOfAnAddedRulesSender`, `TestATickScansADelistedSendersMessages`, `TestATickSpendsInTheSyncClass`, `TestOnceTheSecondPassHasEndedATickScansWhatWaits`
- **Break (9):** each tick reads from the first waiting message, forgetting where the last stopped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickDecidesABoundedNumberAndTheNextGoesOn`
- **Break (10):** a tick scans whether or not backfill's second pass has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`, `TestBeforeTheSecondPassHasEndedATickScansNothing`
- **Break (11):** a throttled body fetch leaves its message waiting and the tick goes on fetching
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestAThrottledBodyStopsTheAccountsScanning`
