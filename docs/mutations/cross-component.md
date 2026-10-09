# Mutations: cross-component

The demonstrations of the controls whose patches sit in more than one component's directory. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A client is checked with the provider before it is stored

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** every answer of the token endpoint to a client check reads as the client accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent`:** `TestTheClientChecksAnswer`
- **Break (2):** every client is stored without asking the provider
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientIsCheckedBeforeItIsStored`, `TestTheRecordedSetupFixturesMatchTheServer`

## A deployable that opens credentials refuses to start unless its public key matches one of its private keys

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on, with breaks (5) and (6), delta sync's patches, recorded in the row for the first time · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** the keyring holds only the first private key named, so a public key matching a later one refuses the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`
- **Break (2):** the keyring seals to the public key the first private key derives, so the mounted public key is never compared
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`
- **Break (3):** the start builds no keyring from the key files, so a public key matching none of the private keys goes unnoticed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/info`, `TestTheConfiguredLevelGovernsTheLog/no_level`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheEffectiveConfigurationIsLogged`, `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`
- **Break (4):** the keyring holds only the first private key named, so a public key matching a later one refuses the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`
- **Break (5):** the keyring holds only the first private key named, so a public key matching a later one refuses the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`
- **Break (6):** the keyring seals to the public key the first private key derives, so the mounted public key is never compared
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`

## A deployable's configuration type is pinned field by field

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** a section is added to backfill's root configuration type without updating the pinned field list
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunDecidesUnderTheSharedThresholdsAndScanner`, `TestAVerdictsRevisionFollowsTheScannerSection`, `TestAnEmptyHostRefusesTheStart`, `TestAnEmptyListOfPrivateKeysRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.link_words=[]`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.subject_threshold=0.9`, `TestAnInvalidScannerSectionRefusesTheStart/--scanner.window=0`, `TestTheConfigurationTypeIsPinned`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/info`, `TestTheConfiguredLevelGovernsTheLog/no_level`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheConfiguredLevelGovernsTheLog/warning`, `TestTheEffectiveConfigurationIsLogged`, `TestThePublicKeyMustMatchAPrivateKey`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_first_of_two`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_only_private_key`, `TestThePublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`
- **Break (2):** a value is added to the credential section without updating the pinned field list
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core`:** `TestTheSectionIsPinned`
- **Break (3):** a value of the database section is renamed without updating the pinned field list
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestTheSectionIsPinned`
- **Break (4):** a value is added to the database section without updating the pinned field list
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestTheSectionIsPinned`
- **Break (5):** a value is added to the root configuration without updating the pinned field list
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheConfigurationTypeIsPinned`, `TestTheEffectiveConfigurationIsLogged`

## A grant whose mailbox's API is not enabled is refused with its cause, and nothing is stored

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a profile read refused for a disabled API reads as any other refusal
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent`:** `TestAProfileReadOfADisabledAPI`
- **Break (2):** a grant whose API is disabled is reported as the provider not answering
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAGrantWhoseAPIIsDisabledIsRefused`

## A hand-over from a unit of work that started before the loader adopted a value someone else stored is discarded

- **Date · evidence:** 2026-10-02 · [pull request #254](https://github.com/ppat/mediated-mailbox-mcp/pull/254), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** every hand-over is discarded, a rotation from the credential the loader holds included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAHandOverFromAReplacedCredentialIsDiscarded`, `TestOverlappingRotationsEachLand`
- **Break (2):** a write-back of the loader's own counts as an outside value, so a rotation handed over by a unit that overlapped a landed one is discarded and lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestOverlappingRotationsEachLand`
- **Break (3):** the hand-over writes what the unit holds whatever it started from
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAHandOverFromAReplacedCredentialIsDiscarded`
- **Break (4):** delta sync hands its token over with no adoption stamp, so a tick that adopted the operator's credential and kept the refused one writes the refused one back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickWhoseRebuildFailsLeavesTheReauthorization`
- **Break (5):** a tick that rebuilt its source over the re-read credential keeps the start's adoption stamp, so the hand-over of a rotation that source received is discarded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestARotationAfterARereadIsWrittenBack`
- **Break (6):** a run that built its source over the re-read credential keeps the snapshot's adoption stamp, so the hand-over of a rotation that source received is discarded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARotationAfterARereadIsWrittenBack`
- **Break (7):** backfill hands its token over with no adoption stamp, so a unit of work that took its source before the loader adopted the operator's credential writes its own over it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential`

## A page made durable twice counts nothing twice

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and break (6) again on 2026-10-02 with its patch moved beside the statements it breaks in `db/senders/statistics`, turning red the same tests, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321)
- **Break (1):** any page token the provider refuses starts the pass over, one the run got in this run included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestOnFailure`, `TestOnFailure/a_refused_token_the_run_got_in_this_run`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestARefusedTokenTheRunGotFailsTheRun`
- **Break (2):** a resumed page token the provider refuses fails every run rather than starting the pass over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1`:** `TestOnFailure`, `TestOnFailure/a_refused_resumed_token`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPassStartedOverCountsNothingTwice`
- **Break (3):** a masking event is recorded for every masked message of a page, the ones the index already held included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPassStartedOverCountsNothingTwice`
- **Break (4):** the run never remembers the token it resumed from, so a refused resumed token fails every run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAPassStartedOverCountsNothingTwice`
- **Break (5):** a page counts every message with an unclassified sender, the ones the index already held included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAPassStartedOverCountsNothingTwice`
- **Break (6):** a sender's statistics add a page's count to the count stored, so a page taken twice counts twice
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1`:** `TestACheckpointWithoutAPageCountResumes`, `TestAKilledRunIsResumedByTheNextRun`, `TestAKilledRunResumesFromItsCheckpoint`, `TestAKilledRunResumesFromItsCheckpoint/rapid-draw`, `TestAKilledRunResumesFromItsCheckpoint/sampled-mix`, `TestAPageFailingEveryAttemptFailsTheRun`, `TestAPassIndexesTheWholeMailbox`, `TestAPassReopenedAfterItsEnumerationEndedFetchesTheStaleSubjectsAgain`, `TestAPassStartedOverCountsNothingTwice`, `TestAThrottledCallIsRetriedAndRecorded`, `TestAThrottledPageIsRetriedAndRecorded`, `TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount`, `TestTheModelAgreesWithPostgreSQL`

## A served account's token source is built from the OAuth client the account connects through and the account's own refresh token

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the client's secret is passed as its identifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (2):** every account is served with the OAuth client of the snapshot's first account rather than its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (3):** the client's identifier is passed as its secret
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (4):** the client's secret is passed as the account's refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAFailedWriteBackEndsTheRunInError`, `TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential`, `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_pass`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_second_pass`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`, `TestARunHandsEachAccountOverWhenItsUnitEnds`, `TestARunRecordsEachAccountsAttemptWhenItsUnitEnds`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheSourcesRefreshTokenIsHandedOver`
- **Break (5):** backfill's re-read after a refusal keeps the client the source was built with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`
- **Break (6):** delta sync passes the client's secret as its identifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`
- **Break (7):** delta sync serves every account with the OAuth client of the snapshot's first account rather than its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`
- **Break (8):** delta sync passes the client's identifier as its secret
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`
- **Break (9):** delta sync passes the client's secret as the account's refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestARefusedCallUsesTheCredentialTheOperatorStored`, `TestARefusedCallUsesTheCredentialTheOperatorStored/a_body_fetch`, `TestARefusedCallUsesTheCredentialTheOperatorStored/the_first_call`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARotationAfterARereadIsWrittenBack`, `TestATickReSealsWhatItOpensWithAnOldKey`, `TestATickTakesItsAccountsFromTheDatabase`, `TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken`, `TestEachTickHandsItsAccountOverAndRecordsItsAttempt`
- **Break (10):** delta sync's re-read after a refusal keeps the client the source was built with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`

## An account identifier no screen or path could reach is refused, and nothing is stored

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and break 1 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** the grammar migration's check refuses only the identifier ., so .. and / are stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoAccountIdentifierIsAPathSegmentThePathCannotHold`, `TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt`
- **Break (2):** the words the UI's top-level paths use are listed as setup and api alone rather than read from the bundle
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAnIdentifierNoScreenCouldReachIsRefused`
- **Break (3):** an identifier exactly ., .. or / is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestAnIdentifierEveryScreenCanReach`
- **Break (4):** an identifier equal to a word the UI's own top-level paths use is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestAnIdentifierEveryScreenCanReach`

## An account of a provider that authenticates through an OAuth client is connected only through its own client

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** an account of a provider that authenticates through a client and names none is connected
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`
- **Break (2):** an account of a provider that authenticates through a client is connected when its client's secret did not open
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAClientReadAgainThatDoesNotOpenLeavesItsAccountsNotConnected`, `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`
- **Break (3):** every account without a client is left not connected, whatever the deployable names, so an account whose provider needs none is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestAReloadServesWhatItReadAndKeepsTheLastGoodSnapshotOnAFailedRead`, `TestARotatedCredentialSurvivesARestart`, `TestAStaleReSealIsRefused`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`, `TestAStoredCredentialSomeoneElseReplacedWins`, `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne`, `TestOverlappingRotationsEachLand`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (4):** a client read again that does not open is taken from its accounts without leaving them not connected
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAClientReadAgainThatDoesNotOpenLeavesItsAccountsNotConnected`
- **Break (5):** an account left without its client still reads as connected, with its credential
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAnAccountWithoutAClientIsSkipped`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAClientReadAgainThatDoesNotOpenLeavesItsAccountsNotConnected`, `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestAnAccountNamingNoClientIsNotConnected`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickTakesItsAccountsFromTheDatabase`
- **Break (6):** backfill names no provider as authenticating through a client, so the loader connects a Gmail account that names none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAnAccountWithoutAClientIsSkipped`
- **Break (7):** the mediator names no provider as authenticating through a client, so the loader connects a Gmail account that names none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestAnAccountNamingNoClientIsNotConnected`
- **Break (8):** delta sync names no provider as authenticating through a client, so the loader connects a Gmail account that names none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickTakesItsAccountsFromTheDatabase`

## An added rule reaches the stored sender classes by its effect

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321)
- **Break (1):** the comparison restricts the messages but leaves the sender's statistics as they were
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestAnAddedRuleRestrictsTheStoredClasses`
- **Break (2):** a run of the second pass makes no comparison for an added rule, so the stored classes stay as ingested
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2`:** `TestARuleAddedAndRemovedDuringThePassReachesTheTransition`, `TestAnAddedRuleRestrictsTheStoredClasses`
- **Break (3):** a listed domain is stored with its own name as the rule rather than the rule that restricts it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestListed`, `TestListed/a_rule_for_the_bank_and_one_for_a_parent_domain`
- **Break (4):** a domain the policy restricts with no rule is listed too, so a policy that never loaded restricts every stored sender
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestListed`, `TestListed/a_policy_that_never_loaded`, `TestListed/a_rule_for_the_bank_and_one_for_a_parent_domain`, `TestListed/no_rule`
- **Break (5):** a tick makes no comparison for an added rule, so the stored classes stay as ingested once the second pass has ended
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickRestrictsTheStoredClassOfAnAddedRulesSender`

## An unknown log level is refused, and each of the four names reads as its own level

- **Date · evidence:** 2026-10-08 · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** a name that is none of the four reads as info instead of being refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/logging`:** `TestALevelIsOneOfFourNames`, `TestALevelIsOneOfFourNames/refused_a_leading_space`, `TestALevelIsOneOfFourNames/refused_a_trailing_newline`, `TestALevelIsOneOfFourNames/refused_an_offset_down`, `TestALevelIsOneOfFourNames/refused_an_offset_up`, `TestALevelIsOneOfFourNames/refused_empty`, `TestALevelIsOneOfFourNames/refused_title_case`, `TestALevelIsOneOfFourNames/refused_trace`, `TestALevelIsOneOfFourNames/refused_upper_case`, `TestALevelIsOneOfFourNames/refused_warning`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (2):** warning is read as warn, a fifth name beside the four
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/logging`:** `TestALevelIsOneOfFourNames`, `TestALevelIsOneOfFourNames/refused_warning`
- **Break (3):** warn reads as the error level
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/logging`:** `TestALevelIsOneOfFourNames`, `TestALevelIsOneOfFourNames/warn`
- **Break (4):** the mediator's entry ignores the level parser's refusal and starts at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (5):** the UI's entry ignores the level parser's refusal and starts at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (6):** backfill's entry ignores the level parser's refusal and starts at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (7):** delta sync's entry ignores the level parser's refusal and starts at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/warning`

## Delta sync decides under backfill's gate thresholds and records backfill's scanner version and revision

- **Date · evidence:** 2026-10-02 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08 at RAPID_SEED=1, after the re-mask from the store changed the code, tests or patches the row rests on · [pull request #321](https://github.com/ppat/mediated-mailbox-mcp/pull/321), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** backfill's default scanner section differs from delta sync's, so its verdicts and masks record another revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunDecidesUnderTheSharedThresholdsAndScanner`, `TestAVerdictsRevisionFollowsTheScannerSection`, `TestAVerdictsRevisionFollowsTheScannerSection/a_flag_changing_the_scanner`, `TestAVerdictsRevisionFollowsTheScannerSection/a_flag_restating_the_scanner's_default`, `TestAVerdictsRevisionFollowsTheScannerSection/the_file_changing_the_scanner`, `TestTheDefaults`, `TestTheEffectiveConfigurationIsLogged`
- **Break (2):** backfill's second pass decides under thresholds of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunDecidesUnderTheSharedThresholdsAndScanner`
- **Break (3):** delta sync's default scanner section differs from backfill's, so its verdicts and masks record another revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickDecidesUnderBackfillsThresholdsAndScanner`, `TestAVerdictsRevisionFollowsTheScannerSection`, `TestAVerdictsRevisionFollowsTheScannerSection/[--scanner.window=8]`, `TestAVerdictsRevisionFollowsTheScannerSection/[--scanner.window=9]`, `TestTheDefaults`, `TestTheEffectiveConfigurationIsLogged`
- **Break (4):** delta sync's gate decides under thresholds of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestATickDecidesUnderBackfillsThresholdsAndScanner`

## Each account spends under the lowered target its state row sets, and a target outside its range stops the run

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and breaks 2 and 3 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** the run builds its limiter without the targets its accounts' state rows set
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAStoredTargetAboveHalfTheCeilingStopsTheRun`, `TestTheLimiterSpendsUnderTheStoredTarget`
- **Break (2):** a target outside its range is taken as it is given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAnOutOfRangeTargetRefusesTheLimiter`
- **Break (3):** every account spends under the default target, whatever target it was given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestALoweredTargetBoundsOnlyItsOwnAccount`

## Each deployable logs at the level its configuration sets

- **Date · evidence:** 2026-10-08 · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** backfill's entry reads log_level and never sets the level variable, so the logger stays at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`
- **Break (2):** the mediator's entry reads log_level and never sets the level variable, so the logger stays at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`
- **Break (3):** the logger is built without the level variable, so it writes at info whatever the variable holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/logging`:** `TestAServersErrorLogWritesAtWarn`, `TestTheLoggerWritesJSONAtTheLevelItsVariableHolds`
- **Break (4):** delta sync's entry reads log_level and never sets the level variable, so the logger stays at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`
- **Break (5):** the UI's entry reads log_level and never sets the level variable, so the logger stays at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`

## Every caller that binds a sender domain to a statement binds the one domain normalizer's output

- **Date · evidence:** 2026-10-08 · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322)
- **Break (1):** the index query's domain term is bound as the client typed it, leaving case to the database
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheDomainAndClassTermsReadTheStoredForm`
- **Break (2):** the index query's domain term is lowercased by Go's simple mapping instead of passing through the domain normalizer
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheDomainAndClassTermsReadTheStoredForm`
- **Break (3):** the failures dataset's sender filter is bound as the operator typed it, leaving case to the database
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestADomainTypedInAnyCaseFindsItsStoredForm`
- **Break (4):** the senders dataset's search is bound as the operator typed it, so it no longer ignores case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestADomainTypedInAnyCaseFindsItsStoredForm`

## Every connection setting comes from the deployable's configuration, never from the PG* variables, and TLS verifies the server by default

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** the TLS mode defaults to prefer, which does not verify the server
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheDefaults`, `TestTheEffectiveConfigurationIsLogged`
- **Break (2):** the database name is left out of the connection string, so PGDATABASE sets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestTheConnectionIsTheConfigurations`
- **Break (3):** the database name is rendered from the user value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestTheConnectionIsTheConfigurations`
- **Break (4):** an empty sslrootcert is left out of the connection string, so PGSSLROOTCERT sets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestTheConnectionIsTheConfigurations`
- **Break (5):** the host is left out of the connection string, so PGHOST sets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestAValueCannotChangeAnotherSetting`, `TestTheConnectionIsTheConfigurations`
- **Break (6):** the port is left out of the connection string, so PGPORT sets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestTheConnectionIsTheConfigurations`
- **Break (7):** the sslmode is left out of the connection string, so PGSSLMODE sets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestTheConnectionIsTheConfigurations`
- **Break (8):** the user is left out of the connection string, so PGUSER sets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestTheConnectionIsTheConfigurations`
- **Break (9):** a value is quoted without escaping its quotes and backslashes, so it can end its string and add a setting of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect`:** `TestAValueCannotChangeAnotherSetting`

## Only the body fetch returns a body, and no metadata type has a field for it

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239)
- **Break (1):** the body fetch returns the message's metadata beside its body, so body and metadata travel together
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestOnlyTheBodyFetchReturnsABody`
- **Break (2):** the body fetch returns the body as a plain string, so no operation returns the body type
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestOnlyTheBodyFetchReturnsABody`
- **Break (3):** a change set gains a field holding the bodies of the added messages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestMetadataTypesHaveNoBodyField`, `TestOnlyTheBodyFetchReturnsABody`
- **Break (4):** thread listing is retyped to return bodies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestOnlyTheBodyFetchReturnsABody`
- **Break (5):** message metadata gains a string field holding the body's text
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestMetadataTypesHaveNoBodyField`
- **Break (6):** message metadata gains a field holding the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestMetadataTypesHaveNoBodyField`, `TestOnlyTheBodyFetchReturnsABody`
- **Break (7):** the port gains a second operation returning the full body as a string
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestOnlyTheBodyFetchReturnsABody`
- **Break (8):** a thread gains a field holding its messages' bodies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/mail`:** `TestMetadataTypesHaveNoBodyField`, `TestOnlyTheBodyFetchReturnsABody`
- **Break (9):** the metadata mask names each part's body beside its type and file name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail`:** `TestMetadataReadsNameNoBody`, `TestMetadataReadsNameNoBody/messages.get`, `TestMetadataReadsNameNoBody/threads.get`, `TestRequests`, `TestRequests/messages.get_through_the_metadata_mask`, `TestRequests/threads.get`
- **Break (10):** the mask's innermost parts are selected whole, so every body beneath them comes back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail`:** `TestMetadataReadsNameNoBody`, `TestMetadataReadsNameNoBody/messages.get`, `TestMetadataReadsNameNoBody/threads.get`, `TestRequests`, `TestRequests/messages.get_through_the_metadata_mask`, `TestRequests/threads.get`
- **Break (11):** a message's metadata read sends the full format without the mask, so the whole body comes back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail`:** `TestMetadataReadsNameNoBody`, `TestMetadataReadsNameNoBody/messages.get`, `TestRequests`, `TestRequests/messages.get_through_the_metadata_mask`
- **Break (12):** a thread read sends the full format without the mask, so every message's body comes back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail`:** `TestMetadataReadsNameNoBody`, `TestMetadataReadsNameNoBody/threads.get`, `TestRequests`, `TestRequests/threads.get`

## The consent's redirect address is the configured loopback address, read by every use

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a pasted address is read against the default loopback address rather than the configured one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent`:** `TestTheUIsConsentRedirectsToItsGivenAddress`
- **Break (2):** the paste box checks a pasted address's host against 127.0.0.1 whatever the configuration names
  - **Went red in `test/setup.test.tsx`:** `a pasted address names what it holds before anything is sent`, `the connect page pictures and checks the configured redirect address`
- **Break (3):** the connect page pictures the default redirect address whatever the configuration names
  - **Went red in `test/setup.test.tsx`:** `the connect page pictures and checks the configured redirect address`
- **Break (4):** the entry document carries no consent redirect for the browser
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheEntryDocumentCarriesTheBrowsersConfiguration`
- **Break (5):** a host name such as localhost is admitted as the redirect's host
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving`:** `TestTheConsentRedirectIsALoopbackAddressWithAPort`
- **Break (6):** a redirect without an explicit port is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving`:** `TestTheConsentRedirectIsALoopbackAddressWithAPort`
- **Break (7):** any consent redirect is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving`:** `TestTheConsentRedirectIsALoopbackAddressWithAPort`

## The credential section is validated before any key file is read

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** the start loads the keyring without validating the credential section
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAnEmptyListOfPrivateKeysRefusesTheStart`
- **Break (2):** an empty list of private keys is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_list_of_private_keys`, `TestAnUnusableValueIsRefused/no_private_key`
- **Break (3):** an empty public key path is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_public_key_path`
- **Break (4):** only the first private key path is checked, so an empty later one is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_later_private_key_path`
- **Break (5):** a second private key is refused, so a key replacement cannot mount the old key beside the new one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core`:** `TestAUsableValueIsAccepted`, `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_first_private_key_path`, `TestAnUnusableValueIsRefused/an_empty_later_private_key_path`

## The database section is validated before anything else starts

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** the start configures the connection without validating the database section
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestAnEmptyHostRefusesTheStart`
- **Break (2):** the TLS mode allow, which the driver accepts, is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAUsableValueIsAccepted`
- **Break (3):** empty database name is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_database_name`
- **Break (4):** empty host is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_host`
- **Break (5):** empty password file path is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_password_file_path`
- **Break (6):** empty user is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_user`
- **Break (7):** port 65535 is refused as outside the range
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAUsableValueIsAccepted`
- **Break (8):** a port above 65535 is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/a_port_above_65535`
- **Break (9):** a port below 1 is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/port_zero`
- **Break (10):** a TLS mode the driver does not accept passes validation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core`:** `TestAnUnusableValueIsRefused`, `TestAnUnusableValueIsRefused/an_empty_TLS_mode`, `TestAnUnusableValueIsRefused/an_unknown_TLS_mode`

## The effective configuration is logged at start, each value with its source

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, with a break per entry package added that sets the level before it logs the configuration, when the logger hand-off moved the control to every running deployable · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** backfill's entry sets the level before it logs the effective configuration, so under warn a refused value's source, log_level's included, is never written
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (2):** the mediator's entry sets the level before it logs the effective configuration, so under warn a refused value's source, log_level's included, is never written
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (3):** delta sync's entry sets the level before it logs the effective configuration, so under warn a refused value's source, log_level's included, is never written
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (4):** the UI's entry sets the level before it logs the effective configuration, so under warn a refused value's source, log_level's included, is never written
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheConfiguredLevelGovernsTheLog/warning`
- **Break (5):** the effective configuration is not logged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/info`, `TestTheConfiguredLevelGovernsTheLog/no_level`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheConfiguredLevelGovernsTheLog/warning`, `TestTheEffectiveConfigurationIsLogged`
- **Break (6):** the password file's text is logged beside its path
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheEffectiveConfigurationIsLogged`
- **Break (7):** each value is logged without the layer that set it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/a_refused_value_at_warn`, `TestTheConfiguredLevelGovernsTheLog/an_offset`, `TestTheConfiguredLevelGovernsTheLog/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/info`, `TestTheConfiguredLevelGovernsTheLog/no_level`, `TestTheConfiguredLevelGovernsTheLog/warn`, `TestTheConfiguredLevelGovernsTheLog/warning`, `TestTheEffectiveConfigurationIsLogged`

## The messages whose sender could not be classified are counted per account

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and break 1 again on 2026-10-08, after the domain normalizer and the sender class term over domains changed the code or tests the row rests on · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** no message is marked as having an unclassified sender
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachPageIsAUnitOfWork`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestDecide`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/tick`:** `TestATickAppliesTheChangesSinceItsCursor`
- **Break (2):** a page's unclassified senders never reach the account's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestEachPageIsAUnitOfWork`
- **Break (3):** a tick's unclassified senders never reach delta sync's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`
- **Break (4):** a reconciliation adds its unclassified senders to the index without counting them in the tick's result
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/app`:** `TestEachTickFeedsItsUnclassifiedAndBacklogSeries`

## The revision a verdict records is the scanner section's, and an empty revision is refused

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, after the logger hand-off moved the code, tests or patches the row rests on · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323)
- **Break (1):** the scanner is built under a fixed revision, so a change to its section leaves the revision as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunDecidesUnderTheSharedThresholdsAndScanner`, `TestAVerdictsRevisionFollowsTheScannerSection`, `TestAVerdictsRevisionFollowsTheScannerSection/a_flag_changing_the_scanner`, `TestAVerdictsRevisionFollowsTheScannerSection/an_environment_variable_changing_the_scanner`, `TestAVerdictsRevisionFollowsTheScannerSection/the_file_changing_the_scanner`
- **Break (2):** the scanner is built under the database section's revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill/app`:** `TestARunDecidesUnderTheSharedThresholdsAndScanner`, `TestAVerdictsRevisionFollowsTheScannerSection`, `TestAVerdictsRevisionFollowsTheScannerSection/a_flag_changing_another_section`, `TestAVerdictsRevisionFollowsTheScannerSection/a_flag_changing_the_scanner`, `TestAVerdictsRevisionFollowsTheScannerSection/an_environment_variable_changing_the_scanner`, `TestAVerdictsRevisionFollowsTheScannerSection/the_file_changing_the_scanner`
- **Break (3):** an empty revision builds a scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestNewRefusesAnInvalidConfiguration`, `TestNewRefusesAnInvalidConfiguration/an_empty_revision`
- **Break (4):** a verdict records no revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestAVerdictRecordsTheRevisionItWasBuiltUnder`, `TestFixtures`, `TestFixtures/alphanumeric_code`, `TestFixtures/bank`, `TestFixtures/login_link`, `TestFixtures/newsletter`, `TestFixtures/one-time_code`, `TestFixtures/receipt`, `TestScanPatterns`, `TestScanPatterns/a_code_alone_on_a_line`, `TestScanPatterns/nine_digits_after_a_trigger,_which_only_tier_2_catches`, `TestScanPatterns/the_alphanumeric_code_fixture,_which_only_tier_2_catches`, `TestScanPatterns/the_login_link_fixture`, `TestScanPatterns/the_newsletter_fixture`, `TestScanPatterns/the_one-time_code_fixture`, `TestScanPatterns/the_receipt_fixture`, `TestSubjectThreshold`, `TestTier1LoginLinks`, `TestTier1LoginLinks/a_dense_path_segment_with_a_link_word`, `TestTier1LoginLinks/a_dense_path_segment_without_a_link_word`, `TestTier1LoginLinks/a_dense_token_parameter`, `TestTier1LoginLinks/a_link_inside_Markdown`, `TestTier1LoginLinks/a_link_word_inside_a_joined_path_word`, `TestTier1LoginLinks/a_link_word_inside_a_longer_path_word`, `TestTier1LoginLinks/a_link_word_only_in_the_fragment`, `TestTier1LoginLinks/a_link_word_only_in_the_host`, `TestTier1LoginLinks/a_link_word_without_a_dense_segment`, `TestTier1LoginLinks/a_long_parameter_of_another_name`, `TestTier1LoginLinks/a_long_parameter_that_is_not_dense`, `TestTier1LoginLinks/a_parameter_name_in_capitals`, `TestTier1LoginLinks/a_short_token_parameter`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_after_a_no-break_space`, `TestTier1OneTimeCodes/a_code_after_a_trigger`, `TestTier1OneTimeCodes/a_code_alone_on_a_line`, `TestTier1OneTimeCodes/a_code_alone_on_a_line_ending_in_a_carriage_return`, `TestTier1OneTimeCodes/a_code_alone_on_a_line_in_bold`, `TestTier1OneTimeCodes/a_code_as_a_table_cell`, `TestTier1OneTimeCodes/a_code_before_a_dash`, `TestTier1OneTimeCodes/a_code_before_a_trigger`, `TestTier1OneTimeCodes/a_code_in_a_heading`, `TestTier1OneTimeCodes/a_code_in_a_second-level_heading`, `TestTier1OneTimeCodes/a_code_in_a_third-level_heading`, `TestTier1OneTimeCodes/a_code_in_curly_quotes`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_narrow_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_thin_space`, `TestTier1OneTimeCodes/a_code_in_groups_of_three`, `TestTier1OneTimeCodes/a_code_joined_by_a_hyphen`, `TestTier1OneTimeCodes/a_code_joined_to_a_letter_after_it`, `TestTier1OneTimeCodes/a_number_inside_a_table_cell's_text`, `TestTier1OneTimeCodes/a_trigger_exactly_the_window_away`, `TestTier1OneTimeCodes/a_trigger_in_capitals`, `TestTier1OneTimeCodes/a_trigger_inside_a_longer_word`, `TestTier1OneTimeCodes/a_trigger_one_past_the_window`, `TestTier1OneTimeCodes/a_two-word_trigger`, `TestTier1OneTimeCodes/digits_joined_to_letters`, `TestTier1OneTimeCodes/nine_digits,_which_only_tier_2_catches`, `TestTier1OneTimeCodes/three_digits`, `TestTier2`, `TestTier2/a_date`, `TestTier2/a_word_with_no_digit`, `TestTier2/an_alphanumeric_code_after_a_trigger`, `TestTier2/an_alphanumeric_code_alone_in_bold`, `TestTier2/an_eleven-character_reference_after_a_trigger`, `TestTier2/an_order_number`
