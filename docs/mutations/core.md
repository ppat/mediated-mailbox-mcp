# Mutations: core

The demonstrations of the controls whose patches sit in `core/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A Config that cannot decide decides nothing

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** a Config that cannot decide leaves a restricted sender undecided rather than skipped as restricted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/a_restricted_sender_under_a_zero_Config`
- **Break (2):** every Config decides, a zero one included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/a_high-volume_mark_below_the_low-volume_mark`, `TestAConfigThatCannotDecide/a_low-volume_mark_of_zero`, `TestAConfigThatCannotDecide/a_size_mark_of_zero`, `TestAConfigThatCannotDecide/a_zero_Config`, `TestAConfigThatCannotDecide/an_age_mark_of_zero`

## A message body cannot carry a denying sensitivity

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** the Body gains an exported field that Text returns, so a Body literal can carry any text while the old field stays unexported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestNoSensitivityCarryingTypeExposesAField`
- **Break (2):** the Body's text field is exported, so a Body literal can hold any text
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestConstructionOutsideTheConstructorsDoesNotCompile`, `TestConstructionOutsideTheConstructorsDoesNotCompile/bodyliteral`, `TestNoSensitivityCarryingTypeExposesAField`
- **Break (3):** NewBody refuses every sensitivity, the releasing ones included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestNewBody`, `TestNewBody/scanned`, `TestNewBody/skipped_by_the_gate`, `TestNoBodyCarriesADenyingSensitivity`
- **Break (4):** NewBody never refuses, so a body is built for any sensitivity
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestNewBody`, `TestNewBody/login_link`, `TestNewBody/never_constructed`, `TestNewBody/one-time_code`, `TestNewBody/pending_scan`, `TestNewBody/restricted_sender`, `TestNewBody/skipped_as_restricted`, `TestNoBodyCarriesADenyingSensitivity`
- **Break (5):** the Sensitivity's class field is exported, so a literal can pair a normal class with any other axes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestConstructionOutsideTheConstructorsDoesNotCompile`, `TestConstructionOutsideTheConstructorsDoesNotCompile/sensitivityliteral`, `TestNoSensitivityCarryingTypeExposesAField`

## A restricted sender's body is never scanned

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** the sender class is checked after the scan rules, so a restricted sender any scan rule names is scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/a_restricted_sender_under_a_zero_Config`, `TestDecide`, `TestDecide/a_restricted_sender_whose_every_other_input_asks_for_a_scan`, `TestDecide/an_input_nobody_filled_in`
- **Break (2):** the sender class is never read, so a restricted sender is decided like a normal one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/a_restricted_sender_under_a_zero_Config`, `TestDecide`, `TestDecide/a_restricted_sender_otherwise_skipped_by_the_high-volume_rule`, `TestDecide/a_restricted_sender_whose_every_other_input_asks_for_a_scan`, `TestDecide/an_input_nobody_filled_in`
- **Break (3):** the restricted reason records skipped_gate rather than skipped_restricted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/a_restricted_sender_under_a_zero_Config`, `TestDecide`, `TestDecide/a_restricted_sender_otherwise_skipped_by_the_high-volume_rule`, `TestDecide/a_restricted_sender_whose_every_other_input_asks_for_a_scan`, `TestDecide/an_input_nobody_filled_in`

## A restricted sender's mail may only be organized

- **Date · evidence:** 2026-09-25 · [pull request #158](https://github.com/ppat/mediated-mailbox-mcp/pull/158)
- **Break (1):** a normal sender's mail may not be archived, trashed or marked spam
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestMatrix`, `TestMatrix/archive`, `TestMatrix/spam`, `TestMatrix/trash`, `TestOnlyTheMatrixVerbsAreAuthorized`, `TestTheAuthorizerAllowsOnlyWhatTheRulesAllow`
- **Break (2):** a restricted sender's mail may not be labelled
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestMatrix`, `TestMatrix/label`, `TestOnlyTheMatrixVerbsAreAuthorized`, `TestTheAuthorizerAllowsOnlyWhatTheRulesAllow`
- **Break (3):** a restricted sender's mail may be archived, trashed or marked spam
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestFailClosed`, `TestFailClosed/archive_under_a_sender_class_nobody_built`, `TestMatrix`, `TestMatrix/archive`, `TestMatrix/spam`, `TestMatrix/trash`, `TestOnlyTheMatrixVerbsAreAuthorized`, `TestTheAuthorizerAllowsOnlyWhatTheRulesAllow`

## A rule's identifier is unique within its scope, and the snapshot's validation refuses a repeat within one scope only

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the validation keys identifiers without their scope, so a base rule and an account's rule of one identifier fail every reload
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestAnIdentifierIsUniqueWithinItsScopeOnly`
- **Break (2):** the validation stops refusing a repeated identifier, so one scope holding two rules of one identifier loads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestLoadRefusesInvalidRows`, `TestLoadRefusesInvalidRows/identifier_repeated_in_one_account's_rules`, `TestLoadRefusesInvalidRows/identifier_repeated_in_the_base_policy`

## A scan verdict carries no text

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196)
- **Break (1):** the verdict records the matched text in place of the rule
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestFixtures`, `TestFixtures/alphanumeric_code`, `TestFixtures/login_link`, `TestFixtures/one-time_code`, `TestNoScannerOutputCarriesBodyText`, `TestScanPatterns`, `TestScanPatterns/a_code_alone_on_a_line`, `TestScanPatterns/the_login_link_fixture`, `TestScanPatterns/the_one-time_code_fixture`, `TestTheVerdictCarriesNoText`, `TestTier1LoginLinks`, `TestTier1LoginLinks/a_dense_path_segment_with_a_link_word`, `TestTier1LoginLinks/a_dense_token_parameter`, `TestTier1LoginLinks/a_link_inside_Markdown`, `TestTier1LoginLinks/a_link_word_inside_a_joined_path_word`, `TestTier1LoginLinks/a_link_word_inside_a_longer_path_word`, `TestTier1LoginLinks/a_parameter_name_in_capitals`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_after_a_no-break_space`, `TestTier1OneTimeCodes/a_code_after_a_trigger`, `TestTier1OneTimeCodes/a_code_alone_on_a_line`, `TestTier1OneTimeCodes/a_code_alone_on_a_line_ending_in_a_carriage_return`, `TestTier1OneTimeCodes/a_code_alone_on_a_line_in_bold`, `TestTier1OneTimeCodes/a_code_as_a_table_cell`, `TestTier1OneTimeCodes/a_code_before_a_dash`, `TestTier1OneTimeCodes/a_code_before_a_trigger`, `TestTier1OneTimeCodes/a_code_in_a_heading`, `TestTier1OneTimeCodes/a_code_in_a_second-level_heading`, `TestTier1OneTimeCodes/a_code_in_curly_quotes`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_narrow_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_thin_space`, `TestTier1OneTimeCodes/a_code_in_groups_of_three`, `TestTier1OneTimeCodes/a_code_joined_by_a_hyphen`, `TestTier1OneTimeCodes/a_code_joined_to_a_letter_after_it`, `TestTier1OneTimeCodes/a_trigger_exactly_the_window_away`, `TestTier1OneTimeCodes/a_trigger_in_capitals`, `TestTier1OneTimeCodes/a_two-word_trigger`, `TestTier1OneTimeCodes/digits_joined_to_letters`, `TestTier1OneTimeCodes/nine_digits,_which_only_tier_2_catches`, `TestTier2`, `TestTier2/an_alphanumeric_code_after_a_trigger`, `TestTier2/an_alphanumeric_code_alone_in_bold`
- **Break (2):** the verdict gains a field that could hold an excerpt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTheVerdictCarriesNoText`

## A scanner names the version and revision its maskings and verdicts are made under

- **Date · evidence:** 2026-10-01 · [pull request #223](https://github.com/ppat/mediated-mailbox-mcp/pull/223)
- **Break (1):** a scanner names no revision, so a subject masked under it records none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestAScannerNamesTheVersionAndRevisionItDecidesUnder`
- **Break (2):** a scanner nobody built names the version a built one does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestAScannerNamesTheVersionAndRevisionItDecidesUnder`

## A scanner nobody built, or built from an invalid configuration, refuses to decide

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196)
- **Break (1):** a subject threshold above the body's builds a scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestNewRefusesAnInvalidConfiguration`, `TestNewRefusesAnInvalidConfiguration/a_subject_threshold_above_the_body's`
- **Break (2):** an invalid configuration builds a scanner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestNewRefusesAnInvalidConfiguration`, `TestNewRefusesAnInvalidConfiguration/a_blank_link_parameter`, `TestNewRefusesAnInvalidConfiguration/a_dense_entropy_no_value_can_reach`, `TestNewRefusesAnInvalidConfiguration/a_dense_entropy_that_is_not_a_number`, `TestNewRefusesAnInvalidConfiguration/a_negative_weight`, `TestNewRefusesAnInvalidConfiguration/a_subject_threshold_above_the_body's`, `TestNewRefusesAnInvalidConfiguration/a_subject_threshold_of_zero`, `TestNewRefusesAnInvalidConfiguration/a_threshold_above_1`, `TestNewRefusesAnInvalidConfiguration/a_threshold_of_zero`, `TestNewRefusesAnInvalidConfiguration/a_threshold_that_is_not_a_number`, `TestNewRefusesAnInvalidConfiguration/a_trigger_with_surrounding_space`, `TestNewRefusesAnInvalidConfiguration/a_weight_that_is_not_a_number`, `TestNewRefusesAnInvalidConfiguration/a_zero_dense_length`, `TestNewRefusesAnInvalidConfiguration/a_zero_window`, `TestNewRefusesAnInvalidConfiguration/all_weights_zero`, `TestNewRefusesAnInvalidConfiguration/an_empty_revision`, `TestNewRefusesAnInvalidConfiguration/an_empty_trigger`, `TestNewRefusesAnInvalidConfiguration/an_infinite_weight`, `TestNewRefusesAnInvalidConfiguration/no_link_word`, `TestNewRefusesAnInvalidConfiguration/no_trigger_word`, `TestNewRefusesAnInvalidConfiguration/weights_summing_past_the_largest_number`
- **Break (3):** a scanner nobody built reports a scan that found nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestFailClosed`
- **Break (4):** a dense entropy above what any value of bytes can reach builds a scanner whose link rules never fire
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestNewRefusesAnInvalidConfiguration`, `TestNewRefusesAnInvalidConfiguration/a_dense_entropy_no_value_can_reach`
- **Break (5):** a scanner nobody built scans
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestFailClosed`

## A sender the classifier cannot read is restricted

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** every domain that has a registrable domain is classified unclassifiable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/classify`:** `TestClassify`, `TestClassify/a_leading_dot`, `TestClassify/a_listed_domain`, `TestClassify/a_listed_name_followed_by_another_domain`, `TestClassify/a_listed_name_inside_a_longer_label`, `TestClassify/a_public_suffix_alone`, `TestClassify/a_second_suffix_of_one_rule`, `TestClassify/a_sender_in_Unicode_against_a_rule_in_punycode`, `TestClassify/a_sender_in_punycode_against_a_rule_in_Unicode`, `TestClassify/a_subdomain_of_a_listed_domain`, `TestClassify/a_trailing_dot`, `TestClassify/an_empty_label_after_the_trailing_dot`, `TestClassify/an_empty_label_inside`, `TestClassify/an_unlisted_domain`, `TestClassify/another_account's_overlay`, `TestClassify/mixed_case`, `TestClassify/the_account's_own_overlay`, `TestClassify/the_last_at_sign_ends_the_local_part`
- **Break (2):** a domain with no registrable domain is classified against the rules like any other
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/classify`:** `TestClassify`, `TestClassify/a_leading_dot`, `TestClassify/a_public_suffix_alone`, `TestClassify/an_empty_label_after_the_trailing_dot`, `TestClassify/an_empty_label_inside`

## A stored sender domain is stored in the normalizer's form and classifies as the address it came from

- **Date · evidence:** 2026-10-08 · [pull request #322](https://github.com/ppat/mediated-mailbox-mcp/pull/322)
- **Break (1):** the index stores the part of a sender's address after its last @ as written, without the domain normalizer
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestAStoredDomainClassifiesAsItsAddress`
- **Break (2):** the domain normalizer lowercases by Go's simple mapping, which lowers the dotted capital I to a plain i where the classifier reads i followed by U+0307
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/index`:** `TestAStoredDomainClassifiesAsItsAddress`, `TestEveryCodePointStoresAsTheClassifierReadsIt`, `TestStoredDomain`

## A stored state no message can be in is denied as invalid, never as pending

- **Date · evidence:** 2026-09-23 · [pull request #145](https://github.com/ppat/mediated-mailbox-mcp/pull/145)
- **Break (1):** every message not scanned, or with a flag, is denied as invalid
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestDecide`, `TestDecide/a_delisted_sender_before_its_messages_return_to_pending`, `TestDecide/a_login_link`, `TestDecide/a_normal_sender,_skipped_by_the_scan_gate`, `TestDecide/a_one-time_code`, `TestDecide/a_restricted_sender,_pending`, `TestDecide/a_restricted_sender,_skipped_as_restricted`, `TestDecide/a_restricted_sender,_skipped_by_the_scan_gate`, `TestDecide/a_restricted_sender_with_a_flag`, `TestDecide/both_flags`, `TestFailClosed`, `TestFailClosed/an_unscanned_message`, `TestTheGateReleasesOnlyWhatTheRuleReleases`
- **Break (2):** content flags on a message the scanner has not read are decided like any other state
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestFailClosed`, `TestFailClosed/a_flag_on_a_message_skipped_as_restricted`, `TestFailClosed/a_flag_on_a_message_skipped_by_the_scan_gate`, `TestFailClosed/stored_values_nobody_built`

## A verb no constant names is refused

- **Date · evidence:** 2026-09-23 · [pull request #150](https://github.com/ppat/mediated-mailbox-mcp/pull/150)
- **Break (1):** a value no constant names is authorized
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestFailClosed`, `TestFailClosed/a_value_past_the_last_verb`, `TestFailClosed/the_largest_value`, `TestOnlyTheMatrixVerbsAreAuthorized`, `TestTheAuthorizerAllowsOnlyWhatTheRulesAllow`
- **Break (2):** a value no constant names is authorized for a restricted sender's mail alone
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestOnlyTheMatrixVerbsAreAuthorized`, `TestTheAuthorizerAllowsOnlyWhatTheRulesAllow`
- **Break (3):** the zero Verb is authorized
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestFailClosed`, `TestFailClosed/the_zero_verb_from_a_normal_sender`, `TestFailClosed/the_zero_verb_from_a_restricted_sender`, `TestOnlyTheMatrixVerbsAreAuthorized`, `TestTheAuthorizerAllowsOnlyWhatTheRulesAllow`

## A verdict nobody built decides nothing

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** an undecided verdict scans, the zero value included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/a_high-volume_mark_below_the_low-volume_mark`, `TestAConfigThatCannotDecide/a_low-volume_mark_of_zero`, `TestAConfigThatCannotDecide/a_size_mark_of_zero`, `TestAConfigThatCannotDecide/a_zero_Config`, `TestAConfigThatCannotDecide/an_age_mark_of_zero`, `TestTheZeroVerdictDecidesNothing`
- **Break (2):** an undecided verdict records skipped_gate, the zero value included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/a_high-volume_mark_below_the_low-volume_mark`, `TestAConfigThatCannotDecide/a_low-volume_mark_of_zero`, `TestAConfigThatCannotDecide/a_size_mark_of_zero`, `TestAConfigThatCannotDecide/a_zero_Config`, `TestAConfigThatCannotDecide/an_age_mark_of_zero`, `TestTheZeroVerdictDecidesNothing`

## A verdict reason no case handles refuses the mutation

- **Date · evidence:** 2026-09-23 · [pull request #150](https://github.com/ppat/mediated-mailbox-mcp/pull/150)
- **Break (1):** a reason no case names authorizes the mutation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestAnUnhandledReasonRefuses`
- **Break (2):** a reason no case names reads as authorized
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestAnUnhandledReasonRefuses`

## A verdict reason no case handles withholds the body

- **Date · evidence:** 2026-09-23 · [pull request #145](https://github.com/ppat/mediated-mailbox-mcp/pull/145)
- **Break (1):** a reason no case names reads as released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAnUnhandledReasonWithholds`
- **Break (2):** a reason no case names releases the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAnUnhandledReasonWithholds`

## ADR-0041's absence-denies rule

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a policy that never loaded composes each named account's policy as a loaded one with no rules, restricting no sender
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestSwap`, `TestSwap/an_invalid_first_update_leaves_no_policy`, `TestTheZeroSnapshotRestrictsEverySender`
- **Break (2):** a loaded policy also restricts every sender
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestASnapshotCannotBeChangedAfterLoading`, `TestForComposesTheBaseWithTheAccountsOverlay`, `TestForComposesTheBaseWithTheAccountsOverlay/acct-a`, `TestForComposesTheBaseWithTheAccountsOverlay/acct-c`, `TestSwap`, `TestSwap/a_valid_update_takes_effect`, `TestSwap/an_invalid_update_leaves_the_active_policy_in_place`

## An invalid policy update never displaces the active policy

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** Swap adopts every update, so an invalid one replaces the active policy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestSwap`, `TestSwap/an_invalid_first_update_leaves_no_policy`, `TestSwap/an_invalid_update_leaves_the_active_policy_in_place`
- **Break (2):** Swap reports a valid update accepted and keeps the active policy anyway
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestSwap`, `TestSwap/a_valid_update_takes_effect`
- **Break (3):** every domain suffix is refused, the valid ones included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestASnapshotCannotBeChangedAfterLoading`, `TestAnEmptyAccountNameRestrictsEverySender`, `TestAnIdentifierIsUniqueWithinItsScopeOnly`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestComposingAnOverlayNeverDropsABaseRuleMix`, `TestForComposesTheBaseWithTheAccountsOverlay`, `TestLoadAcceptsOverlaysOnlyAnEmptyPolicyAndAnyScript`, `TestLoadAcceptsOverlaysOnlyAnEmptyPolicyAndAnyScript/any_script`, `TestLoadAcceptsOverlaysOnlyAnEmptyPolicyAndAnyScript/overlays_only`, `TestSwap`
- **Break (4):** load finds problems and reports none, so every update is valid
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicyMix`, `TestLoadRefusesInvalidRows`, `TestLoadRefusesInvalidRows/at_sign`, `TestLoadRefusesInvalidRows/class_other_than_restricted`, `TestLoadRefusesInvalidRows/comma`, `TestLoadRefusesInvalidRows/control_character`, `TestLoadRefusesInvalidRows/empty_identifier`, `TestLoadRefusesInvalidRows/empty_label`, `TestLoadRefusesInvalidRows/empty_suffix`, `TestLoadRefusesInvalidRows/identifier_repeated_in_one_account's_rules`, `TestLoadRefusesInvalidRows/identifier_repeated_in_the_base_policy`, `TestLoadRefusesInvalidRows/identifier_with_surrounding_space`, `TestLoadRefusesInvalidRows/leading_dot`, `TestLoadRefusesInvalidRows/no_class`, `TestLoadRefusesInvalidRows/no_domain_suffix`, `TestLoadRefusesInvalidRows/slash`, `TestLoadRefusesInvalidRows/space`, `TestLoadRefusesInvalidRows/trailing_dot`, `TestLoadRefusesInvalidRows/wildcard`, `TestSwap`, `TestSwap/an_invalid_first_update_leaves_no_policy`, `TestSwap/an_invalid_update_leaves_the_active_policy_in_place`

## An overlay only adds restrictions to its own account

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** an account's policy is its overlay alone, without the base rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestASnapshotCannotBeChangedAfterLoading`, `TestAnIdentifierIsUniqueWithinItsScopeOnly`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestComposingAnOverlayNeverDropsABaseRule`, `TestForComposesTheBaseWithTheAccountsOverlay`, `TestForComposesTheBaseWithTheAccountsOverlay/acct-a`, `TestForComposesTheBaseWithTheAccountsOverlay/acct-c`, `TestSwap`, `TestSwap/a_valid_update_takes_effect`, `TestSwap/an_invalid_update_leaves_the_active_policy_in_place`
- **Break (2):** every account's policy holds every account's overlay rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/policy`:** `TestAnIdentifierIsUniqueWithinItsScopeOnly`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestComposingAnOverlayNeverDropsABaseRule`, `TestForComposesTheBaseWithTheAccountsOverlay`, `TestForComposesTheBaseWithTheAccountsOverlay/acct-a`, `TestForComposesTheBaseWithTheAccountsOverlay/acct-c`

## Scanning stays linear in a body's length

- **Date · evidence:** 2026-09-24 · [pull request #151](https://github.com/ppat/mediated-mailbox-mcp/pull/151)
- **Break (1):** finding a token's line walks every line before it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestScanningStaysLinear`
- **Break (2):** checking a trigger's distance walks every trigger before it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestScanningStaysLinear`

## Sender class alone governs mutation, and the verdict holds its reason alone

- **Date · evidence:** 2026-09-23 · [pull request #150](https://github.com/ppat/mediated-mailbox-mcp/pull/150)
- **Break (1):** the decision gains a parameter through which content flags could change a mutation right
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestTheDecisionTakesOnlyTheVerbAndTheSenderClass`
- **Break (2):** the decision gains a parameter through which a provider could be passed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestTheDecisionTakesOnlyTheVerbAndTheSenderClass`
- **Break (3):** the verdict gains a field holding the message's subject
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/authorize`:** `TestTheDecisionTakesOnlyTheVerbAndTheSenderClass`

## Subject masking masks the whole subject without a scanner

- **Date · evidence:** 2026-09-24 · [pull request #151](https://github.com/ppat/mediated-mailbox-mcp/pull/151)
- **Break (1):** without a scanner the subject is left as it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestMaskingFailsClosedWithoutAScanner`
- **Break (2):** masking the whole subject records no event
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestMaskingFailsClosedWithoutAScanner`
- **Break (3):** masking the whole subject writes one █ per byte rather than per character
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestMaskingFailsClosedWithoutAScanner`

## Subject masking replaces every detected code and nothing else

- **Date · evidence:** 2026-09-24 · [pull request #151](https://github.com/ppat/mediated-mailbox-mcp/pull/151)
- **Break (1):** a detected code is left in the subject
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestARestrictedSendersCodeSubjectIsMasked`, `TestMaskSubject`, `TestMaskSubject/a_code_after_a_trigger`, `TestMaskSubject/a_code_among_accented_letters`, `TestMaskSubject/a_code_in_groups`, `TestMaskSubject/a_code_in_groups_joined_by_a_no-break_space`, `TestMaskSubject/a_link_before_a_full_stop`, `TestMaskSubject/a_link_holding_an_accented_letter`, `TestMaskSubject/an_alphanumeric_code`, `TestMaskSubject/an_alphanumeric_code_before_a_trigger`, `TestMaskingRemovesEveryCode`
- **Break (2):** every character of the subject is masked, detected or not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestARestrictedSendersCodeSubjectIsMasked`, `TestMaskSubject`, `TestMaskSubject/a_code_after_a_trigger`, `TestMaskSubject/a_code_among_accented_letters`, `TestMaskSubject/a_code_in_groups`, `TestMaskSubject/a_code_in_groups_joined_by_a_no-break_space`, `TestMaskSubject/a_link_before_a_full_stop`, `TestMaskSubject/a_link_holding_an_accented_letter`, `TestMaskSubject/a_subject_with_nothing_to_mask`, `TestMaskSubject/an_alphanumeric_code`, `TestMaskSubject/an_alphanumeric_code_before_a_trigger`, `TestMaskSubject/an_alphanumeric_order_number`, `TestMaskSubject/an_order_number`, `TestMaskingRemovesEveryCode`
- **Break (3):** a masked character written in several bytes becomes several █
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestMaskSubject`, `TestMaskSubject/a_code_in_groups_joined_by_a_no-break_space`, `TestMaskSubject/a_link_holding_an_accented_letter`

## Subject masking takes no sender class and records no text

- **Date · evidence:** 2026-09-24 · [pull request #151](https://github.com/ppat/mediated-mailbox-mcp/pull/151)
- **Break (1):** masking gains a parameter through which a sender class could skip it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestMaskingTakesNoSenderClass`
- **Break (2):** a masking event gains a field that could hold the masked text
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestMaskingTakesNoSenderClass`

## The classifier restricts every sender while no policy has loaded

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a policy that never loaded is classified against like an empty one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/classify`:** `TestNoPolicyRestrictsEverySender`
- **Break (2):** a loaded policy restricts every sender as if none had loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/classify`:** `TestClassify`, `TestClassify/a_domain_that_is_not_a_name`, `TestClassify/a_leading_dot`, `TestClassify/a_listed_domain`, `TestClassify/a_listed_name_followed_by_another_domain`, `TestClassify/a_listed_name_inside_a_longer_label`, `TestClassify/a_public_suffix_alone`, `TestClassify/a_second_suffix_of_one_rule`, `TestClassify/a_sender_in_Unicode_against_a_rule_in_punycode`, `TestClassify/a_sender_in_punycode_against_a_rule_in_Unicode`, `TestClassify/a_subdomain_of_a_listed_domain`, `TestClassify/a_trailing_dot`, `TestClassify/an_address_literal`, `TestClassify/an_empty_label_after_the_trailing_dot`, `TestClassify/an_empty_label_inside`, `TestClassify/an_unlisted_domain`, `TestClassify/another_account's_overlay`, `TestClassify/mixed_case`, `TestClassify/no_at_sign`, `TestClassify/no_domain`, `TestClassify/the_account's_own_overlay`, `TestClassify/the_last_at_sign_ends_the_local_part`, `TestNoPolicyRestrictsEverySender`

## The default thresholds are ADR-0093's figures

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** the default age mark is 2 hours rather than 24
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDefaultConfigIsADR0007s`
- **Break (2):** the default local-part list holds only noreply and no-reply
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDefaultConfigIsADR0007s`

## The first rule in ADR-0093's order gives the reason

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** the low-volume rule is checked before the recent small rule
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_recent_small_message_wins_over_low_volume`
- **Break (2):** the prior hit rule is checked before the low-volume rule
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/low_volume_wins_over_a_prior_hit`
- **Break (3):** the recent small rule is checked before the local-part rule
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_noreply_local_part_wins_over_the_recent_small_rule`

## The gate decides without a provider or a body

- **Date · evidence:** 2026-09-23 · [pull request #145](https://github.com/ppat/mediated-mailbox-mcp/pull/145)
- **Break (1):** the decision gains a parameter through which a body could be passed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestTheDecisionTakesNoProviderAndNoBody`
- **Break (2):** the decision gains a parameter through which a provider could be passed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestTheDecisionTakesNoProviderAndNoBody`
- **Break (3):** the classifier's lookups gain a function through which a provider could be reached
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestTheDecisionTakesNoProviderAndNoBody`

## The gate's verdict cannot carry the body it withholds

- **Date · evidence:** 2026-09-23 · [pull request #145](https://github.com/ppat/mediated-mailbox-mcp/pull/145)
- **Break (1):** the sender's classification inside the verdict gains a field holding the snippet
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestTheVerdictCannotCarryContent`
- **Break (2):** the verdict gains an exported field holding the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestTheVerdictCannotCarryContent`
- **Break (3):** the verdict gains a field holding the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestTheVerdictCannotCarryContent`
- **Break (4):** the verdict gains a field of another name and type, holding the snippet
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestTheVerdictCannotCarryContent`

## The high-volume rule skips only a message carrying a List-Id from a sender above the mark with no prior hit

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** the high-volume rule skips whatever the prior hit count
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_sender_with_a_negative_hit_count`
- **Break (2):** the high-volume rule skips a message with no List-Id
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_local_part_holding_a_listed_one_inside_it`, `TestDecide/a_message_with_no_List-Id_no_other_rule_names`, `TestDecide/a_recent_message_at_the_size_mark`, `TestDecide/a_small_message_at_the_age_mark`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_default_local_part_left_out_of_the_configuration`, `TestDecideReadsTheConfig/a_message_at_a_lowered_size_mark`, `TestDecideReadsTheConfig/a_message_at_a_shortened_age_mark`
- **Break (3):** a sender exactly at the high-volume mark counts as high volume
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_sender_at_the_high-volume_mark`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_sender_at_a_lowered_high-volume_mark`
- **Break (4):** the high-volume rule never skips
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/the_default_Config`, `TestDecide`, `TestDecide/a_high-volume_sender_with_no_prior_hit,_a_List-Id_and_no_subject_signal`, `TestDecide/a_noreply_local_part_with_a_List-Id`, `TestDecide/a_sender_far_above_the_high-volume_mark`, `TestDecide/a_small_recent_message_with_a_List-Id`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_sender_above_a_lowered_high-volume_mark`

## The policy list alone decides sender class

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a rule matches its own domain only, never a subdomain
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/classify`:** `TestClassify`, `TestClassify/a_second_suffix_of_one_rule`, `TestClassify/a_sender_in_punycode_against_a_rule_in_Unicode`, `TestClassify/a_subdomain_of_a_listed_domain`, `TestClassify/mixed_case`
- **Break (2):** a rule matches any domain ending in its text, across label boundaries
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/classify`:** `TestClassify`, `TestClassify/a_listed_name_inside_a_longer_label`
- **Break (3):** domains are compared as written, without lowercasing or punycode decoding
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/classify`:** `TestClassify`, `TestClassify/a_sender_in_Unicode_against_a_rule_in_punycode`, `TestClassify/a_sender_in_punycode_against_a_rule_in_Unicode`, `TestClassify/mixed_case`

## The Redaction Gate classifies the sender against the policy it is given

- **Date · evidence:** 2026-09-23 · [pull request #145](https://github.com/ppat/mediated-mailbox-mcp/pull/145)
- **Break (1):** the sender is classified against an empty policy instead of the one given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAJustListedDomainDeniesOnTheNextDecision`, `TestDecide`, `TestDecide/a_restricted_sender,_pending`, `TestDecide/a_restricted_sender,_scanned_before_it_was_listed`, `TestDecide/a_restricted_sender,_skipped_as_restricted`, `TestDecide/a_restricted_sender,_skipped_by_the_scan_gate`, `TestDecide/a_restricted_sender_with_a_flag`, `TestFailClosed`, `TestFailClosed/a_flag_on_a_message_skipped_as_restricted`, `TestFailClosed/a_policy_that_never_loaded`, `TestTheGateReleasesOnlyWhatTheRuleReleases`
- **Break (2):** the sender is classified as if no policy had loaded, whatever policy is given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAJustListedDomainDeniesOnTheNextDecision`, `TestDecide`, `TestDecide/a_delisted_sender_before_its_messages_return_to_pending`, `TestDecide/a_login_link`, `TestDecide/a_normal_sender,_scanned_clean`, `TestDecide/a_normal_sender,_skipped_by_the_scan_gate`, `TestDecide/a_one-time_code`, `TestDecide/a_restricted_sender,_pending`, `TestDecide/a_restricted_sender,_scanned_before_it_was_listed`, `TestDecide/a_restricted_sender,_skipped_as_restricted`, `TestDecide/a_restricted_sender,_skipped_by_the_scan_gate`, `TestDecide/a_restricted_sender_with_a_flag`, `TestDecide/both_flags`, `TestFailClosed`, `TestFailClosed/a_flag_on_a_message_skipped_as_restricted`, `TestFailClosed/a_flag_on_a_message_skipped_by_the_scan_gate`, `TestFailClosed/a_sender_the_classifier_cannot_read`, `TestFailClosed/a_sender_with_no_registrable_domain`, `TestFailClosed/an_unscanned_message`, `TestFailClosed/stored_values_nobody_built`, `TestTheGateReleasesOnlyWhatTheRuleReleases`

## The Redaction Gate releases a body only for a sender the policy does not restrict, with no content flag, scanned or skipped by the scan gate

- **Date · evidence:** 2026-09-23 · [pull request #145](https://github.com/ppat/mediated-mailbox-mcp/pull/145)
- **Break (1):** a message with a one-time code or a login link is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestDecide`, `TestDecide/a_login_link`, `TestDecide/a_one-time_code`, `TestDecide/both_flags`, `TestTheGateReleasesOnlyWhatTheRuleReleases`
- **Break (2):** only a sender a rule lists is restricted, so an unreadable address and a policy that never loaded are released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestFailClosed`, `TestFailClosed/a_policy_that_never_loaded`, `TestFailClosed/a_sender_the_classifier_cannot_read`, `TestFailClosed/a_sender_with_no_registrable_domain`
- **Break (3):** a message the scanner has not reached is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestDecide`, `TestDecide/a_restricted_sender,_pending`, `TestFailClosed`, `TestFailClosed/an_unscanned_message`, `TestTheGateReleasesOnlyWhatTheRuleReleases`
- **Break (4):** no body is ever released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAJustListedDomainDeniesOnTheNextDecision`, `TestDecide`, `TestDecide/a_normal_sender,_scanned_clean`, `TestDecide/a_normal_sender,_skipped_by_the_scan_gate`, `TestTheGateReleasesOnlyWhatTheRuleReleases`
- **Break (5):** a sender the policy restricts is released
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAJustListedDomainDeniesOnTheNextDecision`, `TestDecide`, `TestDecide/a_restricted_sender,_scanned_before_it_was_listed`, `TestDecide/a_restricted_sender,_skipped_by_the_scan_gate`, `TestDecide/a_restricted_sender_with_a_flag`, `TestFailClosed`, `TestFailClosed/a_policy_that_never_loaded`, `TestFailClosed/a_sender_the_classifier_cannot_read`, `TestFailClosed/a_sender_with_no_registrable_domain`, `TestTheGateReleasesOnlyWhatTheRuleReleases`

## The scan gate scans a message with no List-Id from a listed local part

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** a local part matches a listed one only in the same letter case
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_noreply_local_part_in_another_letter_case`
- **Break (2):** a listed local part asks for a scan even on a message carrying a List-Id
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_noreply_local_part_with_a_List-Id`
- **Break (3):** a listed local part no longer asks for a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_noreply_local_part_in_another_letter_case`, `TestDecide/a_noreply_local_part_wins_over_the_recent_small_rule`, `TestDecide/a_noreply_local_part_with_no_List-Id`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_configured_local_part`
- **Break (4):** a local part matches when it holds a listed one anywhere inside it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_local_part_holding_a_listed_one_inside_it`

## The scan gate scans a sender below the low-volume mark

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** a sender exactly at the low-volume mark counts as low volume
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_sender_at_the_low-volume_mark`
- **Break (2):** a low sender volume no longer asks for a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_sender_below_the_low-volume_mark`, `TestDecide/low_volume_wins_over_a_prior_hit`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_sender_below_a_raised_low-volume_mark`

## The scan gate scans a sender with a prior scan hit

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** a sender with no prior hit counts as having one, so the prior-hit rule scans every sender it reaches
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/the_default_Config`, `TestDecide`, `TestDecide/a_high-volume_sender_with_no_prior_hit,_a_List-Id_and_no_subject_signal`, `TestDecide/a_local_part_holding_a_listed_one_inside_it`, `TestDecide/a_message_with_no_List-Id_no_other_rule_names`, `TestDecide/a_noreply_local_part_with_a_List-Id`, `TestDecide/a_recent_message_at_the_size_mark`, `TestDecide/a_sender_at_the_high-volume_mark`, `TestDecide/a_sender_at_the_low-volume_mark`, `TestDecide/a_sender_far_above_the_high-volume_mark`, `TestDecide/a_small_message_at_the_age_mark`, `TestDecide/a_small_recent_message_with_a_List-Id`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_default_local_part_left_out_of_the_configuration`, `TestDecideReadsTheConfig/a_message_at_a_lowered_size_mark`, `TestDecideReadsTheConfig/a_message_at_a_shortened_age_mark`, `TestDecideReadsTheConfig/a_sender_above_a_lowered_high-volume_mark`, `TestDecideReadsTheConfig/a_sender_at_a_lowered_high-volume_mark`
- **Break (2):** a sender needs more than one prior hit to be scanned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_sender_with_one_prior_hit`
- **Break (3):** a prior scan hit no longer asks for a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_sender_with_one_prior_hit`

## The scan gate scans a small recent message with no List-Id

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** a message exactly at the age mark counts as recent
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_small_message_at_the_age_mark`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_message_at_a_shortened_age_mark`
- **Break (2):** a small recent message asks for a scan even when it carries a List-Id
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_small_recent_message_with_a_List-Id`
- **Break (3):** a small recent message no longer asks for a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_recent_small_message_wins_over_low_volume`, `TestDecide/a_small_recent_message_with_no_List-Id`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_message_below_a_lowered_size_mark`
- **Break (4):** a message exactly at the size mark counts as small
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_recent_message_at_the_size_mark`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_message_at_a_lowered_size_mark`

## The scan gate scans a subject pass 1 masked

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** an unmasked subject asks for a scan and a masked one does not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestAConfigThatCannotDecide`, `TestAConfigThatCannotDecide/the_default_Config`, `TestDecide`, `TestDecide/a_high-volume_sender_with_no_prior_hit,_a_List-Id_and_no_subject_signal`, `TestDecide/a_local_part_holding_a_listed_one_inside_it`, `TestDecide/a_message_with_no_List-Id_no_other_rule_names`, `TestDecide/a_noreply_local_part_in_another_letter_case`, `TestDecide/a_noreply_local_part_wins_over_the_recent_small_rule`, `TestDecide/a_noreply_local_part_with_a_List-Id`, `TestDecide/a_noreply_local_part_with_no_List-Id`, `TestDecide/a_recent_message_at_the_size_mark`, `TestDecide/a_recent_small_message_wins_over_low_volume`, `TestDecide/a_sender_at_the_high-volume_mark`, `TestDecide/a_sender_at_the_low-volume_mark`, `TestDecide/a_sender_below_the_low-volume_mark`, `TestDecide/a_sender_far_above_the_high-volume_mark`, `TestDecide/a_sender_with_a_negative_hit_count`, `TestDecide/a_sender_with_one_prior_hit`, `TestDecide/a_small_message_at_the_age_mark`, `TestDecide/a_small_recent_message_with_a_List-Id`, `TestDecide/a_small_recent_message_with_no_List-Id`, `TestDecide/a_subject_pass_1_masked`, `TestDecide/a_subject_signal_wins_over_every_later_rule`, `TestDecide/low_volume_wins_over_a_prior_hit`, `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_configured_local_part`, `TestDecideReadsTheConfig/a_default_local_part_left_out_of_the_configuration`, `TestDecideReadsTheConfig/a_message_at_a_lowered_size_mark`, `TestDecideReadsTheConfig/a_message_at_a_shortened_age_mark`, `TestDecideReadsTheConfig/a_message_below_a_lowered_size_mark`, `TestDecideReadsTheConfig/a_sender_above_a_lowered_high-volume_mark`, `TestDecideReadsTheConfig/a_sender_at_a_lowered_high-volume_mark`, `TestDecideReadsTheConfig/a_sender_below_a_raised_low-volume_mark`
- **Break (2):** a masked subject no longer asks for a scan
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecide`, `TestDecide/a_subject_pass_1_masked`, `TestDecide/a_subject_signal_wins_over_every_later_rule`

## The serve-time pattern check runs the structural patterns and not the scoring

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** the pattern check reads an empty body instead of the one it is given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestScanPatterns`, `TestScanPatterns/a_code_alone_on_a_line`, `TestScanPatterns/the_login_link_fixture`, `TestScanPatterns/the_one-time_code_fixture`
- **Break (2):** a scan of the patterns alone goes on to tier 2's scoring
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestScanPatterns`, `TestScanPatterns/nine_digits_after_a_trigger,_which_only_tier_2_catches`, `TestScanPatterns/the_alphanumeric_code_fixture,_which_only_tier_2_catches`, `TestScanPatterns/the_receipt_fixture`

## The snippet and attachment filenames follow the body

- **Date · evidence:** 2026-09-23 · [pull request #145](https://github.com/ppat/mediated-mailbox-mcp/pull/145)
- **Break (1):** attachment filenames are withheld even with a released body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAJustListedDomainDeniesOnTheNextDecision`, `TestDecide`, `TestDecide/a_normal_sender,_scanned_clean`, `TestDecide/a_normal_sender,_skipped_by_the_scan_gate`, `TestTheGateReleasesOnlyWhatTheRuleReleases`
- **Break (2):** the snippet is shown whatever happens to the body
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/redact`:** `TestAJustListedDomainDeniesOnTheNextDecision`, `TestAnUnhandledReasonWithholds`, `TestDecide`, `TestDecide/a_delisted_sender_before_its_messages_return_to_pending`, `TestDecide/a_login_link`, `TestDecide/a_one-time_code`, `TestDecide/a_restricted_sender,_pending`, `TestDecide/a_restricted_sender,_scanned_before_it_was_listed`, `TestDecide/a_restricted_sender,_skipped_as_restricted`, `TestDecide/a_restricted_sender,_skipped_by_the_scan_gate`, `TestDecide/a_restricted_sender_with_a_flag`, `TestDecide/both_flags`, `TestFailClosed`, `TestFailClosed/a_flag_on_a_message_skipped_as_restricted`, `TestFailClosed/a_flag_on_a_message_skipped_by_the_scan_gate`, `TestFailClosed/a_policy_that_never_loaded`, `TestFailClosed/a_sender_the_classifier_cannot_read`, `TestFailClosed/a_sender_with_no_registrable_domain`, `TestFailClosed/an_unscanned_message`, `TestFailClosed/stored_values_nobody_built`, `TestTheGateReleasesOnlyWhatTheRuleReleases`, `TestTheZeroVerdictWithholds`

## The thresholds are the ones the caller passes

- **Date · evidence:** 2026-09-25 · [pull request #167](https://github.com/ppat/mediated-mailbox-mcp/pull/167)
- **Break (1):** the high-volume mark is fixed at 500 whatever the Config says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_sender_above_a_lowered_high-volume_mark`
- **Break (2):** the local-part list is ADR-0093's whatever the Config says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_configured_local_part`, `TestDecideReadsTheConfig/a_default_local_part_left_out_of_the_configuration`
- **Break (3):** the low-volume mark is fixed at 20 whatever the Config says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_sender_below_a_raised_low-volume_mark`
- **Break (4):** the size and age marks are fixed at 30,720 bytes and 24 hours whatever the Config says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scangate`:** `TestDecideReadsTheConfig`, `TestDecideReadsTheConfig/a_message_at_a_lowered_size_mark`, `TestDecideReadsTheConfig/a_message_at_a_shortened_age_mark`

## Tier 1 flags a login link

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** a long value counts as dense whatever its entropy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1LoginLinks`, `TestTier1LoginLinks/a_long_parameter_that_is_not_dense`
- **Break (2):** the URL fragment is read as part of the path
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1LoginLinks`, `TestTier1LoginLinks/a_link_word_only_in_the_fragment`
- **Break (3):** a link word in the host counts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1LoginLinks`, `TestTier1LoginLinks/a_link_word_only_in_the_host`
- **Break (4):** a dense path segment with a link word is not flagged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestEvaluationSet`, `TestEvaluationSet/a_reset_path_with_a_hex_token`, `TestEvaluationSet/a_reset_path_with_an_email_parameter`, `TestEvaluationSet/a_token_under_a_passwords_path`, `TestEvaluationSet/an_article_slug_holding_a_link_word`, `TestEvaluationSet/an_email_login_path`, `TestFixtures`, `TestFixtures/login_link`, `TestNoScannerOutputCarriesBodyText`, `TestScanPatterns`, `TestScanPatterns/the_login_link_fixture`, `TestTier1LoginLinks`, `TestTier1LoginLinks/a_dense_path_segment_with_a_link_word`, `TestTier1LoginLinks/a_link_inside_Markdown`, `TestTier1LoginLinks/a_link_word_inside_a_joined_path_word`, `TestTier1LoginLinks/a_link_word_inside_a_longer_path_word`
- **Break (5):** a dense value of a configured parameter is not flagged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestEvaluationSet`, `TestEvaluationSet/a_callback_with_a_token`, `TestEvaluationSet/a_confirmation_code_parameter`, `TestEvaluationSet/a_confirmation_parameter`, `TestEvaluationSet/a_hosted_verify_endpoint`, `TestEvaluationSet/a_magic_link_of_letters`, `TestEvaluationSet/a_magic_link_token`, `TestEvaluationSet/a_password_reset_key`, `TestEvaluationSet/a_password_reset_parameter`, `TestEvaluationSet/a_recovery_flow`, `TestEvaluationSet/a_verification_ticket`, `TestEvaluationSet/an_action_token`, `TestEvaluationSet/an_out-of-band_code`, `TestFixtures`, `TestFixtures/login_link`, `TestScanPatterns`, `TestScanPatterns/the_login_link_fixture`, `TestTier1LoginLinks`, `TestTier1LoginLinks/a_dense_token_parameter`, `TestTier1LoginLinks/a_parameter_name_in_capitals`
- **Break (6):** a dense path segment is flagged without a link word
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestEvaluationSet`, `TestEvaluationSet/a_marketing_link`, `TestTier1LoginLinks`, `TestTier1LoginLinks/a_dense_path_segment_without_a_link_word`, `TestTier1LoginLinks/a_link_word_only_in_the_fragment`, `TestTier1LoginLinks/a_link_word_only_in_the_host`
- **Break (7):** a link word counts only as a whole word, so /confirmation/ and /resetPassword/ are missed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestEvaluationSet`, `TestEvaluationSet/a_token_under_a_passwords_path`, `TestTier1LoginLinks`, `TestTier1LoginLinks/a_link_word_inside_a_joined_path_word`, `TestTier1LoginLinks/a_link_word_inside_a_longer_path_word`

## Tier 1 flags a one-time code near a trigger, alone on a line, or in a heading or table cell

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** a code in a heading of any level is flagged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_in_a_third-level_heading`
- **Break (2):** a code in a heading or a table cell is not flagged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestFixtures`, `TestFixtures/one-time_code`, `TestScanPatterns`, `TestScanPatterns/the_one-time_code_fixture`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_as_a_table_cell`, `TestTier1OneTimeCodes/a_code_in_a_heading`, `TestTier1OneTimeCodes/a_code_in_a_second-level_heading`
- **Break (3):** every character outside ASCII counts as part of a word, so a code beside a dash, a curly quote or a no-break space is missed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_after_a_no-break_space`, `TestTier1OneTimeCodes/a_code_before_a_dash`, `TestTier1OneTimeCodes/a_code_in_curly_quotes`
- **Break (4):** a code alone on a line is not flagged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestScanPatterns`, `TestScanPatterns/a_code_alone_on_a_line`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_alone_on_a_line`, `TestTier1OneTimeCodes/a_code_alone_on_a_line_ending_in_a_carriage_return`, `TestTier1OneTimeCodes/a_code_alone_on_a_line_in_bold`
- **Break (5):** a run of three digits counts as a code
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/three_digits`
- **Break (6):** a digit run joined to a letter after it counts as a code
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_joined_to_a_letter_after_it`
- **Break (7):** a digit run near a trigger word is not flagged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestEvaluationSet`, `TestEvaluationSet/a_code_in_groups_joined_by_a_hyphen`, `TestEvaluationSet/a_code_in_groups_of_three`, `TestFixtures`, `TestFixtures/one-time_code`, `TestNoScannerOutputCarriesBodyText`, `TestScanPatterns`, `TestScanPatterns/the_one-time_code_fixture`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_after_a_no-break_space`, `TestTier1OneTimeCodes/a_code_after_a_trigger`, `TestTier1OneTimeCodes/a_code_before_a_dash`, `TestTier1OneTimeCodes/a_code_before_a_trigger`, `TestTier1OneTimeCodes/a_code_in_curly_quotes`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_narrow_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_thin_space`, `TestTier1OneTimeCodes/a_code_in_groups_of_three`, `TestTier1OneTimeCodes/a_code_joined_by_a_hyphen`, `TestTier1OneTimeCodes/a_trigger_exactly_the_window_away`, `TestTier1OneTimeCodes/a_trigger_in_capitals`, `TestTier1OneTimeCodes/a_two-word_trigger`
- **Break (8):** only a space or a hyphen joins digit groups, so a code split by a no-break space is missed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_narrow_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_no-break_space`, `TestTier1OneTimeCodes/a_code_in_groups_joined_by_a_thin_space`
- **Break (9):** a trigger word counts at any distance
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_trigger_one_past_the_window`
- **Break (10):** a trigger word inside a longer word counts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestAutomaton`, `TestAutomaton/a_word_inside_a_longer_word,_bounded`, `TestAutomaton/a_word_next_to_a_letter_in_another_script`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_trigger_inside_a_longer_word`

## Tier 2 flags a code tier 1 misses, at the configured threshold

- **Date · evidence:** 2026-09-24 · [pull request #151](https://github.com/ppat/mediated-mailbox-mcp/pull/151)
- **Break (1):** tier 2 scores references of 11 and 12 characters, longer than any one-time code
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestTier2`, `TestTier2/an_eleven-character_reference_after_a_trigger`
- **Break (2):** every scored span is flagged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestEvaluationSet`, `TestEvaluationSet/a_date`, `TestEvaluationSet/a_flight_number`, `TestEvaluationSet/a_phone_number`, `TestEvaluationSet/a_product_reference`, `TestEvaluationSet/a_year_in_prose`, `TestEvaluationSet/an_alphanumeric_order_number`, `TestEvaluationSet/an_order_number`, `TestEvaluationSet/in_a_subject,_a_flight_number`, `TestEvaluationSet/in_a_subject,_a_reservation_number`, `TestEvaluationSet/in_a_subject,_an_alphanumeric_order_number`, `TestEvaluationSet/in_a_subject,_an_invoice_number`, `TestEvaluationSet/in_a_subject,_an_order_number`, `TestFixtures`, `TestFixtures/receipt`, `TestSubjectThreshold`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_in_a_third-level_heading`, `TestTier1OneTimeCodes/a_number_inside_a_table_cell's_text`, `TestTier1OneTimeCodes/a_trigger_inside_a_longer_word`, `TestTier1OneTimeCodes/a_trigger_one_past_the_window`, `TestTier2`, `TestTier2/a_date`, `TestTier2/an_order_number`
- **Break (3):** no score reaches the threshold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestEvaluationSet`, `TestEvaluationSet/an_alphanumeric_code_after_a_trigger`, `TestEvaluationSet/an_alphanumeric_code_alone_in_bold`, `TestEvaluationSet/an_alphanumeric_code_on_its_own_line`, `TestEvaluationSet/an_alphanumeric_login_code`, `TestEvaluationSet/in_a_subject,_an_alphanumeric_code_after_a_trigger`, `TestFixtures`, `TestFixtures/alphanumeric_code`, `TestSubjectThreshold`, `TestTier1OneTimeCodes`, `TestTier1OneTimeCodes/a_code_joined_to_a_letter_after_it`, `TestTier1OneTimeCodes/digits_joined_to_letters`, `TestTier1OneTimeCodes/nine_digits,_which_only_tier_2_catches`, `TestTier2`, `TestTier2/an_alphanumeric_code_after_a_trigger`, `TestTier2/an_alphanumeric_code_alone_in_bold`
- **Break (4):** a subject is scored against the body threshold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestSubjectThreshold`
- **Break (5):** a body is scored against the subject threshold
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/scan`:** `TestSubjectThreshold`
