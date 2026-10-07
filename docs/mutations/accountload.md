# Mutations: accountload

The demonstrations of the controls whose patches sit in `accountload/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A failed write-back is held in memory and logged

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239)
- **Break (1):** a write-back that failed puts back the credential held before the rotation, so a reload serves the old one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`
- **Break (2):** a write-back that failed is logged at debug level, which a deployable does not emit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`

## A read the account snapshot cannot trust never replaces it

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** a read that lists no account keeps the previous snapshot rather than serving none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAReloadServesWhatItReadAndKeepsTheLastGoodSnapshotOnAFailedRead`
- **Break (2):** a read that failed replaces the snapshot with one serving no account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAReloadServesWhatItReadAndKeepsTheLastGoodSnapshotOnAFailedRead`

## A reload keeps a held rotated credential only while the stored bytes are the ones last known

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** a reload always opens the stored bytes, so it drops a rotated credential whose write-back failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestOverlappingRotationsEachLand`
- **Break (2):** a reload keeps what it holds even when someone else replaced the stored bytes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAHandOverFromAReplacedCredentialIsDiscarded`, `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`, `TestAStaleReSealIsRefused`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`, `TestAStoredCredentialSomeoneElseReplacedWins`

## A unit of work keeps the account snapshot it took

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break:** a write-back changes the active snapshot's accounts in place, under a unit of work that took it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`

## A value that does not open leaves its account not connected or the accounts naming its client without one, and the refusal is logged

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** a client whose secret did not open is kept in the snapshot with no secret
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAClientReadAgainThatDoesNotOpenLeavesItsAccountsNotConnected`, `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`, `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`
- **Break (2):** a client secret that does not open is logged at debug level, which a deployable does not emit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`
- **Break (3):** a credential that does not open is logged at debug level, which a deployable does not emit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`

## A write of a sealed value by a deployable is a compare-and-set on the bytes it last knew

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239)
- **Break (1):** the re-seal compares against bytes it reads just before writing rather than the bytes it last knew, so it puts back a value the operator replaced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAStaleReSealIsRefused`
- **Break (2):** a write-back that found other bytes keeps its own credential rather than reading the stored one again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`
- **Break (3):** the write-back's statement no longer compares the stored bytes, so it replaces whatever the row holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAStaleReSealIsRefused`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`

## An account is loaded with the OAuth client it names, and an account that names none with none

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** an account that names no client is handed any client that opened, another provider's included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`
- **Break (2):** an account that names no client is handed the client named after its provider
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`
- **Break (3):** an account is handed the first client of its provider rather than the one it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`
- **Break (4):** no OAuth client is loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (5):** a client read again after a refused re-seal is handed to every account that names a client, the ones naming another client included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal`:** `TestAReSealNeverPutsBackAReplacedSecret`
- **Break (6):** a credential read again keeps the client the snapshot held rather than the one the account now names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo`

## At the end of a unit of work a changed credential is written back and an unchanged one is not

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239)
- **Break (1):** every call writes the credential back, an unchanged one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestARotatedCredentialSurvivesARestart`
- **Break (2):** the credential is compared with the one held rather than the one last read or written, so a failed write-back is never tried again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`
- **Break (3):** a changed credential is reported as written without the write running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestAHandOverFromAReplacedCredentialIsDiscarded`, `TestARotatedCredentialSurvivesARestart`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`, `TestAStoredCredentialSomeoneElseReplacedWins`, `TestOverlappingRotationsEachLand`

## Delta sync's re-seal moves a credential to the current key

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** the re-seal writes nothing, so every value stays on its old key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestAStaleReSealIsRefused`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** every credential is sealed again and written, the ones already on the current key included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`

## The scan has an entry for every listed account and every OAuth client, true while its value is on an old key or does not open

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** no OAuth client has an entry, so the scan is silent on client secrets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** an account with no stored credential has no entry, so the scan is silent on it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (3):** a credential that does not open reads as done
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/accountload`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
