# Mutations: api

The demonstrations of the controls whose patches sit in `api/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A body decision's reason is a token of the closed set the contract states

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the contract's reason enum leaves out conversion_refused, which a denial returns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheReasonEnumIsTheTokensABodyDecisionReturns`
- **Break (2):** the gate names a pending scan as a phrase rather than its token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheReasonEnumIsTheTokensABodyDecisionReturns`

## A body request is decided under the policy loaded after it arrived, so a newly listed domain is denied on the next call

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the registry hands the body operation the policy in force in place of the fresh load, so the load in api/internal/bodies is never called
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestANewlyListedDomainIsDeniedOnTheNextCall`
- **Break (2):** the body operation is handed the policy in force and nothing is loaded for the request
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestANewlyListedDomainIsDeniedOnTheNextCall`
- **Break (3):** a request whose policy load fails is decided under the policy that restricts every sender rather than the one in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestANewlyListedDomainIsDeniedOnTheNextCall`

## A body request waits for its leases no longer than its deadline for them, and a wait cut off by it is the API server's failure

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the request's arrival is taken once the gate has released the message, after the policy load and the gate's read, so the time they take does not count against the wait
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`
- **Break (2):** the deadline is counted from the unit of work's beginning rather than the request's arrival, so the policy load and the gate's read before it lengthen the wait
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`
- **Break (3):** a wait cut off by the deadline is wrapped as the provider's failure, so the rate controller's hold reads as the provider's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`
- **Break (4):** the lease is waited for on the request's own context, so a lease that never comes holds the request for as long as its client waits
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`

## A body request's provider calls lease in the interactive class, and a gate denial leases nothing

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the body request's provider calls take no lease
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`
- **Break (2):** the body operation's caller leases in the batch class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`

## A body with no HTML part, the snippet and each filename are released as fenced code blocks showing the text exactly

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** each filename is released as the provider gave it, unfenced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAReleasedBodyIsCleanMarkdown`
- **Break (2):** a body with no HTML part is released as its text part, unfenced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAReleasedBodyIsCleanMarkdown`
- **Break (3):** the snippet is released as the provider gave it, unfenced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAReleasedBodyIsCleanMarkdown`

## A call its client abandons at 2026-07-28 or later has its operation's context cancelled

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the SDK is not told to follow the request's cancellation, so the operation runs on after its client has gone
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestAnAbandonedCallIsCancelled`

## A call's arguments hold at most 1024 top-level keys

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** arguments holding any number of keys reach the operation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestACallHoldingTooManyArgumentsIsRefused`

## A deployable that opens credentials refuses to start unless its public key matches one of its private keys

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the connections build no keyring from the key files, so a public key matching none of the private keys goes unnoticed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyPastTheBoundIsDeniedAsRefused`, `TestABodyTheConversionRefusesIsDenied`, `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestAFailedPolicyLoadStillFollowsTheList`, `TestAFirstReloadThatFailsLeavesTheAPIServerStartedAndNotReady`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestAPathOutsideBothRootsIsNotFound`, `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestAReasonTheAuditCheckDoesNotKnowReleasesNothing`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`, `TestARereadAfterAMoveBuildsTheTokenFromTheNewClient`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAServedBodyFetchesNoImage`, `TestAStaleWriteBackLeavesTheReauthorization`, `TestAnAccountNamingNoClientIsNotConnected`, `TestAnAccountWithoutAStoredCredentialIsNotConnected`, `TestEachBodyRequestWritesBackAndRecordsTheAttempt`, `TestEachOriginIsToldApartOnBothRoots`, `TestEveryReloadLoadsThePolicy`, `TestNoBodyIsReleasedWithoutItsAuditRow`, `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheAPIServersPublicKeyMustMatchAPrivateKey`, `TestTheAPIServersPublicKeyMustMatchAPrivateKey/a_public_key_matching_no_private_key`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`, `TestTheReadOperationsReadTheSession`, `TestTheServeTimeCheckReadsEverythingReleased`, `TestTheSurfaceGivenNoKeyPairIsPlain`, `TestTheSurfaceGivenNoKeyPairIsPlain/both_blank`, `TestTheSurfaceGivenNoKeyPairIsPlain/neither_file`, `TestTheSurfaceIsServedOverTLSWhenGivenAKeyPair`
- **Break (2):** the keyring holds only the first private key named, so a public key matching a later one refuses the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheAPIServersPublicKeyMustMatchAPrivateKey`, `TestTheAPIServersPublicKeyMustMatchAPrivateKey/a_public_key_matching_the_second_of_two`

## A failed call answers with the service layer's structured failure on both roots

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** an MCP error result carries the failure as text only, with no structured content
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestAServedToolTakesOnlyTheArgumentsItDeclares`, `TestAToolCallRunsTheOperation`

## A gate-skipped body the structural patterns match is withheld at serve time

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the serve-time check runs the full scanner, tier 2's scoring included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestServeTimePatternCheck`, `TestServeTimePatternCheck/a_gate-skipped_code_only_tier_2_catches`
- **Break (2):** a pattern match on a gate-skipped body no longer withholds it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`, `TestServeTimePatternCheck`, `TestServeTimePatternCheck/a_gate-skipped_login_link`, `TestServeTimePatternCheck/a_gate-skipped_one-time_code`, `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_code_in_the_body_and_a_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_login_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_one-time_code_in_a_filename`

## A listing's cursor continues only the listing, account and filter it came from, and paging serves every row once

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a count's cursor is bound to its query but not its grouping, so a cursor another grouping returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (2):** a cursor another account's listing returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`, `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (3):** a cursor a listing with another filter returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`, `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (4):** a cursor another listing returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`, `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (5):** a page of the groups the service layer pages starts after the last group's count alone, so a group tied with it is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`
- **Break (6):** the thread listing returns no next cursor on a full page, so the rows after it are never served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (7):** the masking-events listing's next page starts after the last event's time alone, so an event sharing that time is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (8):** every listing's next cursor is built from its page's first row, in the one helper they share, so each next page serves rows again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`, `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`
- **Break (9):** the message listing's next cursor names the page's first row instead of its last
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (10):** the message listing's next page starts after the last row's time alone, so a row sharing that time is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (11):** a search's cursor is bound to its sort and order but not its query, so a cursor another query returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (12):** a search by date, oldest first, continues from its last row inclusive, so that row is served again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`
- **Break (13):** a search by date, newest first, breaks a tie by message identifier ascending while it continues after its cursor by the row comparison, so a page boundary inside a tie skips and repeats messages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (14):** the sender listing's next page starts after the last sender's count alone, so a sender tied with it is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (15):** a search sorted by sender continues from its last row's message identifier inclusive, so that row is served again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (16):** the thread listing's next page starts after the last thread's latest time alone, so a thread sharing that time is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAListingPagesThroughEveryRowOnce`

## A message whose body is denied stays in every count, group and search result of the index reads

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the groups by label leave out the messages carrying a content flag
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`
- **Break (2):** the search leaves out every message whose body the gate denies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`, `TestTheDomainAndClassTermsReadTheStoredForm`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (3):** the groups by sender leave out the messages skipped as restricted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`
- **Break (4):** the summary counts as a selection's messages only those whose body could be released, scanned with no content flag
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (5):** the summary of a selection leaves out the messages pending their content scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`

## A provider call that does not answer within the provider timeout is the provider's failure

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a call whose deadline passed is returned as the context's error, which reads as the API server's failure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`
- **Break (2):** a provider call has no deadline of the API server's, so a provider that does not answer holds the request
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`

## A refused credential is read again from its row before the refusal is reported

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an account the re-read finds without a credential has the call made again over a token made from an empty one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`
- **Break (2):** a refusal is reported without reading the credential again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestARereadAfterAMoveBuildsTheTokenFromTheNewClient`
- **Break (3):** the call is made again over the credential read again, but later requests go back to the refused one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`

## A released body sits inside delimiters it cannot close

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the delimiters' words are matched in lower case only
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestTheCheckReadsEverythingReleased`, `TestTheReleasedBodysForm`, `TestTheWindowedNeutralizerReplacesWhatTheFullScanDid`
- **Break (2):** the text after the last replaced stretch is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_multi-byte_letter_before_the_words`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestTheCheckReadsEverythingReleased`, `TestTheReleasedBodysForm`, `TestTheWindowedNeutralizerReplacesWhatTheFullScanDid`
- **Break (3):** each letter's byte range is one byte wide, so a letter wider than a byte inside the words breaks the match
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestTheWindowedNeutralizerReplacesWhatTheFullScanDid`
- **Break (4):** a released body is not wrapped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_letter_of_the_words_just_before_them`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_multi-byte_letter_before_the_words`, `TestABodyCannotCloseTheDelimiters/a_near_miss`, `TestABodyCannotCloseTheDelimiters/a_space_between_every_letter`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/lower_case`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestABodyCannotCloseTheDelimiters/the_words_with_a_word_between_them`, `TestAScannedBodyIsReleasedInTheDelimiters`, `TestNoBodyCarriesADenyingSensitivity`, `TestServeTimePatternCheck`, `TestServeTimePatternCheck/a_gate-skipped_code_only_tier_2_catches`, `TestServeTimePatternCheck/a_gate-skipped_newsletter`, `TestServeTimePatternCheck/a_gate-skipped_receipt`, `TestTheCheckReadsEverythingReleased`, `TestTheReleaseStepAllocatesAFewCopiesOfTheBody`, `TestTheReleaseStepAllocatesAFewCopiesOfTheBody/nothing_to_replace`, `TestTheReleaseStepAllocatesAFewCopiesOfTheBody/the_words_at_the_end`, `TestTheReleasedBodysForm`
- **Break (5):** text spelling the delimiters is released as it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_letter_of_the_words_just_before_them`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_multi-byte_letter_before_the_words`, `TestABodyCannotCloseTheDelimiters/a_space_between_every_letter`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/lower_case`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestTheReleasedBodysForm`
- **Break (6):** characters between the letters break a match
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_multi-byte_letter_before_the_words`, `TestABodyCannotCloseTheDelimiters/a_space_between_every_letter`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/lower_case`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestTheCheckReadsEverythingReleased`, `TestTheReleasedBodysForm`, `TestTheWindowedNeutralizerReplacesWhatTheFullScanDid`

## A served body fetches nothing and holds no image a client could fetch

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** images are no longer on the conversion's list of dropped elements, so a served body keeps a remote image
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAReleasedBodyIsCleanMarkdown`, `TestAServedBodyFetchesNoImage`
- **Break (2):** the API server itself requests each image source in the HTML it fetched before converting it, so serving a body reaches the URL the message names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAServedBodyFetchesNoImage`

## A served message's flags are exactly read and starred, each a boolean

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a stored flag reads as true whatever its value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheFlagsAreServedAsReadAndStarredAlone`
- **Break (2):** the contract's schema admits keys beside read and starred
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheFlagsAreServedAsReadAndStarredAlone`
- **Break (3):** starred is always served as false
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheFlagsAreServedAsReadAndStarredAlone`

## A served operation takes only the arguments its input schema declares, in exactly that case, and never as null

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a key naming no declared argument reaches the decoder, which matches it to one in another case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestAServedToolTakesOnlyTheArgumentsItDeclares`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`
- **Break (2):** a null argument is decoded as absent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestAServedToolTakesOnlyTheArgumentsItDeclares`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`
- **Break (3):** a null index query term is decoded as absent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestAnIndexReadTakesOnlyTheTermsItDeclares`
- **Break (4):** an index query's term naming none the schema declares reaches the decoder, which matches it to one in another case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestAnIndexReadTakesOnlyTheTermsItDeclares`

## A summary and each group count the messages they hold, each thread once, the unread, those with attachments, and the oldest and the newest date

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the groups by domain count the threads holding an unread message as the unread messages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachCountCountsEveryThreadOnce`
- **Break (2):** the groups by month date their oldest message as the latest of their threads' oldest
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachCountCountsEveryThreadOnce`
- **Break (3):** the groups by sender group an address by its spelling, so one written in two cases is two senders
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachCountCountsEveryThreadOnce`
- **Break (4):** the groups by sender count a thread once for each of its messages in the group
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachCountCountsEveryThreadOnce`
- **Break (5):** the summary of a selection of nothing counts its messages as null rather than zero
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestANonUTCTimestampIsRefused`, `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEachCountCountsEveryThreadOnce`, `TestEachQueryTermSelectsItsMessages`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (6):** the summary counts a thread once for each of its messages selected
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachCountCountsEveryThreadOnce`

## A timestamp the client surface takes with any offset but Z is refused, never converted

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a timestamp with any offset is accepted and converted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestANonUTCTimestampIsRefused`
- **Break (2):** a timestamp written with the offset +00:00 is accepted as UTC
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestANonUTCTimestampIsRefused`

## An operation carrying anything less than both roots need fails generation

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an operation with no description generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/no_description`
- **Break (2):** an operation generates whatever its input schema holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/an_input_schema_that_is_not_JSON`, `TestAOneSidedEntryFailsGeneration/an_input_schema_that_is_not_an_object's`, `TestAOneSidedEntryFailsGeneration/no_input_schema`
- **Break (3):** an operation with no handler generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/no_handler`
- **Break (4):** an operation generates under any name, one no tool can carry included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/a_name_no_tool_can_carry`, `TestAOneSidedEntryFailsGeneration/a_name_starting_with_a_digit`, `TestAOneSidedEntryFailsGeneration/a_name_too_long_for_a_tool`, `TestAOneSidedEntryFailsGeneration/no_name`
- **Break (5):** an operation generates whatever its output schema holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/an_output_schema_that_is_not_an_object's`, `TestAOneSidedEntryFailsGeneration/no_output_schema`
- **Break (6):** a path outside /api/accounts/{account_id}/ generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_outside_the_account`
- **Break (7):** a path variable generates whether or not the input declares it a required string
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/account_id_declared_as_a_number`, `TestAOneSidedEntryFailsGeneration/account_id_declared_but_optional`, `TestAOneSidedEntryFailsGeneration/an_input_schema_without_account_id`, `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_variable_not_in_the_schema`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_variable_that_is_not_a_string`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_variable_that_is_optional`
- **Break (8):** two operations generate under one name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/a_name_repeated`
- **Break (9):** two operations on one route generate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestARepeatedRouteFailsGeneration`
- **Break (10):** a path whose last segment does not take the route shape of the operation's effect generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_change_not_on_a_:verb_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_change_on_the_:search_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_create_on_a_:verb_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_read_on_a_:verb_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_structured_read_not_on_a_:search_route`
- **Break (11):** a GET taking an argument that is not a scalar generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_GET_taking_an_array`, `TestTheSurfaceRefusesWhatItCannotCarry/a_GET_taking_an_object`, `TestTheSurfaceRefusesWhatItCannotCarry/a_GET_taking_an_untyped_argument`
- **Break (12):** every operation fails generation, the whole ones included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestACallHoldingTooManyArgumentsIsRefused`, `TestAReadMayFilterOnStatus`, `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestEachEffectDerivesItsRow`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheSurfaceHasNoApprovalVocabulary`, `TestWholeOperationsGenerate`

## An operation's HTTP method and its four MCP annotations are derived from its effect class

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the MCP root registers tools with no annotations
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheToolListIsTheRegistry`
- **Break (2):** the MCP root leaves the destructive hint nil, so the listing omits it and a client reads every tool as destructive
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheToolListIsTheRegistry`
- **Break (3):** the MCP root leaves the open-world hint nil, so the listing omits it and a client reads every tool as open-world
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheToolListIsTheRegistry`
- **Break (4):** a create derives idempotent true
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheToolListIsTheRegistry`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachEffectDerivesItsRow`
- **Break (5):** a disposal derives destructive false
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheToolListIsTheRegistry`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachEffectDerivesItsRow`
- **Break (6):** the read-only hint is derived from the HTTP method rather than the effect, so a structured read over POST is not read-only
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheToolListIsTheRegistry`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachEffectDerivesItsRow`
- **Break (7):** a structured read derives GET
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`, `TestDocument`, `TestTheDocumentPlacesEachArgument`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_structured_read`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachEffectDerivesItsRow`

## Approval is not in the client surface's vocabulary

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the registry carries an operation that approves a plan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheSurfaceHasNoApprovalVocabulary`

## Both roots hand the service layer the same argument object

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the API root passes a body member given twice on to the service layer rather than refusing it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`
- **Break (2):** the accounts listing takes an undeclared account_id from its query string
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_accounts_listing's_query_string`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_accounts_listing's_query_string`
- **Break (3):** the API root takes a path variable as its escaped text rather than its value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_every_typed_query_parameter`, `TestTheRootRebuildsEachOperationsArguments/an_account_holding_a_slash`, `TestTheRootRebuildsEachOperationsArguments/an_account_outside_ASCII`
- **Break (4):** the API root leaves the path's variables other than the account out of the argument object
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_every_typed_query_parameter`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_none`, `TestTheRootRebuildsEachOperationsArguments/a_sub-resource_read`, `TestTheRootRebuildsEachOperationsArguments/an_account_holding_a_slash`, `TestTheRootRebuildsEachOperationsArguments/an_account_outside_ASCII`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`
- **Break (5):** the API root takes a declared argument of a POST from the query string, a second location beside the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_declared_argument_in_a_change's_query_string`
- **Break (6):** the API root reads every query parameter as text, whatever the input declares
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestBothRootsHandTheServiceTheSameObject`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_every_typed_query_parameter`, `TestTheRootRebuildsEachOperationsArguments/a_sub-resource_read`, `TestTheRootRebuildsEachOperationsArguments/the_accounts_listing`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_boolean_written_as_a_number`, `TestTheRootRefusesWhatItCannotBind/a_number_that_is_not_finite`, `TestTheRootRefusesWhatItCannotBind/an_integer_that_is_not_one`
- **Break (7):** keys are compared lower-cased, so two keys strings.EqualFold holds equal but whose lower-case forms differ are both admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_case-variant_key_beside_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/account_id_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_variant_of_it_under_Unicode_case_folding`
- **Break (8):** arguments holding a key twice are refused only when both spellings match exactly
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/account_id_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_case_variant_of_it`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_variant_of_it_under_Unicode_case_folding`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/the_accounts_listing_with_an_argument_given_twice`

## Both roots serve exactly the registry's operations

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the API root also serves each operation by POST at /api/ followed by its name, a route the registry does not carry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/an_operation's_name_as_a_path`
- **Break (2):** the MCP root registers every operation but the registry's last
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestBothRootsCarryExactlyTheRegistry`, `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestABatchIsRefusedAtEveryRevision`, `TestTheToolListIsTheRegistry`

## Each body request hands its account's credential over and records the latest authentication attempt when it ends

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a body request's unit of work ends without recording the attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAStaleWriteBackLeavesTheReauthorization`, `TestEachBodyRequestWritesBackAndRecordsTheAttempt`
- **Break (2):** a body request's unit of work writes back the credential it began from rather than the one its token holds, so a rotation is never written back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARotationWhoseWriteBackFailedIsKept`, `TestEachBodyRequestWritesBackAndRecordsTheAttempt`
- **Break (3):** a body request's unit of work writes its token's credential back without checking the adoption it began from, so a credential replaced while it ran is put back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAStaleWriteBackLeavesTheReauthorization`

## Each failure names its origin, with the status the origin gives, and no detail of an API server or provider failure reaches the client

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the API root answers a provider failure with 500, as it answers a fault of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestEachFailureNamesItsOrigin`
- **Break (2):** the API root answers an argument refusal with 500, as it answers a fault of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_body`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_query_string`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_the_API_server_does_not_serve`
- **Break (3):** a failure inside the API server carries its error's own text, an internal detail included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestAToolCallRunsTheOperation`
- **Break (4):** a provider call's failure is returned as it is, so it reads as the API server's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestEachOriginIsToldApartOnBothRoots`
- **Break (5):** a provider failure's message carries the provider's own error text
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestEachFailureNamesItsOrigin`
- **Break (6):** an argument refusal is classified as the API server's failure, which says only that the call failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestAFailedCallIsLoggedAtItsOriginsLevel`, `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestAFailedCallIsLoggedAtItsOriginsLevel`, `TestAServedToolTakesOnlyTheArgumentsItDeclares`, `TestAToolCallRunsTheOperation`
- **Break (7):** the registry's refusals of a call are classified as the API server's failures
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_body`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_query_string`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_the_API_server_does_not_serve`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestAToolCallRunsTheOperation`

## Enumeration and counting reach the whole corpus, not a recent window

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the search's next cursor names its page's first row, so the next page serves the same messages again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`
- **Break (2):** the search returns no next cursor, so only its first page is ever served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`
- **Break (3):** the search selects only the messages of the last five years
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEnumerationReachesTheWholeCorpus`
- **Break (4):** the summary counts only the messages of the last five years
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEnumerationReachesTheWholeCorpus`

## Every body serve and denial writes its audit row, and nothing is released without it

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the policy rule behind the sender class is audited among the content rules, as one array held both
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`
- **Break (2):** every class rule is audited in the base scope, whichever scope holds it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestADenialNamesTheScopeOfTheRuleBehindIt`
- **Break (3):** a body the conversion refused is audited as though the serve-time pattern check ran
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyTheConversionRefusesIsDenied`
- **Break (4):** a gate denial writes no audit row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`
- **Break (5):** a release whose audit write fails is served all the same
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAReasonTheAuditCheckDoesNotKnowReleasesNothing`, `TestEachOriginIsToldApartOnBothRoots`, `TestNoBodyIsReleasedWithoutItsAuditRow`
- **Break (6):** no audit row records the serve-time pattern check's run or the scanner it ran under
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyPastTheBoundIsDeniedAsRefused`, `TestAReleasedBodyIsCleanMarkdown`, `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (7):** a serve-time denial is audited as a read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyTheConversionRefusesIsDenied`, `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (8):** a serve-time denial's audit row names the gate stage
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyTheConversionRefusesIsDenied`, `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (9):** the serve-time pattern check's rules are audited among the content rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheServeTimeCheckReadsEverythingReleased`

## Every message a read operation serves follows the redaction matrix, decided by the Redaction Gate under the policy in force

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a served message carries its attachment types only when its body is available, as its filenames would
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (2):** a served message carries its attachments' stored media types as its types, never passing them through the mapping
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (3):** no served message carries its attachment types
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (4):** every served message says its body is available, whatever the gate decided
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`, `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (5):** the gate decides every served message under no policy, so it restricts every sender instead of the ones the policy lists
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryMessageStaysInEveryIndexRead`, `TestEveryServedMessageFollowsTheRedactionMatrix`, `TestTheDomainAndClassTermsReadTheStoredForm`
- **Break (6):** every served message is shown with a normal sender, whatever the gate classified
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryServedMessageFollowsTheRedactionMatrix`, `TestTheDomainAndClassTermsReadTheStoredForm`
- **Break (7):** a stored content flag the schema does not name is read as no flag, so the message it marks is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestAnUnnamedStoredStateReadsAsTheMostRestrictive`
- **Break (8):** a stored scan state the schema does not name is read as scanned, so the message is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestAnUnnamedStoredStateReadsAsTheMostRestrictive`, `TestTheStoredNamesReadBackAsWritten`

## Every request to the client surface passes the bearer check before either root

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a token file holding no token admits a request whose bearer token is empty
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestEveryRequestPassesTheBearerCheckFirst`
- **Break (2):** a request carrying no Authorization header is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestEveryRequestPassesTheBearerCheckFirst`
- **Break (3):** a presented token is admitted when it begins with the token rather than equals it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestEveryRequestPassesTheBearerCheckFirst`
- **Break (4):** the MCP endpoint is served without the bearer check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAPathOutsideBothRootsIsNotFound`

## Every response of the client surface is uncacheable, and HEAD is refused

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a HEAD request is served as the GET it matches
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/HEAD_on_a_read`, `TestTheRootRefusesWhatItCannotBind/HEAD_on_the_accounts_listing`
- **Break (2):** a response whose handler writes only its body keeps the caching headers the handler set
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestEveryResponseIsUncacheable`
- **Break (3):** a handler's ETag is kept
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestEveryResponseIsUncacheable`
- **Break (4):** a response whose handler writes nothing carries no Cache-Control
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestEveryResponseIsUncacheable`
- **Break (5):** each endpoint's handler is wrapped in the bearer check without the no-store wrapper
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestEveryResponseIsUncacheable`

## Every timestamp the client surface serves is UTC with the Z suffix

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a timestamp is written in the zone it was read in, with the Z suffix all the same
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEveryTimestampServedIsUTC`
- **Break (2):** a timestamp is written in UTC with the offset +00:00 in place of Z
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestEachCountCountsEveryThreadOnce`, `TestEachQueryTermSelectsItsMessages`, `TestEnumerationReachesTheWholeCorpus`, `TestEveryMessageStaysInEveryIndexRead`, `TestEveryTimestampServedIsUTC`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`, `TestTheStatusReadsTheLatestSucceededTick`, `TestTheSystemStatusCarriesOperationalStateOnly`

## Only the body operation is handed the functions that reach a provider

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an account the accounts listing returns carries a provider session, which every handler that lists the accounts holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestOnlyTheBodyOperationIsHandedAWayToAProvider`
- **Break (2):** Operations hands Bodies to the builder of the index operations as well as to the body operation's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestOnlyTheBodyOperationIsHandedAWayToAProvider`
- **Break (3):** the body operation parks the provider session it opened in a package-level variable, where every other handler can read it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestOnlyTheBodyOperationIsHandedAWayToAProvider`
- **Break (4):** the operations' sources hold Bodies again, so every operation's handler holds the functions that begin a unit of work
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestOnlyTheBodyOperationIsHandedAWayToAProvider`

## Served and denied bodies are counted, a denial by the stage that decided it

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a serve-time denial is counted as a gate denial
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyTheConversionRefusesIsDenied`, `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (2):** a served body is not counted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAReleasedBodyIsCleanMarkdown`

## The API server is ready only while its bearer token file holds a token

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a token file holding no token counts as holding one, at the start and in the readiness report
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestATokenFileThatAdmitsNoClientRefusesTheStart`, `TestATokenFileThatAdmitsNoClientRefusesTheStart/holding_no_token`, `TestTheReportFollowsTheTokenFile`
- **Break (2):** a token file that cannot be read is reported ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestTheReportFollowsTheTokenFile`
- **Break (3):** the report is not made until the first read returns, so a process whose first read has not returned is ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestTheReportIsNotReadyUntilItReadsTheFile`
- **Break (4):** the readiness report reports ready without reading the token file, so an API server whose token file was emptied or removed stays ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestTheReportFollowsTheTokenFile`
- **Break (5):** the readiness report's loop is never started, so the report stays at its first, not ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestTheReportFollowsTheTokenFile`
- **Break (6):** each endpoint that asks starts a report of its own, so two loops read the file and log each failed read twice
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestBothEndpointsShareOneReport`, `TestTheReportIsNotReadyUntilItReadsTheFile`
- **Break (7):** the first endpoint built starts whatever its token file holds, so a process that admits no client starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/bearer`:** `TestATokenFileThatAdmitsNoClientRefusesTheStart`, `TestATokenFileThatAdmitsNoClientRefusesTheStart/absent`, `TestATokenFileThatAdmitsNoClientRefusesTheStart/holding_no_token`, `TestTheReportIsNotReadyUntilItReadsTheFile`

## The API server loads the policy of the accounts it serves, and of none when it serves none

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the application user, which the API server connects as, is not granted the read of the policy rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyPastTheBoundIsDeniedAsRefused`, `TestABodyTheConversionRefusesIsDenied`, `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestAFailedPolicyLoadStillFollowsTheList`, `TestAFirstReloadThatFailsLeavesTheAPIServerStartedAndNotReady`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestAPathOutsideBothRootsIsNotFound`, `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestAReasonTheAuditCheckDoesNotKnowReleasesNothing`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`, `TestARereadAfterAMoveBuildsTheTokenFromTheNewClient`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAServedBodyFetchesNoImage`, `TestAStaleWriteBackLeavesTheReauthorization`, `TestAnAccountNamingNoClientIsNotConnected`, `TestAnAccountWithoutAStoredCredentialIsNotConnected`, `TestEachBodyRequestWritesBackAndRecordsTheAttempt`, `TestEachOriginIsToldApartOnBothRoots`, `TestEveryReloadLoadsThePolicy`, `TestNoBodyIsReleasedWithoutItsAuditRow`, `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`, `TestTheReadOperationsReadTheSession`, `TestTheServeTimeCheckReadsEverythingReleased`, `TestTheSurfaceGivenNoKeyPairIsPlain`, `TestTheSurfaceGivenNoKeyPairIsPlain/both_blank`, `TestTheSurfaceGivenNoKeyPairIsPlain/neither_file`, `TestTheSurfaceIsServedOverTLSWhenGivenAKeyPair`
- **Break (2):** the policies' reload-failure series is registered on a registry the API server does not serve, so its metrics endpoint never carries the alarm's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAFailedPolicyLoadStillFollowsTheList`, `TestANewlyListedDomainIsDeniedOnTheNextCall`
- **Break (3):** the policy is loaded even with no account in force, which the loader refuses, so the Session never loads and the API server serving no account is never ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestReadyOnlyWhileTheSessionHasLoadedAndTheTokenLoads`, `TestTheConfiguredLevelGovernsTheLog`, `TestTheDriversVariablesDoNotMoveTheConnection`, `TestTheMCPRootReportsTheReleaseVersion`

## The API server refuses to start while MCPGODEBUG is set

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the process runs the spec's Init and ignores its refusal, so the API server starts under MCPGODEBUG
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestMCPGODEBUGStopsTheStart`
- **Break (2):** the API server starts whatever MCPGODEBUG holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestMCPGODEBUGStopsTheStart`
- **Break (3):** an MCPGODEBUG set to the empty string is taken as unset
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestMCPGODEBUGStopsTheStart`

## The API server serves the accounts each reload of its Session puts in force, and keeps them when a reload's read fails

- **Date · evidence:** 2026-11-19 · [pull request #479](https://github.com/ppat/mediated-mailbox-mcp/pull/479), demonstrated with `go tool mutproof` on its branch
- **Break (1):** a reload whose policy load fails returns before telling the registry, so an account it dropped stays served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAFailedPolicyLoadStillFollowsTheList`
- **Break (2):** a reload loads the policy only when its accounts differ from the ones the last load was tried for, so a load that failed is never retried and a policy edit never reaches the metadata reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAFailedPolicyLoadStillFollowsTheList`, `TestEveryReloadLoadsThePolicy`
- **Break (3):** a reload whose read of the list fails puts no account in force and tells the registry, so it stops serving every account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheAPIServerServesTheAccountsEachReloadLists`
- **Break (4):** the policy is loaded by the first reload that loads it only, so an account a later reload lists is served under no policy of its own and a policy edit never reaches the metadata reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAFailedPolicyLoadStillFollowsTheList`, `TestEveryReloadLoadsThePolicy`, `TestTheAPIServerServesTheAccountsEachReloadLists`
- **Break (5):** the registry keeps the first set of accounts it was given, whatever a reload serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyPastTheBoundIsDeniedAsRefused`, `TestABodyTheConversionRefusesIsDenied`, `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestAFailedPolicyLoadStillFollowsTheList`, `TestAFirstReloadThatFailsLeavesTheAPIServerStartedAndNotReady`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestAReasonTheAuditCheckDoesNotKnowReleasesNothing`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`, `TestARereadAfterAMoveBuildsTheTokenFromTheNewClient`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAServedBodyFetchesNoImage`, `TestAStaleWriteBackLeavesTheReauthorization`, `TestAnAccountNamingNoClientIsNotConnected`, `TestAnAccountWithoutAStoredCredentialIsNotConnected`, `TestEachBodyRequestWritesBackAndRecordsTheAttempt`, `TestEachOriginIsToldApartOnBothRoots`, `TestEveryReloadLoadsThePolicy`, `TestNoBodyIsReleasedWithoutItsAuditRow`, `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bank`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bankcode`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-flagged`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-oldrestricted`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-pending`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-unreadable`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`, `TestTheReadOperationsReadTheSession`, `TestTheServeTimeCheckReadsEverythingReleased`, `TestTheSurfaceGivenNoKeyPairIsPlain`, `TestTheSurfaceGivenNoKeyPairIsPlain/both_blank`, `TestTheSurfaceGivenNoKeyPairIsPlain/neither_file`, `TestTheSurfaceIsServedOverTLSWhenGivenAKeyPair`
- **Break (6):** a reload putting no account in force keeps the accounts served before it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheAPIServerServesTheAccountsEachReloadLists`
- **Break (7):** a reload puts its accounts in force and never hands them to the registry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyPastTheBoundIsDeniedAsRefused`, `TestABodyTheConversionRefusesIsDenied`, `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestAFailedPolicyLoadStillFollowsTheList`, `TestAFirstReloadThatFailsLeavesTheAPIServerStartedAndNotReady`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestAReasonTheAuditCheckDoesNotKnowReleasesNothing`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARequestWaitsForItsLeasesNoLongerThanItsDeadline`, `TestARereadAfterAMoveBuildsTheTokenFromTheNewClient`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAServedBodyFetchesNoImage`, `TestAStaleWriteBackLeavesTheReauthorization`, `TestAnAccountNamingNoClientIsNotConnected`, `TestAnAccountWithoutAStoredCredentialIsNotConnected`, `TestEachBodyRequestWritesBackAndRecordsTheAttempt`, `TestEachOriginIsToldApartOnBothRoots`, `TestEveryReloadLoadsThePolicy`, `TestNoBodyIsReleasedWithoutItsAuditRow`, `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bank`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bankcode`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-flagged`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-oldrestricted`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-pending`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-unreadable`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`, `TestTheReadOperationsReadTheSession`, `TestTheServeTimeCheckReadsEverythingReleased`, `TestTheSurfaceGivenNoKeyPairIsPlain`, `TestTheSurfaceGivenNoKeyPairIsPlain/both_blank`, `TestTheSurfaceGivenNoKeyPairIsPlain/neither_file`, `TestTheSurfaceIsServedOverTLSWhenGivenAKeyPair`

## The API server's metrics endpoint carries F3's rate-state series for each account it serves through the Gmail adapter

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the rate-state collector is built and never registered on the metrics endpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyPastTheBoundIsDeniedAsRefused`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`
- **Break (2):** an account of a provider no shipped adapter serves is reported, with no ceiling
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheMetricsEndpointCarriesEachServedAccountsRateState`

## The API server's token is made from the OAuth client the account connects through and the account's stored credential, and from nothing else

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the client's identifier and secret are swapped when the token is made
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (2):** every account is served with the OAuth client of the lowest identifier among those that opened rather than its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (3):** an account whose state row holds no credential takes one from the environment
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAnAccountWithoutAStoredCredentialIsNotConnected`
- **Break (4):** the re-read after a refusal makes the token with the client the refused one was made with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestARereadAfterAMoveBuildsTheTokenFromTheNewClient`

## The chart renders the API server a file it starts with, holding every API server key the chart admits

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the schema admits a key in the API server's own block that the API server does not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheChartAdmitsEveryAPIServerKeyItDoesNotRender`
- **Break (2):** the chart leaves the API server's pool size out of its file though the values set one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheChartRendersAFileTheAPIServerStartsWith`
- **Break (3):** the chart leaves the TLS files out of the API server's file though their Secret is named
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheChartRendersAFileTheAPIServerStartsWith`
- **Break (4):** the chart writes the client surface's address under a key the API server does not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheChartRendersAFileTheAPIServerStartsWith`

## The configuration surface accepts no value that disables the gate, skips masking or weakens deny-by-default

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the configuration gains a value switching the gate, which a flag then sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestNoConfigurationValueWeakensTheGate`, `TestNoConfigurationValueWeakensTheGate/a_flag_disabling_the_gate`, `TestTheChartAdmitsEveryAPIServerKeyItDoesNotRender`, `TestTheEffectiveConfigurationIsLogged`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheProgramTypesArePinned`
- **Break (2):** a scanner part the scanner refuses is accepted, so the serve-time check runs with nothing to match
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestNoConfigurationValueWeakensTheGate`, `TestNoConfigurationValueWeakensTheGate/a_scanner_part_with_nothing_to_match`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheAPIConfigurationIsChecked`, `TestTheAPIConfigurationIsChecked/a_scanner_with_nothing_to_match`

## The gate decides every body request from the index, and a denial never reaches the provider

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the body is fetched from the provider before the gate decides, and dropped on a denial
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyPastTheBoundIsDeniedAsRefused`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`
- **Break (2):** a body the gate denies is released all the same
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestADenialNamesTheScopeOfTheRuleBehindIt`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bank`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bankcode`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-flagged`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-oldrestricted`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-pending`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-unreadable`

## The MCP root offers tools and nothing else

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the server is built with no capabilities declared, so the SDK's default capabilities are advertised
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheRootOffersToolsOnly`
- **Break (2):** requests for prompts, resources and a log level reach the SDK, which answers them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheRootOffersToolsOnly`
- **Break (3):** requests for resources reach the SDK, which answers them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheRootOffersToolsOnly`
- **Break (4):** the MCP root also offers a prompt, surface the API root does not carry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestTheRootOffersToolsOnly`

## The MCP root refuses a JSON-RPC batch before any of its calls runs, at every revision

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the check reads the first byte as it comes, so whitespace before the bracket hides a batch
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestABatchIsRefusedAtEveryRevision`
- **Break (2):** the root hands every request to the SDK, which runs a batch at a revision before 2025-06-18
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestABatchIsRefusedAtEveryRevision`

## The read operations read the accounts and the policy the Session holds in force, and a clock within thirty seconds of the real one when the read is made

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the read operations are given an accounts listing that lists none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheReadOperationsReadTheSession`
- **Break (2):** the read operations are given a clock ten minutes behind the real one, so a sync cursor written an hour ago reads fifty minutes old
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheReadOperationsReadTheSession`
- **Break (3):** the read operations are given a clock stopped at the zero time, behind every recorded instant, so a backoff that ended an hour ago reads as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheReadOperationsReadTheSession`
- **Break (4):** the read operations are given a clock two hours ahead of the real one, so a backoff recorded to end within the hour reads as over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheReadOperationsReadTheSession`
- **Break (5):** the read operations are given no policy, so every sender reads restricted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAFailedPolicyLoadStillFollowsTheList`, `TestEveryReloadLoadsThePolicy`, `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheReadOperationsReadTheSession`

## The registry refuses at build time anything that could express approval or bind an argument to a header

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a schema carrying x-mcp-header generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/x-mcp-header_in_the_input`, `TestTheSurfaceRefusesWhatItCannotCarry/x-mcp-header_in_the_output`
- **Break (2):** an operation name holding a lifecycle stem generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_applying`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_approving`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approval`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approved`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_roll_back`
- **Break (3):** a path segment or :verb holding a lifecycle stem generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_:verb_approving`, `TestTheSurfaceRefusesWhatItCannotCarry/a_:verb_holding_approve`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_applying`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_holding_approve`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_holding_roll-back`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_rolling_back`
- **Break (4):** a lifecycle word is refused only as a whole token, so approved, approval, applied and approve-all generate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approval`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approved`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_applying`
- **Break (5):** roll_back and roll-back generate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_roll_back`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_holding_roll-back`
- **Break (6):** status is refused on a read too, so a read cannot filter on a plan's state
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAReadMayFilterOnStatus`
- **Break (7):** a write's input may declare status
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/status_in_another_case_on_a_reversible_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_array's_items_on_a_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_object_on_a_disposal`, `TestTheSurfaceRefusesWhatItCannotCarry/status_on_a_create`
- **Break (8):** status is refused only as a top-level property spelled in lower case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/status_in_another_case_on_a_reversible_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_array's_items_on_a_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_object_on_a_disposal`
- **Break (9):** an effect the table has no row for derives PATCH instead of failing generation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/an_effect_with_no_row`, `TestTheSurfaceRefusesWhatItCannotCarry/no_effect`

## The release step withholds a body it cannot decide on

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a gate-skipped body with no scanner to check it is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`, `TestFailClosed`, `TestFailClosed/a_gate-skipped_body_and_a_scanner_nobody_built`, `TestFailClosed/a_gate-skipped_login_link_and_a_scanner_nobody_built`
- **Break (2):** the release step reads the sender class and the content flags but not the scan state, so a scan state the gate never releases falls through to the pattern check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestFailClosed`, `TestFailClosed/a_pending_scan`, `TestFailClosed/refused`, `TestNoBodyCarriesADenyingSensitivity`
- **Break (3):** the zero Verdict reports a released body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestFailClosed`, `TestFailClosed/the_zero_verdict`

## The sender class an index read selects, groups and lists by is the Redaction Gate's under the policy in force

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the class term passes the domains classified restricted and counts every other domain as normal, so a domain the call did not classify fails open
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryMessageStaysInEveryIndexRead`, `TestTheDomainAndClassTermsReadTheStoredForm`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (2):** a grouping by sender class counts both classes whatever class the query selects, so a group holds messages outside the selection
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (3):** the sender class term is never resolved to the domains classified normal, so it selects every message
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryMessageStaysInEveryIndexRead`, `TestTheDomainAndClassTermsReadTheStoredForm`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (4):** the transaction helper opens an index read's transaction at the default isolation level, so each statement reads the state committed when it starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`
- **Break (5):** the search runs its statements outside the snapshot, so each reads the state committed when it starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`
- **Break (6):** the sender listing shows every sender domain as normal, whatever the policy classifies it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheDomainAndClassTermsReadTheStoredForm`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (7):** the sender listing classifies each domain under no policy, which restricts every sender, instead of the policy the call took
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheDomainAndClassTermsReadTheStoredForm`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (8):** the index reads classify the sender domains under no policy, which restricts every sender, instead of the policy the call took
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryMessageStaysInEveryIndexRead`, `TestTheDomainAndClassTermsReadTheStoredForm`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`

## The serve-time pattern check reads the snippet and the attachment filenames that follow a gate-skipped body, and the filenames of every message, and they are released with the delimiters' words replaced

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the snippet and the filenames are released as they arrived, with the delimiters' words left in them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestTheCheckReadsEverythingReleased`, `TestTheWindowedNeutralizerReplacesWhatTheFullScanDid`
- **Break (2):** the check reads the body and the snippet and not the filenames
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_one-time_code_in_a_filename`
- **Break (3):** a pattern match withholds the content without naming the rules that matched
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`, `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_code_in_the_body_and_a_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_login_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_one-time_code_in_a_filename`
- **Break (4):** a scanned message's filenames are released without the pattern check, as its body and snippet are
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`
- **Break (5):** the check reads the body and the filenames and not the snippet
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/core/release`:** `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_login_link_in_the_snippet`

## The service layer refuses an operation whose account is missing or unknown, except the accounts listing

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the API root drops an account_id given in the body, so the path's account is taken without a check for a second one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`
- **Break (2):** the API root drops an account_id given in the query string, so the path's account is taken without a check for a second one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_accounts_listing's_query_string`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_query_string`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_accounts_listing's_query_string`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`
- **Break (3):** a key differing from account_id only in case, given alone, is passed on to the operation rather than refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_case-variant_key_alone`
- **Break (4):** the accounts listing is put through the account check too
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/the_accounts_listing_with_no_account`
- **Break (5):** a call carrying no account_id string reaches the operation, with an empty account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_null_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_id_that_is_not_a_string`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_empty_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/no_account_id`
- **Break (6):** every operation runs as the accounts listing does, with no account check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_case-variant_key_alone`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_null_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account_and_nothing_else`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_differing_only_in_case`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_id_that_is_not_a_string`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_the_API_server_does_not_serve`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_empty_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/no_account_id`
- **Break (7):** the operation receives the arguments as sent, account_id included, so it can read an account other than the one checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account_and_nothing_else`
- **Break (8):** arguments holding a key twice, exactly or in another case, the account included, reach the operation on either root
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/mcp`:** `TestARepeatedArgumentIsRefused`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestACallHoldingTooManyArgumentsIsRefused`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/account_id_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_case_variant_of_it`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_variant_of_it_under_Unicode_case_folding`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_given_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/arguments_that_are_not_an_object`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/the_accounts_listing_with_an_argument_given_twice`
- **Break (9):** a call whose account_id names no served account reaches the operation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_differing_only_in_case`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_the_API_server_does_not_serve`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_empty_account_id`

## The serving API server reloads its account snapshot on the interval its configuration sets

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the Session's reload is scheduled a day apart whatever interval the configuration sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAFailedPolicyLoadStillFollowsTheList`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAStaleWriteBackLeavesTheReauthorization`, `TestEveryReloadLoadsThePolicy`, `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`
- **Break (2):** each tick of the schedule after the first load passes without a reload
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAFailedPolicyLoadStillFollowsTheList`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAStaleWriteBackLeavesTheReauthorization`, `TestEveryReloadLoadsThePolicy`, `TestTheAPIServerServesTheAccountsEachReloadLists`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`

## The snippet and attachment filenames follow the body, and a denial releases none of them

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a serve-time denial carries the snippet it withheld
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (2):** a released body is served without its snippet
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestAReleasedBodyIsCleanMarkdown`, `TestTheServeTimeCheckReadsEverythingReleased`

## The system status carries recorded operational state only

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the status reads a message and shows its subject as the last authentication's outcome
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/internal/service`:** `TestTheSystemStatusCarriesOperationalStateOnly`
