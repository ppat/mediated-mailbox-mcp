# Mutations: ui

The demonstrations of the controls whose patches sit in `ui/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A base rule's lift asks for the rule identifier typed before its button enables

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the lift of a whole base rule asks for no identifier typed, so its button is enabled at once
  - **Went red in `test/basepolicy.test.tsx`:** `a base rule's lift names every account that loses it, needs its identifier typed, and offers Put it back`
  - **Went red in `test/policy.test.tsx`:** `a base rule's lift reads the base sentences and enables only once its identifier is typed`
- **Break (2):** Edit domains removing a base rule's suffix asks for no identifier typed, so its lift button is enabled at once
  - **Went red in `test/policy.test.tsx`:** `removing a suffix from a base rule asks for the rule's identifier typed too`
- **Break (3):** the typed confirmation is taken when it is any part of the expected value rather than the value itself
  - **Went red in `test/policy.test.tsx`:** `a base rule's lift reads the base sentences and enables only once its identifier is typed`, `removing a suffix from a base rule asks for the rule's identifier typed too`

## A client an account connects through is not removed

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the removal an account's reference refuses is reported as the database failing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientIsReplacedAndRemoved`
- **Break (2):** every removal is refused as a client in use, an unused client's included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientIsReplacedAndRemoved`

## A client replaced or removed while an attempt runs fails its finish, and nothing is stored

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a finish exchanges and stores through the client the attempt started with, whatever the client holds now
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientReplacedMidAttemptFailsTheFinish`
- **Break (2):** a finish compares the client's name with its identifier, so a client left as it was fails every finish
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientIsReplacedAndRemoved`, `TestAClientsSecretReachesNoAnswerCookieOrLog`, `TestAConnectedAccountHoldsItsSealedGrant`, `TestAConnectionFailedBetweenItsWritesStoresNeitherRow`, `TestAGrantWhoseAPIIsDisabledIsRefused`, `TestAMoveWritesTheClientAndTheCredentialTogether`, `TestAPastedAddressIsRefusedForItsCause`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`, `TestASecondConnectionUnderOneIdentifierIsRefused`, `TestAccountSettingsReadAndSetTheTarget`, `TestAnAccountWithNoStateIsConnectedByReauthorizing`, `TestAnAttemptFinishesOnAnyReplicaSharingTheKey`, `TestAnotherConfiguredRedirectIsUsedEverywhere`, `TestReauthorizingCannotChangeTheMailbox`, `TestTheMailboxIsComparedAsGoogleIdentifiesIt`, `TestTheRecordedSetupFixturesMatchTheServer`

## A client's name reaches one client, and one client is never stored twice

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a client identifier another client holds is refused as a name taken, so the screen cannot name the client holding it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientsNameReachesOneClient`
- **Break (2):** a client's name of any shape is admitted, new included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestAClientNameIsShapedLikeAProjectID`

## A client's secret in plaintext reaches no answer, cookie or log line

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** client setup answers with the secret it checked in place of the project ID
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientsSecretReachesNoAnswerCookieOrLog`, `TestTheRecordedSetupFixturesMatchTheServer`
- **Break (2):** the code exchange logs the secret it obtained with the request's line
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientsSecretReachesNoAnswerCookieOrLog`

## A completed consent records its code exchange's attempt as the account's latest

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a completed consent records its attempt as refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAConnectedAccountHoldsItsSealedGrant`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`
- **Break (2):** a consent stores the credential and records no attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAConnectedAccountHoldsItsSealedGrant`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`

## A connected account remembers the mailbox the provider confirmed

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a re-authorization answers with the mailbox the provider confirmed in place of the one the account remembers
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`
- **Break (2):** a connection stores and answers the mailbox as the operator typed it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAConnectedAccountHoldsItsSealedGrant`, `TestAnAccountWithNoStateIsConnectedByReauthorizing`

## A consent attempt opens only in the session that started it, and only its newest state finishes

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the attempt is sealed with no session, so another session's cookie opens
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAnAttemptOpensOnlyInItsSession`
- **Break (2):** a redirect carrying another attempt's state finishes this one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestARedirectFinishesOnlyItsOwnAttempt`

## A consent attempt that expired finishes nothing

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a finish reads the pasted address before it checks the attempt's expiry, so an expired attempt answers with the address's refusal
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPastedAddressIsRefusedForItsCause`
- **Break (2):** a finish never checks the attempt's expiry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPastedAddressIsRefusedForItsCause`
- **Break (3):** an attempt past its expiry finishes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestAnAttemptExpiresAtItsExpiry`
- **Break (4):** an attempt is refused as expired a moment before its expiry
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestAnAttemptExpiresAtItsExpiry`

## A delta sync gap recovery shows on Home as the sync-gap card

- **Date · evidence:** 2026-10-01 · [pull request #230](https://github.com/ppat/mediated-mailbox-mcp/pull/230)
- **Break (1):** the card is worded from the first recovery in the window rather than the latest
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheSyncGapRuleShowsTheRecovery`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention`:** `TestASyncGapIsOneCardWordedFromTheLatestRecovery`
- **Break (2):** the rule reads the gap recoveries that failed beside the ones that succeeded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheSyncGapRuleShowsTheRecovery`
- **Break (3):** the rule reads the last seven days whatever its configured days, so a recovery within a longer window is missed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheSyncGapRuleShowsTheRecovery`

## A grant for a mailbox other than the one named, or on re-authorization the one the account remembers, is refused

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a grant is stored whatever mailbox it belongs to
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestReauthorizingCannotChangeTheMailbox`, `TestTheMailboxIsComparedAsGoogleIdentifiesIt`, `TestTheRecordedSetupFixturesMatchTheServer`
- **Break (2):** a re-authorization takes the mailbox its request names over the one the account remembers
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestReauthorizingCannotChangeTheMailbox`

## A group's filter word selects the rows the group counts

- **Date · evidence:** 2026-09-30 · [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205) for (1) and (2), and 2026-09-30 · [pull request #208](https://github.com/ppat/mediated-mailbox-mcp/pull/208), which adds (3) to (7)
- **Break (1):** the word empty is compiled to the null group's word, so sender=empty selects the items with no message instead of the one stored with an empty domain
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheEmptyGroupsWordSelectsTheRowsItCounts`
- **Break (2):** the word empty reaches the statements as it is written instead of as the empty string, so sender=empty selects no row of the group it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheEmptyGroupsWordSelectsTheRowsItCounts`
- **Break (3):** the empty group is linked with the null group's word, so its bar and its row open the items with no message
  - **Went red in `test/run.test.tsx`:** `the empty group links with the word empty, and a stored value the grammar cannot name as itself links nowhere`
- **Break (4):** the empty group is given no filter word, so its bar and its row lead nowhere
  - **Went red in `test/run.test.tsx`:** `the empty group links with the word empty, and a stored value the grammar cannot name as itself links nowhere`
- **Break (5):** a value stored empty is linked with empty on every dimension, which the endpoint refuses on a dimension that does not declare it
  - **Went red in `test/url.test.ts`:** `a group's filter word is its value only where the grammar reads the value back as itself`
- **Break (6):** a stored value holding a comma or starting with ! is linked with its own spelling, which the grammar reads as any of or as an exclusion, so its bar and its row open other rows
  - **Went red in `test/run.test.tsx`:** `the empty group links with the word empty, and a stored value the grammar cannot name as itself links nowhere`
  - **Went red in `test/url.test.ts`:** `a group's filter word is its value only where the grammar reads the value back as itself`
- **Break (7):** a stored value spelled none or empty is linked with its own spelling, so its bar and its row open the null or the empty group instead
  - **Went red in `test/run.test.tsx`:** `the empty group links with the word empty, and a stored value the grammar cannot name as itself links nowhere`

## A guide window holds only its page's own text and controls and nothing about any account

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the paste box's small window copies the connect page's text, the account's identifier and mailbox included
  - **Went red in `test/setup.test.tsx`:** `the paste box's own window and its side window hold nothing about any account`
- **Break (2):** the paste box's small window holds no paste box
  - **Went red in `test/setup.test.tsx`:** `the paste box's own window and its side window hold nothing about any account`

## A lift every account loses needs the rule's identifier typed, the server's check

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** any typed text confirms a base lift, not the rule identifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestABaseLiftNeedsTheIdentifierTyped`, `TestAnImportThatLiftsNeedsItsConfirmation`
- **Break (2):** a base rule is lifted with no confirmation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestABaseLiftNeedsTheIdentifierTyped`

## A lowered target is stored only within the range ADR-0024 allows

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a target of exactly 5% is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestALoweredTargetStaysInItsRange`
- **Break (2):** a target of exactly 50% is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup`:** `TestALoweredTargetStaysInItsRange`

## A policy file reaches the stored rules only whole and only of its form

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a file's rules are not checked with a write's checks, so a bad rule reaches the preview and the stored rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAFileIsCheckedWholeAndImportsIntoAnyScope`
- **Break (2):** a key the file's form does not hold is ignored rather than refusing the file
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAFileIsCheckedWholeAndImportsIntoAnyScope`

## A policy write records only a declared identity, never an empty one

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** an identity that resolves empty with no header declared is recorded, so a write under an operator name configured empty lands with no one named
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPolicyWriteWithAnEmptyIdentityIsRefused`
- **Break (2):** a header other than the declared one is read, so a request carrying the declared header is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPolicyWriteRecordsTheDeclaredIdentity`
- **Break (3):** a write missing the declared header falls back to the operator name, so an unauthenticated write is recorded as the operator
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPolicyWriteRecordsTheDeclaredIdentity`
- **Break (4):** the declared identity header is never read, so every write, one without the header included, is recorded under the operator name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPolicyWriteRecordsTheDeclaredIdentity`

## A policy write the snapshot's validation or the UI's route words refuse is refused before it is written

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a write checks the scope's rules before the one it changes and never the changed rule itself
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAWriteTheValidationRefusesIsRefusedBeforeItIsWritten`, `TestTheRecordedPolicyFixturesMatchTheServer`
- **Break (2):** a write is never checked, so a rule the snapshot's validation refuses is written and fails every reload
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAWriteTheValidationRefusesIsRefusedBeforeItIsWritten`, `TestTheRecordedPolicyFixturesMatchTheServer`
- **Break (3):** an identifier that is one of a scope's route words is admitted, so its rule cannot be reached at its own address
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules`:** `TestCheck`, `TestCheck/an_account's_route_word`, `TestCheck/base_is_an_account's_route_word`, `TestCheck/several_problems,_in_rule_order`, `TestCheck/the_base_policy's_route_word`
- **Break (4):** each scope is held to the other scope's route words
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules`:** `TestCheck`, `TestCheck/an_account's_route_word`, `TestCheck/base_is_an_account's_route_word`, `TestCheck/base_is_no_route_word_of_the_base_policy`, `TestCheck/pick_is_no_route_word_of_the_base_policy`

## A rule changing where it applies is added under its new scope before the old one is lifted

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** Add a rule opened by a change of where a rule applies goes on to the old rule's lift when its add is refused
  - **Went red in `test/policy.test.tsx`:** `a change of where a rule applies whose add fails never sends the lift`
- **Break (2):** a change of where a rule applies lifts the old rule before it sends the add under the new scope
  - **Went red in `test/policy.test.tsx`:** `Change where this applies adds under the other scope first, then lifts the old rule with nothing lost`, `a change of where a rule applies whose add fails never sends the lift`

## A screen's text, rows, cursor, focus, range fields and loading timers follow the route, answers, range and read in view

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** closing an account's connection keeps its objects, so a connection opened later starts from the last one's events
  - **Went red in `test/stream.test.tsx`:** `surfaces following one account share one connection, which closes when the last stops`
- **Break (2):** every surface following an account opens a connection of its own, so the strip and the banner hold two
  - **Went red in `test/home.test.tsx`:** `a run event for a run the strip does not show reads the jobs endpoint again, and a poll every read`, `switching accounts closes the last account's connection and follows the new one's`, `the strip and the banner share the account's one connection, which closes with the screen`
  - **Went red in `test/stream.test.tsx`:** `surfaces following one account share one connection, which closes when the last stops`
- **Break (3):** the row cursor holds a position, so after a re-read puts a new run above it Enter opens that run instead
  - **Went red in `test/jobs.test.tsx`:** `the row cursor stays on the chosen run when a re-read puts a new run above it`
- **Break (4):** a run event whose state differs from the one Home shows reads neither the jobs nor the system endpoint again, so the cells' workload states and the chrome's running mark keep the old state
  - **Went red in `test/home.test.tsx`:** `a shown run's change of state reads the jobs and system endpoints again, and its cell follows`
- **Break (5):** live text builds its computed once and never again, so another run or an answer read again never reaches it
  - **Went red in `test/jobs.test.tsx`:** `a new tick replaces the card's run, and its next event reaches the card`, `a poll's re-read reaches the cards and the rows`, `a reconnect's re-read puts a new run first, and every row keeps its own run's cells`
- **Break (6):** a region's loading state is keyed by its attempt alone, so a region that moves to another slow read keeps the first read's timers
  - **Went red in `test/lens.test.tsx`:** `a region that moves to another slow read starts its timers again`
- **Break (7):** the range control is not keyed by the view's range, so its custom fields keep the last range's dates after a preset changes it
  - **Went red in `test/lens.test.tsx`:** `the custom range's fields start again from a new range`
- **Break (8):** a rate event reads nothing again while the answer shown has no rate state, so an account's first spend never brings up the rate budget or the strip's batch class
  - **Went red in `test/home.test.tsx`:** `an account's first rate event reads the jobs endpoint again and brings up the strip's batch class`
  - **Went red in `test/jobs.test.tsx`:** `an account's first rate event reads the jobs endpoint again and brings up the rate budget`
- **Break (9):** the router mounts a route's component without its route identity as the key, so another account or another run keeps the first screen
  - **Went red in `test/run.test.tsx`:** `following a failed item's recovering run to another run shows that run's strip and follows its events`, `following recovered by to another run shows that run's strip and follows its events`, `following the resumer to another run shows that run's strip and follows its events`, `switching account on the run screen shows the other account's run and none of the last one's`
- **Break (10):** the route table registers the jobs route's component without going through screen(), so that screen is not keyed by its route identity
  - **Went red in `test/routes.test.ts`:** `every route but the entry and not-found routes mounts its screen by its route identity`
- **Break (11):** the rows table keys its rows by position, so a new first run moves keyboard focus off the run the operator chose
  - **Went red in `test/jobs.test.tsx`:** `keyboard focus stays on the chosen run's link when a re-read puts a new run above it`
- **Break (12):** a surface that stops following stays among the account's followers, so a later event or poll still calls it and its connection never closes
  - **Went red in `test/home.test.tsx`:** `switching accounts closes the last account's connection and follows the new one's`, `the strip and the banner share the account's one connection, which closes with the screen`
  - **Went red in `test/stream.test.tsx`:** `each account has its own connection`, `surfaces following one account share one connection, which closes when the last stops`

## A state-changing request is accepted only with the request token of the session that loaded the page

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the token is the same for every session, so one page's token passes from any other session
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAStateChangingRequestWithoutItsTokenIsRefused`
- **Break (2):** a request carrying no token header passes, so a page on another origin sends one without it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAStateChangingRequestWithoutItsTokenIsRefused`
- **Break (3):** the check passes every request, whatever its token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAStateChangingRequestWithoutItsTokenIsRefused`, `TestAnAttemptFinishesOnAnyReplicaSharingTheKey`

## A streamed event redraws a live surface's text without re-rendering a component

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223) for the first nine, and [pull request #230](https://github.com/ppat/mediated-mailbox-mcp/pull/230), which reproduces them and adds the tenth to the twelfth
- **Break (1):** the sync class's bar is given the interactive class's measure, so it draws another class's used of reserved
  - **Went red in `test/jobs.test.tsx`:** `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`
- **Break (2):** the jobs screen reads the stream's update time while rendering, so every event re-renders the whole screen
  - **Went red in `test/jobs.test.tsx`:** `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`
- **Break (3):** live text reads its object's signal while rendering instead of through a computed signal, so every event re-renders it
  - **Went red in `test/jobs.test.tsx`:** `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`
  - **Went red in `test/run.test.tsx`:** `a run event redraws the strip and the indicator shows, and the screen never re-runs`
- **Break (4):** live text reads its object's value without subscribing to it, so an event never redraws the text
  - **Went red in `test/jobs.test.tsx`:** `a new tick replaces the card's run, and its next event reaches the card`, `a pass 2 that has just started shows its bar on its first page event`, `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`
  - **Went red in `test/run.test.tsx`:** `a run event redraws the strip and the indicator shows, and the screen never re-runs`, `following a failed item's recovering run to another run shows that run's strip and follows its events`, `following recovered by to another run shows that run's strip and follows its events`, `following the resumer to another run shows that run's strip and follows its events`
- **Break (5):** pass 2's bar is drawn only when the first answer's checkpoint records a page, so a pass that has just started shows no bar on its first page event
  - **Went red in `test/jobs.test.tsx`:** `a pass 2 that has just started shows its bar on its first page event`
- **Break (6):** a progress bar reads its object's signal while rendering instead of through a computed signal, so every event re-renders it
  - **Went red in `test/jobs.test.tsx`:** `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`
- **Break (7):** a progress bar reads its object's value without subscribing to it, so an event never moves its fill
  - **Went red in `test/jobs.test.tsx`:** `a pass 2 that has just started shows its bar on its first page event`, `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`
- **Break (8):** every rate event reads the jobs endpoint again, whether or not the answer shown has a rate state, so a rate event re-renders the screen
  - **Went red in `test/jobs.test.tsx`:** `an account's first rate event reads the jobs endpoint again and brings up the rate budget`, `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`
- **Break (9):** the run screen reads the summary's state while rendering, so an event's refetch re-renders the whole screen
  - **Went red in `test/run.test.tsx`:** `a run event redraws the strip and the indicator shows, and the screen never re-runs`
- **Break (10):** the home screen reads the stream's update time while rendering, so every event re-renders the whole screen
  - **Went red in `test/home.test.tsx`:** `a pass that starts while Home is open draws its checkpoint and bar on its first page event, and nothing re-runs`, `an event redraws the strip's run and the batch class, and nothing re-runs`
- **Break (11):** the running-work strip reads the rate's signal while rendering its cells, so every rate event re-renders every cell
  - **Went red in `test/home.test.tsx`:** `an event redraws the strip's run and the batch class, and nothing re-runs`
- **Break (12):** the strip draws the backfill checkpoint and its bar only when the first answer's checkpoint records a page, so a pass that starts while Home is open shows no progress on its first page event
  - **Went red in `test/home.test.tsx`:** `a pass that starts while Home is open draws its checkpoint and bar on its first page event, and nothing re-runs`

## A write row-level security empties is reported as a rule the account does not hold

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a rule the account does not hold is reported as the database failing rather than as unknown
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAReleaseCountsTheSetOfSuffixesRemoved`, `TestAnotherAccountsRuleIsUnknown`
- **Break (2):** an edit of a rule the account does not hold is reported as a stale conflict
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAnotherAccountsRuleIsUnknown`

## An account's two rows, and a move's client and credential, are written together or not at all

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a connection writes the account's listing row outside the transaction that writes its state row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAConnectionFailedBetweenItsWritesStoresNeitherRow`
- **Break (2):** a move writes the account's new client outside the transaction that writes its credential
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAMoveWritesTheClientAndTheCredentialTogether`

## An identifier checked free when an attempt started is checked again when it finishes

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a finish refuses an identifier no account took meanwhile as taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientIsReplacedAndRemoved`, `TestAClientReplacedMidAttemptFailsTheFinish`, `TestAClientsSecretReachesNoAnswerCookieOrLog`, `TestAConnectedAccountHoldsItsSealedGrant`, `TestAConnectionFailedBetweenItsWritesStoresNeitherRow`, `TestAGrantWhoseAPIIsDisabledIsRefused`, `TestAMoveWritesTheClientAndTheCredentialTogether`, `TestAPastedAddressIsRefusedForItsCause`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`, `TestASecondConnectionUnderOneIdentifierIsRefused`, `TestAccountSettingsReadAndSetTheTarget`, `TestAnAttemptFinishesOnAnyReplicaSharingTheKey`, `TestAnotherConfiguredRedirectIsUsedEverywhere`, `TestReauthorizingCannotChangeTheMailbox`, `TestTheMailboxIsComparedAsGoogleIdentifiesIt`, `TestTheRecordedSetupFixturesMatchTheServer`
- **Break (2):** a finish writes over an account connected under the identifier meanwhile
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientIsReplacedAndRemoved`, `TestAClientsSecretReachesNoAnswerCookieOrLog`, `TestAConnectedAccountHoldsItsSealedGrant`, `TestAMoveWritesTheClientAndTheCredentialTogether`, `TestAPastedAddressIsRefusedForItsCause`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`, `TestASecondConnectionUnderOneIdentifierIsRefused`, `TestAccountSettingsReadAndSetTheTarget`, `TestAnAccountWithNoStateIsConnectedByReauthorizing`, `TestAnAttemptFinishesOnAnyReplicaSharingTheKey`, `TestAnotherConfiguredRedirectIsUsedEverywhere`, `TestReauthorizingCannotChangeTheMailbox`, `TestTheMailboxIsComparedAsGoogleIdentifiesIt`, `TestTheRecordedSetupFixturesMatchTheServer`

## An identifier is unique within its scope

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** an account's scope is read as the base rules rather than its own, so its own rules are unknown to it and the base policy's identifiers taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAFileIsCheckedWholeAndImportsIntoAnyScope`, `TestAPolicyWriteLeavesItsHistoryRowOrNothing`, `TestAReleaseCountsTheSetOfSuffixesRemoved`, `TestAWriteTheValidationRefusesIsRefusedBeforeItIsWritten`, `TestAnIdentifierComesBackInItsScopeAndBesideTheOther`, `TestAnImportIsAppliedWholeOrNotAtAll`, `TestAnImportThatLiftsNeedsItsConfirmation`, `TestAnotherAccountsRuleIsUnknown`, `TestTheRecordedPolicyFixturesMatchTheServer`
- **Break (2):** an account's scope is read as holding the base rules too, so an identifier the base policy holds is refused as taken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAFileIsCheckedWholeAndImportsIntoAnyScope`, `TestAnIdentifierComesBackInItsScopeAndBesideTheOther`, `TestAnImportIsAppliedWholeOrNotAtAll`, `TestAnImportThatLiftsNeedsItsConfirmation`, `TestTheRecordedPolicyFixturesMatchTheServer`

## An import is applied whole or not at all, against the stored rules its preview was computed against

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** an import that fails after its first change commits what it wrote
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAnImportIsAppliedWholeOrNotAtAll`
- **Break (2):** the preview carries the stored rules' identifiers and classes without their suffixes, so an edit of a rule's suffixes since the preview goes unseen
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAnImportIsAppliedWholeOrNotAtAll`, `TestTheRecordedPolicyFixturesMatchTheServer`
- **Break (3):** an import applies over rules another write changed since its preview
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAnImportIsAppliedWholeOrNotAtAll`, `TestTheRecordedPolicyFixturesMatchTheServer`

## An import that lifts anything needs lift {k} for the count it lifts, the server's check

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the confirmation's count is every change the import makes rather than the restrictions it lifts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAFileIsCheckedWholeAndImportsIntoAnyScope`, `TestAnImportThatLiftsNeedsItsConfirmation`, `TestTheRecordedPolicyFixturesMatchTheServer`
- **Break (2):** an import lifting restrictions applies with no confirmation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAnImportThatLiftsNeedsItsConfirmation`

## An import's lifts are confirmed in one dialog, typed as lift {k} for the base policy, and the request carries lift {k}

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a base import's dialog asks for no lift {k} typed, so its button is enabled at once
  - **Went red in `test/basepolicy.test.tsx`:** `a base import lifting a rule enables only once lift {k} is typed, and sends it`
- **Break (2):** an import that lifts restrictions is applied on its button, with no dialog naming the lifts
  - **Went red in `test/policy.test.tsx`:** `an import against a policy changed since its preview is refused with the words to preview again`, `an import that lifts asks one dialog naming every lift, and Put back adds them back`, `markup in a file's identifiers and suffixes arrives as text in the preview and the import dialog`
- **Break (3):** an import that lifts is sent with no confirmation, so the request no longer carries lift {k}
  - **Went red in `test/basepolicy.test.tsx`:** `a base import lifting a rule enables only once lift {k} is typed, and sends it`
  - **Went red in `test/policy.test.tsx`:** `an import that lifts asks one dialog naming every lift, and Put back adds them back`

## Body serves far above their daily median raise Home's body-serve card

- **Date · evidence:** 2026-10-01 · [pull request #230](https://github.com/ppat/mediated-mailbox-mcp/pull/230)
- **Break (1):** the median is taken over the baseline days that had a serve alone, so a quiet day no longer counts as 0
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestADayWithoutAServeCountsZero`, `TestCardsComeNewestFirst`, `TestTheRecordedFixturesMatchTheServer`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention`:** `TestBodyServesFireAboveTheFactorOfTheMedian`, `TestBodyServesFireAboveTheFactorOfTheMedian/four_quiet_days_hold_the_median_at_0`, `TestTheMedianCountsADayWithoutAServeAsZero`
- **Break (2):** the rule fires at exactly the factor times the median, so a count equal to its threshold raises the card
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheBodyServeRuleFiresFarAboveItsMedian`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention`:** `TestBodyServesFireAboveTheFactorOfTheMedian`, `TestBodyServesFireAboveTheFactorOfTheMedian/at_twice_the_median`
- **Break (3):** the count of the last 24 hours takes body denials as serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestADayWithoutAServeCountsZero`, `TestTheBodyServeRuleFiresFarAboveItsMedian`, `TestTheRecordedFixturesMatchTheServer`
- **Break (4):** the baseline is the seven whole UTC days before today rather than before the 24 hours start, so it takes in the day the 24 hours overlap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheBodyServeRuleFiresFarAboveItsMedian`
- **Break (5):** the baseline is the 7 times 24 hours before the 24 hours start rather than the seven whole UTC days before them, a rolling week that leaves out the early hours of its first day and takes in those of the day the 24 hours start in
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheBodyServeRuleFiresFarAboveItsMedian`

## Every account-scoped read refuses a request with no account, with the value meaning every account, or with an unknown account

- **Date · evidence:** 2026-10-01 · [pull request #230](https://github.com/ppat/mediated-mailbox-mcp/pull/230), which reproduces the breaks of [pull request #208](https://github.com/ppat/mediated-mailbox-mcp/pull/208) and adds the seventh
- **Break (1):** the account check is inverted, so a listed account is refused and an unlisted one is read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestADatabaseFailureReportsTheDatabaseOrigin`, `TestADayWithoutAServeCountsZero`, `TestCardsComeNewestFirst`, `TestEveryReadIsPerAccount`, `TestTheBacklogRuleFiresAboveItsShare`, `TestTheBodyServeRuleFiresFarAboveItsMedian`, `TestTheEmptyGroupsWordSelectsTheRowsItCounts`, `TestTheFailuresDatasetAnswersItsLevels`, `TestTheLensAnswersItsLevels`, `TestTheMaskingRuleCountsEachSenderAndRule`, `TestTheMetricsCarryReadsAndSubscribers`, `TestTheRecordedFixturesMatchTheServer`, `TestTheRegistryRefusesWhatItDoesNotDeclare`, `TestTheRowDetailAnswersItsRow`, `TestTheRunSummaryAnswersItsRun`, `TestTheRunsDatasetAnswersItsLevels`, `TestTheStreamSendsEachChangedObject`, `TestTheSyncGapRuleShowsTheRecovery`
- **Break (2):** the value meaning every account is not refused for what it is, so it reads as an unknown account and an account named all would be read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryReadIsPerAccount`
- **Break (3):** the row detail checks its account only for the value meaning every account, so an unknown account's row is read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryReadIsPerAccount`
- **Break (4):** the row detail reads its row without checking its account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryReadIsPerAccount`
- **Break (5):** the run summary route is mounted without the account check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryReadIsPerAccount`, `TestTheRecordedFixturesMatchTheServer`
- **Break (6):** the account is not checked against the accounts table, so an unknown account is read as if it existed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryReadIsPerAccount`, `TestTheRecordedFixturesMatchTheServer`
- **Break (7):** the attention route is declared without its account, so no account check admits its requests
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryReadIsPerAccount`

## Every account-scoped route refuses all and an unknown account

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the export route admits any account, all and an unknown one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyRoutesAreScoped`
- **Break (2):** the add panel's match route admits any account, all and an unknown one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyRoutesAreScoped`

## Every policy write leaves its history row or nothing

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** an edit of a rule writes no history row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPolicyWriteLeavesItsHistoryRowOrNothing`, `TestTheRecordedPolicyFixturesMatchTheServer`
- **Break (2):** a lift is recorded as an edit, so the history no longer says a restriction was lifted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAPolicyWriteLeavesItsHistoryRowOrNothing`, `TestAnIdentifierComesBackInItsScopeAndBesideTheOther`

## Every response a handler of the UI answers carries the content security policy, exact

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the policy allows connections to any origin, so a leaked subject could reach one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryResponseCarriesThePolicy`, `TestTheStreamSendsEachChangedObject`
- **Break (2):** the policy is set on the entry document and the bundle only, so the read API's answers and errors leave without it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryResponseCarriesThePolicy`, `TestTheStreamSendsEachChangedObject`
- **Break (3):** the probes and the metrics endpoint answer without the policy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestEveryResponseCarriesThePolicy`

## Every route registered on the probes listener is one of its three probe routes

- **Date · evidence:** 2026-09-30 · [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205)
- **Break (1):** a route other than the three probes is registered on the probes listener's recording mux after New stores it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestTheProbesListenerServesOnlyTheProbes`
- **Break (2):** a route other than the three probes is registered on the probes listener's recording mux
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestTheProbesListenerServesOnlyTheProbes`

## Every route registered through the recording mux's Handle is compared with the contract document

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a bespoke handler the document describes is left unmounted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestTheServerServesExactlyTheDocumentsRoutes`
- **Break (2):** the recording mux embeds its ServeMux again, so its promoted Handle registers a route unrecorded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheRecordingMuxEmbedsNothing`
- **Break (3):** a route the contract document lacks is registered through the recording mux's Handle after New stores the mux
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestTheServerServesExactlyTheDocumentsRoutes`
- **Break (4):** a route outside /api that the contract document lacks is registered through the recording mux's Handle
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestTheServerServesExactlyTheDocumentsRoutes`
- **Break (5):** a route the contract document lacks is registered through the recording mux's Handle
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestTheServerServesExactlyTheDocumentsRoutes`

## Every stream event is its object's whole current state

- **Date · evidence:** 2026-09-30 · [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205)
- **Break (1):** a run event leaves out the plan an apply run carries
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheStreamSendsEachChangedObject`
- **Break (2):** a run event carries the run's state as first read and never the changed checkpoint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheStreamSendsEachChangedObject`

## `go vet` refuses a copy of a recording mux

- **Date · evidence:** 2026-09-29 · [pull request #193](https://github.com/ppat/mediated-mailbox-mcp/pull/193), repeated with its patch regenerated in [pull request #207](https://github.com/ppat/mediated-mailbox-mcp/pull/207), and repeated in [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205)
- **Break:** the recording mux drops its noCopy field, so a copy of it passes go vet and records its routes unread
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheRecordingMuxRefusesACopy`

## Message-derived text renders inert

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a worth-a-look card hands its sentence, which carries a masked sender's domain, to the raw-markup escape hatch
  - **Went red in `test/home.test.tsx`:** `message-derived and attacker-written text arrives inert on every surface of the screen`
- **Break (2):** the bars hand each group's label to the raw-markup escape hatch, and the group table keeps it as text
  - **Went red in `test/run.test.tsx`:** `a group's value carrying markup arrives as text in its bar and its row`
- **Break (3):** the decision inbox hands a candidate's domain to the raw-markup escape hatch
  - **Went red in `test/home.test.tsx`:** `message-derived and attacker-written text arrives inert on every surface of the screen`, `the inbox lists the pending candidates by score with their strongest signals, unlinked`
- **Break (4):** the rows table writes a cell's value into the element through the DOM's innerHTML after rendering, so the value is parsed as markup outside the framework
  - **Went red in `test/jobs.test.tsx`:** `a poll's re-read reaches the cards and the rows`, `a reconnect's re-read puts a new run first, and every row keeps its own run's cells`, `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`, `keyboard focus stays on the chosen run's link when a re-read puts a new run above it`, `the plans' titles carrying markup arrive as text in their cards and their row`, `the recent runs open without delta-sync ticks, each linking to its run`, `the row cursor stays on the chosen run when a re-read puts a new run above it`, `the runs at level 1 are bars and a table, and a click on a group descends to level 2`, `times judged against now read the live clock, not the time the page was read`
  - **Went red in `test/lens.test.tsx`:** `a description carrying markup arrives as text, and nothing is built from it`, `level 3 is one page of rows under the strip`
  - **Went red in `test/run.test.tsx`:** `a group's value carrying markup arrives as text in its bar and its row`, `a subject carrying markup arrives as text in its message row, and nothing is built from it`, `following a failed item's recovering run to another run shows that run's strip and follows its events`, `markup in every field of every message row and page row arrives as text`, `the empty group links with the word empty, and a stored value the grammar cannot name as itself links nowhere`, `the items render message rows with the failure's columns, and a page item as a page row`
- **Break (5):** the rows table hands a cell's value to the raw-markup escape hatch instead of rendering it as text
  - **Went red in `test/jobs.test.tsx`:** `a poll's re-read reaches the cards and the rows`, `a reconnect's re-read puts a new run first, and every row keeps its own run's cells`, `an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs`, `keyboard focus stays on the chosen run's link when a re-read puts a new run above it`, `the plans' titles carrying markup arrive as text in their cards and their row`, `the recent runs open without delta-sync ticks, each linking to its run`, `the row cursor stays on the chosen run when a re-read puts a new run above it`, `the runs at level 1 are bars and a table, and a click on a group descends to level 2`, `times judged against now read the live clock, not the time the page was read`
  - **Went red in `test/lens.test.tsx`:** `a description carrying markup arrives as text, and nothing is built from it`, `level 3 is one page of rows under the strip`
  - **Went red in `test/run.test.tsx`:** `a group's value carrying markup arrives as text in its bar and its row`, `a subject carrying markup arrives as text in its message row, and nothing is built from it`, `following a failed item's recovering run to another run shows that run's strip and follows its events`, `markup in every field of every message row and page row arrives as text`, `the empty group links with the word empty, and a stored value the grammar cannot name as itself links nowhere`, `the items render message rows with the failure's columns, and a page item as a page row`
- **Break (6):** the policy change row hands the identifier of a rule it links to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `markup in a change's identifier, actor and suffixes arrives as text in its history row`, `markup in a rule's detail arrives as text in its panel and its lift dialog`
- **Break (7):** the failure's panel hands the rule that set the message's class to the raw-markup escape hatch
  - **Went red in `test/run.test.tsx`:** `markup in every message-derived field of a failure's panel arrives as text`
- **Break (8):** the lift dialog hands its title, which names the rule or suffix, to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `markup in a rule's detail arrives as text in its panel and its lift dialog`
- **Break (9):** the failure's panel hands the recorded error summary to the raw-markup escape hatch
  - **Went red in `test/run.test.tsx`:** `a failure opens as a panel with what happened, what it means, its provenance and audit rows`, `markup in every message-derived field of a failure's panel arrives as text`
- **Break (10):** the reorg apply card hands the last run's plan title to the raw-markup escape hatch
  - **Went red in `test/jobs.test.tsx`:** `the cards show each workload's state and fields, with the decisions block's counts`, `the plans' titles carrying markup arrive as text in their cards and their row`
- **Break (11):** the run timeline writes a mark's hover into its title element through the DOM's innerHTML
  - **Went red in `test/run.test.tsx`:** `a failure opens as a panel with what happened, what it means, its provenance and audit rows`, `a failure's panel shows its message's rule ids, scan time and scanner version`, `a subject carrying markup arrives as text in its message row, and nothing is built from it`, `an event's detail carrying markup arrives in its mark's hover as text`
- **Break (12):** a rule's panel hands each matched sender's domain to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `markup in a rule's detail arrives as text in its panel and its lift dialog`
- **Break (13):** the rows table keeps every column but its second as text and hands the second column's value to the raw-markup escape hatch, so inert rendering covers less of the table
  - **Went red in `test/jobs.test.tsx`:** `the plans' titles carrying markup arrive as text in their cards and their row`, `the recent runs open without delta-sync ticks, each linking to its run`
  - **Went red in `test/lens.test.tsx`:** `a description carrying markup arrives as text, and nothing is built from it`, `level 3 is one page of rows under the strip`
  - **Went red in `test/run.test.tsx`:** `a subject carrying markup arrives as text in its message row, and nothing is built from it`, `markup in every field of every message row and page row arrives as text`, `the items render message rows with the failure's columns, and a page item as a page row`
- **Break (14):** the system screen keeps every row's value but its second as text and hands the second row's value to the raw-markup escape hatch, so inert rendering covers less of the screen
  - **Went red in `test/system.test.tsx`:** `an authentication outcome carrying markup arrives as text, and nothing is built from it`
- **Break (15):** account settings hand the credential's sentence to the raw-markup escape hatch instead of rendering it as text
  - **Went red in `test/setup.test.tsx`:** `account settings show the client, the credential's health, the target and the rows, never the credential`
- **Break (16):** the page row writes its item into the spanning cell's link through the DOM's innerHTML
  - **Went red in `test/run.test.tsx`:** `markup in every field of every message row and page row arrives as text`, `the items render message rows with the failure's columns, and a page item as a page row`
- **Break (17):** the failure's panel hands the message's subject to the raw-markup escape hatch
  - **Went red in `test/run.test.tsx`:** `a failure's panel shows its message's rule ids, scan time and scanner version`, `markup in every message-derived field of a failure's panel arrives as text`
- **Break (18):** the reorg apply card hands the applying plan's title to the raw-markup escape hatch
  - **Went red in `test/jobs.test.tsx`:** `the cards show each workload's state and fields, with the decisions block's counts`, `the plans' titles carrying markup arrive as text in their cards and their row`
- **Break (19):** the import preview hands a lifted rule's identifier to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `markup in a file's identifiers and suffixes arrives as text in the preview and the import dialog`
- **Break (20):** the sender row's Restrict {domain}… control hands its words to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `a sender domain carrying markup arrives as text in the picker's row, its box's label and its Restrict control`, `markup in a sender's domain arrives as text in the sender row`
- **Break (21):** the rule row hands the rule's identifier to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `markup in a rule's identifier and suffixes arrives as text in its row, and nothing is built from it`
- **Break (22):** the sender row hands its domain to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `a sender domain carrying markup arrives as text in the picker's row, its box's label and its Restrict control`, `markup in a sender's domain arrives as text in the sender row`
- **Break (23):** a candidate's strongest signal, worded from its evidence, is handed to the raw-markup escape hatch
  - **Went red in `test/home.test.tsx`:** `message-derived and attacker-written text arrives inert on every surface of the screen`, `the inbox lists the pending candidates by score with their strongest signals, unlinked`
- **Break (24):** the running-work strip's reorg apply cell hands the applying plan's title to the raw-markup escape hatch
  - **Went red in `test/home.test.tsx`:** `message-derived and attacker-written text arrives inert on every surface of the screen`, `the strip shows each workload's cell, with the pass running and the batch class`
- **Break (25):** the message row hands its subject to the raw-markup escape hatch inside the link to the row's detail, and keeps its other fields as text
  - **Went red in `test/run.test.tsx`:** `a failure opens as a panel with what happened, what it means, its provenance and audit rows`, `a failure's panel shows its message's rule ids, scan time and scanner version`, `a subject carrying markup arrives as text in its message row, and nothing is built from it`, `an event's detail carrying markup arrives in its mark's hover as text`, `markup in every field of every message row and page row arrives as text`
- **Break (26):** a rule's suffix chips hand each suffix to the raw-markup escape hatch
  - **Went red in `test/policy.test.tsx`:** `markup in a rule's identifier and suffixes arrives as text in its row, and nothing is built from it`
- **Break (27):** a signal whose identifier no template knows hands that identifier to the raw-markup escape hatch
  - **Went red in `test/home.test.tsx`:** `message-derived and attacker-written text arrives inert on every surface of the screen`, `the inbox lists the pending candidates by score with their strongest signals, unlinked`
- **Break (28):** the system screen writes each row's value into its element through the DOM's innerHTML after rendering, so the value is parsed as markup outside the framework
  - **Went red in `test/system.test.tsx`:** `an authentication outcome carrying markup arrives as text, and nothing is built from it`, `the screen shows each operational value, and the rows UI.md sends to Jobs link there`
- **Break (29):** the system screen hands each row's value to the raw-markup escape hatch instead of rendering it as text
  - **Went red in `test/system.test.tsx`:** `an authentication outcome carrying markup arrives as text, and nothing is built from it`, `the screen shows each operational value, and the rows UI.md sends to Jobs link there`

## No route is claimed by both the registry and a bespoke handler, or by two bespoke handlers

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a bespoke handler is checked against the row-detail paths alone, so one claiming the dataset endpoint's path is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestAPathClaimedByBothSourcesIsRefused`
- **Break (2):** a bespoke handler is checked against the accounts listing's path rather than the registry's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestAPathClaimedByBothSourcesIsRefused`, `TestDocument`, `TestTheProbesListenerServesOnlyTheProbes`, `TestTheServerServesExactlyTheDocumentsRoutes`
- **Break (3):** two bespoke handlers claiming one path are admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestAPathClaimedByBothSourcesIsRefused`
- **Break (4):** a bespoke handler claiming the registry's path is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestAPathClaimedByBothSourcesIsRefused`
- **Break (5):** a bespoke handler is checked against the dataset endpoint's path alone, so one claiming a row-detail path is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/contract`:** `TestAPathClaimedByBothSourcesIsRefused`

## The base policy's installation screens read no account's state

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the installation's count of base rules is read in the first account's transaction, counting that account's own rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheBasePolicyReadsNoAccountsState`, `TestTheRecordedPolicyFixturesMatchTheServer`
- **Break (2):** the base policy is read in the first account's transaction, so that account's own rules reach the installation screen
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheBasePolicyReadsNoAccountsState`, `TestTheRecordedPolicyFixturesMatchTheServer`

## The dataset endpoint refuses a dataset, dimension, filter, filter value, group, sort column or row-detail parameter the registry does not declare

- **Date · evidence:** 2026-09-30 · [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205)
- **Break (1):** empty is admitted on every dimension, so a dimension that holds no value stored empty takes it as a value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRefusesWhatTheEntryDoesNotDeclare`, `TestParseRefusesWhatTheEntryDoesNotDeclare/empty_excluded_on_a_text_dimension_that_holds_no_value_stored_empty`, `TestParseRefusesWhatTheEntryDoesNotDeclare/empty_on_a_number_dimension`, `TestParseRefusesWhatTheEntryDoesNotDeclare/empty_on_a_text_dimension_that_holds_no_value_stored_empty`
- **Break (2):** a filter value is admitted whatever the dimension's stored type, so a word reaches a number's or a date's statement
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRefusesWhatTheEntryDoesNotDeclare`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_date_any-of_with_one_bad_member`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_date_filter_holding_a_time`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_date_filter_past_the_month's_end`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_number_exclusion_holding_a_word`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_number_filter_holding_a_sign`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_number_filter_holding_a_word`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_number_filter_past_a_stored_integer`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_number_filter_with_a_leading_zero`
- **Break (3):** a group is admitted when the entry declares the dimension at all, groupable or not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRefusesWhatTheEntryDoesNotDeclare`, `TestParseRefusesWhatTheEntryDoesNotDeclare/an_undeclared_group`, `TestParseRefusesWhatTheEntryDoesNotDeclare/an_undeclared_group_at_level_3`
- **Break (4):** the row detail of a nested dataset is admitted without its parent filter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRowRefusesWhatTheEntryDoesNotDeclare`, `TestParseRowRefusesWhatTheEntryDoesNotDeclare/no_parent`
- **Break (5):** none is admitted on every dimension, so a dimension with no null group takes it as a value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRefusesWhatTheEntryDoesNotDeclare`, `TestParseRefusesWhatTheEntryDoesNotDeclare/none_excluded_on_a_text_dimension_with_no_null_group`, `TestParseRefusesWhatTheEntryDoesNotDeclare/none_on_a_dimension_with_no_null_group`, `TestParseRefusesWhatTheEntryDoesNotDeclare/none_on_a_text_dimension_with_no_null_group`
- **Break (6):** the row detail reads the parent filter's value as the row and leaves the parent empty
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRowAdmitsTheRowAndItsParent`
- **Break (7):** the row detail's row is checked only for being present, so a row of the wrong kind reaches the statement
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRowRefusesWhatTheEntryDoesNotDeclare`, `TestParseRowRefusesWhatTheEntryDoesNotDeclare/a_row_past_a_big_serial`, `TestParseRowRefusesWhatTheEntryDoesNotDeclare/a_row_that_is_not_a_number`, `TestParseRowRefusesWhatTheEntryDoesNotDeclare/a_row_with_a_leading_zero`, `TestParseRowRefusesWhatTheEntryDoesNotDeclare/a_row_with_a_sign`
- **Break (8):** the row detail admits any parameter beside the parent filter and ignores it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRowRefusesWhatTheEntryDoesNotDeclare`, `TestParseRowRefusesWhatTheEntryDoesNotDeclare/a_filter_beside_the_parent`, `TestParseRowRefusesWhatTheEntryDoesNotDeclare/a_level_beside_the_parent`
- **Break (9):** a sort column is checked against the filterable dimensions instead of the sortable ones
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseAdmitsWhatTheEntryDeclares`, `TestParseAdmitsWhatTheEntryDeclares/level_2_with_a_group_and_a_filter,_a_sort_and_a_page`
- **Break (10):** a filter is admitted without checking the entry declares it filterable, so any parameter reaches the statements as a filter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRefusesWhatTheEntryDoesNotDeclare`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_declared_dimension_that_is_not_filterable`, `TestParseRefusesWhatTheEntryDoesNotDeclare/a_sortable_column_used_as_a_filter`, `TestParseRefusesWhatTheEntryDoesNotDeclare/an_undeclared_filter`
- **Break (11):** an undeclared dataset falls back to the registry's first entry instead of being refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens`:** `TestParseRefusesWhatTheEntryDoesNotDeclare`, `TestParseRefusesWhatTheEntryDoesNotDeclare/an_undeclared_dataset`

## The dataset endpoint refuses a request the registry does not declare before any statement runs

- **Date · evidence:** 2026-09-30 · [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205)
- **Break (1):** the account check, which runs a statement, comes before the lens request is read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheRegistryRefusesWhatItDoesNotDeclare`
- **Break (2):** the row detail's account check, which runs a statement, comes before its request is read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheRegistryRefusesWhatItDoesNotDeclare`

## The entry template and the built bundle hold no inline script and reference no other origin

- **Date · evidence:** 2026-09-30 · [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205)
- **Break (1):** a font inlined as a data: URI inside an @font-face rule is never reported, so a stylesheet whose fonts the policy blocks passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyScanReportsEachPlantedViolation`
- **Break (2):** the scan passes an inline event handler attribute
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyScanReportsEachPlantedViolation`
- **Break (3):** the scan passes a script element without a src attribute
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyScanReportsEachPlantedViolation`
- **Break (4):** the scan reads only URLs with a scheme, so a scheme-relative reference to another origin passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyScanReportsEachPlantedViolation`
- **Break (5):** the scan passes a javascript: URL
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyScanReportsEachPlantedViolation`
- **Break (6):** a stylesheet resource is taken as the UI's own unless it is scheme-relative, so one with a scheme passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestThePolicyScanReportsEachPlantedViolation`

## The installation screens show no account's state

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the installation endpoint lists no account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheInstallationShowsNoAccountsState`, `TestTheRecordedSetupFixturesMatchTheServer`
- **Break (2):** the installation endpoint reads each account's mailbox and shows it as the account's client
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestTheInstallationShowsNoAccountsState`, `TestTheRecordedSetupFixturesMatchTheServer`

## The operator's re-authorization replaces the stored credential whatever a deployable wrote

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break:** a re-authorization writes only where the account holds no credential, so a credential a deployable rewrote is never replaced. The mechanism has one direction only. It is a write with no condition but its account, so the one break is a condition added. A write landing on another account's row is the opposite, and `account_state_account`'s row-level security refuses it under the transaction set to the account, a control of its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAMoveWritesTheClientAndTheCredentialTogether`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`, `TestAnAccountWithNoStateIsConnectedByReauthorizing`

## The statement-set check refuses a dataset without the statements its declarations require, and a dimension named as a parameter

- **Date · evidence:** 2026-09-30 · [pull request #205](https://github.com/ppat/mediated-mailbox-mcp/pull/205)
- **Break (1):** an aggregate statement is accepted for any declared dimension, groupable or not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheStatementSetCheckReportsEachMismatch`
- **Break (2):** a dimension named as a common parameter is not reported, only one named as the parent filter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheStatementSetCheckReportsEachMismatch`
- **Break (3):** a groupable dimension with no aggregate statement is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheStatementSetCheckReportsEachMismatch`
- **Break (4):** a dataset declaring a row identity with no provenance query is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheStatementSetCheckReportsEachMismatch`
- **Break (5):** a dataset with no rows statement is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheStatementSetCheckReportsEachMismatch`
- **Break (6):** a dimension named as the parent filter is not reported, only one named as a common parameter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheStatementSetCheckReportsEachMismatch`
- **Break (7):** every dataset without a provenance query is reported, whether or not it declares a row identity
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheRegistryHasItsStatements`, `TestTheStatementSetCheckReportsEachMismatch`
- **Break (8):** a dataset declaring a provenance query with no row identity is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/registry`:** `TestTheStatementSetCheckReportsEachMismatch`

## The UI refuses plain HTTP unless its binary is built with the devloop build tag

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272)
- **Break (1):** the tag's sense is inverted, so a binary built without it admits plain HTTP and one built with it refuses
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving`:** `TestPlainHTTPIsRefusedOutsideTheDevLoop`
- **Break (2):** plain HTTP is admitted in every binary
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving`:** `TestPlainHTTPIsRefusedOutsideTheDevLoop`
- **Break (3):** a binary built without the tag reports itself built with it, so every image admits plain HTTP
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestPlainHTTPRefusesTheStart`

## The UI refuses to start while PGPASSWORD or PGSSLPASSWORD is set

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272)
- **Break (1):** the UI starts whatever PGPASSWORD and PGSSLPASSWORD hold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestAPasswordVariableRefusesTheStart`
- **Break (2):** the refusal runs after the configuration is read, so a start whose configuration fails never reports the password variable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestAPasswordVariableRefusesTheStart`

## The UI's configuration type is pinned field by field

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272)
- **Break (1):** a value's configuration path is renamed, so the file, the environment and the flags name another key than the one declared
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestPlainHTTPRefusesTheStart`, `TestTheEffectiveConfigurationIsLogged`
- **Break (2):** a value is added to the UI's root configuration type without updating the pinned field list
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheConfigurationTypeIsPinned`, `TestTheEffectiveConfigurationIsLogged`

## The UI's one opening part opens a client's secret and never an account's credential

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the opener opens a client's secret bound to whichever row it was sealed for
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret`:** `TestTheOpenerOpensAClientsSecretAndNothingElse`
- **Break (2):** the opener opens a client's row bound to an account credential's purpose
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret`:** `TestTheOpenerOpensAClientsSecretAndNothingElse`

## What the UI stores opens only with the private key

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** a client's secret is stored as the operator brought it back
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAClientIsCheckedBeforeItIsStored`, `TestAClientsSecretReachesNoAnswerCookieOrLog`, `TestAConnectedAccountHoldsItsSealedGrant`, `TestAConnectionFailedBetweenItsWritesStoresNeitherRow`, `TestAGrantWhoseAPIIsDisabledIsRefused`, `TestAMoveWritesTheClientAndTheCredentialTogether`, `TestAPastedAddressIsRefusedForItsCause`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`, `TestASecondConnectionUnderOneIdentifierIsRefused`, `TestAccountSettingsReadAndSetTheTarget`, `TestAnAccountWithNoStateIsConnectedByReauthorizing`, `TestAnAttemptFinishesOnAnyReplicaSharingTheKey`, `TestAnotherConfiguredRedirectIsUsedEverywhere`, `TestReauthorizingCannotChangeTheMailbox`, `TestTheMailboxIsComparedAsGoogleIdentifiesIt`, `TestTheRecordedSetupFixturesMatchTheServer`
- **Break (2):** an account's credential is stored as the consent returned it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/internal/api`:** `TestAConnectedAccountHoldsItsSealedGrant`, `TestAReauthorizationRecordsItsAttemptAndAlwaysLands`
