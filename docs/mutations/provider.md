# Mutations: provider

The demonstrations of the controls whose patches sit in `provider/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A call through the provider library's mailbox answers within its caller's timeout

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** every call has a fixed thirty seconds to answer, whatever its caller's timeout
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallIsCutOffAtTheCallersTimeoutAsTheProvidersFailure`
- **Break (2):** a call cut off by the caller's timeout returns the context's error rather than the provider's failure, which tells the controller nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallIsCutOffAtTheCallersTimeoutAsTheProvidersFailure`

## A call through the provider library's mailbox is made once more over a credential read again when the provider refuses its own

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a label's creation is leased and reported but never read again and made once more over a refused credential
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/EnsureLabel`
- **Break (2):** a refused credential is never read again, so the refusal stands over a credential the operator replaced
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore`, `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore/a_read_that_fails`, `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore/another_credential`, `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore/another_credential_refused_too`, `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore/the_refused_credential_again`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ChangesSince`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/CurrentCursor`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/EnsureLabel`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/EnumerateAll`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetMessageBody`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetMessageMetadata`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetThreadMetadata`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ListLabels`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ListThreads`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/Mutate`
- **Break (3):** the call is made once more over the token the provider refused rather than the one made from the credential read again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore`, `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore/another_credential`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ChangesSince`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/CurrentCursor`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/EnsureLabel`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/EnumerateAll`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetMessageBody`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetMessageMetadata`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetThreadMetadata`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ListLabels`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ListThreads`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/Mutate`
- **Break (4):** the token made from the credential read again serves the call made once more and no later call
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore`, `TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore/another_credential`

## A call through the provider library's mailbox leases its cost in its caller's class at the account's target

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a change listing is leased at the price of a label listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ChangesSince`
- **Break (2):** every lease is asked in the batch class, whatever the caller's class
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget`, `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget/interactive_at_a_lowered_target`, `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget/sync_at_the_default_target`
- **Break (3):** the mailbox's limiter spends every account at the default target, the lowered one ignored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget`, `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget/interactive_at_a_lowered_target`
- **Break (4):** a metadata fetch is leased at the cost of a call naming no message, whatever the identifiers it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallersSeriesAreOnTheRegistererItWasGiven`, `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget`, `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget/batch_at_the_default_target`, `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget/interactive_at_a_lowered_target`, `TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget/sync_at_the_default_target`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetMessageMetadata`
- **Break (5):** a mutation reaches the provider with no lease, so it spends nothing and tells the limiter nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/Mutate`
- **Break (6):** a mutation is leased at the price of one carrying no op, whatever the ops it carries
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/Mutate`
- **Break (7):** the second body call, naming a part held apart, reaches the provider without a lease
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestABodyHeldApartTakesTwoLeasedCallsThroughTheMailbox`

## A call through the provider library's mailbox tells the rate limiter how it went

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a call the provider failed tells the controller nothing, so the account's rate is never cut
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallIsCutOffAtTheCallersTimeoutAsTheProvidersFailure`, `TestEachCallTellsTheLimiterHowItWent`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ChangesSince`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/CurrentCursor`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/EnsureLabel`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/EnumerateAll`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetMessageBody`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetMessageMetadata`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/GetThreadMetadata`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ListLabels`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/ListThreads`, `TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore/Mutate`
- **Break (2):** every error the call returns is told to the controller as a server error, a refused body included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEachCallTellsTheLimiterHowItWent`
- **Break (3):** a call that succeeded tells the controller nothing, so its latency never joins the account's window
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEachCallTellsTheLimiterHowItWent`
- **Break (4):** a throttle is told to the controller as a server error, so the rate falls to 80% with no backoff rather than halving
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestALeaseIsWaitedForNoLaterThanTheUnitsDeadline`, `TestEachCallTellsTheLimiterHowItWent`

## A call through the provider library's mailbox waits for its lease no later than its unit's deadline

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the unit's deadline never reaches the lease, so the lease is waited for as long as the caller's context allows
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestALeaseIsWaitedForNoLaterThanTheUnitsDeadline`
- **Break (2):** a lease cut off by the unit's deadline is reported as the provider failing the call
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestALeaseIsWaitedForNoLaterThanTheUnitsDeadline`

## A caller registers the rate limiter's and the shared provider series on the registerer it is given

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the caller's rate limiter series are registered on a registry no one was given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallersSeriesAreOnTheRegistererItWasGiven`
- **Break (2):** the shared provider series the caller hands its adapters are registered on a registry no one was given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallersSeriesAreOnTheRegistererItWasGiven`

## A canonical value cannot change the Gmail request it is written into

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** any non-empty identifier is written into a request path
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestMessageBodyRefusesAPartItCannotRead`, `TestMessageBodyRefusesAPartItCannotRead/an_unreadable_reference`, `TestReadBodyPartRefusesAForeignPart`, `TestReadBodyPartRefusesAForeignPart/a_reference_that_is_not_an_identifier`, `TestValidID`
- **Break (2):** an address is written into the search text whatever characters it holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestQueryValuesCannotInjectOperators`, `TestQueryValuesCannotInjectOperators/an_address_closing_its_quotes`, `TestQueryValuesCannotInjectOperators/an_address_whose_only_unusual_character_is_a_quote`, `TestQueryValuesCannotInjectOperators/an_address_with_a_backslash`, `TestQueryValuesCannotInjectOperators/an_address_with_a_brace`, `TestQueryValuesCannotInjectOperators/an_address_with_a_line_break`, `TestQueryValuesCannotInjectOperators/an_address_with_a_parenthesis`, `TestQueryValuesCannotInjectOperators/an_address_with_a_space`, `TestQueryValuesCannotInjectOperators/an_injected_address_beside_a_plain_one`
- **Break (3):** a label path is written into the search text as a label term rather than sent as a label identifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCompileQuery`, `TestCompileQuery/a_system_label`, `TestCompileQuery/a_user_label`, `TestCompileQuery/every_node_together`, `TestQueryValuesCannotInjectOperators`, `TestQueryValuesCannotInjectOperators/a_label_holding_operators`
- **Break (4):** the check on an address admits the double quote, so an address can close its quotes and add an operator
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestQueryValuesCannotInjectOperators`, `TestQueryValuesCannotInjectOperators/an_address_whose_only_unusual_character_is_a_quote`

## A ceiling that is not a finite positive number fixes no budget

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an infinite ceiling fixes an infinite budget
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssueRefuses`, `TestIssueRefuses/an_infinite_ceiling`, `TestLimitsFor`, `TestLimitsFor/positive_infinity`, `TestNoBudgetHoldsTheRateAtZero`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** a negative ceiling fixes negative limits rather than none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLimitsFor`, `TestLimitsFor/negative`, `TestLimitsFor/negative_infinity`, `TestNoBudgetHoldsTheRateAtZero`

## A Gmail port call sends no request past the cost it declares, and a caller-sized call fits one lease

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a call compares what it spent before the request with its declared cost, so the request that takes it past is sent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestACallIsHeldToItsDeclaredCost`, `TestWithin`
- **Break (2):** a call sends every request whatever it has spent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestACallIsHeldToItsDeclaredCost`, `TestWithin`
- **Break (3):** a metadata fetch takes any number of identifiers, so a call can cost more than the rate limiter issues at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCallerSizedCallsPastTheSecondAreRefused`
- **Break (4):** a mutation takes any number of ops, so a call can cost more than the rate limiter issues at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCallerSizedCallsPastTheSecondAreRefused`

## A lease counts against its class's share until it expires one second after it was issued

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** every grant is stored as batch's, so a lease counts against the wrong class's share
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestALiveLeaseCountsAgainstItsClassShare`
- **Break (2):** a stored grant is kept for a minute and its lease counts against its class's share all that time
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`
- **Break (3):** a stored grant's lease expires the instant it is issued, so it never counts against its class's share
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestALiveLeaseCountsAgainstItsClassShare`

## A lease holds the whole request, and a request larger than the bucket can ever hold is refused

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a request larger than the bucket can ever hold is left waiting
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/a_request_past_one_second's_worth_at_the_hard_cap_is_refused_and_changes_nothing`, `TestIssueRefuses`, `TestIssueRefuses/a_ceiling_of_zero`, `TestIssueRefuses/a_ceiling_that_is_not_a_number`, `TestIssueRefuses/a_negative_ceiling`, `TestIssueRefuses/an_infinite_ceiling`, `TestIssueRefuses/infinite_tokens`
- **Break (2):** a request the bucket cannot cover is granted what the bucket holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestACallCostingMoreThanASecondAtTheFloorIsGranted`, `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAThreadFetchIsGrantedOnceTheBucketHoldsIt`, `TestAnExpiredLeaseIsNotRefunded`, `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`, `TestIssue/a_rate_above_the_target_refills_at_the_target`, `TestIssue/a_request_the_bucket_cannot_cover_yet_waits,_and_is_still_asking`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestIssueSplitsByClass`, `TestIssueSplitsByClass/a_stored_lease_of_negative_tokens_counts_as_none`, `TestIssueSplitsByClass/and_no_more`, `TestIssueSplitsByClass/another_class's_lease_does_not_count_against_interactive's_share`, `TestIssueSplitsByClass/batch_waits_on_both_shares`, `TestIssueSplitsByClass/interactive's_expired_lease_no_longer_counts,_so_its_share_is_reserved_again`, `TestIssueSplitsByClass/sync_waits_on_interactive's_share`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestIssueUnderALoweredTarget/and_no_more_than_the_share_lends`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`
- **Break (3):** a request larger than what the bucket holds now is refused rather than left waiting
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestAnExpiredLeaseIsNotRefunded`, `TestInteractiveKeepsItsShareThroughACut`, `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`, `TestIssue/a_negative_rate_refills_nothing`, `TestIssue/a_negative_stored_level_is_taken_as_empty`, `TestIssue/a_rate_above_the_target_refills_at_the_target`, `TestIssue/a_rate_that_is_not_a_number_refills_nothing`, `TestIssue/a_request_the_bucket_cannot_cover_yet_waits,_and_is_still_asking`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_grant_that_is_not_a_number_fills_the_window`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestIssue/a_stored_level_that_is_not_a_number_is_taken_as_empty`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestIssueSplitsByClass`, `TestIssueSplitsByClass/a_stored_lease_of_negative_tokens_counts_as_none`, `TestIssueSplitsByClass/and_no_more`, `TestIssueSplitsByClass/another_class's_lease_does_not_count_against_interactive's_share`, `TestIssueSplitsByClass/batch_waits_on_both_shares`, `TestIssueSplitsByClass/interactive's_expired_lease_no_longer_counts,_so_its_share_is_reserved_again`, `TestIssueSplitsByClass/sync_waits_on_interactive's_share`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestIssueUnderALoweredTarget/and_no_more_than_the_share_lends`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`

## A list of adapters and a token expose nothing a program could add an adapter or reach a token's source through

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the list's adapters are an exported field, which any code holding a list can add to
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/internal/adapters`:** `TestAListAndATokenExposeNoField`
- **Break (2):** a token's source is an exported field, so code holding a token reaches the adapter's source
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/internal/adapters`:** `TestAListAndATokenExposeNoField`

## A part held apart is decoded from its declared charset, and a label the table does not name is refused

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a part held apart is read as UTF-8 whatever charset it declares
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestPartText`, `TestPartText/decoded_text_past_the_bound`
- **Break (2):** a label the table maps to its replacement encoding is decoded, so the part reads as one replacement character
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestPartText`, `TestPartText/a_charset_that_decodes_to_replacement`
- **Break (3):** a label the table does not name is read as UTF-8 rather than refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestPartText`, `TestPartText/a_charset_the_standard_does_not_name`
- **Break (4):** a Content-Type that does not parse is read as declaring no charset, so the part is decoded as us-ascii whatever charset it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestMessageBodyOfAPartHeldApartReadsItsContentType`, `TestMessageBodyOfAPartHeldApartReadsItsContentType/a_comment`, `TestMessageBodyOfAPartHeldApartReadsItsContentType/a_parameter_given_twice`, `TestMessageBodyOfAPartHeldApartReadsItsContentType/an_empty_parameter`
- **Break (5):** a part declaring x-user-defined is decoded, so its high bytes read as private-use characters
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestPartText`, `TestPartText/a_user-defined_part`
- **Break (6):** a part declaring UTF-16 is decoded, so a big-endian part reads as text it does not hold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestPartText`, `TestPartText/a_UTF-16BE_part`, `TestPartText/a_UTF-16_part`

## A request costing more than one second's worth at the hard cap is refused at once, never left waiting

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a refused request returns an empty lease and no error, as if granted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestARefusedLeaseIsNotTimed`, `TestARequestPastTheHardCapIsRefusedAtOnce`
- **Break (2):** a refused request is left waiting like one the bucket cannot yet fill
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestARequestPastTheHardCapIsRefusedAtOnce`

## A request draws all but what the classes that outrank it and asked within the last lease period have still to draw of their shares, so an idle class's share is lent

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a class's leases do not count against its share, so its whole share stays reserved however much it holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive's_live_lease_counts_against_its_share`
- **Break (2):** every other class that asked reserves its share, so interactive waits on the classes it outranks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive_does_not_wait_on_batch's_share`, `TestIssueSplitsByClass/sync_does_not_wait_on_batch's_share`
- **Break (3):** an idle class's share is reserved as if it had asked, so nothing is lent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestACallCostingMoreThanASecondAtTheFloorIsGranted`, `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestAValidRequestIsGrantedOnceTheBucketHoldsIt`, `TestAnExpiredLeaseIsNotRefunded`, `TestInteractiveKeepsItsShareThroughACut`, `TestIssue`, `TestIssue/a_grant_a_second_old_has_left_the_window`, `TestIssue/a_request_of_one_second's_worth_at_the_hard_cap_is_granted_from_a_full_bucket`, `TestIssue/a_stored_grant_of_negative_tokens_counts_as_none`, `TestIssue/after_a_backoff_the_bucket_fills_only_from_its_end`, `TestIssue/an_infinite_rate_refills_at_the_target`, `TestIssue/an_instant_earlier_than_the_last_is_taken_as_the_last,_and_fills_nothing`, `TestIssue/the_bucket_refills_at_the_rate`, `TestIssueSplitsByClass`, `TestIssueSplitsByClass/an_idle_interactive_share_is_lent`, `TestIssueSplitsByClass/batch_may_draw_all_but_what_interactive_has_still_to_draw_of_its_share`, `TestIssueSplitsByClass/interactive's_live_lease_counts_against_its_share`, `TestIssueSplitsByClass/sync_does_not_wait_on_batch's_share`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/a_full_bucket_still_holds_one_second's_worth_at_the_hard_cap`, `TestIssueUnderALoweredTarget/a_second_refills_the_lowered_target's_worth`, `TestIssueUnderALoweredTarget/batch_may_draw_all_but_interactive's_share_of_the_lowered_target`

## A rotated refresh token is held, sent on the next refresh and handed to the deployable

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a refresh response that leaves the refresh token out counts as rotating it to an empty token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARefreshThatDoesNotRotateKeepsTheToken`, `TestARefreshThatDoesNotRotateKeepsTheToken/field_left_out`, `TestARotatedRefreshTokenIsHeldSentAndHandedOver`, `TestAnAccessTokenIsUsedUntilItNearlyExpires`
- **Break (2):** a rotation holds the new access token as the refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARotatedRefreshTokenIsHeldSentAndHandedOver`
- **Break (3):** a returned refresh token counts as a rotation only when it equals the held one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARotatedRefreshTokenIsHeldSentAndHandedOver`
- **Break (4):** no refresh response counts as a rotation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARotatedRefreshTokenIsHeldSentAndHandedOver`
- **Break (5):** a rotation is detected but the source goes on with the refresh token it started with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARotatedRefreshTokenIsHeldSentAndHandedOver`
- **Break (6):** the source hands the deployable its access token rather than its refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARefreshThatDoesNotRotateKeepsTheToken`, `TestARefreshThatDoesNotRotateKeepsTheToken/field_left_out`, `TestARefreshThatDoesNotRotateKeepsTheToken/same_token`, `TestARotatedRefreshTokenIsHeldSentAndHandedOver`, `TestAnAccessTokenIsUsedUntilItNearlyExpires`, `TestTheSourceHoldsOnlyTheCredentialItIsGiven`, `TestTheSourceHoldsOnlyTheCredentialItIsGiven/a_refresh_token`
- **Break (7):** the token response's refresh token is read from a field Google does not send, so no rotation is ever seen
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARotatedRefreshTokenIsHeldSentAndHandedOver`, `TestGrantFrom`, `TestParseTokenResponse`, `TestParseTokenResponse/refresh_that_rotates`

## A stored grant stamped later than the request that sees it counts for one second from that request and then leaves the window

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a stored grant's stamp is held to the stored latest instant rather than the request's, so a stored instant that came back behind moves every real grant out of the window
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestIssue`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** a stored grant keeps its stamp however far ahead it lies, so it fills the window until the clock reaches it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestIssue`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`

## A stored latest instant ahead of the clock never moves a real grant out of the one-second window

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** window membership is judged from the instant issuance raised the request to, so a stored instant ahead of the clock moves every real grant out of the window
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** window membership is judged from the stored latest instant rather than the request's own, so a stored instant ahead of the clock moves every real grant out of the window
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestNoRunIssuesPastTheHardCap`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`

## A throttle halves the rate, a server error cuts it to 80% and a latency median above twice the baseline cuts it to 90%

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a latency median above the baseline itself cuts the rate, not only one above twice it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestALastingRiseBecomesTheBaseline`, `TestLatencyMeasured`, `TestLatencyMeasured/below_twice_the_baseline_does_not`, `TestLatencyMeasured/exactly_twice_the_baseline_does_not`
- **Break (2):** a latency median never cuts the rate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestALastingRiseBecomesTheBaseline`, `TestLatencyMeasured`, `TestLatencyMeasured/held_at_the_floor`, `TestLatencyMeasured/more_than_twice_the_baseline_cuts_to_90%`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_latency_cut_falls_from_it`
- **Break (3):** a server error halves the rate as a throttle does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestServerErrored`, `TestServerErrored/a_stored_rate_above_the_target_falls_from_the_target`, `TestServerErrored/falls_to_80%`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_server_error_falls_from_it`
- **Break (4):** a throttle leaves the rate as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_throttle_halves_from_it`, `TestThrottled`, `TestThrottled/a_draw_above_one_waits_the_whole_bound`, `TestThrottled/a_draw_just_below_one_waits_almost_the_bound`, `TestThrottled/a_draw_of_one_waits_the_whole_bound`, `TestThrottled/a_draw_of_zero_waits_nothing`, `TestThrottled/a_draw_that_is_not_a_number_waits_the_whole_bound`, `TestThrottled/a_jitter_throttle_during_a_one-hour_retry-after_keeps_the_hour`, `TestThrottled/a_later_throttle_extends_a_backoff_from_its_own_instant`, `TestThrottled/a_negative_count_starts_over`, `TestThrottled/a_negative_draw_waits_the_whole_bound`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_retry-after_near_the_clock's_limit_saturates`, `TestThrottled/a_retry-after_not_marked_present_counts_as_absent`, `TestThrottled/a_retry-after_of_a_project_throttle_is_honored_the_same`, `TestThrottled/a_retry-after_of_an_hour_is_honored_as_given`, `TestThrottled/a_stored_rate_above_the_target_halves_from_the_target`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/halves_the_rate_and_honors_the_retry-after`, `TestThrottled/held_at_the_floor`, `TestThrottled/the_bound_doubles_per_earlier_throttle`, `TestThrottled/the_bound_holds_at_32_seconds_at_the_largest_count`, `TestThrottled/the_bound_holds_at_32_seconds_past_it`, `TestThrottled/the_bound_stops_doubling_at_32_seconds`, `TestThrottled/the_first_throttle_draws_within_one_second`, `TestThrottled/the_largest_retry-after_saturates_instead_of_wrapping`, `TestThrottled/the_smallest_retry-after_counts_as_absent`

## A throttle waits out the provider's retry-after when it gives one and a full-jitter draw otherwise

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a retry-after of zero or in the past is honored, so the backoff ends at once or before it starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/the_smallest_retry-after_counts_as_absent`
- **Break (2):** a success no longer ends the run of throttles, so the bound keeps doubling across successes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestSucceeded`, `TestSucceeded/a_success_ends_a_run_of_throttles_and_keeps_the_backoff`
- **Break (3):** the draw is ignored, so every backoff waits the whole bound as a fixed backoff does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_throttle_halves_from_it`, `TestThrottled`, `TestThrottled/a_draw_just_below_one_waits_almost_the_bound`, `TestThrottled/a_draw_of_zero_waits_nothing`, `TestThrottled/a_negative_count_starts_over`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_retry-after_not_marked_present_counts_as_absent`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/the_bound_doubles_per_earlier_throttle`, `TestThrottled/the_bound_holds_at_32_seconds_at_the_largest_count`, `TestThrottled/the_bound_holds_at_32_seconds_past_it`, `TestThrottled/the_bound_stops_doubling_at_32_seconds`, `TestThrottled/the_first_throttle_draws_within_one_second`, `TestThrottled/the_smallest_retry-after_counts_as_absent`
- **Break (4):** the jitter's bound stays at one second whatever the count of throttles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_draw_above_one_waits_the_whole_bound`, `TestThrottled/a_draw_just_below_one_waits_almost_the_bound`, `TestThrottled/a_draw_of_one_waits_the_whole_bound`, `TestThrottled/a_draw_that_is_not_a_number_waits_the_whole_bound`, `TestThrottled/a_negative_draw_waits_the_whole_bound`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_retry-after_not_marked_present_counts_as_absent`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/the_bound_doubles_per_earlier_throttle`, `TestThrottled/the_bound_holds_at_32_seconds_at_the_largest_count`, `TestThrottled/the_bound_holds_at_32_seconds_past_it`, `TestThrottled/the_bound_stops_doubling_at_32_seconds`, `TestThrottled/the_smallest_retry-after_counts_as_absent`
- **Break (5):** a throttle arriving during a backoff replaces its end, so a shorter draw cuts a retry-after short
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_jitter_throttle_during_a_one-hour_retry-after_keeps_the_hour`
- **Break (6):** a retry-after is ignored and full jitter always applies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_later_throttle_extends_a_backoff_from_its_own_instant`, `TestThrottled/a_retry-after_of_a_project_throttle_is_honored_the_same`, `TestThrottled/a_retry-after_of_an_hour_is_honored_as_given`, `TestThrottled/a_stored_rate_above_the_target_halves_from_the_target`, `TestThrottled/halves_the_rate_and_honors_the_retry-after`, `TestThrottled/held_at_the_floor`, `TestThrottled/the_largest_retry-after_saturates_instead_of_wrapping`
- **Break (7):** the backoff's end is a plain sum that wraps into the past near the clock's limit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_retry-after_near_the_clock's_limit_saturates`, `TestThrottled/the_largest_retry-after_saturates_instead_of_wrapping`

## A throttle's cut is stored, and batch absorbs it while interactive keeps its share

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a throttle is stored as a server error, cutting the rate to 80% rather than half
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAFloorRateReadsAsAtTheFloor`, `TestTheLimiterEmitsItsProcesssSeries`
- **Break (2):** a throttle stores the state it read, so the rate is never cut
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAFloorRateReadsAsAtTheFloor`, `TestTheControllerConvergesBelowARealCeilingAndRecovers`, `TestTheLimiterEmitsItsProcesssSeries`

## A token request the provider refuses is reported as refused, one that gets no answer as failed, and a cancelled request or a held access token as no attempt

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a refresh is never held as an attempt, so the source reports none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARefreshPastItsDeadlineIsAFailedAttempt`, `TestARefreshThatGetsNoAnswerIsAFailedAttempt`
- **Break (2):** a refresh its caller cancelled is held as an attempt that failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestACancelledRefreshIsNoAttempt`
- **Break (3):** a refresh whose deadline passed is held as no attempt, like a cancelled one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARefreshPastItsDeadlineIsAFailedAttempt`
- **Break (4):** every token endpoint answer other than success counts as a refusal, whatever its status
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARefreshsOutcome`
- **Break (5):** handing out a held access token is held as an attempt that succeeded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestAHeldAccessTokenIsNoAttempt`
- **Break (6):** the token endpoint's answer loses its status, so a refusal reads as an attempt that failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARefreshsOutcome`, `TestTheClientChecksAnswer`

## A token request the provider refuses surfaces as a refused credential, and one that fails any other way before the call is cancelled or its deadline passes as a provider failure

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** every failure to obtain an access token is a refused credential, whatever its outcome
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestATokenFailureIsThePortErrorItsOutcomeNames`, `TestATokenRequestThatGetsNoAnswerIsAProviderError`
- **Break (2):** a port call reports every failure to obtain an access token as a provider failure, without reading its outcome
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestAPortCallWhoseTokenRequestWasRefusedIsARefusedCredential`
- **Break (3):** a port call reports every failure to obtain an access token as a refused credential, without reading its outcome
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestATokenRequestThatGetsNoAnswerIsAProviderError`
- **Break (4):** a refusal from the token endpoint is both a refused credential and a provider failure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestAPortCallWhoseTokenRequestWasRefusedIsARefusedCredential`, `TestATokenFailureIsThePortErrorItsOutcomeNames`
- **Break (5):** a refusal from the token endpoint is a provider failure, like any other failure to obtain an access token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestAPortCallWhoseTokenRequestWasRefusedIsARefusedCredential`, `TestATokenFailureIsThePortErrorItsOutcomeNames`

## A worker that waited on the account's lock is stamped with the clock read after it got the lock

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the clock is read before the lock is taken, so a worker that waited is stamped early
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`
- **Break (2):** no lock is taken, so issuers for one account do not take turns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`

## A worker's provider call that does not answer within the deadline is cut off as the provider's failure

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a call its caller gave up on is read as the provider's failure and cuts the rate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallIsCutOffAtTheCallersTimeoutAsTheProvidersFailure`
- **Break (2):** a call cut off at the deadline fails with the context's error, which the controller does not count as the provider's failure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestACallIsCutOffAtTheCallersTimeoutAsTheProvidersFailure`

## Additive increase grows the rate by 2% of the target times the call's cost over the current rate

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the step is no longer divided by the current rate, so growth per second rises with the rate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestSucceeded`, `TestSucceeded/a_cost_of_the_whole_rate_grows_it_by_2%_of_the_target`, `TestSucceeded/a_success_ends_a_run_of_throttles_and_keeps_the_backoff`, `TestSucceeded/from_the_floor`, `TestSucceeded/from_twice_the_floor`, `TestTheClimbHoldsForManySmallSuccesses`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_success_grows_the_rate_by_2%_of_the_lowered_target`, `TestTheRateClimbsFromTheFloorToTheTargetIn45Seconds`, `TestTheRateClimbsToALoweredTargetAndNoFurther`
- **Break (2):** the step ignores the call's cost, so a costly success grows the rate no more than a cheap one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestSucceeded`, `TestSucceeded/a_cost_of_the_whole_rate_grows_it_by_2%_of_the_target`, `TestSucceeded/a_huge_cost_reaches_only_the_target`, `TestSucceeded/from_the_floor`, `TestSucceeded/from_twice_the_floor`, `TestSucceeded/held_at_the_target`, `TestTheClimbHoldsForManySmallSuccesses`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_huge_success_reaches_only_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_success_grows_the_rate_by_2%_of_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_success_stops_at_the_lowered_target`, `TestTheRateClimbsFromTheFloorToTheTargetIn45Seconds`, `TestTheRateClimbsToALoweredTargetAndNoFurther`

## An account may lower its target to any value above the floor, and nothing may raise it above half the ceiling

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a target at the floor is accepted, so the rate is held at the floor
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/at_the_floor_under_a_ceiling_that_fixes_no_budget`
- **Break (2):** a target at the floor or below it is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/at_the_floor_under_a_ceiling_that_fixes_no_budget`, `TestLimitsWithTarget/below_the_floor_under_a_ceiling_that_fixes_no_budget`
- **Break (3):** a target must lie above 0.0625 of the ceiling rather than above the floor, so a target close to the floor is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/lowered_close_to_the_floor`, `TestLimitsWithTarget/lowered_to_the_first_target_stored_above_the_floor`, `TestNoRunIssuesPastTheHardCap`
- **Break (4):** a target above half the ceiling is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/infinite`, `TestLimitsWithTarget/raised_above_half_the_ceiling`, `TestLimitsWithTarget/raised_to_the_hard_cap`, `TestLimitsWithTarget/raised_under_a_ceiling_that_fixes_no_budget`
- **Break (5):** the stored target is compared with the floor in full precision, so a target the four-byte rate state stores equal to the floor is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/lowered_to_the_next_number_above_the_floor,_stored_equal_to_it`, `TestLimitsWithTarget/lowered_under_a_ceiling_past_the_four-byte_range`

## An expired lease stops counting against its class's share, and nothing it drew is put back into the bucket

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a lease stops counting half a second before it expires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive's_live_lease_counts_against_its_share`, `TestLive`
- **Break (2):** a lease never expires, so a crashed worker's tokens never return to its class's share
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive's_expired_lease_no_longer_counts,_so_its_share_is_reserved_again`, `TestLive`
- **Break (3):** a lease that expired since the last request is put back into the bucket
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAnExpiredLeaseIsNotRefunded`, `TestNoRunIssuesPastTheHardCap`

## An idle account is reported at the target it will spend under, and a stored target the limiter refuses fails the scrape

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the collector reports an account with no rate state at the default target, ignoring its lowered target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestTheCollectorReportsAnIdleAccountAtItsLoweredTarget`
- **Break (2):** a stored lowered target the limiter refuses is reported at the default target rather than failing the scrape
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestTheCollectorReportsAnIdleAccountAtItsLoweredTarget`

## Cost counted for an account with no hard cap emitted fires the missing-threshold rule

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the cost's window is two minutes, so one missed scrape at a one-minute scrape empties the condition and resets the five minutes it must hold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (2):** the rule fires on its first evaluation rather than after the condition has held for five minutes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (3):** the hard cap's window is five minutes, so a hard cap that left five minutes ago still hides its absence and the account goes ten minutes with nothing firing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (4):** the rule no longer leaves out the accounts that have a hard cap, so every spending account fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (5):** the rule fires for the accounts that have a hard cap rather than those that lack one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`

## Each account spends under the lowered target its state row sets, and a target outside its range stops the run

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a target outside its range is taken as it is given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAnOutOfRangeTargetRefusesTheLimiter`
- **Break (2):** every account spends under the default target, whatever target it was given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestALoweredTargetBoundsOnlyItsOwnAccount`

## Every ask records its class's ask instant, and a waiting worker asks again within each lease period

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a waiting worker asks again every two seconds, longer than a lease period
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAnAskIsRecordedAndRenewedUntilTheWorkerStops`
- **Break (2):** a request left waiting stores nothing, so its ask is never recorded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestAnAskIsRecordedAndRenewedUntilTheWorkerStops`

## Every call the Gmail rate profile sizes fits one second's worth at the hard cap, read from the hard-cap fraction

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the hard-cap fraction falls to 0.60 while the Gmail pages stay sized for 0.80, so an enumeration page no longer fits
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestTheProfileMeetsTheContract`, `TestTheProfileMeetsTheContract/every_call_the_rate_profile_sizes_fits_one_second_at_the_hard_cap`
- **Break (2):** an enumeration page holds four messages, 87 units, past the hard cap of 80
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCostDeclaresEachCallsWorstCase`, `TestCostDeclaresEachCallsWorstCase/{10_0}`, `TestEveryPageFitsAndCallerSizedCallsSplitWhereTheSecondEnds`, `TestParseProfile`, `TestTheProfileMeetsTheContract`, `TestTheProfileMeetsTheContract/every_call_the_rate_profile_sizes_fits_one_second_at_the_hard_cap`

## Every grant of the last second is stored and counted in the one-second window

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the grants of the last second are deleted with the old ones, so the window loses them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`
- **Break (2):** the stored grants are not read into the window, so it counts none of them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`

## Every request the Gmail adapter sends is counted at its cost where it is sent, failed requests included

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a call is counted at the cost it declares when it begins, what its lease covers, rather than each request at what it costs when it is sent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestARequestNeverSentIsNotCounted`, `TestAnOpChangingNothingReadsItsMessage`, `TestEveryRequestSentIsCounted`, `TestEveryRequestSentIsCounted/a_thread`, `TestEveryRequestSentIsCounted/an_enumeration`, `TestEveryRequestSentIsCounted/changes`
- **Break (2):** a request is counted only once Gmail answers it with success, so a failed request goes uncounted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestACallIsHeldToItsDeclaredCost`, `TestAnOpChangingNothingReadsItsMessage`, `TestEveryRequestSentIsCounted`, `TestEveryRequestSentIsCounted/a_body`, `TestEveryRequestSentIsCounted/a_body's_part_held_apart`, `TestEveryRequestSentIsCounted/a_cursor`, `TestEveryRequestSentIsCounted/a_label_listing`, `TestEveryRequestSentIsCounted/a_thread`, `TestEveryRequestSentIsCounted/an_enumeration`, `TestEveryRequestSentIsCounted/changes`
- **Break (3):** a read of a part held apart is counted at no cost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestEveryRequestSentIsCounted`, `TestEveryRequestSentIsCounted/a_body's_part_held_apart`, `TestRequests`, `TestRequests/messages.attachments.get_for_a_body_part_held_apart`
- **Break (4):** a thread read carries the price of a thread listing, so it is counted and held at a quarter of what Gmail charges
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestACallIsHeldToItsDeclaredCost`, `TestEveryRequestSentIsCounted`, `TestEveryRequestSentIsCounted/a_thread`, `TestRequests`, `TestRequests/threads.get`
- **Break (5):** each request is counted as one, whatever Gmail charges for it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryRequestTheAdapterSendsIsOnTheSharedSeries`
- **Break (6):** the count is labelled with a provider value other than the adapter's own name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryRequestTheAdapterSendsIsOnTheSharedSeries`

## Gmail's cost profile declares each call's worst case, every read of the label table included, and sizes its pages to one second's worth at the hard cap

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a body call is declared at both methods' price, so every body declares the cost of its rarest path
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCostDeclaresEachCallsWorstCase`, `TestCostDeclaresEachCallsWorstCase/{4_0}`, `TestCostDeclaresEachCallsWorstCase/{4_1}`
- **Break (2):** EnsureLabel's second read of the label table, after Gmail answers that the label exists, is left out of its cost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCostDeclaresEachCallsWorstCase`, `TestCostDeclaresEachCallsWorstCase/{6_0}`, `TestEachOperationsDearestPathFitsItsCall`, `TestEachOperationsDearestPathFitsItsCall/EnsureLabel`
- **Break (3):** a metadata fetch naming no identifier is declared to cost nothing, so the rate limiter refuses it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCostDeclaresEachCallsWorstCase`, `TestCostDeclaresEachCallsWorstCase/{3_-1}`, `TestCostDeclaresEachCallsWorstCase/{3_0}`, `TestTheProfileMeetsTheContract`, `TestTheProfileMeetsTheContract/the_rate_profile_declares_a_budget_and_a_cost_for_every_operation`
- **Break (4):** a metadata fetch's read of the label table is left out of its cost, so four fetches seem to fit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCostDeclaresEachCallsWorstCase`, `TestCostDeclaresEachCallsWorstCase/{3_-1}`, `TestCostDeclaresEachCallsWorstCase/{3_0}`, `TestCostDeclaresEachCallsWorstCase/{3_3}`, `TestEachOperationsDearestPathFitsItsCall`, `TestEachOperationsDearestPathFitsItsCall/GetMessageMetadata`, `TestEveryPageFitsAndCallerSizedCallsSplitWhereTheSecondEnds`, `TestTheProfileMeetsTheContract`, `TestTheProfileMeetsTheContract/the_rate_profile_declares_a_budget_and_a_cost_for_every_operation`
- **Break (5):** EnumerateAll's read of the profile for its page's total is left out of its cost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCostDeclaresEachCallsWorstCase`, `TestCostDeclaresEachCallsWorstCase/{10_0}`, `TestEachOperationsDearestPathFitsItsCall`, `TestEachOperationsDearestPathFitsItsCall/EnumerateAll`
- **Break (6):** a page of threads holds two threads, whose worst case passes one second's worth at the hard cap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestCostDeclaresEachCallsWorstCase`, `TestCostDeclaresEachCallsWorstCase/{1_0}`, `TestEveryPageFitsAndCallerSizedCallsSplitWhereTheSecondEnds`, `TestTheProfileMeetsTheContract`, `TestTheProfileMeetsTheContract/every_call_the_rate_profile_sizes_fits_one_second_at_the_hard_cap`

## Interactive keeps 30% of the target and sync 20% while a cut comes out of batch, then sync, then interactive

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a cut comes out of interactive first and batch keeps its half of the target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestInteractiveKeepsItsShareThroughACut`, `TestShare`, `TestShare/a_cut_past_batch_comes_out_of_sync`, `TestShare/a_cut_past_sync_comes_out_of_interactive`, `TestShare/a_halving_comes_out_of_batch_alone`, `TestShare/a_small_cut_comes_out_of_batch`, `TestShare/at_the_floor_interactive_has_it_all`
- **Break (2):** the shares are fractions of the budget, so every cut takes from interactive in proportion
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestInteractiveKeepsItsShareThroughACut`, `TestShare`, `TestShare/a_cut_past_batch_comes_out_of_sync`, `TestShare/a_cut_past_sync_comes_out_of_interactive`, `TestShare/a_halving_comes_out_of_batch_alone`, `TestShare/a_small_cut_comes_out_of_batch`, `TestShare/at_the_floor_interactive_has_it_all`

## Issuance holds to the hard cap the declared ceiling gives, whatever the rate state stores

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the ceiling is read back from the stored hard cap, and the limits it gives are written back, so the cap column is the cap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`
- **Break (2):** the ceiling is read back from the stored target, and the limits it gives are written back, so raising the target raises the cap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`

## Issuance never refills the bucket past a lowered target

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the bucket refills at the stored rate held at half the ceiling rather than at the account's lowered target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/a_second_refills_the_lowered_target's_worth`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestNoRunIssuesPastTheHardCap`

## Limits carries no exported field, so no code outside the package can build one and raise its target

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the hard cap becomes an exported field, so code outside the package can build a Limits around its constructors
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLimitsIsBuiltOnlyByItsConstructors`

## Missing account-level series fire the absence rule rather than reading as a healthy account

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a failed read of the accounts reports no series and a successful scrape
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestTheCollectorFailsTheScrapeWhenItCannotRead`
- **Break (2):** the absence rule watches a job workload's series rather than the API server's account-level ones
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`

## No credential is read from the environment

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an empty refresh token falls back to an environment variable, read through a dot import
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestNoSourceFileReadsTheEnvironment`
- **Break (2):** an empty refresh token falls back to an environment variable, read inside a composite literal whose field is selected
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestNoSourceFileReadsTheEnvironment`
- **Break (3):** an empty refresh token falls back to an environment variable, read inside the receiver of a method call
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestNoSourceFileReadsTheEnvironment`
- **Break (4):** an empty refresh token falls back to an environment variable under a name no test sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestNoSourceFileReadsTheEnvironment`
- **Break (5):** an empty refresh token falls back to an environment variable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestNoSourceFileReadsTheEnvironment`, `TestTheSourceHoldsOnlyTheCredentialItIsGiven`, `TestTheSourceHoldsOnlyTheCredentialItIsGiven/no_refresh_token`
- **Break (6):** the refresh token is read from an environment variable rather than taken from the deployable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestARefreshThatDoesNotRotateKeepsTheToken`, `TestARefreshThatDoesNotRotateKeepsTheToken/field_left_out`, `TestAnAccessTokenIsUsedUntilItNearlyExpires`, `TestNoSourceFileReadsTheEnvironment`, `TestTheRefreshRequest`, `TestTheSourceHoldsOnlyTheCredentialItIsGiven`, `TestTheSourceHoldsOnlyTheCredentialItIsGiven/a_refresh_token`, `TestTheSourceHoldsOnlyTheCredentialItIsGiven/no_refresh_token`

## Nothing is issued during a backoff, and the bucket does not fill during one

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the bucket fills during a backoff
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/after_a_backoff_the_bucket_fills_only_from_its_end`, `TestIssue/nothing_is_issued_during_a_backoff,_and_the_bucket_does_not_fill`
- **Break (2):** a request during a backoff is decided as if there were none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/nothing_is_issued_during_a_backoff,_and_the_bucket_does_not_fill`

## Nothing past the bound is downloaded where the provider declares a part's size, nothing past it is returned, and a response past its cap is refused

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the provider fake returns a body past the bound rather than refusing it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_body_past_the_bound_is_refused_at_its_first_call`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_body_past_the_bound_is_refused_at_its_first_call`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_body_past_the_bound_is_refused_at_its_first_call`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_body_past_the_bound_is_refused_at_its_first_call`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_body_past_the_bound_is_refused_at_its_first_call`
- **Break (2):** a part held apart is named for a second call whatever size it declares, so a part past the bound is downloaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestMessageBodyOfAPartHeldApartOrPastTheBound`, `TestMessageBodyOfAPartHeldApartOrPastTheBound/held_apart_past_the_bound`
- **Break (3):** the message read is capped for one part at the bound rather than two, so a body whose rendered part and its alternative both come inline within the bound is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestTheReadCapsFollowTheBound`
- **Break (4):** an answer past a body read's cap is read as the provider's failure, so the message is fetched again on every tick rather than recorded refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestABodyReadsAnswerPastItsCapIsRefused`, `TestABodyReadsAnswerPastItsCapIsRefused/the_message_read`, `TestABodyReadsAnswerPastItsCapIsRefused/the_read_of_a_part_held_apart`, `TestEachRequestIsSentWithItsBuildersCap`, `TestEachRequestIsSentWithItsBuildersCap/the_body_read`, `TestEachRequestIsSentWithItsBuildersCap/the_part_read`, `TestRequests`, `TestRequests/messages.attachments.get_for_a_body_part_held_apart`, `TestRequests/messages.get_for_the_body,_through_the_body_mask`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestTheReadCapsFollowTheBound`
- **Break (5):** the Gmail adapter returns a part's text whatever its length, inline or fetched apart
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core`:** `TestMessageBodyOfAPartHeldApartOrPastTheBound`, `TestMessageBodyOfAPartHeldApartOrPastTheBound/inline_past_the_bound`, `TestPartText`, `TestPartText/decoded_text_past_the_bound`
- **Break (6):** a body read is capped at every other request's cap rather than its own, so an answer past the body read's cap is returned whole
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestABodyReadsAnswerPastItsCapIsRefused`, `TestABodyReadsAnswerPastItsCapIsRefused/the_message_read`, `TestEachRequestIsSentWithItsBuildersCap`, `TestEachRequestIsSentWithItsBuildersCap/the_body_read`, `TestRequests`, `TestRequests/messages.get_for_the_body,_through_the_body_mask`
- **Break (7):** the body read is built with no read cap, so every body read is refused as the provider's failure before it is sent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestABodyReadsAnswerPastItsCapIsRefused`, `TestABodyReadsAnswerPastItsCapIsRefused/the_message_read`, `TestEachRequestIsSentWithItsBuildersCap`, `TestEachRequestIsSentWithItsBuildersCap/the_body_read`, `TestEveryRequestSentIsCounted`, `TestEveryRequestSentIsCounted/a_body`, `TestEveryRequestSentIsTimedByItsEndpoint`, `TestEveryRequestSentIsTimedByItsEndpoint/a_body`, `TestRequests`, `TestRequests/messages.get_for_the_body,_through_the_body_mask`
- **Break (8):** an answer past a body read's cap is returned cut at the cap, so the part is read from a truncated answer rather than refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestABodyReadsAnswerPastItsCapIsRefused`, `TestABodyReadsAnswerPastItsCapIsRefused/the_message_read`, `TestABodyReadsAnswerPastItsCapIsRefused/the_read_of_a_part_held_apart`
- **Break (9):** a response past its read cap is returned cut at the cap and read as whole
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestABodyReadsAnswerPastItsCapIsRefused`, `TestABodyReadsAnswerPastItsCapIsRefused/the_message_read`, `TestABodyReadsAnswerPastItsCapIsRefused/the_read_of_a_part_held_apart`, `TestCapped`
- **Break (10):** a request is sent carrying every other request's cap whatever its builder gave it, so a body read is read to maxResponse and past it is the provider's failure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestEachRequestIsSentWithItsBuildersCap`, `TestEachRequestIsSentWithItsBuildersCap/the_body_read`, `TestEachRequestIsSentWithItsBuildersCap/the_part_read`
- **Break (11):** a request is sent without the read cap its builder gave it, so every request is refused before it is sent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestACallIsHeldToItsDeclaredCost`, `TestARequestIsTimedForItsExchange`, `TestAnOpChangingNothingReadsItsMessage`, `TestEachRequestIsSentWithItsBuildersCap`, `TestEachRequestIsSentWithItsBuildersCap/the_body_read`, `TestEachRequestIsSentWithItsBuildersCap/the_cursor_read`, `TestEachRequestIsSentWithItsBuildersCap/the_label_listing`, `TestEachRequestIsSentWithItsBuildersCap/the_metadata_read`, `TestEachRequestIsSentWithItsBuildersCap/the_part_read`, `TestEveryRequestSentIsCounted`, `TestEveryRequestSentIsCounted/a_body`, `TestEveryRequestSentIsCounted/a_body's_part_held_apart`, `TestEveryRequestSentIsCounted/a_cursor`, `TestEveryRequestSentIsCounted/a_label_listing`, `TestEveryRequestSentIsCounted/a_thread`, `TestEveryRequestSentIsCounted/an_enumeration`, `TestEveryRequestSentIsCounted/changes`, `TestEveryRequestSentIsTimedByItsEndpoint`, `TestEveryRequestSentIsTimedByItsEndpoint/a_body`, `TestEveryRequestSentIsTimedByItsEndpoint/a_body's_part_held_apart`, `TestEveryRequestSentIsTimedByItsEndpoint/a_cursor`, `TestEveryRequestSentIsTimedByItsEndpoint/a_label_listing`, `TestEveryRequestSentIsTimedByItsEndpoint/a_thread`, `TestEveryRequestSentIsTimedByItsEndpoint/changes`
- **Break (12):** a request is sent with its cap's size and not whether past it refuses, so a body read past its cap is the provider's failure
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestEachRequestIsSentWithItsBuildersCap`, `TestEachRequestIsSentWithItsBuildersCap/the_body_read`, `TestEachRequestIsSentWithItsBuildersCap/the_part_read`
- **Break (13):** a request carrying no read cap is sent, so its answer is read to no cap it was given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestARequestWithNoReadCapIsNeverSent`, `TestARequestWithNoReadCapIsNeverSent/a_cap_of_no_bytes`, `TestARequestWithNoReadCapIsNeverSent/no_cap`

## One runaway rule serves every provider, from the hard cap each spending process emits for its account

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the runaway rule ignores the emitted hard cap and compares against Gmail's hard cap of 80 units a second written into the rule
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`

## Only the consent's own redirect address finishes a consent

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a pasted address on any host is read as the redirect
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestParseRedirect`, `TestTheUIsConsentRedirectsToItsGivenAddress`
- **Break (2):** the consent's own address pasted without its path, as a browser shows it, is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestParseRedirect`

## The chart ships the tested alerting rules where the Prometheus Operator's API is served

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the template renders the rules whether or not the operator's API is served
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestTheChartShipsTheRulesWhereTheOperatorsAPIIsServed`
- **Break (2):** the resource carries no rules rather than the tested file
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestTheChartShipsTheRulesWhereTheOperatorsAPIIsServed`

## The collapse rules fire on five minutes at the floor or with nothing granted, and only while a class asks

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the floor rule fires whether or not a class asked in the last minute
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (2):** the floor rule waits ten minutes at the floor before it fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (3):** the floor is emitted at full precision, so a rate held at a floor a four-byte float rounds up reads as above it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAFloorRateReadsAsAtTheFloor`
- **Break (4):** the stall rule fires after one second with nothing granted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (5):** an instant never set reads as just now, so an account waiting on its first grant reads as healthy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestTheCollectorReportsEachAccountsRateState`
- **Break (6):** the stall rule fires only after fifty minutes with nothing granted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`

## The consent accepts a grant of the modify scope alone

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the code exchange's response is read as a grant without checking its scope
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestGrantFrom`, `TestGrantFrom/modify_and_the_full_scope`, `TestGrantFrom/no_scope`, `TestGrantFrom/the_full_mail_scope`
- **Break (2):** a grant holding the modify scope among others is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestAGrantsScopeIsToldApart`, `TestGrantFrom`, `TestGrantFrom/modify_and_the_full_scope`, `TestTheExchangeAcceptsOnlyTheModifyScope`
- **Break (3):** a code exchange's response without a refresh token is read as a grant
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestGrantFrom`, `TestGrantFrom/no_refresh_token`
- **Break (4):** a grant naming no scope is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestAGrantsScopeIsToldApart`, `TestGrantFrom`, `TestGrantFrom/no_scope`, `TestTheExchangeAcceptsOnlyTheModifyScope`

## The consent and the contract run refuse a grant belonging to an account other than the one named, and name neither account

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a grant is accepted whatever account it belongs to
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestRequireAccount`, `TestRequireAccount/an_account_sharing_a_prefix`, `TestRequireAccount/another_account`, `TestRequireAccount/no_account_granted`, `TestRequireAccount/no_account_wanted`
- **Break (2):** the refusal names the account the grant belongs to
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestRequireAccount`, `TestRequireAccount/an_account_sharing_a_prefix`, `TestRequireAccount/another_account`, `TestRequireAccount/no_account_wanted`
- **Break (3):** a grant is accepted for an account whose address begins with the wanted one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestRequireAccount`, `TestRequireAccount/an_account_sharing_a_prefix`

## The consent redirect hands over a code only with the consent's state

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a redirect carrying an error is handed over as a code
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestTheConsentRedirectRefusesAnErrorOrNoCode`, `TestTheConsentRedirectRefusesAnErrorOrNoCode/error`
- **Break (2):** a redirect with the right state and no code is handed over as an empty code
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestTheConsentRedirectRefusesAnErrorOrNoCode`, `TestTheConsentRedirectRefusesAnErrorOrNoCode/no_code`
- **Break (3):** the consent's state is compared with the redirect's code rather than its state
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestTheConsentRedirectHandsOverTheCode`, `TestTheConsentRedirectRefusesAnErrorOrNoCode`, `TestTheConsentRedirectRefusesAnErrorOrNoCode/error`, `TestTheConsentRedirectRefusesAnErrorOrNoCode/no_code`
- **Break (4):** the redirect's state is never checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestTheConsentRedirectRefusesAWrongOrMissingState`, `TestTheConsentRedirectRefusesAWrongOrMissingState/error_with_wrong_state`, `TestTheConsentRedirectRefusesAWrongOrMissingState/missing_state`, `TestTheConsentRedirectRefusesAWrongOrMissingState/wrong_state`

## The contract suite compares, changes and prints only the mail each case added, and a scoped case's listings return only threads carrying its own label

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** each page of an enumeration is logged whole, the mail the case did not add included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheScopedSuitePassesOverMailItDidNotAdd`
- **Break (2):** the change feed keeps the changes to mail the case did not add
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheScopedSuitePassesOverMailItDidNotAdd`
- **Break (3):** a case stars the first message an enumeration returns rather than one it added
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheScopedSuitePassesOverMailItDidNotAdd`
- **Break (4):** a message the case did not add is taken as the case's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheScopedSuitePassesOverMailItDidNotAdd`
- **Break (5):** a message the case did not add fails the test with its subject printed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheScopedSuitePassesOverMailItDidNotAdd`
- **Break (6):** a scoped case lists its queries without its label
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheScopedSuitePassesOverMailItDidNotAdd`

## The contract suite fails an implementation that breaks a promise of the port

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the fake's After selects the messages dated at or before the instant
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_window`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/after_an_instant,_selecting_a_thread_through_its_newest_message`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/a_window`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/after_an_instant,_selecting_a_thread_through_its_newest_message`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheFakePassesTheContractOnOnePage/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/a_window`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/after_an_instant,_selecting_a_thread_through_its_newest_message`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/a_window`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/after_an_instant,_selecting_a_thread_through_its_newest_message`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_window`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/after_an_instant,_selecting_a_thread_through_its_newest_message`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheThrottledFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`
- **Break (2):** the fake's After excludes a message dated at the instant
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/after_the_instant_a_message_is_dated,_which_it_includes`
- **Break (3):** the fake's archive leaves the message in the inbox
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/each_verb_changes_exactly_what_it_names`, `TestTheFakePassesTheContract/each_verb_changes_exactly_what_it_names/archive`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/each_verb_changes_exactly_what_it_names`, `TestTheFakePassesTheContractOnOnePage/each_verb_changes_exactly_what_it_names/archive`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/each_verb_changes_exactly_what_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/each_verb_changes_exactly_what_it_names/archive`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/each_verb_changes_exactly_what_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/each_verb_changes_exactly_what_it_names/archive`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/each_verb_changes_exactly_what_it_names`, `TestTheThrottledFakePassesTheContract/each_verb_changes_exactly_what_it_names/archive`
- **Break (4):** the fake's Before includes a message dated at the instant
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/before_the_instant_a_message_is_dated,_which_it_excludes`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/before_the_instant_a_message_is_dated,_which_it_excludes`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/before_the_instant_a_message_is_dated,_which_it_excludes`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/before_the_instant_a_message_is_dated,_which_it_excludes`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/before_the_instant_a_message_is_dated,_which_it_excludes`
- **Break (5):** the fake's current cursor is always its first
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestExpiredCursorsAreGaps`, `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_change_is_reported_once,_as_a_modification`, `TestTheFakePassesTheContract/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakePassesTheContract/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakePassesTheContract/delivered_mail_is_added`, `TestTheFakePassesTheContract/no_activity_means_no_changes`, `TestTheFakePassesTheContract/removed_mail_is_removed`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_change_is_reported_once,_as_a_modification`, `TestTheFakePassesTheContractOnOnePage/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakePassesTheContractOnOnePage/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakePassesTheContractOnOnePage/delivered_mail_is_added`, `TestTheFakePassesTheContractOnOnePage/no_activity_means_no_changes`, `TestTheFakePassesTheContractOnOnePage/removed_mail_is_removed`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_change_is_reported_once,_as_a_modification`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/delivered_mail_is_added`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/no_activity_means_no_changes`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/removed_mail_is_removed`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_change_is_reported_once,_as_a_modification`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/delivered_mail_is_added`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/no_activity_means_no_changes`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/removed_mail_is_removed`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_change_is_reported_once,_as_a_modification`, `TestTheThrottledFakePassesTheContract/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheThrottledFakePassesTheContract/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheThrottledFakePassesTheContract/delivered_mail_is_added`, `TestTheThrottledFakePassesTheContract/no_activity_means_no_changes`, `TestTheThrottledFakePassesTheContract/removed_mail_is_removed`
- **Break (6):** the fake reports delivered mail as modified
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestDeliveryAndRemovalAreChanges`, `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakePassesTheContract/delivered_mail_is_added`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakePassesTheContractOnOnePage/delivered_mail_is_added`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/delivered_mail_is_added`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/delivered_mail_is_added`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_message_delivered_and_then_changed_is_reported_once,_as_added`, `TestTheThrottledFakePassesTheContract/delivered_mail_is_added`
- **Break (7):** the fake's enumeration skips the first message of every page after the first
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/enumeration_returns_every_message_once_with_what_was_seeded`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_returns_every_message_once_with_what_was_seeded`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/enumeration_returns_every_message_once_with_what_was_seeded`
- **Break (8):** the fake's query selects every message wherever it asks for a label
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_label`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_label_and_a_sender_no_message_has_together`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_nested_label,_selecting_the_thread_through_each_of_its_messages`, `TestTheFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/the_inbox`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/a_label`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/a_label_and_a_sender_no_message_has_together`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/a_nested_label,_selecting_the_thread_through_each_of_its_messages`, `TestTheFakePassesTheContractOnOnePage/a_query_selects_exactly_the_threads_it_names/the_inbox`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/a_label`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/a_label_and_a_sender_no_message_has_together`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/a_nested_label,_selecting_the_thread_through_each_of_its_messages`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_query_selects_exactly_the_threads_it_names/the_inbox`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/a_label`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/a_label_and_a_sender_no_message_has_together`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/a_nested_label,_selecting_the_thread_through_each_of_its_messages`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_query_selects_exactly_the_threads_it_names/the_inbox`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_label`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_label_and_a_sender_no_message_has_together`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/a_nested_label,_selecting_the_thread_through_each_of_its_messages`, `TestTheThrottledFakePassesTheContract/a_query_selects_exactly_the_threads_it_names/the_inbox`
- **Break (9):** the fake reports a mutated message as added
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_change_is_reported_once,_as_a_modification`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_change_is_reported_once,_as_a_modification`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_change_is_reported_once,_as_a_modification`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_change_is_reported_once,_as_a_modification`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_change_is_reported_once,_as_a_modification`
- **Break (10):** the fake records no change when a mutation changes a message
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_change_is_reported_once,_as_a_modification`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_change_is_reported_once,_as_a_modification`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_change_is_reported_once,_as_a_modification`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_change_is_reported_once,_as_a_modification`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_change_is_reported_once,_as_a_modification`
- **Break (11):** the fake applies a batch's ops in reverse order
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/each_verb_changes_exactly_what_it_names`, `TestTheFakePassesTheContract/each_verb_changes_exactly_what_it_names/two_ops_on_one_message,_applied_in_order`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/each_verb_changes_exactly_what_it_names`, `TestTheFakePassesTheContractOnOnePage/each_verb_changes_exactly_what_it_names/two_ops_on_one_message,_applied_in_order`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/each_verb_changes_exactly_what_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/each_verb_changes_exactly_what_it_names/two_ops_on_one_message,_applied_in_order`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/each_verb_changes_exactly_what_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/each_verb_changes_exactly_what_it_names/two_ops_on_one_message,_applied_in_order`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/each_verb_changes_exactly_what_it_names`, `TestTheThrottledFakePassesTheContract/each_verb_changes_exactly_what_it_names/two_ops_on_one_message,_applied_in_order`
- **Break (12):** the fake's enumeration starts from the first message whatever page token it is given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/enumeration_resumes_from_any_page_token`, `TestTheFakePassesTheContract/enumeration_returns_every_message_once_with_what_was_seeded`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_resumes_from_any_page_token`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_returns_every_message_once_with_what_was_seeded`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheSuiteStopsAReserveBeforeItsDeadline`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/enumeration_resumes_from_any_page_token`, `TestTheThrottledFakePassesTheContract/enumeration_returns_every_message_once_with_what_was_seeded`
- **Break (13):** the fake refuses an op removing a label the account lacks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/removing_a_label_the_message_or_the_account_lacks_changes_nothing`
- **Break (14):** the fake refuses an op removing a label the message does not carry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/removing_a_label_the_message_or_the_account_lacks_changes_nothing`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/removing_a_label_the_message_or_the_account_lacks_changes_nothing`
- **Break (15):** the fake records no change when mail is removed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestDeliveryAndRemovalAreChanges`, `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakePassesTheContract/removed_mail_is_removed`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakePassesTheContractOnOnePage/removed_mail_is_removed`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/removed_mail_is_removed`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/removed_mail_is_removed`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_message_delivered_and_then_removed_is_reported_once,_as_removed`, `TestTheThrottledFakePassesTheContract/removed_mail_is_removed`
- **Break (16):** the resume cases restart from the start of a listing that issued no page token, so an implementation whose listing fits one page fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/enumeration_resumes_from_any_page_token`, `TestTheFakePassesTheContractOnOnePage/thread_listing_resumes_from_any_page_token`
- **Break (17):** the fake's thread listing leaves out a thread holding a message marked as spam
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`
- **Break (18):** the fake's thread listing starts from the first thread when given a page token it did not issue
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/thread_listing_refuses_a_page_token_it_did_not_issue`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/thread_listing_refuses_a_page_token_it_did_not_issue`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/thread_listing_refuses_a_page_token_it_did_not_issue`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/thread_listing_refuses_a_page_token_it_did_not_issue`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/thread_listing_refuses_a_page_token_it_did_not_issue`
- **Break (19):** the fake's thread listing leaves out a thread holding a message in the trash
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`
- **Break (20):** the fake's metadata read leaves out a message in the trash
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/each_verb_changes_exactly_what_it_names`, `TestTheFakePassesTheContract/each_verb_changes_exactly_what_it_names/trash`, `TestTheFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/each_verb_changes_exactly_what_it_names`, `TestTheFakePassesTheContractOnOnePage/each_verb_changes_exactly_what_it_names/trash`, `TestTheFakePassesTheContractOnOnePage/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/each_verb_changes_exactly_what_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/each_verb_changes_exactly_what_it_names/trash`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/each_verb_changes_exactly_what_it_names`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/each_verb_changes_exactly_what_it_names/trash`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/the_trash_and_spam_are_listed_and_read_like_any_other_mail`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/each_verb_changes_exactly_what_it_names`, `TestTheThrottledFakePassesTheContract/each_verb_changes_exactly_what_it_names/trash`, `TestTheThrottledFakePassesTheContract/the_trash_and_spam_are_listed_and_read_like_any_other_mail`
- **Break (21):** the fake applies a batch holding an op no constructor built
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/a_batch_holding_an_op_no_constructor_built_applies_nothing`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/a_batch_holding_an_op_no_constructor_built_applies_nothing`, `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/a_batch_holding_an_op_no_constructor_built_applies_nothing`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/a_batch_holding_an_op_no_constructor_built_applies_nothing`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/a_batch_holding_an_op_no_constructor_built_applies_nothing`

## The contract suite fails an implementation whose enumeration reports a total it was not configured to, reports none when configured to, passes its page limit, or counts fewer messages than the case added

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the fake reports a page limit one below the most items its pages hold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_resumes_from_any_page_token`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_returns_every_message_once_with_what_was_seeded`
- **Break (2):** the fake reports a total whether or not it is configured to
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakePassesTheContract`, `TestTheFakePassesTheContract/enumeration_resumes_from_any_page_token`, `TestTheFakePassesTheContract/enumeration_returns_every_message_once_with_what_was_seeded`, `TestTheFakePassesTheContractOnOnePage`, `TestTheFakePassesTheContractOnOnePage/enumeration_resumes_from_any_page_token`, `TestTheFakePassesTheContractOnOnePage/enumeration_returns_every_message_once_with_what_was_seeded`, `TestTheScopedSuitePassesOverMailItDidNotAdd`, `TestTheSuiteStopsAReserveBeforeItsDeadline`, `TestTheThrottledFakePassesTheContract`, `TestTheThrottledFakePassesTheContract/enumeration_resumes_from_any_page_token`, `TestTheThrottledFakePassesTheContract/enumeration_returns_every_message_once_with_what_was_seeded`
- **Break (3):** the fake's total counts the messages of the page rather than of the mailbox
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_resumes_from_any_page_token`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_returns_every_message_once_with_what_was_seeded`
- **Break (4):** the fake reports no total even when configured to
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheFakeReportingATotalPassesTheContract`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/enumeration_resumes_from_any_page_token`, `TestTheFakeReportingATotalPassesTheContract/pages_of_100/enumeration_returns_every_message_once_with_what_was_seeded`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_resumes_from_any_page_token`, `TestTheFakeReportingATotalPassesTheContract/pages_of_2/enumeration_returns_every_message_once_with_what_was_seeded`

## The contract suite stops a reserve before its test's deadline, inside a case as well as between cases, so every case it ran cleans up

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the suite starts every case whatever time is left
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheSuiteStopsAReserveBeforeItsDeadline`
- **Break (2):** an enumeration pages on past the reserve
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestTheSuiteStopsAReserveBeforeItsDeadline`

## The controller converges below a real ceiling and recovers after throttling

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a success stores no additive increase, so the rate never climbs back after a cut
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestTheControllerConvergesBelowARealCeilingAndRecovers`
- **Break (2):** a throttle stores the state it read, so the rate stays at the target above the real ceiling
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAFloorRateReadsAsAtTheFloor`, `TestTheControllerConvergesBelowARealCeilingAndRecovers`, `TestTheLimiterEmitsItsProcesssSeries`

## The controller keeps the rate between the floor and the target

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** a throttle's halving is no longer held at the floor, so the rate falls below it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestNoRunIssuesPastTheHardCap`, `TestThrottled`, `TestThrottled/held_at_the_floor`
- **Break (2):** additive increase is no longer held at the target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestNoRunIssuesPastTheHardCap`, `TestSucceeded`, `TestSucceeded/a_huge_cost_reaches_only_the_target`, `TestSucceeded/a_stored_rate_above_the_target_is_brought_down`, `TestSucceeded/at_the_target`, `TestSucceeded/held_at_the_target`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_huge_success_reaches_only_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_stored_rate_at_the_default_target_is_brought_down_to_it`, `TestTheControllerHoldsALoweredTarget/a_success_stops_at_the_lowered_target`, `TestTheRateClimbsToALoweredTargetAndNoFurther`
- **Break (3):** a stored rate outside the floor and the target is used as it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestNoBudgetHoldsTheRateAtZero`, `TestNoRunIssuesPastTheHardCap`, `TestServerErrored`, `TestServerErrored/a_stored_rate_above_the_target_falls_from_the_target`, `TestServerErrored/a_stored_rate_that_is_not_a_number_is_taken_as_the_floor`, `TestSucceeded`, `TestSucceeded/a_stored_rate_below_the_floor_is_brought_up`, `TestSucceeded/a_stored_rate_that_is_not_a_number_is_taken_as_the_floor`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_latency_cut_falls_from_it`, `TestTheControllerHoldsALoweredTarget/a_server_error_falls_from_it`, `TestTheControllerHoldsALoweredTarget/a_throttle_halves_from_it`, `TestThrottled`, `TestThrottled/a_stored_rate_above_the_target_halves_from_the_target`

## The controller never raises the rate above a lowered target

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break:** the additive increase stops at half the ceiling rather than at the account's lowered target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestNoRunIssuesPastTheHardCap`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_huge_success_reaches_only_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_stored_rate_at_the_default_target_is_brought_down_to_it`, `TestTheControllerHoldsALoweredTarget/a_success_stops_at_the_lowered_target`, `TestTheRateClimbsToALoweredTargetAndNoFurther`

## The Gmail adapter emits its account's hard cap beside the count of request cost, from the ceiling its rate profile declares

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the adapter emits the declared ceiling rather than the hard cap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryRequestTheAdapterSendsIsOnTheSharedSeries`
- **Break (2):** the adapter never sets the hard cap, so the runaway rule has nothing to compare the count against
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestEveryRequestTheAdapterSendsIsOnTheSharedSeries`

## The Gmail adapter requires only a label an op adds to exist, and an op left with nothing to change still finds its message

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an op left with nothing to change is reported applied without reading its message, so an unknown message is never found
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestAnOpChangingNothingReadsItsMessage`
- **Break (2):** a move from a label the account lacks is not found
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestOpChange`, `TestOpChange/move_from_a_label_the_account_lacks`
- **Break (3):** an unlabel of a label the account lacks is not found
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail`:** `TestAnOpChangingNothingReadsItsMessage`, `TestOpChange`, `TestOpChange/unlabel_a_label_the_account_lacks`, `TestOpChange/unlabel_a_label_the_model_leaves_out`

## The Gmail grant requests only the modify scope

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the consent requests the full mail scope beside the modify scope
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestTheConsentAsksForAnOfflineGrant`, `TestTheConsentRequestsOnlyTheModifyScope`
- **Break (2):** the consent asks Google to fold every scope the account granted this client before into the new token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestTheConsentAsksForAnOfflineGrant`, `TestTheConsentRequestsOnlyTheModifyScope`
- **Break (3):** the consent requests the full mail scope, which grants permanent delete, in place of the modify scope
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestAGrantsScopeIsToldApart`, `TestGrantFrom`, `TestTheConsentAsksForAnOfflineGrant`, `TestTheConsentRequestsOnlyTheModifyScope`, `TestTheExchangeAcceptsOnlyTheModifyScope`
- **Break (4):** the consent requests no scope
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestTheConsentAsksForAnOfflineGrant`, `TestTheConsentRequestsOnlyTheModifyScope`

## The latency baseline is the lowest of the last ten medians of one-minute windows holding at least 20 samples

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** every remembered median counts, so an early low median holds the baseline down forever
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestALastingRiseBecomesTheBaseline`, `TestBaseline`, `TestBaseline/only_the_last_ten_count`, `TestRememberMedian`, `TestRememberMedian/a_longer_stored_list_keeps_its_last_nine`, `TestRememberMedian/the_eleventh_drops_the_oldest`
- **Break (2):** the baseline is the latest median rather than the lowest, so a rise becomes the baseline at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestBaseline`, `TestBaseline/only_the_last_ten_count`, `TestBaseline/the_lowest`
- **Break (3):** a window with fewer than 20 samples has a median
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLatencyWindowAdd`, `TestLatencyWindowAdd/a_window_closing_with_too_few_samples_has_no_median`, `TestMedian`, `TestMedian/19_samples_are_too_few`
- **Break (4):** a window never closes, so no median is ever taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestLatencyWindowAdd`, `TestLatencyWindowAdd/a_sample_a_minute_after_the_start_closes_the_window_with_its_median`, `TestLatencyWindowAdd/a_window_closing_with_too_few_samples_has_no_median`

## The mailbox a grant belongs to is compared as Google identifies a mailbox

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the mailboxes are compared with their case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestRequireAccount`, `TestSameMailbox`
- **Break (2):** the mailboxes are compared ignoring case alone
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestSameMailbox`
- **Break (3):** the dots of every domain's local part are ignored, so another mailbox outside Gmail passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/google`:** `TestSameMailbox`

## The provider fake throttles on schedule, and a throttled call has no effect

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the ceiling counts the calls it refuses as spent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestCeilingThrottlesAboveTheRate`
- **Break (2):** a throttled mutation still reaches the wrapped port
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/mail/fake`:** `TestAThrottledCallHasNoEffect`

## The provider library makes an account's token from the OAuth client it connects through and its own credential

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the client's identifier and secret are swapped in the token's credentials
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestAGmailTokenIsMadeFromItsClientAndItsCredential`
- **Break (2):** the client's secret is taken as the account's refresh token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider`:** `TestAGmailTokenIsMadeFromItsClientAndItsCredential`, `TestAMailboxNeedsAShippedAdapterAndItsToken`

## The runaway rule fires on two minutes of provider request cost above the hard cap times 120 seconds, summed over every spending process

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the runaway rule takes the busiest process for the account rather than summing them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (2):** the runaway threshold is twice the hard cap times 120 seconds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (3):** the runaway window is one minute, which holds a single sample when the platform scrapes once a minute, so the rule never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`

## The runaway rule judges each account by the highest hard cap its processes emitted over its two minutes, so a process that has gone keeps its threshold

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the hard cap is read at the instant of evaluation, so a process whose series went stale takes the threshold with it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (2):** the account is judged by the lowest hard cap its processes emitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`
- **Break (3):** the account is judged by the mean of the hard caps its processes emitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit`:** `TestAlertingRules`

## The tokens issued inside any one-second window sum to at most the hard cap, and over a longer window stay within the hard cap plus the target times its length, while the stored state is kept whole and current, and the clock does not step forward

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the bucket holds two seconds' worth at the hard cap rather than one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/a_fresh_bucket_fills_to_its_capacity_and_grants`, `TestIssue/a_request_past_one_second's_worth_at_the_hard_cap_is_refused_and_changes_nothing`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestIssue/a_stored_level_above_the_capacity_is_brought_down_to_it`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** the issuer's clock follows an instant earlier than the last, so a stretch of time fills the bucket twice
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/an_instant_earlier_than_the_last_is_taken_as_the_last,_and_fills_nothing`, `TestNoRunIssuesPastTheHardCap`
- **Break (3):** the bucket refills at whatever rate the state holds, not held at the target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/a_rate_above_the_target_refills_at_the_target`, `TestIssue/an_infinite_rate_refills_at_the_target`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/a_second_refills_the_lowered_target's_worth`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestNoRunIssuesPastTheHardCap`
- **Break (4):** a stored level above the capacity is used as it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/a_stored_level_above_the_capacity_is_brought_down_to_it`, `TestNoRunIssuesPastTheHardCap`
- **Break (5):** the window counts only the grants of the last half second
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestIssue`, `TestIssue/a_grant_that_fits_the_window_is_issued_beside_the_others`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestNoRunIssuesPastTheHardCap`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`
- **Break (6):** the one-second window is not checked, so the bucket alone decides
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_grant_that_is_not_a_number_fills_the_window`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestNoRunIssuesPastTheHardCap`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`
- **Break (7):** a grant's window end is its stamp plus a second, which saturates at the clock's limit, so a grant there leaves the window at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core`:** `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`
