# Mutations: mediate

The demonstrations of the controls whose patches sit in `mediate/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A body request has the policy loaded by a load that started after it arrived, sharing a load only with the requests waiting when it started

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235)
- **Break (1):** each caller runs a load of its own, so a burst of callers runs a burst of loads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/reload`:** `TestACallerThatStopsWaitingLeavesTheLoadRunning`, `TestCallersWaitingTogetherShareOneLoad`, `TestLoadsRunOneAtATime`
- **Break (2):** a load runs under the context of the caller that started it, so a caller that stops waiting cancels the load the others wait on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/reload`:** `TestACallerThatStopsWaitingLeavesTheLoadRunning`
- **Break (3):** a caller asking while a load runs is answered by that load
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/reload`:** `TestACallerThatStopsWaitingLeavesTheLoadRunning`, `TestALoadRunningWhenACallerAsksDoesNotCount`, `TestCallersWaitingTogetherShareOneLoad`

## A body request is decided under the policy loaded after it arrived, so a newly listed domain is denied on the next call

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the composition root hands the body operation the active policy and loads nothing for the request
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestANewlyListedDomainIsDeniedOnTheNextCall`
- **Break (2):** a request whose policy load fails is decided under the policy that restricts every sender rather than the active valid one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestANewlyListedDomainIsDeniedOnTheNextCall`
- **Break (3):** the body operation takes the active policy without having it loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestANewlyListedDomainIsDeniedOnTheNextCall`

## A body request's provider calls lease in the interactive class, and a gate denial leases nothing

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the body request's provider calls take no lease
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`
- **Break (2):** the body request's provider calls lease in the batch class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`

## A body with no HTML part, the snippet and each filename are released as fenced code blocks showing the text exactly

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a body with no HTML part is released as its text part, unfenced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAReleasedBodyIsCleanMarkdown`
- **Break (2):** the snippet is released as the provider gave it, unfenced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAReleasedBodyIsCleanMarkdown`
- **Break (3):** each filename is released as the provider gave it, unfenced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAReleasedBodyIsCleanMarkdown`

## A call's arguments hold at most 1024 top-level keys

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break:** arguments holding any number of keys reach the operation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestACallHoldingTooManyArgumentsIsRefused`

## A failed call answers with the service layer's structured failure on both roots

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186)
- **Break:** an MCP error result carries the failure as text only, with no structured content
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestAServedToolTakesOnlyTheArgumentsItDeclares`, `TestAToolCallRunsTheOperation`

## A gate-skipped body the structural patterns match is withheld at serve time

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the serve-time check runs the full scanner, tier 2's scoring included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestServeTimePatternCheck`, `TestServeTimePatternCheck/a_gate-skipped_code_only_tier_2_catches`
- **Break (2):** a pattern match on a gate-skipped body no longer withholds it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`, `TestServeTimePatternCheck`, `TestServeTimePatternCheck/a_gate-skipped_login_link`, `TestServeTimePatternCheck/a_gate-skipped_one-time_code`, `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_code_in_the_body_and_a_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_login_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_one-time_code_in_a_filename`

## A listing's cursor continues only the listing, account and filter it came from, and paging serves every row once

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again, (9) to (13) for the index reads, on 2026-10-02, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a cursor another account's listing returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`, `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (2):** a cursor a listing with another filter returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`, `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (3):** a cursor another listing returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`, `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (4):** the thread listing returns no next cursor on a full page, so the rows after it are never served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (5):** the masking-events listing's next page starts after the last event's time alone, so an event sharing that time is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (6):** the message listing's next cursor names the page's first row instead of its last
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (7):** the message listing's next page starts after the last row's time alone, so a row sharing that time is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (8):** the thread listing's next page starts after the last thread's latest time alone, so a thread sharing that time is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAListingPagesThroughEveryRowOnce`
- **Break (9):** a search's cursor is bound to its sort and order but not its query, so a cursor another query returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (10):** a count's cursor is bound to its query but not its grouping, so a cursor another grouping returned is taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (11):** a page of the groups the service layer pages starts after the last group's count alone, so a group tied with it is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`
- **Break (12):** a search sorted by sender continues from its last row's message identifier inclusive, so that row is served again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`
- **Break (13):** the sender listing's next page starts after the last sender's count alone, so a sender tied with it is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`

## A message whose body is denied stays in every count, group and search result of the index reads

- **Date · evidence:** 2026-10-02 · [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the search leaves out every message whose body the gate denies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (2):** the summary of a selection leaves out the messages pending their content scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`
- **Break (3):** the summary counts as a selection's messages only those whose body could be released, scanned with no content flag
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (4):** the groups by sender leave out the messages skipped as restricted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`
- **Break (5):** the groups by label leave out the messages carrying a content flag
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`

## A provider call that does not answer within the provider timeout is the provider's failure

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a provider call has no deadline of the mediator's, so a provider that does not answer holds the request
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`
- **Break (2):** a call whose deadline passed is returned as the context's error, which reads as the mediator's failure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`

## A refused credential is read again from its row before the refusal is reported

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** an account the re-read finds not connected has the call made again over a source built from empty credentials
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`
- **Break (2):** a refusal is reported without reading the credential again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`
- **Break (3):** the call is made again over the credential read again, but later requests go back to the refused one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`

## A released body sits inside delimiters it cannot close

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the delimiters' words are matched in lower case only
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestTheCheckReadsEverythingReleased`, `TestTheReleasedBodysForm`
- **Break (2):** the text after the last replaced stretch is dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_near_miss`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_with_a_word_between_them`, `TestAScannedBodyIsReleasedInTheDelimiters`, `TestServeTimePatternCheck`, `TestServeTimePatternCheck/a_gate-skipped_code_only_tier_2_catches`, `TestServeTimePatternCheck/a_gate-skipped_newsletter`, `TestServeTimePatternCheck/a_gate-skipped_receipt`, `TestTheCheckReadsEverythingReleased`, `TestTheReleasedBodysForm`
- **Break (3):** a released body is not wrapped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_near_miss`, `TestABodyCannotCloseTheDelimiters/a_space_between_every_letter`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/lower_case`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestABodyCannotCloseTheDelimiters/the_words_with_a_word_between_them`, `TestAScannedBodyIsReleasedInTheDelimiters`, `TestServeTimePatternCheck`, `TestServeTimePatternCheck/a_gate-skipped_code_only_tier_2_catches`, `TestServeTimePatternCheck/a_gate-skipped_newsletter`, `TestServeTimePatternCheck/a_gate-skipped_receipt`, `TestTheCheckReadsEverythingReleased`, `TestTheReleasedBodysForm`
- **Break (4):** text spelling the delimiters is released as it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_space_between_every_letter`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/lower_case`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestTheReleasedBodysForm`
- **Break (5):** characters between the letters break a match
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestABodyCannotCloseTheDelimiters`, `TestABodyCannotCloseTheDelimiters/Markdown_emphasis_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_line_break_between_the_words`, `TestABodyCannotCloseTheDelimiters/a_space_between_every_letter`, `TestABodyCannotCloseTheDelimiters/a_zero-width_space_between_the_words`, `TestABodyCannotCloseTheDelimiters/lower_case`, `TestABodyCannotCloseTheDelimiters/mixed_case_and_a_hyphen`, `TestABodyCannotCloseTheDelimiters/the_closing_line`, `TestABodyCannotCloseTheDelimiters/the_long_s,_which_folds_to_s`, `TestABodyCannotCloseTheDelimiters/the_opening_line`, `TestABodyCannotCloseTheDelimiters/the_words_twice_in_a_row`, `TestTheCheckReadsEverythingReleased`, `TestTheReleasedBodysForm`

## A served body fetches nothing and holds no image a client could fetch

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** images are no longer on the conversion's list of dropped elements, so a served body keeps a remote image
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAReleasedBodyIsCleanMarkdown`, `TestAServedBodyFetchesNoImage`
- **Break (2):** the mediator itself requests each image source in the HTML it fetched before converting it, so serving a body reaches the URL the message names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAServedBodyFetchesNoImage`

## A served operation takes only the arguments its input schema declares, in exactly that case, and never as null

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and breaks (1) and (2) again and (3) and (4) for the index query on 2026-10-02, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a key naming no declared argument reaches the decoder, which matches it to one in another case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestAServedToolTakesOnlyTheArgumentsItDeclares`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`
- **Break (2):** a null argument is decoded as absent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestAServedToolTakesOnlyTheArgumentsItDeclares`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`
- **Break (3):** an index query's term naming none the schema declares reaches the decoder, which matches it to one in another case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadTakesOnlyTheTermsItDeclares`
- **Break (4):** a null index query term is decoded as absent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadTakesOnlyTheTermsItDeclares`

## A timestamp the client surface takes with any offset but Z is refused, never converted

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and again on 2026-10-02 with the index query's `after` and `before` added to its tests, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a timestamp with any offset is accepted and converted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestANonUTCTimestampIsRefused`
- **Break (2):** a timestamp written with the offset +00:00 is accepted as UTC
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestANonUTCTimestampIsRefused`

## An operation carrying anything less than both roots need fails generation

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** an operation with no description generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/no_description`
- **Break (2):** an operation generates whatever its input schema holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/an_input_schema_that_is_not_JSON`, `TestAOneSidedEntryFailsGeneration/an_input_schema_that_is_not_an_object's`, `TestAOneSidedEntryFailsGeneration/no_input_schema`
- **Break (3):** an operation with no handler generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/no_handler`
- **Break (4):** an operation generates under any name, one no tool can carry included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/a_name_no_tool_can_carry`, `TestAOneSidedEntryFailsGeneration/a_name_starting_with_a_digit`, `TestAOneSidedEntryFailsGeneration/a_name_too_long_for_a_tool`, `TestAOneSidedEntryFailsGeneration/no_name`
- **Break (5):** an operation generates whatever its output schema holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/an_output_schema_that_is_not_an_object's`, `TestAOneSidedEntryFailsGeneration/no_output_schema`
- **Break (6):** a path outside /api/accounts/{account_id}/ generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_outside_the_account`
- **Break (7):** a path variable generates whether or not the input declares it a required string
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/account_id_declared_as_a_number`, `TestAOneSidedEntryFailsGeneration/account_id_declared_but_optional`, `TestAOneSidedEntryFailsGeneration/an_input_schema_without_account_id`, `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_variable_not_in_the_schema`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_variable_that_is_not_a_string`, `TestTheSurfaceRefusesWhatItCannotCarry/a_path_variable_that_is_optional`
- **Break (8):** two operations generate under one name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAOneSidedEntryFailsGeneration`, `TestAOneSidedEntryFailsGeneration/a_name_repeated`
- **Break (9):** two operations on one route generate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestARepeatedRouteFailsGeneration`
- **Break (10):** a path whose last segment does not take the route shape of the operation's effect generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_change_not_on_a_:verb_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_change_on_the_:search_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_create_on_a_:verb_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_read_on_a_:verb_route`, `TestTheSurfaceRefusesWhatItCannotCarry/a_structured_read_not_on_a_:search_route`
- **Break (11):** a GET taking an argument that is not a scalar generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_GET_taking_an_array`, `TestTheSurfaceRefusesWhatItCannotCarry/a_GET_taking_an_object`, `TestTheSurfaceRefusesWhatItCannotCarry/a_GET_taking_an_untyped_argument`
- **Break (12):** every operation fails generation, the whole ones included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestACallHoldingTooManyArgumentsIsRefused`, `TestAReadMayFilterOnStatus`, `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestEachEffectDerivesItsRow`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheSurfaceHasNoApprovalVocabulary`, `TestWholeOperationsGenerate`

## An operation's HTTP method and its four MCP annotations are derived from its effect class

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the MCP root registers tools with no annotations
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheToolListIsTheRegistry`
- **Break (2):** the MCP root leaves the destructive hint nil, so the listing omits it and a client reads every tool as destructive
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheToolListIsTheRegistry`
- **Break (3):** the MCP root leaves the open-world hint nil, so the listing omits it and a client reads every tool as open-world
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheToolListIsTheRegistry`
- **Break (4):** a create derives idempotent true
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheToolListIsTheRegistry`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEachEffectDerivesItsRow`
- **Break (5):** a disposal derives destructive false
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheToolListIsTheRegistry`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEachEffectDerivesItsRow`
- **Break (6):** the read-only hint is derived from the HTTP method rather than the effect, so a structured read over POST is not read-only
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheToolListIsTheRegistry`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEachEffectDerivesItsRow`
- **Break (7):** a structured read derives GET
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`, `TestDocument`, `TestTheDocumentPlacesEachArgument`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_structured_read`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEachEffectDerivesItsRow`

## Approval is not in the client surface's vocabulary

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and break (1) again on 2026-10-02 with its patch regenerated, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the registry carries an operation that approves a plan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheSurfaceHasNoApprovalVocabulary`
- **Break (2):** the mediator's role is granted the columns a plan's approval writes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMediatorCannotApproveAPlan`
- **Break (3):** the mediator's role is granted inserting plans, an approved one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMediatorCannotApproveAPlan`

## Both roots hand the service layer the same argument object

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the API root passes a body member given twice on to the service layer rather than refusing it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`
- **Break (2):** the accounts listing takes an undeclared account_id from its query string
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_accounts_listing's_query_string`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_accounts_listing's_query_string`
- **Break (3):** the API root takes a path variable as its escaped text rather than its value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_every_typed_query_parameter`, `TestTheRootRebuildsEachOperationsArguments/an_account_holding_a_slash`, `TestTheRootRebuildsEachOperationsArguments/an_account_outside_ASCII`
- **Break (4):** the API root leaves the path's variables other than the account out of the argument object
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_every_typed_query_parameter`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_none`, `TestTheRootRebuildsEachOperationsArguments/a_sub-resource_read`, `TestTheRootRebuildsEachOperationsArguments/an_account_holding_a_slash`, `TestTheRootRebuildsEachOperationsArguments/an_account_outside_ASCII`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`
- **Break (5):** the API root takes a declared argument of a POST from the query string, a second location beside the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_declared_argument_in_a_change's_query_string`
- **Break (6):** the API root reads every query parameter as text, whatever the input declares
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestBothRootsHandTheServiceTheSameObject`, `TestTheRootRebuildsEachOperationsArguments`, `TestTheRootRebuildsEachOperationsArguments/a_read_with_every_typed_query_parameter`, `TestTheRootRebuildsEachOperationsArguments/a_sub-resource_read`, `TestTheRootRebuildsEachOperationsArguments/the_accounts_listing`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_boolean_written_as_a_number`, `TestTheRootRefusesWhatItCannotBind/a_number_that_is_not_finite`, `TestTheRootRefusesWhatItCannotBind/an_integer_that_is_not_one`
- **Break (7):** keys are compared lower-cased, so two keys strings.EqualFold holds equal but whose lower-case forms differ are both admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_case-variant_key_beside_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/account_id_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_variant_of_it_under_Unicode_case_folding`
- **Break (8):** arguments holding a key twice are refused only when both spellings match exactly
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/account_id_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_case_variant_of_it`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_variant_of_it_under_Unicode_case_folding`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/the_accounts_listing_with_an_argument_given_twice`

## Both roots serve exactly the registry's operations

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186)
- **Break (1):** the API root also serves each operation by POST at /api/ followed by its name, a route the registry does not carry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/an_operation's_name_as_a_path`
- **Break (2):** the MCP root registers every operation but the registry's last
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestBothRootsCarryExactlyTheRegistry`, `TestBothRootsHandEveryServedOperationTheSameObject`, `TestBothRootsHandTheServiceTheSameObject`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestARepeatedArgumentIsRefused`, `TestTheToolListIsTheRegistry`

## Each body request hands its account's credential over and records the latest authentication attempt when it ends

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a body request hands over the credential it started from rather than the one its source holds, so a rotation is never written back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARotationWhoseWriteBackFailedIsKept`, `TestEachBodyRequestHandsOverAndRecordsTheAttempt`
- **Break (2):** a body request hands its source's credential over without naming the adoption it started from, so a credential replaced while it ran is put back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAStaleHandOverLeavesTheReauthorization`
- **Break (3):** a body request ends without recording the attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEachBodyRequestHandsOverAndRecordsTheAttempt`

## Each failure names its origin, with the status the origin gives, and no detail of a mediator or provider failure reaches the client

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the API root answers a provider failure with 500, as it answers a fault of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestEachFailureNamesItsOrigin`
- **Break (2):** the API root answers an argument refusal with 500, as it answers a fault of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_body`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_query_string`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_the_mediator_does_not_serve`
- **Break (3):** a failure inside the mediator carries its error's own text, an internal detail included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestAToolCallRunsTheOperation`
- **Break (4):** a provider call's failure is returned as it is, so it reads as the mediator's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestEachOriginIsToldApartOnBothRoots`
- **Break (5):** a provider failure's message carries the provider's own error text
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestEachFailureNamesItsOrigin`
- **Break (6):** an argument refusal is classified as the mediator's failure, which says only that the call failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestAServedToolTakesOnlyTheArgumentsItDeclares`, `TestAToolCallRunsTheOperation`
- **Break (7):** the registry's refusals of a call are classified as the mediator's failures
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestEachFailureNamesItsOrigin`, `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_body`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_query_string`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_the_mediator_does_not_serve`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestAToolCallRunsTheOperation`

## Enumeration and counting reach the whole corpus, not a recent window

- **Date · evidence:** 2026-10-02 · [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the search selects only the messages of the last five years
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEnumerationReachesTheWholeCorpus`
- **Break (2):** the summary counts only the messages of the last five years
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEnumerationReachesTheWholeCorpus`
- **Break (3):** the search returns no next cursor, so only its first page is ever served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`
- **Break (4):** the search's next cursor names its page's first row, so the next page serves the same messages again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadPagesThroughEveryRowOnce`, `TestEnumerationReachesTheWholeCorpus`

## Every body serve and denial writes its audit row, and nothing is released without it

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a release whose audit write fails is served all the same
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEachOriginIsToldApartOnBothRoots`, `TestNoBodyIsReleasedWithoutItsAuditRow`
- **Break (2):** a gate denial writes no audit row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`
- **Break (3):** a serve-time denial is audited as a read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyTheConversionRefusesIsDenied`, `TestTheServeTimeCheckReadsEverythingReleased`

## Every message a read operation serves follows the redaction matrix, decided by the Redaction Gate under the policy in force

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** every served message says its body is available, whatever the gate decided
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`, `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (2):** the gate decides every served message under no policy, so it restricts every sender instead of the ones the policy lists
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryMessageStaysInEveryIndexRead`, `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (3):** every served message is shown with a normal sender, whatever the gate classified
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (4):** a stored content flag the schema does not name is read as no flag, so the message it marks is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryServedMessageFollowsTheRedactionMatrix`
- **Break (5):** a stored scan state the schema does not name is read as scanned, so the message is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryMessageStaysInEveryIndexRead`, `TestEveryServedMessageFollowsTheRedactionMatrix`

## Every request to the client surface passes the bearer check before either root

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a token file holding no token admits a request whose bearer token is empty
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEveryRequestPassesTheBearerCheckFirst`, `TestEveryRequestPassesTheBearerCheckFirst/a_token_file_that_is_absent`, `TestEveryRequestPassesTheBearerCheckFirst/an_empty_token_file`
- **Break (2):** the MCP root is served beside the bearer check rather than behind it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEveryRequestPassesTheBearerCheckFirst`, `TestEveryRequestPassesTheBearerCheckFirst/an_empty_token_file_and_a_token`, `TestEveryRequestPassesTheBearerCheckFirst/the_MCP_root_with_a_wrong_token`, `TestEveryRequestPassesTheBearerCheckFirst/the_MCP_root_without_a_token`
- **Break (3):** a request carrying no Authorization header is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEveryRequestPassesTheBearerCheckFirst`, `TestEveryRequestPassesTheBearerCheckFirst/a_path_outside_both_roots,_unauthenticated`, `TestEveryRequestPassesTheBearerCheckFirst/the_API_root_without_a_token`, `TestEveryRequestPassesTheBearerCheckFirst/the_MCP_root_without_a_token`, `TestTheSurfaceBehindADeclaredIngressIsPlain`
- **Break (4):** a presented token is admitted when it begins with the token rather than equals it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEveryRequestPassesTheBearerCheckFirst`, `TestEveryRequestPassesTheBearerCheckFirst/a_token_with_the_right_one_as_its_prefix`

## Every response of the client surface is uncacheable, and HEAD is refused

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a HEAD request is served as the GET it matches
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/HEAD_on_a_read`, `TestTheRootRefusesWhatItCannotBind/HEAD_on_the_accounts_listing`
- **Break (2):** a response whose handler writes only its body keeps the caching headers the handler set
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEveryResponseIsUncacheable`, `TestNoStoreOverridesAHandlersCaching`
- **Break (3):** a handler's ETag is kept
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestNoStoreOverridesAHandlersCaching`
- **Break (4):** a response whose handler writes nothing carries no Cache-Control
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestNoStoreOverridesAHandlersCaching`
- **Break (5):** the client surface is served without the no-store wrapper
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEveryResponseIsUncacheable`

## Every timestamp the client surface serves is UTC with the Z suffix

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and again on 2026-10-02 with the index reads' timestamps added to its tests, [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a timestamp is written in the zone it was read in, with the Z suffix all the same
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEveryTimestampServedIsUTC`
- **Break (2):** a timestamp is written in UTC with the offset +00:00 in place of Z
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestEachQueryTermSelectsItsMessages`, `TestEnumerationReachesTheWholeCorpus`, `TestEveryMessageStaysInEveryIndexRead`, `TestEveryTimestampServedIsUTC`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`, `TestTheSystemStatusCarriesOperationalStateOnly`

## Served and denied bodies are counted, a denial by the stage that decided it

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a serve-time denial is counted as a gate denial
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyTheConversionRefusesIsDenied`, `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (2):** a served body is not counted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAReleasedBodyIsCleanMarkdown`

## The client surface is served over TLS unless an ingress in front is declared to terminate it, with the key pair read from its mounted files

- **Date · evidence:** 2026-10-01 · [pull request #240](https://github.com/ppat/mediated-mailbox-mcp/pull/240), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the surface is served over TLS even when an ingress is declared to terminate it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheSurfaceBehindADeclaredIngressIsPlain`
- **Break (2):** the key pair is read once, when the configuration is built, so a rotated pair never serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheSurfaceIsServedOverTLSOnly`
- **Break (3):** the client surface is served over plain HTTP whatever its settings
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheSurfaceIsServedOverTLSOnly`

## The configuration surface accepts no value that disables the gate, skips masking or weakens deny-by-default

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the configuration gains a value switching the gate, which a flag then sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestNoConfigurationValueWeakensTheGate`, `TestNoConfigurationValueWeakensTheGate/a_flag_disabling_the_gate`, `TestTheConfigurationTypeIsPinned`, `TestTheEffectiveConfigurationIsLogged`
- **Break (2):** a scanner section the scanner refuses is accepted, so the serve-time check runs with nothing to match
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestNoConfigurationValueWeakensTheGate`, `TestNoConfigurationValueWeakensTheGate/a_scanner_section_with_nothing_to_match`

## The gate decides every body request from the index, and a denial never reaches the provider

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the body is fetched from the provider before the gate decides, and dropped on a denial
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`
- **Break (2):** a body the gate denies is released all the same
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestEachOriginIsToldApartOnBothRoots`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bank`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-flagged`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-oldrestricted`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-pending`

## The MCP root offers tools and nothing else

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186)
- **Break (1):** the server is built with no options, so the SDK's default capabilities are advertised
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheRootOffersToolsOnly`
- **Break (2):** requests for prompts, resources and a log level reach the SDK, which answers them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheRootOffersToolsOnly`
- **Break (3):** requests for resources reach the SDK, which answers them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheRootOffersToolsOnly`
- **Break (4):** the MCP root also offers a prompt, surface the API root does not carry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestTheRootOffersToolsOnly`

## The mediator loads the policy of the accounts it serves, and of none when it serves none

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the mediator's role is not granted the policy rules' columns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyTheConversionRefusesIsDenied`, `TestAFailedPolicyLoadStillFollowsTheSnapshot`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAServedBodyFetchesNoImage`, `TestAStaleHandOverLeavesTheReauthorization`, `TestAnAccountNamingNoClientIsNotConnected`, `TestAnAccountWithoutAStoredCredentialIsNotConnected`, `TestEachBodyRequestHandsOverAndRecordsTheAttempt`, `TestEachOriginIsToldApartOnBothRoots`, `TestEveryReloadLoadsThePolicy`, `TestNoBodyIsReleasedWithoutItsAuditRow`, `TestTheCredentialsAreTheClientAndTheAccountsToken`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheMediatorServesTheAccountsEachReloadLists`, `TestTheMetricsEndpointCarriesEachServedAccountsRateState`, `TestTheReadOperationsReadTheServingState`, `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (2):** the policy loader's reload-failure series is registered on a registry the mediator does not serve, so its metrics endpoint never carries the alarm's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAFailedPolicyLoadStillFollowsTheSnapshot`, `TestANewlyListedDomainIsDeniedOnTheNextCall`
- **Break (3):** the policy loader is built even with no account served, which it refuses
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAFailedPolicyLoadStillFollowsTheSnapshot`, `TestTheMediatorReloadsOnItsInterval`, `TestTheMediatorServesTheAccountsEachReloadLists`

## The mediator refuses a configuration it cannot serve with before anything is read or connected

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a token file path of spaces is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAConfigurationTheMediatorCannotServeWithIsRefused`
- **Break (2):** the database section is never validated
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAConfigurationTheMediatorCannotServeWithIsRefused`
- **Break (3):** a key pair given beside a declared ingress is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAConfigurationTheMediatorCannotServeWithIsRefused`
- **Break (4):** the mediator starts with no key pair and no ingress declared
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAConfigurationTheMediatorCannotServeWithIsRefused`
- **Break (5):** a reload interval of zero is accepted, on which the schedule never ticks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAConfigurationTheMediatorCannotServeWithIsRefused`

## The mediator refuses to start while MCPGODEBUG is set

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the mediator starts whatever MCPGODEBUG holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestMCPGODEBUGStopsTheStart`
- **Break (2):** an MCPGODEBUG set to the empty string is taken as unset
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestMCPGODEBUGStopsTheStart`

## The mediator refuses to start while PGPASSWORD or PGSSLPASSWORD is set

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the refusal runs after the configuration is read, logged and validated, so a start whose environment sets a password variable reads its configuration first
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAPasswordInTheEnvironmentStopsTheStart`, `TestAPasswordVariableIsRefusedBeforeTheConfigurationIsRead`, `TestMCPGODEBUGStopsTheStart`
- **Break (2):** the start never checks the environment for a database password
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAPasswordInTheEnvironmentStopsTheStart`, `TestAPasswordVariableIsRefusedBeforeTheConfigurationIsRead`

## The mediator reports ready only once its TLS key pair and bearer token load

- **Date · evidence:** 2026-10-01 · [pull request #240](https://github.com/ppat/mediated-mailbox-mcp/pull/240), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a token file holding no token counts as loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAStartThatCannotServeFailsAndNeverReportsReady`, `TestReadyOnlyOnceTheKeysLoad`, `TestReadyOnlyOnceTheKeysLoad/a_token_file_holding_no_token`
- **Break (2):** the key pair is not loaded before the mediator is marked ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestReadyOnlyOnceTheKeysLoad`, `TestReadyOnlyOnceTheKeysLoad/a_certificate_that_is_absent`, `TestReadyOnlyOnceTheKeysLoad/a_key_that_does_not_match`
- **Break (3):** the mediator is marked ready before the key pair and token are loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestReadyOnlyOnceTheKeysLoad`, `TestReadyOnlyOnceTheKeysLoad/a_certificate_that_is_absent`, `TestReadyOnlyOnceTheKeysLoad/a_key_that_does_not_match`, `TestReadyOnlyOnceTheKeysLoad/a_token_file_holding_no_token`, `TestReadyOnlyOnceTheKeysLoad/a_token_file_that_is_absent`
- **Break (4):** the start marks the mediator ready without loading the key pair and the token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAStartThatCannotServeFailsAndNeverReportsReady`

## The mediator serves the accounts each account snapshot it reloads lists, and keeps them when a reload's read fails

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a reload listing no account keeps the accounts served before it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMediatorServesTheAccountsEachReloadLists`
- **Break (2):** a reload whose policy load fails returns before serving its snapshot, so an account it dropped stays served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAFailedPolicyLoadStillFollowsTheSnapshot`
- **Break (3):** a reload loads the policy only when its accounts differ from the ones the last load was tried for, so a load that failed is never retried and a policy edit never reaches the metadata reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAFailedPolicyLoadStillFollowsTheSnapshot`, `TestEveryReloadLoadsThePolicy`
- **Break (4):** a reload whose read fails stops serving every account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMediatorReloadsOnItsInterval`, `TestTheMediatorServesTheAccountsEachReloadLists`
- **Break (5):** the policy is loaded at the first reload that serves an account only, so an account a later reload lists is served under no policy of its own and a policy edit never reaches the metadata reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAFailedPolicyLoadStillFollowsTheSnapshot`, `TestEveryReloadLoadsThePolicy`, `TestTheMediatorServesTheAccountsEachReloadLists`
- **Break (6):** the registry keeps the first set of accounts it was given, whatever a reload serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyTheConversionRefusesIsDenied`, `TestAFailedPolicyLoadStillFollowsTheSnapshot`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAServedBodyFetchesNoImage`, `TestAnAccountNamingNoClientIsNotConnected`, `TestAnAccountWithoutAStoredCredentialIsNotConnected`, `TestEachBodyRequestHandsOverAndRecordsTheAttempt`, `TestEachOriginIsToldApartOnBothRoots`, `TestEveryReloadLoadsThePolicy`, `TestNoBodyIsReleasedWithoutItsAuditRow`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bank`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-flagged`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-oldrestricted`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-pending`, `TestTheMediatorReloadsOnItsInterval`, `TestTheMediatorServesTheAccountsEachReloadLists`, `TestTheReadOperationsReadTheServingState`, `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (7):** a reload loads the snapshot and never hands its accounts to the registry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestABodyFetchSpendsFromTheInteractiveReservation`, `TestABodyTheConversionRefusesIsDenied`, `TestAFailedPolicyLoadStillFollowsTheSnapshot`, `TestANewlyListedDomainIsDeniedOnTheNextCall`, `TestAProviderThatDoesNotAnswerIsTheProvidersFailure`, `TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported`, `TestAReleasedBodyIsCleanMarkdown`, `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/its_credential_removed`, `TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal/moved_to_a_client_that_does_not_open`, `TestARotationWhoseWriteBackFailedIsKept`, `TestAServedBodyFetchesNoImage`, `TestAnAccountNamingNoClientIsNotConnected`, `TestAnAccountWithoutAStoredCredentialIsNotConnected`, `TestEachBodyRequestHandsOverAndRecordsTheAttempt`, `TestEachOriginIsToldApartOnBothRoots`, `TestEveryReloadLoadsThePolicy`, `TestNoBodyIsReleasedWithoutItsAuditRow`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-bank`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-flagged`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-oldrestricted`, `TestTheGateDeniesFromTheIndexWithoutReachingTheProvider/m-pending`, `TestTheMediatorReloadsOnItsInterval`, `TestTheMediatorServesTheAccountsEachReloadLists`, `TestTheReadOperationsReadTheServingState`, `TestTheServeTimeCheckReadsEverythingReleased`

## The mediator's metrics endpoint carries F3's rate-state series for each account it serves through the Gmail adapter

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the rate-state collector is built and never registered on the metrics endpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMetricsEndpointCarriesEachServedAccountsRateState`
- **Break (2):** an account of a provider the mediator has no rate profile for is reported with the Gmail ceiling
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMetricsEndpointCarriesEachServedAccountsRateState`

## The mediator's token source is built from the OAuth client the account connects through and the account's stored credential, and from nothing else

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the client's identifier and secret are swapped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`, `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (2):** every account is served with the OAuth client of the snapshot's first account rather than its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheCredentialsAreTheClientAndTheAccountsToken`
- **Break (3):** an account whose state row holds no credential takes one from the environment
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAnAccountWithoutAStoredCredentialIsNotConnected`
- **Break (4):** the re-read after a refusal keeps the client the source was built with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestARereadAfterAMoveBuildsTheSourceFromTheNewClient`

## The read operations read the accounts and the policy the serving state holds in force, and a clock within thirty seconds of the real one when the read is made

- **Date · evidence:** 2026-10-01 · [pull request #240](https://github.com/ppat/mediated-mailbox-mcp/pull/240), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the read operations are given an accounts listing that lists none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheReadOperationsReadTheServingState`
- **Break (2):** the read operations are given a clock ten minutes behind the real one, so a sync cursor written an hour ago reads fifty minutes old
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheReadOperationsReadTheServingState`
- **Break (3):** the read operations are given a clock stopped at the zero time, behind every recorded instant, so a backoff that ended an hour ago reads as running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheReadOperationsReadTheServingState`
- **Break (4):** the read operations are given a clock two hours ahead of the real one, so a backoff recorded to end within the hour reads as over
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheReadOperationsReadTheServingState`
- **Break (5):** the read operations are given no policy, so every sender reads restricted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestEveryReloadLoadsThePolicy`, `TestTheReadOperationsReadTheServingState`

## The registry refuses at build time anything that could express approval or bind an argument to a header

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a schema carrying x-mcp-header generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/x-mcp-header_in_the_input`, `TestTheSurfaceRefusesWhatItCannotCarry/x-mcp-header_in_the_output`
- **Break (2):** an operation name holding a lifecycle stem generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_applying`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_approving`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approval`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approved`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_roll_back`
- **Break (3):** a path segment or :verb holding a lifecycle stem generates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_:verb_approving`, `TestTheSurfaceRefusesWhatItCannotCarry/a_:verb_holding_approve`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_applying`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_holding_approve`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_holding_roll-back`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_rolling_back`
- **Break (4):** a lifecycle word is refused only as a whole token, so approved, approval, applied and approve-all generate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approval`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_approved`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_applying`
- **Break (5):** roll_back and roll-back generate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/a_name_holding_roll_back`, `TestTheSurfaceRefusesWhatItCannotCarry/a_segment_holding_roll-back`
- **Break (6):** status is refused on a read too, so a read cannot filter on a plan's state
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAReadMayFilterOnStatus`
- **Break (7):** a write's input may declare status
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/status_in_another_case_on_a_reversible_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_array's_items_on_a_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_object_on_a_disposal`, `TestTheSurfaceRefusesWhatItCannotCarry/status_on_a_create`
- **Break (8):** status is refused only as a top-level property spelled in lower case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/status_in_another_case_on_a_reversible_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_array's_items_on_a_change`, `TestTheSurfaceRefusesWhatItCannotCarry/status_nested_in_an_object_on_a_disposal`
- **Break (9):** an effect the table has no row for derives PATCH instead of failing generation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSurfaceRefusesWhatItCannotCarry`, `TestTheSurfaceRefusesWhatItCannotCarry/an_effect_with_no_row`, `TestTheSurfaceRefusesWhatItCannotCarry/no_effect`

## The release step withholds a body it cannot decide on

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a gate-skipped body with no scanner to check it is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`, `TestFailClosed`, `TestFailClosed/a_gate-skipped_body_and_a_scanner_nobody_built`, `TestFailClosed/a_gate-skipped_login_link_and_a_scanner_nobody_built`
- **Break (2):** a scan state the gate never releases falls through to release
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestFailClosed`, `TestFailClosed/a_pending_scan`, `TestFailClosed/skipped_as_restricted`, `TestFailClosed/the_zero_scan_state`
- **Break (3):** the zero Decision reports a released body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestFailClosed`, `TestFailClosed/the_zero_decision`

## The sender class an index read selects, groups and lists by is the Redaction Gate's under the policy in force

- **Date · evidence:** 2026-10-02 · [pull request #246](https://github.com/ppat/mediated-mailbox-mcp/pull/246), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the sender class term is never resolved to the addresses classified normal, so it selects every message
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryMessageStaysInEveryIndexRead`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (2):** the index reads classify the senders under no policy, which restricts every sender, instead of the policy the call took
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`, `TestEveryMessageStaysInEveryIndexRead`, `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (3):** the sender listing shows every sender domain whose addresses the index holds as normal, whatever the policy classifies them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (4):** the sender listing shows a domain the index holds no address for as normal, without classifying it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (5):** a grouping by sender class counts both classes whatever class the query selects, so a group holds messages outside the selection
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheIndexReadsClassifySendersUnderThePolicyInForce`
- **Break (6):** the transaction helper opens an index read's transaction at the default isolation level, so each statement reads the state committed when it starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`
- **Break (7):** the class term passes the addresses classified restricted and counts every other address as normal, so an address the call did not classify fails open
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_false`
- **Break (8):** the search runs its statements outside the snapshot, so each reads the state committed when it starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAnIndexReadSeesOneStateOfTheIndex`, `TestAnIndexReadSeesOneStateOfTheIndex/snapshot_honoured_true`

## The serve-time pattern check reads the snippet and the attachment filenames that follow a gate-skipped body, and the filenames of every message, and they are released with the delimiters' words replaced

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the snippet and the filenames are released as they arrived, with the delimiters' words left in them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestTheCheckReadsEverythingReleased`
- **Break (2):** the check reads the body and the snippet and not the filenames
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_one-time_code_in_a_filename`
- **Break (3):** a pattern match withholds the content without naming the rules that matched
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`, `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_code_in_the_body_and_a_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_login_link_in_the_snippet`, `TestTheCheckReadsEverythingReleased/a_one-time_code_in_a_filename`
- **Break (4):** a scanned message's filenames are released without the pattern check, as its body and snippet are
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestAScannedMessagesFilenamesAreChecked`
- **Break (5):** the check reads the body and the filenames and not the snippet
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release`:** `TestTheCheckReadsEverythingReleased`, `TestTheCheckReadsEverythingReleased/a_login_link_in_the_snippet`

## The service layer refuses an operation whose account is missing or unknown, except the accounts listing

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the API root drops an account_id given in the body, so the path's account is taken without a check for a second one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`
- **Break (2):** the API root drops an account_id given in the query string, so the path's account is taken without a check for a second one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_accounts_listing's_query_string`, `TestTheRootRefusesWhatItCannotBind/a_case-variant_account_in_the_query_string`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_accounts_listing's_query_string`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`
- **Break (3):** a key differing from account_id only in case, given alone, is passed on to the operation rather than refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_case-variant_key_alone`
- **Break (4):** the accounts listing is put through the account check too
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/the_accounts_listing_with_no_account`
- **Break (5):** a call carrying no account_id string reaches the operation, with an empty account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_null_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_id_that_is_not_a_string`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_empty_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/no_account_id`
- **Break (6):** every operation runs as the accounts listing does, with no account check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_case-variant_key_alone`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_null_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account_and_nothing_else`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_differing_only_in_case`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_id_that_is_not_a_string`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_the_mediator_does_not_serve`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_empty_account_id`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/no_account_id`
- **Break (7):** the operation receives the arguments as sent, account_id included, so it can read an account other than the one checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestAServedOperationTakesOnlyTheArgumentsItDeclares`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/a_served_account_and_nothing_else`
- **Break (8):** arguments holding a key twice, exactly or in another case, the account included, reach the operation on either root
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/api`:** `TestTheRootRefusesWhatItCannotBind`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_body_member`, `TestTheRootRefusesWhatItCannotBind/a_case_variant_of_a_path_variable_in_the_body`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_body_beside_the_path's`, `TestTheRootRefusesWhatItCannotBind/an_account_in_the_query_string_beside_the_path's`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp`:** `TestARepeatedArgumentIsRefused`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestACallHoldingTooManyArgumentsIsRefused`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/account_id_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_case_variant_of_it`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_beside_a_variant_of_it_under_Unicode_case_folding`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_argument_given_twice`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/arguments_that_are_not_an_object`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/the_accounts_listing_with_an_argument_given_twice`
- **Break (9):** a call whose account_id names no served account reaches the operation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheServiceLayerRefusesAMissingOrUnknownAccount`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_differing_only_in_case`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_account_the_mediator_does_not_serve`, `TestTheServiceLayerRefusesAMissingOrUnknownAccount/an_empty_account_id`

## The serving mediator reloads its account snapshot on the interval its configuration sets

- **Date · evidence:** 2026-10-01 · [pull request #240](https://github.com/ppat/mediated-mailbox-mcp/pull/240), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the serving mediator starts no scheduled reload
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMediatorReloadsOnItsInterval`
- **Break (2):** each tick of the schedule passes without a reload
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheMediatorReloadsOnItsInterval`

## The snippet and attachment filenames follow the body, and a denial releases none of them

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** a serve-time denial carries the snippet it withheld
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestTheServeTimeCheckReadsEverythingReleased`
- **Break (2):** a released body is served without its snippet
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/app`:** `TestAReleasedBodyIsCleanMarkdown`, `TestTheServeTimeCheckReadsEverythingReleased`

## The system status carries recorded operational state only

- **Date · evidence:** 2026-09-28 · [pull request #186](https://github.com/ppat/mediated-mailbox-mcp/pull/186), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break:** the status reads a message and shows its subject as the last authentication's outcome
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/mediate/internal/service`:** `TestTheSystemStatusCarriesOperationalStateOnly`
