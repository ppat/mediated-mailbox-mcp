# Mutations: executioncontext

The demonstrations of the controls whose patches sit in `executioncontext/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A base edit landing between two accounts' reads is read again, and fails the reload only when edits land through both reads

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a reload whose accounts read different base rules fails at once, so one base edit landing mid-reload pages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`
- **Break (2):** a reload reads once more for each disagreement, up to two more times, so edits landing through two reads no longer fail it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`

## A failed write-back is held in memory and logged

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a write-back that failed puts back the credential held before the rotation, so a reload serves the old one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`
- **Break (2):** a write-back that failed is logged at debug level, which a deployable does not emit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`

## A keyring re-seals a value to the current key

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a value already sealed to the current key is sealed again and reported as new
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** every value comes back as it was, whichever key it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`

## A keyring refuses a public key matching none of its private keys

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the public key is compared with the first private key alone, so it must match that one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAKeyringRefusesAPublicKeyMatchingNoPrivateKey`, `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** the check that the public key matches one of the private keys is removed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAKeyringRefusesAPublicKeyMatchingNoPrivateKey`

## A read of the policy tables the loader cannot trust never replaces the active policy

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the base rules are taken from the first account's read without comparing the others', so accounts are composed from different base policies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`
- **Break (2):** a read that found no rules is taken as a failed read, so a policy with no rules read whole never takes effect
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAnEmptyPolicyReadWholeIsAccepted`
- **Break (3):** a read that failed is taken as a whole read that found no rules, so the policy with no rules replaces the active one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`
- **Break (4):** a read that fails for one account keeps the rows read before it, so the accounts read earlier replace the active policy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`

## A read the account snapshot cannot trust never replaces it

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a read that lists no account keeps the previous snapshot rather than serving none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAReloadServesWhatItReadAndKeepsTheLastGoodSnapshotOnAFailedRead`
- **Break (2):** a read that failed replaces the snapshot with one serving no account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAReloadServesWhatItReadAndKeepsTheLastGoodSnapshotOnAFailedRead`

## A reload its caller cancelled keeps the active policy and raises no alarm

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a reload its caller cancelled is counted as a failed read, so a process shutting down mid-reload raises the alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAReloadItsCallerCancelledRaisesNoAlarm`
- **Break (2):** a reload whose deadline passed is taken as one its caller cancelled, so reloads that keep timing out raise no alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAReloadItsCallerCancelledRaisesNoAlarm`
- **Break (3):** every failed read is taken as a reload its caller cancelled, so no failed read raises the alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`

## A reload keeps a held rotated credential only while the stored bytes are the ones last known

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a reload always opens the stored bytes, so it drops a rotated credential whose write-back failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestOverlappingRotationsEachLand`
- **Break (2):** a reload keeps what it holds even when someone else replaced the stored bytes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAHandOverFromAReplacedCredentialIsDiscarded`, `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`, `TestAStaleReSealIsRefused`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`, `TestAStoredCredentialSomeoneElseReplacedWins`

## A sealed value is bound to its row and purpose

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** both lengths are written as zero, so the additional data loses the lengths that would keep a later purpose from running into its row, which the pinned bytes of the additional data catch
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal`:** `TestTheAdditionalDataBindsTheHeaderAndTheContext`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/an_account's_credential`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/the_client's_secret`
- **Break (2):** the additional data leaves the purpose out, so an account's credential opens as the client's secret under the same row name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal`:** `TestTheAdditionalDataBindsTheHeaderAndTheContext`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/an_account's_credential`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/the_client's_secret`
- **Break (3):** the additional data leaves the row out, so a value opens in any account's row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/a_row_whose_name_extends_the_account`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/another_account's_credential`, `TestReSealingMovesAValueToTheCurrentKey`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal`:** `TestTheAdditionalDataBindsTheHeaderAndTheContext`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/an_account's_credential`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/the_client's_secret`

## A sealed value opens only with the private key it was sealed to, and an altered, truncated or re-versioned value is refused

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the AEAD's refusal is ignored, so an altered value opens as whatever the failed decryption returns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAMissingKeyIsToldApartFromTampering`, `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/a_row_whose_name_extends_the_account`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/another_account's_credential`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_single-bit_flip`, `TestEveryAlterationIsRefused/every_truncation`, `TestOnlyThePrivateKeyItWasSealedToOpensAValue`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** the version byte is never checked, so a value under any version byte opens
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_other_version_byte`, `TestEveryAlterationIsRefused/every_single-bit_flip`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal`:** `TestSplitRefusesATruncatedOrReversionedValue`

## A unit of work keeps the account snapshot it took

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break:** a write-back changes the active snapshot's accounts in place, under a unit of work that took it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`

## A value is sealed only under a context naming its purpose and its row

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** no context is refused, so a value is sealed bound to no purpose and no row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal`:** `TestSealingRefusesAMissingKeyOrContext`, `TestSealingRefusesAMissingKeyOrContext/a_client_with_no_row`, `TestSealingRefusesAMissingKeyOrContext/an_account_with_no_identifier`, `TestSealingRefusesAMissingKeyOrContext/the_zero_context`
- **Break (2):** the row check tests the purpose instead, so a context naming no row is accepted and every client secret is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal`:** `TestSealingRefusesAMissingKeyOrContext`, `TestSealingRefusesAMissingKeyOrContext/an_account_with_no_identifier`

## A value naming no key the keyring holds is refused as naming an unknown key, and one naming a held key that fails authentication as altered

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a value the AEAD refuses is reported as naming a key the keyring does not hold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAMissingKeyIsToldApartFromTampering`, `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/a_row_whose_name_extends_the_account`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/another_account's_credential`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_single-bit_flip`, `TestEveryAlterationIsRefused/every_truncation`, `TestOnlyThePrivateKeyItWasSealedToOpensAValue`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** a value naming a key the keyring does not hold is refused as altered
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAMissingKeyIsToldApartFromTampering`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_single-bit_flip`

## A value that does not open leaves its account not connected or the accounts naming its client without one, and the refusal is logged

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a client whose secret did not open is kept in the snapshot with no secret
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAClientReadAgainThatDoesNotOpenLeavesItsAccountsNotConnected`, `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`, `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`
- **Break (2):** a client secret that does not open is logged at debug level, which a deployable does not emit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`
- **Break (3):** a credential that does not open is logged at debug level, which a deployable does not emit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`

## A write of a sealed value by a deployable is a compare-and-set on the bytes it last knew

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the re-seal compares against bytes it reads just before writing rather than the bytes it last knew, so it puts back a value the operator replaced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAStaleReSealIsRefused`
- **Break (2):** a write-back that found other bytes keeps its own credential rather than reading the stored one again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`
- **Break (3):** the write-back's statement no longer compares the stored bytes, so it replaces whatever the row holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAStaleReSealIsRefused`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`

## An account is loaded with the OAuth client it names, and an account that names none with none

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** an account that names no client is handed any client that opened, another provider's included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`
- **Break (2):** an account that names no client is handed the client named after its provider
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`
- **Break (3):** an account is handed the first client of its provider rather than the one it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`
- **Break (4):** no OAuth client is loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (5):** a client read again after a refused re-seal is handed to every account that names a client, the ones naming another client included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`
- **Break (6):** a credential read again keeps the client the snapshot held rather than the one the account now names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`

## An account whose own rules the loader did not read is held to the policy that restricts every sender

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the accounts the loader read are held to the policy that restricts every sender, and the ones it did not read get the base rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestSetAccountsChangesWhatTheNextReloadReads`, `TestTheLoaderComposesEachAccountsPolicy`
- **Break (2):** an account the loader did not read gets the base rules alone, without the restrictions its own rules add
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestSetAccountsChangesWhatTheNextReloadReads`, `TestTheLoaderComposesEachAccountsPolicy`

## At the end of a unit of work a changed credential is written back and an unchanged one is not

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** every call writes the credential back, an unchanged one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestARotatedCredentialSurvivesARestart`
- **Break (2):** the credential is compared with the one held rather than the one last read or written, so a failed write-back is never tried again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`
- **Break (3):** a changed credential is reported as written without the write running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestAHandOverFromAReplacedCredentialIsDiscarded`, `TestARotatedCredentialSurvivesARestart`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`, `TestAStoredCredentialSomeoneElseReplacedWins`, `TestOverlappingRotationsEachLand`

## Delta sync's re-seal moves a credential to the current key

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the re-seal writes nothing, so every value stays on its old key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestAStaleReSealIsRefused`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** every credential is sealed again and written, the ones already on the current key included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`

## Each value is sealed with a sender of its own

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a plaintext sealed before under the same key and context is not sealed again, and the earlier value is returned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/two_seals_of_the_same_plaintext`
- **Break (2):** one sender, and its encapsulated key, is kept and reused for every value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAKeyringLoadsItsKeysFromFiles`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/two_seals_of_the_same_plaintext`, `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`

## Every sealer seals to one current key, named by an identifier derived from it

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a keyring seals to its first private key's public key rather than the current one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** the key identifier is hashed from the public key without the domain string
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal`:** `TestAKeyIdentifierIsDerivedFromTheKey`

## Opening takes a private key type only

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the private key becomes an interface the public key also satisfies, so a public key passed where a private key belongs compiles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAPublicKeyIsNotAPrivateKey`
- **Break (2):** the private key becomes a type any value satisfies, so a public key passed where a private key belongs compiles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open`:** `TestAPublicKeyIsNotAPrivateKey`

## The key-generation command writes a pair the library seals to and opens with

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** an existing key file is truncated and overwritten rather than refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/cmd/keygen`:** `TestTheCommandPrintsTheKeyAndOverwritesNothing`
- **Break (2):** the mode is left to the process's umask, so a restrictive umask narrows the public key file's mode
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/cmd/keygen`:** `TestTheKeyFilesModesHoldUnderAnyUmask`
- **Break (3):** the private key file is written with mode 0644, readable by every user
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/cmd/keygen`:** `TestTheKeyFilesModesHoldUnderAnyUmask`
- **Break (4):** the public key file holds another pair's public key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/cmd/keygen`:** `TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith`
- **Break (5):** the private key file holds the seed followed by a newline
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/cmd/keygen`:** `TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith`

## The policy loader reads the accounts it is set to from its next reload on

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** SetAccounts leaves the accounts the loader reads as they were
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestSetAccountsChangesWhatTheNextReloadReads`
- **Break (2):** SetAccounts takes no account or an empty name, as New refuses
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestSetAccountsChangesWhatTheNextReloadReads`

## The policy reload rule fires while a process's latest reload failed, and only then

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the rule fires for every process that loads policy, whether its latest reload failed or succeeded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAlertingRules`
- **Break (2):** the rule compares the series with a value it never exceeds, so it never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAlertingRules`
- **Break (3):** the rule fires only after the failure has lasted five minutes, so a failure is not raised at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAlertingRules`

## The reload-failure series reads 1 while the latest policy reload failed and 0 once one succeeds

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a reload that succeeds leaves the series as it was, so it reads 1 after the first failure for as long as the process runs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (2):** a reload that succeeds sets the series to 1, as a failure does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (3):** a reload whose rows do not validate leaves the series as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (4):** a reload whose read failed leaves the series as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`

## The scan has an entry for every listed account and every OAuth client, true while its value is on an old key or does not open

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** no OAuth client has an entry, so the scan is silent on client secrets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** an account with no stored credential has no entry, so the scan is silent on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (3):** a credential that does not open reads as done
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
