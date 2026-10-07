# Mutations: credential

The demonstrations of the controls whose patches sit in `credential/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A keyring re-seals a value to the current key

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** a value already sealed to the current key is sealed again and reported as new
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** every value comes back as it was, whichever key it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`

## A keyring refuses a public key matching none of its private keys

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** the public key is compared with the first private key alone, so it must match that one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAKeyringRefusesAPublicKeyMatchingNoPrivateKey`, `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** the check that the public key matches one of the private keys is removed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAKeyringRefusesAPublicKeyMatchingNoPrivateKey`

## A sealed value is bound to its row and purpose

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** both lengths are written as zero, so the additional data loses the lengths that would keep a later purpose from running into its row, which the pinned bytes of the additional data catch
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/seal`:** `TestTheAdditionalDataBindsTheHeaderAndTheContext`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/an_account's_credential`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/the_client's_secret`
- **Break (2):** the additional data leaves the purpose out, so an account's credential opens as the client's secret under the same row name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/seal`:** `TestTheAdditionalDataBindsTheHeaderAndTheContext`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/an_account's_credential`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/the_client's_secret`
- **Break (3):** the additional data leaves the row out, so a value opens in any account's row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/a_row_whose_name_extends_the_account`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/another_account's_credential`, `TestReSealingMovesAValueToTheCurrentKey`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/seal`:** `TestTheAdditionalDataBindsTheHeaderAndTheContext`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/an_account's_credential`, `TestTheAdditionalDataBindsTheHeaderAndTheContext/the_client's_secret`

## A sealed value opens only with the private key it was sealed to, and an altered, truncated or re-versioned value is refused

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** the AEAD's refusal is ignored, so an altered value opens as whatever the failed decryption returns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAMissingKeyIsToldApartFromTampering`, `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/a_row_whose_name_extends_the_account`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/another_account's_credential`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_single-bit_flip`, `TestEveryAlterationIsRefused/every_truncation`, `TestOnlyThePrivateKeyItWasSealedToOpensAValue`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** the version byte is never checked, so a value under any version byte opens
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_other_version_byte`, `TestEveryAlterationIsRefused/every_single-bit_flip`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/seal`:** `TestSplitRefusesATruncatedOrReversionedValue`

## A value is sealed only under a context naming its purpose and its row

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** no context is refused, so a value is sealed bound to no purpose and no row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/seal`:** `TestSealingRefusesAMissingKeyOrContext`, `TestSealingRefusesAMissingKeyOrContext/a_client_with_no_row`, `TestSealingRefusesAMissingKeyOrContext/an_account_with_no_identifier`, `TestSealingRefusesAMissingKeyOrContext/the_zero_context`
- **Break (2):** the row check tests the purpose instead, so a context naming no row is accepted and every client secret is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/seal`:** `TestSealingRefusesAMissingKeyOrContext`, `TestSealingRefusesAMissingKeyOrContext/an_account_with_no_identifier`

## A value naming no key the keyring holds is refused as naming an unknown key, and one naming a held key that fails authentication as altered

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** a value the AEAD refuses is reported as naming a key the keyring does not hold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAMissingKeyIsToldApartFromTampering`, `TestAValueOpensOnlyInItsOwnRowAndPurpose`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/a_row_whose_name_extends_the_account`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/another_account's_credential`, `TestAValueOpensOnlyInItsOwnRowAndPurpose/the_client's_secret_under_the_same_name`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_single-bit_flip`, `TestEveryAlterationIsRefused/every_truncation`, `TestOnlyThePrivateKeyItWasSealedToOpensAValue`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** a value naming a key the keyring does not hold is refused as altered
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAMissingKeyIsToldApartFromTampering`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/every_single-bit_flip`

## Each value is sealed with a sender of its own

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** a plaintext sealed before under the same key and context is not sealed again, and the earlier value is returned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/two_seals_of_the_same_plaintext`
- **Break (2):** one sender, and its encapsulated key, is kept and reused for every value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAKeyringLoadsItsKeysFromFiles`, `TestEveryAlterationIsRefused`, `TestEveryAlterationIsRefused/two_seals_of_the_same_plaintext`, `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`

## Every sealer seals to one current key, named by an identifier derived from it

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** a keyring seals to its first private key's public key rather than the current one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestEverySealerNamesTheCurrentKey`, `TestReSealingMovesAValueToTheCurrentKey`
- **Break (2):** the key identifier is hashed from the public key without the domain string
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/seal`:** `TestAKeyIdentifierIsDerivedFromTheKey`

## Opening takes a private key type only

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** the private key becomes an interface the public key also satisfies, so a public key passed where a private key belongs compiles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAPublicKeyIsNotAPrivateKey`
- **Break (2):** the private key becomes a type any value satisfies, so a public key passed where a private key belongs compiles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/open`:** `TestAPublicKeyIsNotAPrivateKey`

## The key-generation command writes a pair the library seals to and opens with

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178)
- **Break (1):** an existing key file is truncated and overwritten rather than refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/cmd/keygen`:** `TestTheCommandPrintsTheKeyAndOverwritesNothing`
- **Break (2):** the mode is left to the process's umask, so a restrictive umask narrows the public key file's mode
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/cmd/keygen`:** `TestTheKeyFilesModesHoldUnderAnyUmask`
- **Break (3):** the private key file is written with mode 0644, readable by every user
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/cmd/keygen`:** `TestTheKeyFilesModesHoldUnderAnyUmask`
- **Break (4):** the public key file holds another pair's public key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/cmd/keygen`:** `TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith`
- **Break (5):** the private key file holds the seed followed by a newline
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/credential/cmd/keygen`:** `TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith`
