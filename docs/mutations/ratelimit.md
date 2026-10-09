# Mutations: ratelimit

The demonstrations of the controls whose patches sit in `ratelimit/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A ceiling that is not a finite positive number fixes no budget

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** an infinite ceiling fixes an infinite budget
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssueRefuses`, `TestIssueRefuses/an_infinite_ceiling`, `TestLimitsFor`, `TestLimitsFor/positive_infinity`, `TestNoBudgetHoldsTheRateAtZero`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** a negative ceiling fixes negative limits rather than none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLimitsFor`, `TestLimitsFor/negative`, `TestLimitsFor/negative_infinity`, `TestNoBudgetHoldsTheRateAtZero`

## A lease counts against its class's share until it expires one second after it was issued

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** a stored grant is kept for a minute and its lease counts against its class's share all that time
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`
- **Break (2):** a stored grant's lease expires the instant it is issued, so it never counts against its class's share
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestALiveLeaseCountsAgainstItsClassShare`
- **Break (3):** every grant is stored as batch's, so a lease counts against the wrong class's share
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestALiveLeaseCountsAgainstItsClassShare`

## A lease holds the whole request, and a request larger than the bucket can ever hold is refused

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a request larger than the bucket can ever hold is left waiting
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/a_request_past_one_second's_worth_at_the_hard_cap_is_refused_and_changes_nothing`, `TestIssueRefuses`, `TestIssueRefuses/a_ceiling_of_zero`, `TestIssueRefuses/a_ceiling_that_is_not_a_number`, `TestIssueRefuses/a_negative_ceiling`, `TestIssueRefuses/an_infinite_ceiling`, `TestIssueRefuses/infinite_tokens`
- **Break (2):** a request the bucket cannot cover is granted what the bucket holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestACallCostingMoreThanASecondAtTheFloorIsGranted`, `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAThreadFetchIsGrantedOnceTheBucketHoldsIt`, `TestAnExpiredLeaseIsNotRefunded`, `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`, `TestIssue/a_rate_above_the_target_refills_at_the_target`, `TestIssue/a_request_the_bucket_cannot_cover_yet_waits,_and_is_still_asking`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestIssueSplitsByClass`, `TestIssueSplitsByClass/a_stored_lease_of_negative_tokens_counts_as_none`, `TestIssueSplitsByClass/and_no_more`, `TestIssueSplitsByClass/another_class's_lease_does_not_count_against_interactive's_share`, `TestIssueSplitsByClass/batch_waits_on_both_shares`, `TestIssueSplitsByClass/interactive's_expired_lease_no_longer_counts,_so_its_share_is_reserved_again`, `TestIssueSplitsByClass/sync_waits_on_interactive's_share`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestIssueUnderALoweredTarget/and_no_more_than_the_share_lends`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`
- **Break (3):** a request larger than what the bucket holds now is refused rather than left waiting
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestAnExpiredLeaseIsNotRefunded`, `TestInteractiveKeepsItsShareThroughACut`, `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`, `TestIssue/a_negative_rate_refills_nothing`, `TestIssue/a_negative_stored_level_is_taken_as_empty`, `TestIssue/a_rate_above_the_target_refills_at_the_target`, `TestIssue/a_rate_that_is_not_a_number_refills_nothing`, `TestIssue/a_request_the_bucket_cannot_cover_yet_waits,_and_is_still_asking`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_grant_that_is_not_a_number_fills_the_window`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestIssue/a_stored_level_that_is_not_a_number_is_taken_as_empty`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestIssueSplitsByClass`, `TestIssueSplitsByClass/a_stored_lease_of_negative_tokens_counts_as_none`, `TestIssueSplitsByClass/and_no_more`, `TestIssueSplitsByClass/another_class's_lease_does_not_count_against_interactive's_share`, `TestIssueSplitsByClass/batch_waits_on_both_shares`, `TestIssueSplitsByClass/interactive's_expired_lease_no_longer_counts,_so_its_share_is_reserved_again`, `TestIssueSplitsByClass/sync_waits_on_interactive's_share`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestIssueUnderALoweredTarget/and_no_more_than_the_share_lends`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`

## A request costing more than one second's worth at the hard cap is refused at once, never left waiting

- **Date · evidence:** 2026-09-24 · [pull request #157](https://github.com/ppat/mediated-mailbox-mcp/pull/157), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** a refused request returns an empty lease and no error, as if granted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestARequestPastTheHardCapIsRefusedAtOnce`
- **Break (2):** a refused request is left waiting like one the bucket cannot yet fill
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestARequestPastTheHardCapIsRefusedAtOnce`

## A request draws all but what the classes that outrank it and asked within the last lease period have still to draw of their shares, so an idle class's share is lent

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a class's leases do not count against its share, so its whole share stays reserved however much it holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive's_live_lease_counts_against_its_share`
- **Break (2):** every other class that asked reserves its share, so interactive waits on the classes it outranks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive_does_not_wait_on_batch's_share`, `TestIssueSplitsByClass/sync_does_not_wait_on_batch's_share`
- **Break (3):** an idle class's share is reserved as if it had asked, so nothing is lent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestACallCostingMoreThanASecondAtTheFloorIsGranted`, `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestAValidRequestIsGrantedOnceTheBucketHoldsIt`, `TestAnExpiredLeaseIsNotRefunded`, `TestInteractiveKeepsItsShareThroughACut`, `TestIssue`, `TestIssue/a_grant_a_second_old_has_left_the_window`, `TestIssue/a_request_of_one_second's_worth_at_the_hard_cap_is_granted_from_a_full_bucket`, `TestIssue/a_stored_grant_of_negative_tokens_counts_as_none`, `TestIssue/after_a_backoff_the_bucket_fills_only_from_its_end`, `TestIssue/an_infinite_rate_refills_at_the_target`, `TestIssue/an_instant_earlier_than_the_last_is_taken_as_the_last,_and_fills_nothing`, `TestIssue/the_bucket_refills_at_the_rate`, `TestIssueSplitsByClass`, `TestIssueSplitsByClass/an_idle_interactive_share_is_lent`, `TestIssueSplitsByClass/batch_may_draw_all_but_what_interactive_has_still_to_draw_of_its_share`, `TestIssueSplitsByClass/interactive's_live_lease_counts_against_its_share`, `TestIssueSplitsByClass/sync_does_not_wait_on_batch's_share`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/a_full_bucket_still_holds_one_second's_worth_at_the_hard_cap`, `TestIssueUnderALoweredTarget/a_second_refills_the_lowered_target's_worth`, `TestIssueUnderALoweredTarget/batch_may_draw_all_but_interactive's_share_of_the_lowered_target`

## A stored grant stamped later than the request that sees it counts for one second from that request and then leaves the window

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a stored grant's stamp is held to the stored latest instant rather than the request's, so a stored instant that came back behind moves every real grant out of the window
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestIssue`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** a stored grant keeps its stamp however far ahead it lies, so it fills the window until the clock reaches it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestIssue`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`

## A stored latest instant ahead of the clock never moves a real grant out of the one-second window

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** window membership is judged from the instant issuance raised the request to, so a stored instant ahead of the clock moves every real grant out of the window
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** window membership is judged from the stored latest instant rather than the request's own, so a stored instant ahead of the clock moves every real grant out of the window
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestNoRunIssuesPastTheHardCap`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`

## A throttle halves the rate, a server error cuts it to 80% and a latency median above twice the baseline cuts it to 90%

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a latency median above the baseline itself cuts the rate, not only one above twice it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestALastingRiseBecomesTheBaseline`, `TestLatencyMeasured`, `TestLatencyMeasured/below_twice_the_baseline_does_not`, `TestLatencyMeasured/exactly_twice_the_baseline_does_not`
- **Break (2):** a latency median never cuts the rate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestALastingRiseBecomesTheBaseline`, `TestLatencyMeasured`, `TestLatencyMeasured/held_at_the_floor`, `TestLatencyMeasured/more_than_twice_the_baseline_cuts_to_90%`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_latency_cut_falls_from_it`
- **Break (3):** a server error halves the rate as a throttle does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestServerErrored`, `TestServerErrored/a_stored_rate_above_the_target_falls_from_the_target`, `TestServerErrored/falls_to_80%`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_server_error_falls_from_it`
- **Break (4):** a throttle leaves the rate as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_throttle_halves_from_it`, `TestThrottled`, `TestThrottled/a_draw_above_one_waits_the_whole_bound`, `TestThrottled/a_draw_just_below_one_waits_almost_the_bound`, `TestThrottled/a_draw_of_one_waits_the_whole_bound`, `TestThrottled/a_draw_of_zero_waits_nothing`, `TestThrottled/a_draw_that_is_not_a_number_waits_the_whole_bound`, `TestThrottled/a_jitter_throttle_during_a_one-hour_retry-after_keeps_the_hour`, `TestThrottled/a_later_throttle_extends_a_backoff_from_its_own_instant`, `TestThrottled/a_negative_count_starts_over`, `TestThrottled/a_negative_draw_waits_the_whole_bound`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_retry-after_near_the_clock's_limit_saturates`, `TestThrottled/a_retry-after_not_marked_present_counts_as_absent`, `TestThrottled/a_retry-after_of_a_project_throttle_is_honored_the_same`, `TestThrottled/a_retry-after_of_an_hour_is_honored_as_given`, `TestThrottled/a_stored_rate_above_the_target_halves_from_the_target`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/halves_the_rate_and_honors_the_retry-after`, `TestThrottled/held_at_the_floor`, `TestThrottled/the_bound_doubles_per_earlier_throttle`, `TestThrottled/the_bound_holds_at_32_seconds_at_the_largest_count`, `TestThrottled/the_bound_holds_at_32_seconds_past_it`, `TestThrottled/the_bound_stops_doubling_at_32_seconds`, `TestThrottled/the_first_throttle_draws_within_one_second`, `TestThrottled/the_largest_retry-after_saturates_instead_of_wrapping`, `TestThrottled/the_smallest_retry-after_counts_as_absent`

## A throttle waits out the provider's retry-after when it gives one and a full-jitter draw otherwise

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a retry-after of zero or in the past is honored, so the backoff ends at once or before it starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/the_smallest_retry-after_counts_as_absent`
- **Break (2):** a success no longer ends the run of throttles, so the bound keeps doubling across successes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestSucceeded`, `TestSucceeded/a_success_ends_a_run_of_throttles_and_keeps_the_backoff`
- **Break (3):** the draw is ignored, so every backoff waits the whole bound as a fixed backoff does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_throttle_halves_from_it`, `TestThrottled`, `TestThrottled/a_draw_just_below_one_waits_almost_the_bound`, `TestThrottled/a_draw_of_zero_waits_nothing`, `TestThrottled/a_negative_count_starts_over`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_retry-after_not_marked_present_counts_as_absent`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/the_bound_doubles_per_earlier_throttle`, `TestThrottled/the_bound_holds_at_32_seconds_at_the_largest_count`, `TestThrottled/the_bound_holds_at_32_seconds_past_it`, `TestThrottled/the_bound_stops_doubling_at_32_seconds`, `TestThrottled/the_first_throttle_draws_within_one_second`, `TestThrottled/the_smallest_retry-after_counts_as_absent`
- **Break (4):** the jitter's bound stays at one second whatever the count of throttles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_draw_above_one_waits_the_whole_bound`, `TestThrottled/a_draw_just_below_one_waits_almost_the_bound`, `TestThrottled/a_draw_of_one_waits_the_whole_bound`, `TestThrottled/a_draw_that_is_not_a_number_waits_the_whole_bound`, `TestThrottled/a_negative_draw_waits_the_whole_bound`, `TestThrottled/a_retry-after_in_the_past_counts_as_absent`, `TestThrottled/a_retry-after_not_marked_present_counts_as_absent`, `TestThrottled/a_zero_retry-after_counts_as_absent`, `TestThrottled/the_bound_doubles_per_earlier_throttle`, `TestThrottled/the_bound_holds_at_32_seconds_at_the_largest_count`, `TestThrottled/the_bound_holds_at_32_seconds_past_it`, `TestThrottled/the_bound_stops_doubling_at_32_seconds`, `TestThrottled/the_smallest_retry-after_counts_as_absent`
- **Break (5):** a throttle arriving during a backoff replaces its end, so a shorter draw cuts a retry-after short
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_jitter_throttle_during_a_one-hour_retry-after_keeps_the_hour`
- **Break (6):** a retry-after is ignored and full jitter always applies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_later_throttle_extends_a_backoff_from_its_own_instant`, `TestThrottled/a_retry-after_of_a_project_throttle_is_honored_the_same`, `TestThrottled/a_retry-after_of_an_hour_is_honored_as_given`, `TestThrottled/a_stored_rate_above_the_target_halves_from_the_target`, `TestThrottled/halves_the_rate_and_honors_the_retry-after`, `TestThrottled/held_at_the_floor`, `TestThrottled/the_largest_retry-after_saturates_instead_of_wrapping`
- **Break (7):** the backoff's end is a plain sum that wraps into the past near the clock's limit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestThrottled`, `TestThrottled/a_retry-after_near_the_clock's_limit_saturates`, `TestThrottled/the_largest_retry-after_saturates_instead_of_wrapping`

## A throttle's cut is stored, and batch absorbs it while interactive keeps its share

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** a throttle stores the state it read, so the rate is never cut
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAFloorRateReadsAsAtTheFloor`, `TestTheControllerConvergesBelowARealCeilingAndRecovers`, `TestTheLimiterEmitsItsProcesssSeries`
- **Break (2):** a throttle is stored as a server error, cutting the rate to 80% rather than half
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAFloorRateReadsAsAtTheFloor`, `TestTheLimiterEmitsItsProcesssSeries`

## A worker that waited on the account's lock is stamped with the clock read after it got the lock

- **Date · evidence:** 2026-09-24 · [pull request #157](https://github.com/ppat/mediated-mailbox-mcp/pull/157), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** the clock is read before the lock is taken, so a worker that waited is stamped early
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`
- **Break (2):** no lock is taken, so issuers for one account do not take turns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`

## Additive increase grows the rate by 2% of the target times the call's cost over the current rate

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** the step is no longer divided by the current rate, so growth per second rises with the rate
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestSucceeded`, `TestSucceeded/a_cost_of_the_whole_rate_grows_it_by_2%_of_the_target`, `TestSucceeded/a_success_ends_a_run_of_throttles_and_keeps_the_backoff`, `TestSucceeded/from_the_floor`, `TestSucceeded/from_twice_the_floor`, `TestTheClimbHoldsForManySmallSuccesses`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_success_grows_the_rate_by_2%_of_the_lowered_target`, `TestTheRateClimbsFromTheFloorToTheTargetIn45Seconds`, `TestTheRateClimbsToALoweredTargetAndNoFurther`
- **Break (2):** the step ignores the call's cost, so a costly success grows the rate no more than a cheap one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestSucceeded`, `TestSucceeded/a_cost_of_the_whole_rate_grows_it_by_2%_of_the_target`, `TestSucceeded/a_huge_cost_reaches_only_the_target`, `TestSucceeded/from_the_floor`, `TestSucceeded/from_twice_the_floor`, `TestSucceeded/held_at_the_target`, `TestTheClimbHoldsForManySmallSuccesses`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_huge_success_reaches_only_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_success_grows_the_rate_by_2%_of_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_success_stops_at_the_lowered_target`, `TestTheRateClimbsFromTheFloorToTheTargetIn45Seconds`, `TestTheRateClimbsToALoweredTargetAndNoFurther`

## An account may lower its target to any value above the floor, and nothing may raise it above half the ceiling

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a target at the floor is accepted, so the rate is held at the floor
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/at_the_floor_under_a_ceiling_that_fixes_no_budget`
- **Break (2):** a target at the floor or below it is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/at_the_floor_under_a_ceiling_that_fixes_no_budget`, `TestLimitsWithTarget/below_the_floor_under_a_ceiling_that_fixes_no_budget`
- **Break (3):** a target must lie above 0.0625 of the ceiling rather than above the floor, so a target close to the floor is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/lowered_close_to_the_floor`, `TestLimitsWithTarget/lowered_to_the_first_target_stored_above_the_floor`, `TestNoRunIssuesPastTheHardCap`
- **Break (4):** a target above half the ceiling is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/infinite`, `TestLimitsWithTarget/raised_above_half_the_ceiling`, `TestLimitsWithTarget/raised_to_the_hard_cap`, `TestLimitsWithTarget/raised_under_a_ceiling_that_fixes_no_budget`
- **Break (5):** the stored target is compared with the floor in full precision, so a target the four-byte rate state stores equal to the floor is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLimitsWithTarget`, `TestLimitsWithTarget/lowered_to_the_next_number_above_the_floor,_stored_equal_to_it`, `TestLimitsWithTarget/lowered_under_a_ceiling_past_the_four-byte_range`

## An expired lease stops counting against its class's share, and nothing it drew is put back into the bucket

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a lease stops counting half a second before it expires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive's_live_lease_counts_against_its_share`, `TestLive`
- **Break (2):** a lease never expires, so a crashed worker's tokens never return to its class's share
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssueSplitsByClass`, `TestIssueSplitsByClass/interactive's_expired_lease_no_longer_counts,_so_its_share_is_reserved_again`, `TestLive`
- **Break (3):** a lease that expired since the last request is put back into the bucket
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAnExpiredLeaseIsNotRefunded`, `TestNoRunIssuesPastTheHardCap`

## Cost counted for an account with no hard cap emitted fires the missing-threshold rule

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the cost's window is two minutes, so one missed scrape at a one-minute scrape empties the condition and resets the five minutes it must hold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (2):** the rule fires on its first evaluation rather than after the condition has held for five minutes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (3):** the hard cap's window is five minutes, so a hard cap that left five minutes ago still hides its absence and the account goes ten minutes with nothing firing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (4):** the rule no longer leaves out the accounts that have a hard cap, so every spending account fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (5):** the rule fires for the accounts that have a hard cap rather than those that lack one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`

## Every ask records its class's ask instant, and a waiting worker asks again within each lease period

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** a request left waiting stores nothing, so its ask is never recorded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAnAskIsRecordedAndRenewedUntilTheWorkerStops`
- **Break (2):** a waiting worker asks again every two seconds, longer than a lease period
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAnAskIsRecordedAndRenewedUntilTheWorkerStops`

## Every grant of the last second is stored and counted in the one-second window

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** the stored grants are not read into the window, so it counts none of them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`
- **Break (2):** the grants of the last second are deleted with the old ones, so the window loses them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACrashedWorkersLeaseStopsCountingAtItsExpiry`, `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`, `TestAWorkerThatWaitedOnTheLockIsStampedAfterIt`

## Interactive keeps 30% of the target and sync 20% while a cut comes out of batch, then sync, then interactive

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a cut comes out of interactive first and batch keeps its half of the target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestInteractiveKeepsItsShareThroughACut`, `TestShare`, `TestShare/a_cut_past_batch_comes_out_of_sync`, `TestShare/a_cut_past_sync_comes_out_of_interactive`, `TestShare/a_halving_comes_out_of_batch_alone`, `TestShare/a_small_cut_comes_out_of_batch`, `TestShare/at_the_floor_interactive_has_it_all`
- **Break (2):** the shares are fractions of the budget, so every cut takes from interactive in proportion
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestInteractiveKeepsItsShareThroughACut`, `TestShare`, `TestShare/a_cut_past_batch_comes_out_of_sync`, `TestShare/a_cut_past_sync_comes_out_of_interactive`, `TestShare/a_halving_comes_out_of_batch_alone`, `TestShare/a_small_cut_comes_out_of_batch`, `TestShare/at_the_floor_interactive_has_it_all`

## Issuance holds to the hard cap the declared ceiling gives, whatever the rate state stores

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** the ceiling is read back from the stored hard cap, and the limits it gives are written back, so the cap column is the cap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`
- **Break (2):** the ceiling is read back from the stored target, and the limits it gives are written back, so raising the target raises the cap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance`

## Issuance never refills the bucket past a lowered target

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break:** the bucket refills at the stored rate held at half the ceiling rather than at the account's lowered target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/a_second_refills_the_lowered_target's_worth`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestNoRunIssuesPastTheHardCap`

## Limits carries no exported field, so no code outside the package can build one and raise its target

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break:** the hard cap becomes an exported field, so code outside the package can build a Limits around its constructors
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLimitsIsBuiltOnlyByItsConstructors`

## Missing account-level series fire the absence rule rather than reading as a healthy account

- **Date · evidence:** 2026-09-24 · [pull request #157](https://github.com/ppat/mediated-mailbox-mcp/pull/157), and (2) again with its description reworded 2026-10-07 · [pull request #270](https://github.com/ppat/mediated-mailbox-mcp/pull/270), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and break 1 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** a failed read of the accounts reports no series and a successful scrape
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestTheCollectorFailsTheScrapeWhenItCannotRead`
- **Break (2):** the absence rule watches a job workload's series rather than the mediator's account-level ones
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`

## Nothing is issued during a backoff, and the bucket does not fill during one

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** the bucket fills during a backoff
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/after_a_backoff_the_bucket_fills_only_from_its_end`, `TestIssue/nothing_is_issued_during_a_backoff,_and_the_bucket_does_not_fill`
- **Break (2):** a request during a backoff is decided as if there were none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/nothing_is_issued_during_a_backoff,_and_the_bucket_does_not_fill`

## One runaway rule serves every provider, from the hard cap each spending process emits for its account

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break:** the runaway rule ignores the emitted hard cap and compares against Gmail's hard cap of 80 units a second written into the rule
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`

## The chart ships the tested alerting rules only when its switch is on

- **Date · evidence:** 2026-09-24 · [pull request #157](https://github.com/ppat/mediated-mailbox-mcp/pull/157)
- **Break (1):** the template renders the rules whatever the switch says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestTheChartShipsTheRulesOnlyWhenSwitchedOn`
- **Break (2):** the resource carries no rules rather than the tested file
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestTheChartShipsTheRulesOnlyWhenSwitchedOn`

## The collapse rules fire on five minutes at the floor or with nothing granted, and only while a class asks

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and breaks 1 and 2 again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** the floor is emitted at full precision, so a rate held at a floor a four-byte float rounds up reads as above it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAFloorRateReadsAsAtTheFloor`
- **Break (2):** an instant never set reads as just now, so an account waiting on its first grant reads as healthy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestTheCollectorReportsEachAccountsRateState`
- **Break (3):** the stall rule fires only after fifty minutes with nothing granted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (4):** the floor rule waits ten minutes at the floor before it fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (5):** the stall rule fires after one second with nothing granted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (6):** the floor rule fires whether or not a class asked in the last minute
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`

## The controller converges below a real ceiling and recovers after throttling

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326)
- **Break (1):** a success stores no additive increase, so the rate never climbs back after a cut
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestTheControllerConvergesBelowARealCeilingAndRecovers`
- **Break (2):** a throttle stores the state it read, so the rate stays at the target above the real ceiling
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestACutComesOutOfBatchWhileInteractiveKeepsItsShare`, `TestAFloorRateReadsAsAtTheFloor`, `TestTheControllerConvergesBelowARealCeilingAndRecovers`, `TestTheLimiterEmitsItsProcesssSeries`

## The controller keeps the rate between the floor and the target

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** a throttle's halving is no longer held at the floor, so the rate falls below it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestNoRunIssuesPastTheHardCap`, `TestThrottled`, `TestThrottled/held_at_the_floor`
- **Break (2):** additive increase is no longer held at the target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestNoRunIssuesPastTheHardCap`, `TestSucceeded`, `TestSucceeded/a_huge_cost_reaches_only_the_target`, `TestSucceeded/a_stored_rate_above_the_target_is_brought_down`, `TestSucceeded/at_the_target`, `TestSucceeded/held_at_the_target`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_huge_success_reaches_only_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_stored_rate_at_the_default_target_is_brought_down_to_it`, `TestTheControllerHoldsALoweredTarget/a_success_stops_at_the_lowered_target`, `TestTheRateClimbsToALoweredTargetAndNoFurther`
- **Break (3):** a stored rate outside the floor and the target is used as it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestNoBudgetHoldsTheRateAtZero`, `TestNoRunIssuesPastTheHardCap`, `TestServerErrored`, `TestServerErrored/a_stored_rate_above_the_target_falls_from_the_target`, `TestServerErrored/a_stored_rate_that_is_not_a_number_is_taken_as_the_floor`, `TestSucceeded`, `TestSucceeded/a_stored_rate_below_the_floor_is_brought_up`, `TestSucceeded/a_stored_rate_that_is_not_a_number_is_taken_as_the_floor`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_latency_cut_falls_from_it`, `TestTheControllerHoldsALoweredTarget/a_server_error_falls_from_it`, `TestTheControllerHoldsALoweredTarget/a_throttle_halves_from_it`, `TestThrottled`, `TestThrottled/a_stored_rate_above_the_target_halves_from_the_target`

## The controller never raises the rate above a lowered target

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break:** the additive increase stops at half the ceiling rather than at the account's lowered target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestNoRunIssuesPastTheHardCap`, `TestTheControllerHoldsALoweredTarget`, `TestTheControllerHoldsALoweredTarget/a_huge_success_reaches_only_the_lowered_target`, `TestTheControllerHoldsALoweredTarget/a_stored_rate_at_the_default_target_is_brought_down_to_it`, `TestTheControllerHoldsALoweredTarget/a_success_stops_at_the_lowered_target`, `TestTheRateClimbsToALoweredTargetAndNoFurther`

## The latency baseline is the lowest of the last ten medians of one-minute windows holding at least 20 samples

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** every remembered median counts, so an early low median holds the baseline down forever
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestALastingRiseBecomesTheBaseline`, `TestBaseline`, `TestBaseline/only_the_last_ten_count`, `TestRememberMedian`, `TestRememberMedian/a_longer_stored_list_keeps_its_last_nine`, `TestRememberMedian/the_eleventh_drops_the_oldest`
- **Break (2):** the baseline is the latest median rather than the lowest, so a rise becomes the baseline at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestBaseline`, `TestBaseline/only_the_last_ten_count`, `TestBaseline/the_lowest`
- **Break (3):** a window with fewer than 20 samples has a median
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLatencyWindowAdd`, `TestLatencyWindowAdd/a_window_closing_with_too_few_samples_has_no_median`, `TestMedian`, `TestMedian/19_samples_are_too_few`
- **Break (4):** a window never closes, so no median is ever taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestLatencyWindowAdd`, `TestLatencyWindowAdd/a_sample_a_minute_after_the_start_closes_the_window_with_its_median`, `TestLatencyWindowAdd/a_window_closing_with_too_few_samples_has_no_median`

## The runaway rule fires on two minutes of provider request cost above the hard cap times 120 seconds, summed over every spending process

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the runaway rule takes the busiest process for the account rather than summing them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (2):** the runaway window is one minute, which holds a single sample when the platform scrapes once a minute, so the rule never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (3):** the runaway threshold is twice the hard cap times 120 seconds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`

## The runaway rule judges each account by the highest hard cap its processes emitted over its two minutes, so a process that has gone keeps its threshold

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317)
- **Break (1):** the hard cap is read at the instant of evaluation, so a process whose series went stale takes the threshold with it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (2):** the account is judged by the lowest hard cap its processes emitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`
- **Break (3):** the account is judged by the mean of the hard caps its processes emitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/lease`:** `TestAlertingRules`

## The tokens issued inside any one-second window sum to at most the hard cap, and over a longer window stay within the hard cap plus the target times its length, while the stored state is kept whole and current, and the clock does not step forward

- **Date · evidence:** 2026-09-25 · [pull request #161](https://github.com/ppat/mediated-mailbox-mcp/pull/161)
- **Break (1):** the bucket holds two seconds' worth at the hard cap rather than one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/a_fresh_bucket_fills_to_its_capacity_and_grants`, `TestIssue/a_request_past_one_second's_worth_at_the_hard_cap_is_refused_and_changes_nothing`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestIssue/a_stored_level_above_the_capacity_is_brought_down_to_it`, `TestNoRunIssuesPastTheHardCap`
- **Break (2):** the issuer's clock follows an instant earlier than the last, so a stretch of time fills the bucket twice
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/an_instant_earlier_than_the_last_is_taken_as_the_last,_and_fills_nothing`, `TestNoRunIssuesPastTheHardCap`
- **Break (3):** the bucket refills at whatever rate the state holds, not held at the target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/a_rate_above_the_target_refills_at_the_target`, `TestIssue/an_infinite_rate_refills_at_the_target`, `TestIssueUnderALoweredTarget`, `TestIssueUnderALoweredTarget/a_second_refills_the_lowered_target's_worth`, `TestIssueUnderALoweredTarget/and_no_more_than_that`, `TestNoRunIssuesPastTheHardCap`
- **Break (4):** a stored level above the capacity is used as it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/a_stored_level_above_the_capacity_is_brought_down_to_it`, `TestNoRunIssuesPastTheHardCap`
- **Break (5):** the window counts only the grants of the last half second
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestIssue`, `TestIssue/a_grant_that_fits_the_window_is_issued_beside_the_others`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestNoRunIssuesPastTheHardCap`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`
- **Break (6):** the one-second window is not checked, so the bucket alone decides
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/a_second_and_a_half_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/an_hour_ahead`, `TestAStoredInstantAheadOfTheClockKeepsTheWindow/at_the_clock's_limit`, `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`, `TestIssue/a_stored_grant_stamped_past_the_request_is_taken_as_issued_at_the_request`, `TestIssue/a_stored_grant_that_is_not_a_number_fills_the_window`, `TestIssue/a_stored_instant_that_came_back_behind_the_grants_leaves_them_in_the_window`, `TestIssue/the_window_holds_the_grants_of_the_last_second_to_the_hard_cap_though_the_bucket_holds_more`, `TestNoRunIssuesPastTheHardCap`, `TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond`
- **Break (7):** a grant's window end is its stamp plus a second, which saturates at the clock's limit, so a grant there leaves the window at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ratelimit/core`:** `TestIssue`, `TestIssue/a_grant_at_the_clock's_limit_counts_in_the_window`
