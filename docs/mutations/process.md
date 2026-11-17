# Mutations: process

The demonstrations of the controls whose patches sit in `process/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A background task's panic or failure is its own and stops the process in order, and every task is joined before the pools close

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a task's panic is recovered and dropped, so the process serves on without the task
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestAStartedGoroutineIsTheHosts`, `TestATaskPanicIsItsFailure`
- **Break (2):** a task's panic is returned as an ordinary error, without the stack and not marked as a panic
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestAStartedGoroutineIsTheHosts`, `TestATaskPanicIsItsFailure`
- **Break (3):** a task's failure is logged and the process goes on serving without it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestATaskFailureStopsTheProcess`, `TestATaskPanicIsItsFailure`
- **Break (4):** the process stops without waiting for its tasks, so the pools close under a task still running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestATaskFailureStopsTheProcess`, `TestATaskPanicIsItsFailure`, `TestTheProcessStopsInOrder`

## A base edit landing between two accounts' reads of the policies is read again, and fails the load only when edits land through both reads

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a reload whose accounts read different base rules fails at once, so one base edit landing mid-reload pages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`
- **Break (2):** a reload reads once more for each disagreement, up to two more times, so edits landing through two reads no longer fail it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`

## A boolean is true or false in every layer, wherever it sits in a value

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and breaks (2), (3), (4), (5), (6), (7), (8) again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** a boolean an inline section declares is not checked in the file, so it takes the other words YAML reads as booleans
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn/a_YAML_word_for_true_in_an_inline_section,_in_the_file`
- **Break (2):** a boolean inside a list is not checked, so it takes the other words YAML reads as booleans
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_a_flag`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_environment`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list,_from_the_file`
- **Break (3):** a boolean inside a map is not checked, so it takes the other words YAML reads as booleans
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list,_from_the_file`
- **Break (4):** a boolean inside a section of a value is not checked, so it takes the other words YAML reads as booleans
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list,_from_the_file`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn/a_YAML_word_for_true_in_an_inline_section,_in_the_file`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_false_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_true_in_the_file`
- **Break (5):** a boolean refuses false as well as the other words
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/true_and_false_are_read`, `TestABooleanIsTrueOrFalse`, `TestABooleanIsTrueOrFalse/false`
- **Break (6):** a boolean in the configuration file is not checked, so it takes the other words YAML reads as booleans, such as yes and off
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list,_from_the_file`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn/a_YAML_word_for_true_in_an_inline_section,_in_the_file`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_false_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_true_in_the_file`
- **Break (7):** a boolean from the environment or a flag is not checked, so it takes the other words YAML reads as booleans, such as yes and off
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_a_flag`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_environment`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_true`
- **Break (8):** a boolean in any layer takes the other words YAML reads as booleans, such as yes and off
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_a_flag`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_environment`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list,_from_the_file`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn/a_YAML_word_for_true_in_an_inline_section,_in_the_file`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_false_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_true_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_true`

## A concurrency limit per job kind bounds its runs at once

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and its break again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break:** a run never takes a slot of its job's limit, so the limit bounds nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobsWaitForASlotIsReported`, `TestALimitBoundsTheRunsOfAJobKindAtOnce`

## A configuration file's keys are matched exactly against the declaration

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the file's keys are lower-cased before the strict decode, so a case-changed key sets the value it resembles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_case-changed_key`, `TestEveryMistakeIsRefusedAtStart/a_map_key_in_upper_case`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_hyphen`
- **Break (2):** the file is decoded without KnownFields, so a key the declaration does not hold is ignored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_case-changed_key`, `TestEveryMistakeIsRefusedAtStart/an_unknown_key`

## A configuration text holds exactly one document

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** a text holding no document is read as a document setting no value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_file_of_comments_alone`, `TestEveryMistakeIsRefusedAtStart/an_empty_environment_variable_for_a_value_other_than_a_string`
- **Break (2):** a second document is no longer looked for, so only the first is read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_second_document`, `TestEveryMistakeIsRefusedAtStart/an_empty_second_document`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_second_document`
- **Break (3):** the end of the text is taken for a second document, so every text of one document is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_a_flag`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_environment`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list,_from_the_file`, `TestABooleanInsideAValueIsTrueOrFalse/true_and_false_are_read`, `TestABooleanIsTrueOrFalse`, `TestABooleanIsTrueOrFalse/false`, `TestABooleanIsTrueOrFalse/true`, `TestAFileValueThatIsNotANull`, `TestAKeyedEntryTheFileNamesIsKept`, `TestAListFromALaterLayerReplacesTheWholeList`, `TestARequiredValueIsMetByAnyLayer`, `TestARequiredValueIsMetByAnyLayer/the_file`, `TestASectionsRevisionFollowsItsEffectiveValue`, `TestASectionsRevisionFollowsItsEffectiveValue/a_flag_changes_the_server`, `TestASectionsRevisionFollowsItsEffectiveValue/a_flag_changes_the_threshold`, `TestASectionsRevisionFollowsItsEffectiveValue/the_environment_adds_a_language`, `TestASectionsRevisionFollowsItsEffectiveValue/the_file_changes_a_trigger_word`, `TestASectionsRevisionFollowsItsEffectiveValue/the_file_restates_a_default`, `TestAValueFromTheEnvironmentOrAFlag`, `TestAValueFromTheEnvironmentOrAFlag/a_list_is_decoded_as_YAML`, `TestAValueFromTheEnvironmentOrAFlag/a_map_key_holds_single_underscores`, `TestAValueFromTheEnvironmentOrAFlag/the_language_code_no_is_a_key_and_a_word,_never_a_boolean`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestEachValueRecordsTheLayerThatSetIt`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_false_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_true_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_case-changed_key`, `TestEveryMistakeIsRefusedAtStart/a_flag_given_twice`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_flag_of_the_wrong_type`, `TestEveryMistakeIsRefusedAtStart/a_key_given_twice`, `TestEveryMistakeIsRefusedAtStart/a_key_with_no_value`, `TestEveryMistakeIsRefusedAtStart/a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_in_upper_case`, `TestEveryMistakeIsRefusedAtStart/a_map_key_starting_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_double_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_hyphen`, `TestEveryMistakeIsRefusedAtStart/a_merge_key_with_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_merge_key_without_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_null_document`, `TestEveryMistakeIsRefusedAtStart/a_null_in_a_list`, `TestEveryMistakeIsRefusedAtStart/a_null_section`, `TestEveryMistakeIsRefusedAtStart/a_required_value_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_a_flag_names`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_an_empty_keyed_entry_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_type_error`, `TestEveryMistakeIsRefusedAtStart/an_alias`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_list_with_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_of_the_wrong_type`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_set_twice`, `TestEveryMistakeIsRefusedAtStart/an_unknown_key`, `TestLayersApplyPerValueInOrder`, `TestTheFileIsReadOnlyWhenNamed`, `TestTheFileIsReadOnlyWhenNamed/the_environment_names_the_file`, `TestTheFileIsReadOnlyWhenNamed/the_flag's_file_wins_over_the_environment's`, `TestTwoConfigurationsShareNothing`

## A declaration the layers cannot name or set is refused before any layer is read

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and break (11) again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** a field's yaml tag is no longer checked, so an empty, upper-case or optioned tag is taken as its name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_field_with_no_tag`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_ending_in_an_underscore`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_in_upper_case`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_tag_with_an_option`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_inline_section_whose_own_field_cannot_be_named`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_inlined_field`
- **Break (2):** a field name holding an underscore is refused, so a name such as link_words cannot be declared
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanIsTrueOrFalse`, `TestABooleanIsTrueOrFalse/false`, `TestABooleanIsTrueOrFalse/true`, `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_an_inline_section_in_a_list_also_declares`, `TestADeclarationThatCannotBeConfiguredIsRefused/the_file's_own_name`, `TestAFileValueThatIsNotANull`, `TestAKeyedEntryTheFileNamesIsKept`, `TestAListFromALaterLayerReplacesTheWholeList`, `TestARequiredValueIsMetByAnyLayer`, `TestARequiredValueIsMetByAnyLayer/a_flag`, `TestARequiredValueIsMetByAnyLayer/the_environment`, `TestARequiredValueIsMetByAnyLayer/the_file`, `TestASectionsRevisionFollowsItsEffectiveValue`, `TestAValueFromTheEnvironmentOrAFlag`, `TestAValueFromTheEnvironmentOrAFlag/a_list_is_decoded_as_YAML`, `TestAValueFromTheEnvironmentOrAFlag/a_map_key_holds_single_underscores`, `TestAValueFromTheEnvironmentOrAFlag/a_string_takes_a_text_that_reads_as_a_null_in_YAML`, `TestAValueFromTheEnvironmentOrAFlag/a_string_takes_the_text_verbatim`, `TestAValueFromTheEnvironmentOrAFlag/an_empty_string_is_a_string`, `TestAValueFromTheEnvironmentOrAFlag/the_language_code_no_is_a_key_and_a_word,_never_a_boolean`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestEachValueRecordsTheLayerThatSetIt`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_false_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_YAML_word_for_true_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_case-changed_flag`, `TestEveryMistakeIsRefusedAtStart/a_case-changed_key`, `TestEveryMistakeIsRefusedAtStart/a_file_of_comments_alone`, `TestEveryMistakeIsRefusedAtStart/a_file_that_cannot_be_read`, `TestEveryMistakeIsRefusedAtStart/a_flag_given_twice`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_flag_naming_a_section`, `TestEveryMistakeIsRefusedAtStart/a_flag_naming_an_empty_file`, `TestEveryMistakeIsRefusedAtStart/a_flag_of_the_wrong_type`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_holding_a_double_underscore`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_in_upper_case`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_no_value`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_no_value,_last`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_one_dash`, `TestEveryMistakeIsRefusedAtStart/a_key_given_twice`, `TestEveryMistakeIsRefusedAtStart/a_key_with_no_value`, `TestEveryMistakeIsRefusedAtStart/a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_in_upper_case`, `TestEveryMistakeIsRefusedAtStart/a_map_key_starting_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_double_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_hyphen`, `TestEveryMistakeIsRefusedAtStart/a_merge_key_with_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_merge_key_without_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_null_document`, `TestEveryMistakeIsRefusedAtStart/a_null_in_a_list`, `TestEveryMistakeIsRefusedAtStart/a_null_section`, `TestEveryMistakeIsRefusedAtStart/a_required_value_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_a_flag_names`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_an_empty_keyed_entry_in_the_file`, `TestEveryMistakeIsRefusedAtStart/a_second_document`, `TestEveryMistakeIsRefusedAtStart/a_type_error`, `TestEveryMistakeIsRefusedAtStart/an_alias`, `TestEveryMistakeIsRefusedAtStart/an_argument_that_is_not_a_flag`, `TestEveryMistakeIsRefusedAtStart/an_empty_environment_variable_for_a_value_other_than_a_string`, `TestEveryMistakeIsRefusedAtStart/an_empty_second_document`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_list_with_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_second_document`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_in_mixed_case`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_naming_a_part_of_a_value`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_naming_a_section`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_naming_an_empty_file`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_of_the_wrong_type`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_set_twice`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_a_hyphenated_map_key`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_a_map_key_starting_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_the_prefix_in_lower_case`, `TestEveryMistakeIsRefusedAtStart/an_unknown_environment_variable_under_the_prefix`, `TestEveryMistakeIsRefusedAtStart/an_unknown_flag`, `TestEveryMistakeIsRefusedAtStart/an_unknown_key`, `TestHelpListsEveryValue`, `TestHelpListsEveryValue/--help`, `TestHelpListsEveryValue/-h`, `TestHelpQuotesAnEmptyOrMultiLineStringDefault`, `TestLayersApplyPerValueInOrder`, `TestTheFileIsReadOnlyWhenNamed`, `TestTheFileIsReadOnlyWhenNamed/no_file_is_named`, `TestTheFileIsReadOnlyWhenNamed/the_environment_names_the_file`, `TestTheFileIsReadOnlyWhenNamed/the_flag's_file_wins_over_the_environment's`, `TestTwoConfigurationsShareNothing`, `TestVersionIsRequestedBeforeAnyLayerIsRead`, `TestVersionIsRequestedBeforeAnyLayerIsRead/alone`, `TestVersionIsRequestedBeforeAnyLayerIsRead/beside_a_refused_flag`
- **Break (3):** the root may declare config_file, whose environment name is the file's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/the_file's_own_name`
- **Break (4):** a name an inline section declares beside the embedding section's own field of that name is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_an_inline_section_also_declares`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_an_inline_section_in_a_list_also_declares`
- **Break (5):** an inline field that is not a struct is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_inline_field_that_is_not_a_struct`
- **Break (6):** an inline field with a settings tag is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_required_inline_field`
- **Break (7):** an unexported inline field is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_unexported_inline_field`
- **Break (8):** a map whose keys are not strings is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_map_with_keys_that_are_not_strings`
- **Break (9):** a name declared by two fields is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_declared_twice`
- **Break (10):** a root type that is not a struct is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_root_that_is_not_a_struct`
- **Break (11):** a required value may have a default, which then counts as unset
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_required_value_with_a_default`
- **Break (12):** a section, or a field inside a list, may be required
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_required_field_inside_a_list`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_required_section`
- **Break (13):** a pointer, an interface and the other types no layer can set are taken as values
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_pointer`
- **Break (14):** an unexported field is no longer refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_unexported_field`
- **Break (15):** a settings tag other than required is ignored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_unknown_settings_tag`

## A deployable refuses to start while PGPASSWORD or PGSSLPASSWORD is set, an empty value included

- **Date · evidence:** 2026-09-28 · [pull request #181](https://github.com/ppat/mediated-mailbox-mcp/pull/181), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** a password variable set to the empty value does not refuse the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGSSLPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGSSLPASSWORD=`
- **Break (2):** every PG* variable refuses the start, not only the two password variables
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead`
- **Break (3):** PGPASSWORD does not refuse the start, so a stray one wins over the password file
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGPASSWORD=secret`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=secret`
- **Break (4):** PGSSLPASSWORD does not refuse the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGSSLPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGSSLPASSWORD=secret`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGSSLPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGSSLPASSWORD=secret`

## A failed job is asked again after a capped exponential backoff, which a success resets and a wake waits out

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a success leaves the count of failures as it was, so the backoff never resets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestASuccessResetsTheBackoff`
- **Break (2):** a wake is answered during a backoff, so a failing job woken often runs at the rate it is woken
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAWakeDuringABackoffWaitsForItsEnd`

## A failed write-back is held in the record's token and logged

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a write-back that failed is logged at debug level, which a process does not emit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`
- **Break (2):** a write-back that failed drops the record's token, so a reload makes it again from the stored credential and the rotation is lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`

## A goroutine a component starts through the host's starter is the host's, recovered as the component's failure, waited for before the pools close, and refused once the host waits

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a starter starts a goroutine after the host has waited for them, one that may run after the pools close
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestAStartedGoroutineIsTheHosts`
- **Break (2):** the host closes the pools without waiting for the goroutines its starters started
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestAStartedGoroutineIsTheHosts`
- **Break (3):** a started goroutine's panic is recovered and dropped, so the process serves on as if nothing failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestAStartedGoroutineIsTheHosts`

## A goroutine started through Go is the process's, its panic the failure of what started it, waited for before the slots close, and refused once the process waits

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the process closes its slots without waiting for the goroutines Go started
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAGoroutineStartedThroughGoIsTheProcesss`, `TestAProgramThatRunsOnceDoesItsWorkAndClosesItsSlots`, `TestASchedulerThatRefusesAPlanStopsTheProcessInOrder`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_application_server`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_operations_server`
- **Break (2):** Go starts a goroutine after the process has waited for them, one that may run after the slots close
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAGoroutineStartedThroughGoIsTheProcesss`
- **Break (3):** a goroutine's panic is recovered and dropped, so the process serves on as if nothing failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAGoroutineStartedThroughGoIsTheProcesss`

## A job kind reports not ready until its plan's first listing has succeeded

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** a job kind's first successful listing is never reported, so the process never answers ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAJobKindsPlanRunsOnTheScheduler`
- **Break (2):** a job kind is ready before any listing has succeeded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAJobKindsPlanRunsOnTheScheduler`

## A job's last success is a run that succeeded or a unit of work made durable, and its series follow every way a run ends

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (1) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (5) on 2026-10-10 at RAPID_SEED=1, added with the rule that a loop reports on its key only while it still holds it, with break (1) again, whose went-red list grew by that rule's test · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-11 at RAPID_SEED=1, after the performance measurements changed the code, tests or patches the row rests on · [pull request #341](https://github.com/ppat/mediated-mailbox-mcp/pull/341), and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a job's last success is not emitted until its first success, so a job that never succeeds never ages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobsLatestSuccessStartsFromItsRecordedOne`, `TestARemovedLoopReportsNothingOnTheJobEnsuredInItsPlace`, `TestTheJobSeriesFollowEveryWayARunEnds`, `TestTheRulesReadOnlyEmittedSeries`
- **Break (2):** a run that answers it was not due counts as a success
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestANotDueAnswerChangesNeitherFailuresNorSuccess`, `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (3):** a removed job's series stay, reading the time of its last success
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (4):** a unit of work a run makes durable records no success
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAUnitMadeDurableIsASuccess`, `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (5):** a removed loop reports on the key whatever job holds it, so a run that makes a unit durable as it stops sets the latest success of the job ensured in its place
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestARemovedLoopReportsNothingOnTheJobEnsuredInItsPlace`

## A length of time is read from text such as 5m and written back as such text, in every layer, the help, the effective values and the rendering a revision is taken from

- **Date · evidence:** 2026-11-17 · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), at RAPID_SEED=1
- **Break (1):** a type error in a section holding a length of time names the type the library read in its place, a struct written out field by field, rather than the declared type
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADurationIsReadAndWrittenAsText`
- **Break (2):** an effective config.Duration, alone or in a list or a map, is reported as its number of nanoseconds rather than its text
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADurationHoldsNanosecondsAndIsWrittenAsTheText`, `TestADurationIsReadAndWrittenAsText`
- **Break (3):** every length the library converts between the declared type and the one it reads and writes is divided by a thousand, so a config.Duration holds microseconds where it should hold nanoseconds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADurationHoldsNanosecondsAndIsWrittenAsTheText`, `TestADurationIsReadAndWrittenAsText`
- **Break (4):** a config.Duration is decoded as the number it is held as, so the text 5m is refused in every layer and a bare number is read as nanoseconds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADurationHoldsNanosecondsAndIsWrittenAsTheText`, `TestADurationIsReadAndWrittenAsText`
- **Break (5):** a config.Duration is written as its number of nanoseconds in the help and in the rendering a revision is taken from
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADurationHoldsNanosecondsAndIsWrittenAsTheText`, `TestADurationIsReadAndWrittenAsText`

## A listing that names an empty key fails, and keeps no job for any key it named

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1
- **Break (1):** an empty key is kept as any other, naming the listing's own job, so the listing succeeds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAListingThatNamesAnEmptyKeyFails`
- **Break (2):** an empty key is passed over, so the listing succeeds and keeps jobs for the other keys
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAListingThatNamesAnEmptyKeyFails`

## A load of the policies its caller cancelled keeps the active policy and raises no alarm

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a reload its caller cancelled is counted as a failed read, so a process shutting down mid-reload raises the alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAReloadItsCallerCancelledRaisesNoAlarm`
- **Break (2):** a reload whose deadline passed is taken as one its caller cancelled, so reloads that keep timing out raise no alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestEachReloadItsCallerDidNotCancelIsTimed`
- **Break (3):** every failed read is taken as a reload its caller cancelled, so no failed read raises the alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestEachReloadItsCallerDidNotCancelIsTimed`

## A map key is runs of lower-case letters and digits joined by single underscores

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** a map key may hold a double underscore, so its environment name splits into two segments
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_holding_a_double_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_double_underscore`
- **Break (2):** a map key may start or end with an underscore, so two key paths share one environment name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_starting_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_a_map_key_starting_with_an_underscore`
- **Break (3):** no map key is checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_default_key_outside_the_alphabet`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_holding_a_double_underscore`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_in_upper_case`, `TestEveryMistakeIsRefusedAtStart/a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_in_upper_case`, `TestEveryMistakeIsRefusedAtStart/a_map_key_starting_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_double_underscore`, `TestEveryMistakeIsRefusedAtStart/a_map_key_with_a_hyphen`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_a_hyphenated_map_key`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_a_map_key_starting_with_an_underscore`
- **Break (4):** a map key may not hold an underscore at all, so a key such as pt_br is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAValueFromTheEnvironmentOrAFlag`, `TestAValueFromTheEnvironmentOrAFlag/a_map_key_holds_single_underscores`

## A panic in one job is its run's failure and stops nothing else

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** schedule.Go starts its goroutine with no recovery, so a panic in a goroutine a run started stops the worker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPanicIsItsRunsFailureAndStopsNothingElse`, `TestAPanicIsItsRunsFailureAndStopsNothingElse/in_a_goroutine_it_started`
- **Break (2):** a run is called with no recovery, so its panic stops the worker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPanicIsItsRunsFailureAndStopsNothingElse`, `TestAPanicIsItsRunsFailureAndStopsNothingElse/in_the_run`, `TestTheJobSeriesFollowEveryWayARunEnds`

## A plan's listing exports the lateness bound its plan sets

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1
- **Break (1):** the listing's job is given no lateness bound, so it exports zero and counts as late at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlansListingExportsItsLatenessBound`
- **Break (2):** the listing's lateness bound is its recheck interval, not the bound the plan sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlansListingExportsItsLatenessBound`

## A plan's listing keeps one job per key it lists, tells a dropped key only once its job has returned, and tells its first success once

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1
- **Break (1):** a dropped key is told to the plan before its job is removed and has returned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlansListingAddsAndDropsJobsByKey`
- **Break (2):** the job of a key the listing no longer finds is not removed, so it keeps running
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlansListingAddsAndDropsJobsByKey`
- **Break (3):** the first listing that succeeds is never told, so the job kind never reads as ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlansListingAddsAndDropsJobsByKey`
- **Break (4):** every listing that succeeds is told as the first
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlansListingAddsAndDropsJobsByKey`

## A process answers ready only while it serves and every readiness check of every component it hosts passes

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the readiness probe answers ready without running any component's check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheOperationsEndpoints`
- **Break (2):** the readiness probe runs the first component's checks alone, so a later component that is not ready leaves the process ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheOperationsEndpoints`
- **Break (3):** the readiness probe answers ready while the process stops
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheProcessStopsInOrder`

## A process holds one Session, built from the process with its own capabilities, its reload unobserved by the scheduler's series

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the Session is built with the capabilities of the part that asked first, so its series carry that part's job kind
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestTheSessionIsOnePerProcessBuiltFromTheProcess`
- **Break (2):** each part that asks gets a Session built for it, so the process holds more than one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestAFailedReloadKeepsWhatItCouldNotRead`, `TestEachReloadItsCallerDidNotCancelIsTimed`, `TestOnceLoadedAFailedReloadWaitsForTheNextInterval`, `TestTheSessionIsOnePerProcessBuiltFromTheProcess`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything/the_list`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything/the_policy`, `TestWithoutConnectionsNoAccountIsWorkable`

## A process refuses the environment the database driver and every component it hosts refuse, before its configuration is read

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and break 1 again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the refusals a component hands the host are never run, so the API server starts under MCPGODEBUG
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestMCPGODEBUGStopsTheStart`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/COMPONENTDEBUG=`
- **Break (2):** the process starts whatever PGPASSWORD and PGSSLPASSWORD hold, whichever way it runs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=s3cret`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/PGSSLPASSWORD=`
- **Break (3):** the refusals run after the configuration is read, so a start whose environment sets a refused variable reads its configuration first and reports its file's error instead
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/COMPONENTDEBUG=`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=s3cret`, `TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead/until_stopped/PGSSLPASSWORD=`

## A process serves its application server over TLS when its configuration names a key pair and over plain HTTP when it names neither, with the key pair read from its mounted files, and refuses to start with one of the two or with a pair that does not load

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a TLS file path of spaces counts as set when the configuration is validated, so a certificate beside a blank key path is accepted and two blank paths refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheServingPartIsChecked`, `TestTheServingPartIsChecked/a_certificate_with_a_blank_key`, `TestTheServingPartIsChecked/a_key_with_a_blank_certificate`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheProcessSectionsAreValidated`, `TestTheProcessSectionsAreValidated/a_certificate_with_a_blank_key`, `TestTheProcessSectionsAreValidated/a_key_with_a_blank_certificate`
- **Break (2):** a TLS path of spaces counts as set when the process chooses its transport, so a process given two blank paths tries TLS and never serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheServingPartIsChecked`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheApplicationServerGivenNoKeyPairIsPlain`, `TestTheApplicationServerGivenNoKeyPairIsPlain/both_blank`
- **Break (3):** the key pair is loaded whatever the configuration names, so a process given neither TLS file refuses to start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestEachHandlerIsServedWhereItIsMounted`, `TestTheApplicationServerGivenNoKeyPairIsPlain`, `TestTheApplicationServerGivenNoKeyPairIsPlain/both_blank`, `TestTheApplicationServerGivenNoKeyPairIsPlain/neither_file`, `TestTheOperationsEndpoints`, `TestTheProcessStopsInOrder`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`
- **Break (4):** the key pair is not loaded before the process serves, so a process whose mounts cannot serve starts and answers no handshake
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestAKeyPairThatDoesNotLoadRefusesTheStart`, `TestAKeyPairThatDoesNotLoadRefusesTheStart/a_certificate_that_is_absent`, `TestAKeyPairThatDoesNotLoadRefusesTheStart/a_key_that_does_not_match`, `TestTheStartBannerGoesToStandardError`
- **Break (5):** the key pair is read once, when the server is built, so a rotated pair never serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheApplicationServerServesTLSWhenGivenAKeyPair`
- **Break (6):** the application server is served over TLS even when the configuration names no key pair, so a process given neither file never serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestEachHandlerIsServedWhereItIsMounted`, `TestTheApplicationServerGivenNoKeyPairIsPlain`, `TestTheApplicationServerGivenNoKeyPairIsPlain/both_blank`, `TestTheApplicationServerGivenNoKeyPairIsPlain/neither_file`, `TestTheOperationsEndpoints`, `TestTheProcessStopsInOrder`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`
- **Break (7):** a configuration naming one TLS file without the other is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/config`:** `TestTheAPIConfigurationIsChecked`, `TestTheAPIConfigurationIsChecked/one_TLS_file`, `TestTheServingPartIsChecked`, `TestTheServingPartIsChecked/a_certificate_with_a_blank_key`, `TestTheServingPartIsChecked/a_certificate_without_its_key`, `TestTheServingPartIsChecked/a_key_with_a_blank_certificate`, `TestTheServingPartIsChecked/a_key_without_its_certificate`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheProcessSectionsAreValidated`, `TestTheProcessSectionsAreValidated/a_certificate_with_a_blank_key`, `TestTheProcessSectionsAreValidated/a_certificate_without_its_key`, `TestTheProcessSectionsAreValidated/a_key_with_a_blank_certificate`, `TestTheProcessSectionsAreValidated/a_key_without_its_certificate`
- **Break (8):** the application server is built without its TLS configuration, so a process given both TLS files never answers a handshake
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheApplicationServerServesTLSWhenGivenAKeyPair`

## A process starts in order, refusing the password variables before it reads its configuration, logging every value before it sets the level, and running the configuration's checks before Init

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** Init runs before the configuration's checks, so a start check sees a configuration the checks refuse
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/once/the_configuration's_checks`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/until_stopped/the_configuration's_checks`
- **Break (2):** the level is set before the effective configuration is logged, so under warn no value is logged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/once/a_level_naming_none,_INFO`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/once/a_level_naming_none,_info+2`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/once/a_refused_value_under_warn`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/once/warn`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/until_stopped/a_level_naming_none,_INFO`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/until_stopped/a_level_naming_none,_info+2`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/until_stopped/a_refused_value_under_warn`, `TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks/until_stopped/warn`
- **Break (3):** the password variables are refused after the configuration is read, so a start whose configuration cannot be read reports that error instead
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGPASSWORD=secret`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGSSLPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/once/PGSSLPASSWORD=secret`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGPASSWORD=secret`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGSSLPASSWORD=`, `TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead/until_stopped/PGSSLPASSWORD=secret`

## A process that runs until it is told to stop checks the schema on each pool it opens before handing it out, and refuses a root that hands no check

- **Date · evidence:** 2026-11-12 · [pull request #421](https://github.com/ppat/mediated-mailbox-mcp/pull/421), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the check the root hands is never given to the host that opens the pools, so Run opens every pool unchecked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestEveryPoolIsCheckedBeforeItIsHandedOut`, `TestEveryPoolIsCheckedBeforeItIsHandedOut/every_pool_passes`, `TestEveryPoolIsCheckedBeforeItIsHandedOut/the_second_pool's_schema_is_refused`
- **Break (2):** a root that hands no schema check is run, so its pools open unchecked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestASpecWithoutASchemaCheckIsRefused`
- **Break (3):** a refused schema is logged and its pool handed out, so the process goes on to serve
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestEveryPoolIsCheckedBeforeItIsHandedOut`, `TestEveryPoolIsCheckedBeforeItIsHandedOut/the_second_pool's_schema_is_refused`
- **Break (4):** no pool's schema is checked, so a process starts on a schema it cannot run on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestEveryPoolIsCheckedBeforeItIsHandedOut`, `TestEveryPoolIsCheckedBeforeItIsHandedOut/every_pool_passes`, `TestEveryPoolIsCheckedBeforeItIsHandedOut/the_second_pool's_schema_is_refused`

## A process's servers close an idle connection and cut off a request that has not arrived within their limits, and bound no answer

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the servers bound writing an answer by the read limit, which cuts off a request waiting for its leases and an event stream
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheConnectionLimits`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`
- **Break (2):** the servers set no idle limit, so an idle keep-alive connection is held until the read limit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheConnectionLimits`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`
- **Break (3):** the operations server is built apart from the shared constructor, with only a header limit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`
- **Break (4):** the servers set no read limit, so a client that stops sending its body holds its connection
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheConnectionLimits`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`

## A program that runs once does its work with capabilities carrying its name, and returns once every goroutine the work started has returned and its slots are closed

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the task's logger carries no component, unlike every other part's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAProgramThatRunsOnceDoesItsWorkAndClosesItsSlots`
- **Break (2):** a program that runs once closes its slots before the goroutines its work started have returned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAProgramThatRunsOnceDoesItsWorkAndClosesItsSlots`
- **Break (3):** a program that runs once returns once its goroutines have returned without closing its slots
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAProgramThatRunsOnceDoesItsWorkAndClosesItsSlots`

## A read of the list of accounts that fails is a failure, never a list with no account

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a read that lists no account returns no list, as a failed read does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/accounts`:** `TestAReadThatFailsIsNeverAListWithNoAccount`
- **Break (2):** a read that fails returns the empty list and no error, so the Session would put no account in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/accounts`:** `TestAReadThatFailsIsNeverAListWithNoAccount`

## A read of the policy tables the policies cannot trust never replaces the active policy

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the base rules are taken from the first account's read without comparing the others', so accounts are composed from different base policies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`
- **Break (2):** a read that found no rules is taken as a failed read, so a policy with no rules read whole never takes effect
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAnEmptyPolicyReadWholeIsAccepted`
- **Break (3):** a read that failed is taken as a whole read that found no rules, so the policy with no rules replaces the active one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestEachReloadItsCallerDidNotCancelIsTimed`, `TestTheRulesReadOnlyEmittedSeries`
- **Break (4):** a read that fails for one account keeps the rows read before it, so the accounts read earlier replace the active policy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestEachReloadItsCallerDidNotCancelIsTimed`, `TestTheRulesReadOnlyEmittedSeries`

## A read the connections cannot trust never replaces what is in force

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a read of the OAuth clients that fails is taken as a read that found none, so the reload goes on and puts its list in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReloadConnectsWhatItReadAndKeepsWhatWasOnAFailedRead`
- **Break (2):** a read that fails drops every connection, as if none had ever loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReloadConnectsWhatItReadAndKeepsWhatWasOnAFailedRead`

## A refused credential is read again, and the call is made once more only over another credential, through the client the account's row names, with the record changed in place

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a re-read after a refusal never gives a token, so a credential the operator replaced is not used
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReadAgainMakesTheTokenFromTheClientTheAccountMovedTo`, `TestAStoredCredentialSomeoneElseReplacedWins`
- **Break (2):** a re-read that finds the refused credential gives a token over it, so the provider is called once more with what it refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReadAgainMakesTheTokenFromTheClientTheAccountMovedTo`
- **Break (3):** a token made over a credential read again serves the unit alone, so the units after it call with the refused one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReadAgainMakesTheTokenFromTheClientTheAccountMovedTo`, `TestAStoredCredentialSomeoneElseReplacedWins`

## A reload keeps an account's token while its own current credential equals the stored one and its client is unchanged

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a token is kept whatever client the account now connects through, so an account whose client changed calls through the old one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestATokenServesItsAccountUntilItsCredentialOrClientChanges`
- **Break (2):** every reload makes each account's token again, so a token the provider refreshed is thrown away
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReloadDuringAWriteBackWaitsForIt`, `TestATokenServesItsAccountUntilItsCredentialOrClientChanges`, `TestOverlappingRotationsEachLand`, `TestTheSameCredentialStoredUnderNewBytesKeepsTheToken`
- **Break (3):** a stored value the record did not know makes the token again even when it opens to the credential the token holds now
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestTheSameCredentialStoredUnderNewBytesKeepsTheToken`

## A required value no layer sets is refused

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** an entry of a keyed map the defaults name with nothing set in it is dropped, so its required values are never demanded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAnEmptyKeyedEntryOfTheDefaultsIsKept`, `TestAnEmptyKeyedEntryOfTheDefaultsIsKept/an_empty_map_entry_is_kept`, `TestAnEmptyKeyedEntryOfTheDefaultsIsKept/its_required_values_are_demanded`
- **Break (2):** an entry of a keyed map the file names with nothing set in it is dropped, so its required values are never demanded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_an_empty_keyed_entry_in_the_file`
- **Break (3):** a required value counts as set only when the file sets it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanIsTrueOrFalse`, `TestABooleanIsTrueOrFalse/false`, `TestABooleanIsTrueOrFalse/true`, `TestAListFromALaterLayerReplacesTheWholeList`, `TestARequiredValueIsMetByAnyLayer`, `TestARequiredValueIsMetByAnyLayer/a_flag`, `TestARequiredValueIsMetByAnyLayer/the_environment`, `TestASectionsRevisionFollowsItsEffectiveValue`, `TestAValueFromTheEnvironmentOrAFlag`, `TestAValueFromTheEnvironmentOrAFlag/a_list_is_decoded_as_YAML`, `TestAValueFromTheEnvironmentOrAFlag/a_map_key_holds_single_underscores`, `TestAValueFromTheEnvironmentOrAFlag/a_string_takes_a_text_that_reads_as_a_null_in_YAML`, `TestAValueFromTheEnvironmentOrAFlag/a_string_takes_the_text_verbatim`, `TestAValueFromTheEnvironmentOrAFlag/an_empty_string_is_a_string`, `TestAValueFromTheEnvironmentOrAFlag/the_language_code_no_is_a_key_and_a_word,_never_a_boolean`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestEachValueRecordsTheLayerThatSetIt`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_a_flag_names`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_no_layer_sets`, `TestLayersApplyPerValueInOrder`, `TestTheFileIsReadOnlyWhenNamed`, `TestTheFileIsReadOnlyWhenNamed/no_file_is_named`, `TestTheFileIsReadOnlyWhenNamed/the_environment_names_the_file`, `TestTheFileIsReadOnlyWhenNamed/the_flag's_file_wins_over_the_environment's`, `TestTwoConfigurationsShareNothing`
- **Break (4):** the required values of a keyed map's entries are not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAnEmptyKeyedEntryOfTheDefaultsIsKept`, `TestAnEmptyKeyedEntryOfTheDefaultsIsKept/its_required_values_are_demanded`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_a_flag_names`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_an_empty_keyed_entry_in_the_file`
- **Break (5):** no required value is checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAnEmptyKeyedEntryOfTheDefaultsIsKept`, `TestAnEmptyKeyedEntryOfTheDefaultsIsKept/its_required_values_are_demanded`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn/a_required_value_of_an_inline_section_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_required_value_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_a_flag_names`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_a_keyed_entry_no_layer_sets`, `TestEveryMistakeIsRefusedAtStart/a_required_value_of_an_empty_keyed_entry_in_the_file`

## A section's revision follows its effective value, whichever layer set it

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** each revision is taken over the section's defaults, so a change in a later layer leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestASectionsRevisionFollowsItsEffectiveValue`, `TestASectionsRevisionFollowsItsEffectiveValue/a_flag_changes_the_server`, `TestASectionsRevisionFollowsItsEffectiveValue/a_flag_changes_the_threshold`, `TestASectionsRevisionFollowsItsEffectiveValue/the_environment_adds_a_language`, `TestASectionsRevisionFollowsItsEffectiveValue/the_file_changes_a_trigger_word`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`
- **Break (2):** each revision is taken over the section's values with the layer that set each, so restating a default in the file changes it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADurationIsReadAndWrittenAsText`, `TestASectionsRevisionFollowsItsEffectiveValue`, `TestASectionsRevisionFollowsItsEffectiveValue/the_file_restates_a_default`
- **Break (3):** each revision is taken over the whole configuration, so a change to one section changes every revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestASectionsRevisionFollowsItsEffectiveValue`, `TestASectionsRevisionFollowsItsEffectiveValue/a_flag_changes_the_server`, `TestASectionsRevisionFollowsItsEffectiveValue/a_flag_changes_the_threshold`, `TestASectionsRevisionFollowsItsEffectiveValue/the_environment_adds_a_language`, `TestASectionsRevisionFollowsItsEffectiveValue/the_file_changes_a_trigger_word`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`

## A server that fails while the process serves stops the process in order, with an error naming the server

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** a server's failure is not waited on, so the process serves on until it is told to stop
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAServerThatFailsWhileServingStopsTheProcessInOrder`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_application_server`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_operations_server`
- **Break (2):** a server's failure returns at once without the stop, so the goroutines started through Go are neither told to stop nor waited for
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAServerThatFailsWhileServingStopsTheProcessInOrder`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_application_server`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_operations_server`
- **Break (3):** a server's failure is returned as it came, without naming the server that failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAServerThatFailsWhileServingStopsTheProcessInOrder`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_application_server`, `TestAServerThatFailsWhileServingStopsTheProcessInOrder/the_operations_server`

## A shared thing reads the revision of its settings from the process

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the revision of every key reads empty
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestARevisionIsReadFromTheProcess`
- **Break (2):** the revision read is the process part's whatever key is asked for, so a change to the key's own value leaves it as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestARevisionIsReadFromTheProcess`

## A shared thing's upkeep runs on the process's scheduler, and the scheduler's series observe none of it

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a process whose scheduler runs only upkeep registers the scheduler's series
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved`, `TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved/endpoints_alone`
- **Break (2):** the scheduler is built and the upkeep jobs are never ensured, so no upkeep runs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved`, `TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved/beside_a_job_kind`, `TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved/endpoints_alone`
- **Break (3):** the scheduler's series observe an upkeep job as a job kind of its owner's name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved`, `TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved/beside_a_job_kind`

## A slot holds one thing, built once from the process's own capabilities, and a failed build keeps nothing

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1
- **Break (1):** the slot builds its thing again on every call, so the second build registers its series again and the start fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASlotHoldsOneThingBuiltFromTheProcess`, `TestASlotHoldsOneThingBuiltFromTheProcess/a_job_kind_asks_first`, `TestASlotHoldsOneThingBuiltFromTheProcess/an_endpoint_asks_first`
- **Break (2):** the slot's thing is built with the capabilities of the part that asked first, so its series carry that part's job kind
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASlotHoldsOneThingBuiltFromTheProcess`, `TestASlotHoldsOneThingBuiltFromTheProcess/a_job_kind_asks_first`
- **Break (3):** a build that fails is kept as built, so every later call returns the failed build's empty thing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASlotHoldsOneThingBuiltFromTheProcess`

## A spec that cannot run is refused before anything is built

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** a spec of endpoints whose configuration has no serving part is built and served on an address no configuration names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASpecThatCannotRunIsRefused`
- **Break (2):** a spec that names no program is accepted and run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASpecThatCannotRunIsRefused`
- **Break (3):** a spec that lists no endpoint, no job kind and no task is accepted, so the process serves its probes and runs nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASpecThatCannotRunIsRefused`
- **Break (4):** a task beside job kinds is accepted and run as a task
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASpecThatCannotRunIsRefused`

## A stopped worker and a job whose last success has aged are loud

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and every break again on 2026-10-12, after the rules file moved under `packaging/chart/rules/` · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the provider library and the credential family moved the code, tests or patches the row rests on · [pull request #450](https://github.com/ppat/mediated-mailbox-mcp/pull/450), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the absence rule fires at the first evaluation that finds no worker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAlertingRules`
- **Break (2):** the rule on a job's last success compares it with a fixed fifteen minutes, not the bound the job exports
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAlertingRules`

## A stopping process answers not ready, ends Draining, drains its requests, and only then ends its jobs and goroutines

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** Draining does not end when the process starts to stop, so an event stream holds the drain until it times out
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProcessStopsInOrder`
- **Break (2):** the jobs' and goroutines' context ends before the requests in flight have drained
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProcessStopsInOrder`
- **Break (3):** the readiness probe answers ready while the process stops
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProcessStopsInOrder`

## A stopping process ends what draining components listen for, drains its requests, and only then stops its tasks

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** draining never ends, so an event stream holds the stop until its drain times out
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheProcessStopsInOrder`
- **Break (2):** the tasks are told to stop before the requests in flight have drained
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheProcessStopsInOrder`

## A timed job is asked on its interval's phase, and a job with no interval only when woken

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a job with no interval is asked again at once after every run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobWithNoIntervalIsAskedOnlyWhenWoken`, `TestAUnitMadeDurableIsASuccess`
- **Break (2):** a run moves the phase to its own start, so after a run that outlasts the interval the phase never resumes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestATimedJobIsAskedOnTheIntervalsPhase`
- **Break (3):** an ask carries its own time as its time on the ticker's phase, so an ask at the end of a backoff or at a wake names a time off the phase
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestEachAskCarriesItsTimeOnThePhase`

## A unit of work's end writes back only while the record's adoption stamp is the one it read with the credential, and the process's own write-back leaves the stamp as it is

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** an adopted value leaves the record's stamp as it was, so an end that read before the adoption writes its credential over the operator's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAnAdoptionBetweenAnEndsReadAndItsWriteDiscardsTheWriteBack`
- **Break (2):** the process's own write-back gives the record a new stamp, so an end that overlapped another's is discarded and its rotation lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReloadDuringAWriteBackWaitsForIt`, `TestOverlappingRotationsEachLand`, `TestTwoEndsWhoseWritesInterleaveBothLand`
- **Break (3):** the end writes back whatever the record's stamp, so an adoption between its read and its write is overwritten by the credential read before it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAnAdoptionBetweenAnEndsReadAndItsWriteDiscardsTheWriteBack`

## A value from the environment or a flag decodes as its field's type, and a string takes the text verbatim

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** a value from the environment or a flag that does not decode is taken as the zero value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_flag_of_the_wrong_type`, `TestEveryMistakeIsRefusedAtStart/an_empty_environment_variable_for_a_value_other_than_a_string`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_list_with_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_second_document`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_of_the_wrong_type`
- **Break (2):** a string value is decoded as YAML like any other, so its text is no longer taken verbatim
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAValueFromTheEnvironmentOrAFlag`, `TestAValueFromTheEnvironmentOrAFlag/a_string_takes_a_text_that_reads_as_a_null_in_YAML`, `TestAValueFromTheEnvironmentOrAFlag/a_string_takes_the_text_verbatim`, `TestAValueFromTheEnvironmentOrAFlag/an_empty_string_is_a_string`

## A value the connections cannot open leaves its account not connected or the accounts naming its client without one, and the refusal is logged

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a credential that does not open leaves its account not connected without a log record
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`
- **Break (2):** a client secret that does not open is served as an empty secret, so the accounts naming the client are connected
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReSealNeverPutsBackAReplacedSecret`, `TestAReadAgainMakesTheTokenFromTheClientTheAccountMovedTo`, `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`

## An account is connected through the OAuth client it names, and an account that names none through none

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** an account that names no client is paired with any client that opened, as a lookup by provider would
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** a credential read again after a refusal is paired with the client the record held, not the one the account's row names now
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReadAgainMakesTheTokenFromTheClientTheAccountMovedTo`

## An account is workable only once it is listed, connected and its policy loaded

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** an account in force with a policy is held workable whether or not it has a connection
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`
- **Break (2):** an account in force is held as having a policy whether or not a load read its rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestAFailedReloadKeepsWhatItCouldNotRead`

## An account of a provider that authenticates through an OAuth client is not connected without one, and one whose provider needs none is

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** an account of a provider that authenticates through a client is connected though it has none that opened
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReSealNeverPutsBackAReplacedSecret`, `TestAReadAgainMakesTheTokenFromTheClientTheAccountMovedTo`, `TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestEachAccountIsLoadedWithTheClientItNamesAndNoOther`
- **Break (2):** every provider is taken to authenticate through a client, so an account whose provider needs none is not connected
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestAReSealAndAnEndWhoseWritesInterleaveBothLand`, `TestAReloadConnectsWhatItReadAndKeepsWhatWasOnAFailedRead`, `TestAReloadDuringAWriteBackWaitsForIt`, `TestARotatedCredentialSurvivesARestart`, `TestAStaleReSealIsRefused`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`, `TestAStoredCredentialSomeoneElseReplacedWins`, `TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected`, `TestAnAdoptionBetweenAnEndsReadAndItsWriteDiscardsTheWriteBack`, `TestAnEndAfterAnAdoptionLeavesTheOperatorsCredential`, `TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne`, `TestEachAccountLeasesUnderItsLoweredTarget`, `TestNoUnitOfWorkBeginsOnAnAccountNotInForce`, `TestOverlappingRotationsEachLand`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`, `TestTheEndRecordsTheTokensLatestAttempt`, `TestTheSameCredentialStoredUnderNewBytesKeepsTheToken`, `TestTwoEndsWhoseWritesInterleaveBothLand`, `TestUnitsOfWorkThroughTwoCallersShareTheAccountsToken`

## An account whose own rules the policies did not read is held to the policy that restricts every sender

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the accounts the policies read are held to the policy that restricts every sender
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestEachLoadReadsTheAccountsItNames`, `TestEachReloadPreparesThePolicyItSwapsIn`, `TestTheLoaderComposesEachAccountsPolicy`, `TestTheSharedLoadReadsARuleWrittenSinceTheLastLoad`
- **Break (2):** an account the policies did not read gets the policy of an account it read, without the restrictions its own rules add
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestEachLoadReadsTheAccountsItNames`, `TestTheLoaderComposesEachAccountsPolicy`

## An explicit null, an alias or a merge key is refused in every layer

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** an alias is no longer refused, so it repeats the anchored value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_an_alias`, `TestEveryMistakeIsRefusedAtStart/an_alias`
- **Break (2):** a merge key is no longer refused, so it sets the values of the mapping it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_merge_key_with_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_merge_key_without_an_alias`
- **Break (3):** an explicit null is no longer refused, so it decodes as the zero value
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/a_key_with_no_value`, `TestEveryMistakeIsRefusedAtStart/a_null_document`, `TestEveryMistakeIsRefusedAtStart/a_null_in_a_list`, `TestEveryMistakeIsRefusedAtStart/a_null_section`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_list_with_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_null`
- **Break (4):** a null is told by its empty text rather than its tag, so ~ and null pass and an empty quoted string is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAFileValueThatIsNotANull`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/a_null_document`, `TestEveryMistakeIsRefusedAtStart/a_null_in_a_list`, `TestEveryMistakeIsRefusedAtStart/a_null_section`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_list_with_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_null`

## An inline section's values sit at the paths of the section that embeds it

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1
- **Break (1):** an inline section's fields are reached at their index in the inline section, taken as the embedding section's own, so a value lands in another field
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`
- **Break (2):** the inline tag is read as a name, so a root composed from inline sections is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADeclarationThatCannotBeConfiguredIsRefused`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_an_inline_section_also_declares`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_name_an_inline_section_in_a_list_also_declares`, `TestADeclarationThatCannotBeConfiguredIsRefused/a_required_inline_field`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_inline_field_that_is_not_a_struct`, `TestADeclarationThatCannotBeConfiguredIsRefused/an_inline_section_whose_own_field_cannot_be_named`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`

## At the end of a unit of work a rotated credential is written back by compare-and-set on the stored bytes the record knew, and an unchanged one is not

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the end writes back the token the unit began with under the bytes the record now holds, so a unit begun before an adoption puts its credential back over the operator's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAnEndAfterAnAdoptionLeavesTheOperatorsCredential`
- **Break (2):** a write-back whose compare-and-set finds other bytes keeps the record's credential rather than reading the stored one again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`
- **Break (3):** a credential equal to the one last read or written is written back too
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestARotatedCredentialSurvivesARestart`, `TestAnEndAfterAnAdoptionLeavesTheOperatorsCredential`, `TestTheEndRecordsTheTokensLatestAttempt`

## Both of a process's servers close an idle connection and cut off a request that has not arrived within their limits, and bound no answer

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the servers bound writing an answer by the read limit, which cuts off a request waiting for its leases and an event stream
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheConnectionLimits`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`
- **Break (2):** the servers set no idle limit, so an idle keep-alive connection is held until the read limit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheConnectionLimits`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`
- **Break (3):** the operations server is built apart from the shared constructor, with no limit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`

## Delta sync's re-seal moves each credential and client secret to the current key by compare-and-set, and a value someone replaced is read again

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the re-seal leaves every client secret on its old key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReSealNeverPutsBackAReplacedSecret`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** a re-seal that landed leaves the record on the old bytes, so the scan reads true and the next write-back compares against bytes the row no longer holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReSealAndAnEndWhoseWritesInterleaveBothLand`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (3):** a client secret's re-seal that lost to a replacement keeps the old secret in every account's token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReSealNeverPutsBackAReplacedSecret`
- **Break (4):** a re-seal that lost to a replacement keeps the record's old value rather than reading the stored one again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAStaleReSealIsRefused`

## Each account's units of work lease under the lowered target its state row held at the latest reload

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a unit of work leases at the default target whatever the account's state row holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestEachAccountLeasesUnderItsLoweredTarget`
- **Break (2):** an account's lowered target is read at the first reload alone, so one the operator changes never applies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestEachAccountLeasesUnderItsLoweredTarget`

## Each component's handler is served on the application server at the patterns its root gives and nowhere else

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** only a component's first pattern is mounted, so the API server would answer its API root and never its MCP root
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestEachHandlerIsServedWhereItIsMounted`
- **Break (2):** each handler is mounted at a pattern made from its component's name rather than at the patterns its root gives
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestEachHandlerIsServedWhereItIsMounted`, `TestTheApplicationServerGivenNoKeyPairIsPlain`, `TestTheApplicationServerGivenNoKeyPairIsPlain/both_blank`, `TestTheApplicationServerGivenNoKeyPairIsPlain/neither_file`, `TestTheApplicationServerServesTLSWhenGivenAKeyPair`, `TestTheProcessStopsInOrder`, `TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer`

## Each deployable logs at the level its configuration sets

- **Date · evidence:** 2026-10-08 · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and break 1 again on 2026-10-10, after the worker merged backfill and delta sync into one deployable with one entry, which leaves one patch where each of the two had its own · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-11 at RAPID_SEED=1, after the performance measurements changed the code, tests or patches the row rests on · [pull request #341](https://github.com/ppat/mediated-mailbox-mcp/pull/341), and break 4 again on 2026-10-11 at RAPID_SEED=1, after the UI took the credential section in place of its own key paths, which changed the code and tests the row rests on · [pull request #349](https://github.com/ppat/mediated-mailbox-mcp/pull/349), and breaks 2 and 4 again on 2026-10-11 at RAPID_SEED=1, after the transport taken from the TLS files changed the code, tests or patches the row rests on · [pull request #352](https://github.com/ppat/mediated-mailbox-mcp/pull/352), and breaks 2 and 4 again on 2026-10-11 at RAPID_SEED=1, after the transport tests came to drive each deployable's start and read its serving record, which changed the tests the row rests on · [pull request #352](https://github.com/ppat/mediated-mailbox-mcp/pull/352), and breaks 1, 2 and 4 again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and breaks (1) and (2) again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and breaks (1) and (2) again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and breaks (1) and (2) again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the host sets the level from the default log_level, so the level the configuration names never applies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/a_refused_value_at_error`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/an_offset`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/warn`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/warning`
- **Break (2):** the host reads log_level and never sets the level variable, so the logger stays at info
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/a_refused_value_at_error`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/warn`
- **Break (3):** the logger is built without the level variable, so it writes at info whatever the variable holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/logging`:** `TestAServersErrorLogWritesAtWarn`, `TestTheLoggerWritesJSONAtTheLevelItsVariableHolds`

## Each deployable's start log records which transport it serves

- **Date · evidence:** 2026-10-11 · [pull request #352](https://github.com/ppat/mediated-mailbox-mcp/pull/352), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the serving record says the process serves TLS whatever transport it took
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheApplicationServerGivenNoKeyPairIsPlain`, `TestTheApplicationServerGivenNoKeyPairIsPlain/both_blank`, `TestTheApplicationServerGivenNoKeyPairIsPlain/neither_file`
- **Break (2):** the serving record carries no transport
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheApplicationServerGivenNoKeyPairIsPlain`, `TestTheApplicationServerGivenNoKeyPairIsPlain/both_blank`, `TestTheApplicationServerGivenNoKeyPairIsPlain/neither_file`, `TestTheApplicationServerServesTLSWhenGivenAKeyPair`

## Each endpoint's handler is served at its pattern and nowhere else

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** only the first endpoint is mounted, so every later endpoint's pattern is not found
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestEachEndpointIsServedAtItsPattern`
- **Break (2):** every pattern is served by the first endpoint's handler
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestEachEndpointIsServedAtItsPattern`

## Each job kind has a pool of its own and the process one more, every pool closed with the slots

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1
- **Break (1):** a job kind's pool registers its series on the process's registerer, so the second job kind's pool collides with the first and the start fails
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestEachJobKindHasAPoolOfItsOwnAndTheProcessOneMore`
- **Break (2):** every part shares one pool, so one job kind can starve another of connections
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestEachJobKindHasAPoolOfItsOwnAndTheProcessOneMore`
- **Break (3):** the pools are not closed with the slots, so they outlive the process, the migrator's task's included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestEachJobKindHasAPoolOfItsOwnAndTheProcessOneMore`, `TestGetChecksTheSchemaAndForMigrationDoesNot`

## Each load of the policies prepares the policy it swaps in for the classifier

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break:** a reload keeps the prepared policies of the snapshot it replaces, so an edit it loads never reaches the classifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestEachLoadReadsTheAccountsItNames`, `TestEachReloadPreparesThePolicyItSwapsIn`, `TestTheLoaderComposesEachAccountsPolicy`, `TestTheSharedLoadReadsARuleWrittenSinceTheLastLoad`

## Each reload of the Session its caller did not cancel is timed

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a reload its caller cancelled is timed too
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestEachReloadItsCallerDidNotCancelIsTimed`
- **Break (2):** a reload that fails is not timed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestEachReloadItsCallerDidNotCancelIsTimed`

## Every argument is a flag naming a declared value, once, with a value

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** an argument that is not a flag is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_one_dash`, `TestEveryMistakeIsRefusedAtStart/an_argument_that_is_not_a_flag`
- **Break (2):** a flag's path is lower-cased before it is resolved, so a case-changed flag sets the value it resembles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_case-changed_flag`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_in_upper_case`
- **Break (3):** a flag with no value is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_no_value`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_no_value,_last`
- **Break (4):** a flag given twice is no longer refused, so the later one wins
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_given_twice`
- **Break (5):** a flag that names no value is skipped, as is one whose value does not decode
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_a_flag`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_section_in_a_list`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestAnInlineSectionsValuesSitBesideTheSectionsOwn/an_unknown_key_beside_an_inline_section's`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_case-changed_flag`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/a_flag_holding_an_alias`, `TestEveryMistakeIsRefusedAtStart/a_flag_naming_a_section`, `TestEveryMistakeIsRefusedAtStart/a_flag_of_the_wrong_type`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_ending_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_holding_a_double_underscore`, `TestEveryMistakeIsRefusedAtStart/a_flag_with_a_map_key_in_upper_case`, `TestEveryMistakeIsRefusedAtStart/an_unknown_flag`

## Every environment variable under the prefix names a declared value, in upper case, once

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** every variable is taken as under the prefix, so one that belongs to another program is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestLayersApplyPerValueInOrder`
- **Break (2):** only a name starting with the prefix in upper case is under it, so a variable with the prefix in lower case is skipped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_the_prefix_in_lower_case`
- **Break (3):** a variable under the prefix is matched ignoring case, so a mixed-case name sets the value it resembles
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_in_mixed_case`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_the_prefix_in_lower_case`
- **Break (4):** a variable set twice is no longer refused, so the later entry wins
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_set_twice`
- **Break (5):** a variable under the prefix that names no value is skipped, as is one whose value does not decode
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestABooleanInsideAValueIsTrueOrFalse`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_list,_from_the_environment`, `TestABooleanInsideAValueIsTrueOrFalse/in_a_map_in_a_list`, `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/an_empty_environment_variable_for_a_value_other_than_a_string`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_false`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_YAML_word_for_true`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_list_with_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_null`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_holding_a_second_document`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_naming_a_part_of_a_value`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_naming_a_section`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_of_the_wrong_type`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_a_hyphenated_map_key`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_with_a_map_key_starting_with_an_underscore`, `TestEveryMistakeIsRefusedAtStart/an_unknown_environment_variable_under_the_prefix`

## Every series a component registers carries its name as the component label, and the runtime series carry none

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a component's registerer is the bare registry, so its series carry no component and two components' series collide
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheOperationsEndpoints`
- **Break (2):** the runtime series are registered under a component label, as if a component owned them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheOperationsEndpoints`

## Every series a part registers carries the program's name as component and a job kind's its job_kind, and the runtime series carry neither

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** a job kind's series carry no job_kind, so one series registered by two job kinds collides
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestEverySeriesIsLabelledWithWhatRegisteredIt`
- **Break (2):** the runtime series are registered under the component label, as if a part owned them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestEverySeriesIsLabelledWithWhatRegisteredIt`

## Get runs the schema check on a pool before it first returns it, and ForMigration runs none

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1
- **Break (1):** Get runs no schema check, so a program starts on a schema its chain cannot run on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestGetChecksTheSchemaAndForMigrationDoesNot`
- **Break (2):** a schema the check refuses is ignored and its pool kept and returned, so the build goes on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestGetChecksTheSchemaAndForMigrationDoesNot`

## No file is read unless a flag or the environment names one

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** config.yaml in the working directory is read when no file is named
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestTheFileIsReadOnlyWhenNamed`, `TestTheFileIsReadOnlyWhenNamed/no_file_is_named`
- **Break (2):** an empty --config-file= is taken as naming no file, so every value falls back to the other layers
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/a_flag_naming_an_empty_file`
- **Break (3):** an empty MEDIATED_MAILBOX_CONFIG_FILE is taken as naming no file, so every value falls back to the other layers
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEveryMistakeIsRefusedAtStart`, `TestEveryMistakeIsRefusedAtStart/an_environment_variable_naming_an_empty_file`
- **Break (4):** the file the environment names is not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestTheFileIsReadOnlyWhenNamed`, `TestTheFileIsReadOnlyWhenNamed/the_environment_names_the_file`
- **Break (5):** the environment's file is read in place of the flag's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestTheFileIsReadOnlyWhenNamed`, `TestTheFileIsReadOnlyWhenNamed/the_flag's_file_wins_over_the_environment's`

## No unit of work begins on an account the list in force does not hold, that has no connection, or whose lowered target could not be read

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a unit of work begins on any account the connections loaded, the list in force not consulted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestNoUnitOfWorkBeginsOnAnAccountNotInForce`
- **Break (2):** a unit of work begins on an account whose lowered target could not be read, at the default target
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestEachAccountLeasesUnderItsLoweredTarget`

## One run per job, with wakes coalesced, held in the process

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and break (3) again on 2026-10-10 at RAPID_SEED=1, after the worker's late rule stopped counting a job's waits for a slot and each job started from its recorded latest success, which changed the tests it rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a job ensured again while its removal waits starts its loop at once, beside the removed loop still ending its run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobEnsuredWhileItIsRemovedNeverRunsTwiceAtOnce`, `TestEnsuringAndRemovingAJobRapidlyLeavesNoLoopRunning`
- **Break (2):** removing a job neither cancels nor waits for its loop, so the loop keeps asking the job after its removal
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlansListingAddsAndDropsJobsByKey`, `TestEnsuringAndRemovingAJobRapidlyLeavesNoLoopRunning`, `TestRemoveAndStopCancelTheirRunsAndWaitForThem`, `TestTheJobSeriesFollowEveryWayARunEnds`
- **Break (3):** the loop starts each run on a goroutine of its own and asks again without waiting for it, so runs of one job overlap
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobsWaitForASlotIsReported`, `TestAPlansListingAddsAndDropsJobsByKey`, `TestARunIsAskedAgainOnlyAfterItReturns`, `TestATimedJobIsAskedOnTheIntervalsPhase`, `TestTheJobSeriesFollowEveryWayARunEnds`, `TestWakesDuringARunCoalesceIntoOneRun`
- **Break (4):** every wake waits in a queue of its own, so wakes during a run are counted as further runs, not coalesced into one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobWithNoIntervalIsAskedOnlyWhenWoken`, `TestWakesDuringARunCoalesceIntoOneRun`

## Only the migrator's configuration type can ask for the pool with no schema check

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the process part carries the migrator's unexported method, so every configuration type that embeds it can open a pool with no schema check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestOnlyTheMigratorCanAskForThePoolWithNoSchemaCheck`, `TestOnlyTheMigratorCanAskForThePoolWithNoSchemaCheck/formigrationapi`, `TestOnlyTheMigratorCanAskForThePoolWithNoSchemaCheck/formigrationui`, `TestOnlyTheMigratorCanAskForThePoolWithNoSchemaCheck/formigrationworker`
- **Break (2):** the UI's configuration type is given the migrator's unexported method, so the UI's Build can open a pool with no schema check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestOnlyTheMigratorCanAskForThePoolWithNoSchemaCheck`, `TestOnlyTheMigratorCanAskForThePoolWithNoSchemaCheck/formigrationui`

## Remove and Stop cancel a job's run and wait for it to return

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break:** removing a job cancels its run and returns before the run has returned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobEnsuredWhileItIsRemovedNeverRunsTwiceAtOnce`, `TestAPlansListingAddsAndDropsJobsByKey`, `TestEnsuringAndRemovingAJobRapidlyLeavesNoLoopRunning`, `TestRemoveAndStopCancelTheirRunsAndWaitForThem`

## The application server serves TLS when given a key pair, read from its mounted files on each handshake, and a pair that does not load refuses the start

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the key pair is not loaded before the process serves, so a process whose mounts cannot serve starts and answers no handshake
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAKeyPairThatDoesNotLoadRefusesTheStart`
- **Break (2):** the key pair is read once, when the server is built, so a rotated pair never serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheApplicationServerServesTLSWhenGivenAKeyPair`

## The body-processing histogram is defined once, under its name, labelled by step

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the histogram is registered under another name than the one the job kinds and the API server time bodies under
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/scanner`:** `TestTheBodyProcessingHistogramIsDefinedOnce`
- **Break (2):** every step is observed under one label value, so a conversion and a scan cannot be told apart
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/scanner`:** `TestTheBodyProcessingHistogramIsDefinedOnce`

## The connections attach to the Session once, and a second attach is refused

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a second attach keeps the connections attached first and reports no error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestASecondAttachIsRefused`
- **Break (2):** a second attach replaces the connections attached first
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestASecondAttachIsRefused`

## The connections load the keys that open credentials while the process is built, so a public key matching none of the private keys stops the start

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the keyring is built from the first private key and its own public key, so the public key file is never compared with the private keys
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAPublicKeyMatchingNoPrivateKeyStopsTheStart`, `TestAReSealAndAnEndWhoseWritesInterleaveBothLand`, `TestAReSealNeverPutsBackAReplacedSecret`, `TestAStaleReSealIsRefused`, `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** a keyring that cannot be loaded is logged and the build goes on, so a process whose public key matches no private key starts
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAPublicKeyMatchingNoPrivateKeyStopsTheStart`

## The connections' scan has an entry for every listed account and every OAuth client, true while its value is on an old key or does not open

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** an account with no stored credential reads true, so a key's retirement waits on a value that does not exist
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`
- **Break (2):** an account whose credential does not open reads false, as if its value were on the current key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows`

## The database password is the mounted password file's text less one trailing newline, and an empty one refuses the start

- **Date · evidence:** 2026-09-28 · [pull request #181](https://github.com/ppat/mediated-mailbox-mcp/pull/181), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** a trailing \r\n loses only its \n, so the password keeps a carriage return
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestAnEmptyPasswordIsRefused`, `TestThePasswordIsTheFilesText`
- **Break (2):** a password file holding no password gives an empty password rather than refusing the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestAnEmptyPasswordIsRefused`
- **Break (3):** every trailing newline is trimmed, not only one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestThePasswordIsTheFilesText`
- **Break (4):** the password file is read but its password is not set on the connection
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestTheConnectionIsTheConfigurations`, `TestThePasswordIsTheFilesText`
- **Break (5):** no trailing newline is trimmed from the password
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestAnEmptyPasswordIsRefused`, `TestThePasswordIsTheFilesText`

## The effective configuration is logged at start, each value with its source

- **Date · evidence:** 2026-10-01 · [pull request #239](https://github.com/ppat/mediated-mailbox-mcp/pull/239), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-08, with a break per entry package added that sets the level before it logs the configuration, when the logger hand-off moved the control to every running deployable · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and every break again on 2026-10-09, after the logger hand-off was rebased onto the sender-domain normalizer and the re-mask from the store · [pull request #323](https://github.com/ppat/mediated-mailbox-mcp/pull/323), and breaks 1, 4, 5 and 6 again on 2026-10-10, after the worker merged backfill and delta sync into one deployable with one entry, which leaves one patch where each of the two had its own · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-11 at RAPID_SEED=1, after the performance measurements changed the code, tests or patches the row rests on · [pull request #341](https://github.com/ppat/mediated-mailbox-mcp/pull/341), and break 3 again on 2026-10-11 at RAPID_SEED=1, after the UI took the credential section in place of its own key paths, which changed the code and tests the row rests on · [pull request #349](https://github.com/ppat/mediated-mailbox-mcp/pull/349), and breaks 2 and 3 again on 2026-10-11 at RAPID_SEED=1, after the transport taken from the TLS files changed the code, tests or patches the row rests on · [pull request #352](https://github.com/ppat/mediated-mailbox-mcp/pull/352), and breaks 2 and 3 again on 2026-10-11 at RAPID_SEED=1, after the transport tests came to drive each deployable's start and read its serving record, which changed the tests the row rests on · [pull request #352](https://github.com/ppat/mediated-mailbox-mcp/pull/352), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and breaks 4, 5 and 6 again on 2026-10-13 at RAPID_SEED=1, after the pool sizes changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and breaks 2, 3 and 4 again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397) · and breaks (2), (3) and (4) again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the process naming a failed server and refusing a plan it could never run, with the host's RunOnce removed, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the host sets the level before it logs the effective configuration, so under warn a refused value's source is never written, and a refused level's source is never written at all
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheConfiguredLevelGovernsTheLog`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/a_name_in_upper_case`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/a_refused_value_at_error`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/an_offset`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/error,_from_the_environment`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/warn`, `TestTheConfiguredLevelGovernsTheLog/until_stopped/warning`
- **Break (2):** the effective configuration is not logged
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheEffectiveConfigurationIsLogged`, `TestTheStartBannerGoesToStandardErrorAndStandardOutputStaysJSON`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheEffectiveConfigurationIsLogged`, `TestTheStartBannerGoesToStandardErrorAndStandardOutputStaysJSON`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheEffectiveConfigurationIsLogged`, `TestTheStartBannerGoesToStandardErrorAndStandardOutputStaysJSON`
- **Break (3):** a password file's text is logged beside its path
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheEffectiveConfigurationIsLogged`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheEffectiveConfigurationIsLogged`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheEffectiveConfigurationIsLogged`
- **Break (4):** each value is logged without the layer that set it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/api/app`:** `TestTheEffectiveConfigurationIsLogged`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/ui/app`:** `TestTheEffectiveConfigurationIsLogged`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/app`:** `TestTheEffectiveConfigurationIsLogged`

## The end of a unit of work records the token's latest attempt, and nothing for a token that made none

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the end records no attempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestTheEndRecordsTheTokensLatestAttempt`
- **Break (2):** a token that made no attempt records one at the epoch
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestAStoredCredentialSomeoneElseReplacedWins`, `TestTheEndRecordsTheTokensLatestAttempt`

## The load concurrent body requests share runs on the process's goroutines, and a load that cannot run fails its callers

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the load starts on a goroutine of its own rather than through the starter, so the process neither recovers it nor waits for it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestARefusedStartFailsItsCallers`, `TestEveryLoadRunsThroughTheStarter`
- **Break (2):** a load that panics answers none of its callers, so each waits for an answer that never comes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAPanickedLoadFailsItsCallers`
- **Break (3):** a load the starter refuses leaves its callers waiting for an answer that never comes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestARefusedStartFailsItsCallers`

## The operations endpoints answer the health probe, the readiness probe and the metrics endpoint as plain text, and nothing else

- **Date · evidence:** 2026-10-14 · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), at RAPID_SEED=1
- **Break (1):** every path the operations listener is asked for is answered as the health probe
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheOperationsEndpoints`
- **Break (2):** the probes answer as HTML rather than plain text
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheOperationsEndpoints`
- **Break (3):** the readiness probe answers 200 whatever the readiness it is handed says
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/host`:** `TestTheOperationsEndpoints`, `TestTheProcessStopsInOrder`

## The option an owner exports for a test fills its slot before anything asks, and a run without it builds

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the option fills the slot as not built, so the first part that asks builds over the test's thing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASubstituteFillsTheSlotBeforeAnythingAsks`
- **Break (2):** the option fills nothing, so the owner builds its own thing in place of the test's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASubstituteFillsTheSlotBeforeAnythingAsks`

## The policies read the accounts each load names, and refuse an empty name

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a load leaves the accounts the policies read as they were, so a load naming other accounts reads the old ones
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestEachLoadReadsTheAccountsItNames`
- **Break (2):** a load takes an empty account name, which no account's transaction can set
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestEachLoadReadsTheAccountsItNames`, `TestTheLoaderComposesEachAccountsPolicy`

## The policies register their series at the first load that names an account, so a process that holds none carries none

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** every load builds the loader again, so the second load fails registering series the first registered
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestEachLoadReadsTheAccountsItNames`, `TestEachReloadItsCallerDidNotCancelIsTimed`, `TestEachReloadPreparesThePolicyItSwapsIn`, `TestTheLoaderComposesEachAccountsPolicy`
- **Break (2):** the series are registered when the policies are built, so a process holding no account serves them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestEachReloadItsCallerDidNotCancelIsTimed`

## The policies' reload-failure series reads 1 while their latest load failed and 0 once one succeeds

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a reload that succeeds leaves the series as it was, so it reads 1 after the first failure for as long as the process runs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (2):** a reload that succeeds sets the series to 1, as a failure does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (3):** a reload whose rows do not validate leaves the series as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (4):** a reload whose read failed leaves the series as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`

## The policies' shared load answers a caller only with a load that started after it asked, shared with the callers waiting when it started

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** each caller runs a load of its own, so a burst of callers runs a burst of loads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestACallerThatStopsWaitingLeavesTheLoadRunning`, `TestARefusedStartFailsItsCallers`, `TestCallersWaitingTogetherShareOneLoad`, `TestEveryLoadRunsThroughTheStarter`, `TestLoadsRunOneAtATime`
- **Break (2):** a load runs under the context of the caller that started it, so a caller that stops waiting cancels the load the others wait on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestACallerThatStopsWaitingLeavesTheLoadRunning`
- **Break (3):** a caller asking while a load runs is answered by that load
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestACallerThatStopsWaitingLeavesTheLoadRunning`, `TestALoadRunningWhenACallerAsksDoesNotCount`, `TestCallersWaitingTogetherShareOneLoad`

## The policy reload rule fires while a process's latest policy load failed, and only then, as the policies' tests read it

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the rule fires for every process that loads policy, whether its latest reload failed or succeeded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAlertingRules`
- **Break (2):** the rule compares the series with a value it never exceeds, so it never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAlertingRules`
- **Break (3):** the rule fires only after the failure has lasted five minutes, so a failure is not raised at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/policies`:** `TestAlertingRules`

## The probes run nothing when asked, and the readiness probe answers the state last reported

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the readiness probe answers from the reports as they stood when serving began, so a later report never reaches it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAJobKindsPlanRunsOnTheScheduler`, `TestTheProbesAnswerFromWhatWasReported`, `TestTheProbesRunNothingWhenAsked`
- **Break (2):** each ask of the readiness probe starts a check through Go that runs until the process stops
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProbesRunNothingWhenAsked`

## The process's one scanner is built from its scanner section under the revision the process holds of that section

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** the scanner is built from the default scanner section rather than the configured one, under the configured revision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/scanner`:** `TestTheRevisionFollowsTheScannerSectionAlone`, `TestTheScannerIsOnePerProcessUnderItsSectionsRevision`
- **Break (2):** the scanner is built under the revision of the database section, so a change to the scanner leaves its revision as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/scanner`:** `TestTheRevisionFollowsTheScannerSectionAlone`, `TestTheRevisionFollowsTheScannerSectionAlone/a_flag_changing_a_job_kind's_section`, `TestTheRevisionFollowsTheScannerSectionAlone/a_flag_changing_the_scanner`, `TestTheScannerIsOnePerProcessUnderItsSectionsRevision`

## The readiness probe answers from what was reported, ready only while every reporter is ready, and names the first that is not

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** a reporter that failed reads as ready, so the probe answers ready while a part has failed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProbesAnswerFromWhatWasReported`
- **Break (2):** the readiness probe reads the first reporter's report alone, so a later reporter that is not ready leaves the process ready
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProbesAnswerFromWhatWasReported`
- **Break (3):** the readiness probe answers ready whatever was reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestAJobKindsPlanRunsOnTheScheduler`, `TestTheProbesAnswerFromWhatWasReported`, `TestTheProbesRunNothingWhenAsked`

## The scheduler refuses a plan it could never run, and the refusal stops the process in order naming the job kind

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and breaks (1), (2) and (4) again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the process ignores the scheduler's refusal of a plan and serves on without the job kind
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASchedulerThatRefusesAPlanStopsTheProcessInOrder`
- **Break (2):** the scheduler's refusal is returned without the job kind it refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASchedulerThatRefusesAPlanStopsTheProcessInOrder`
- **Break (3):** a plan with no listing or no job is ensured, so its listing fails on every ask or panics and the process serves on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASchedulerThatRefusesAPlanStopsTheProcessInOrder`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAPlanItCouldNeverRunIsRefused`
- **Break (4):** the job kinds' plans are ensured only once the process serves, so a plan the scheduler refuses fails a process that has already announced it serves
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestASchedulerThatRefusesAPlanStopsTheProcessInOrder`

## The serving record names the transport the application server serves

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the serving record says the application server serves TLS whatever transport it took
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheApplicationServerGivenNoKeyPairIsPlain`
- **Break (2):** the serving record carries no transport
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheApplicationServerGivenNoKeyPairIsPlain`, `TestTheApplicationServerServesTLSWhenGivenAKeyPair`

## The Session is ready once a reload has loaded the list, the connections and the policy, and stays ready after a later reload whose policy fails

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a later reload whose policy fails reports the Session not ready again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestTheSessionIsReadyOnceAReloadHasLoadedEverything`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything/the_list`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything/the_policy`
- **Break (2):** the Session reports ready once a reload has read the list, whether or not its policy loaded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestTheSessionIsReadyOnceAReloadHasLoadedEverything`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything/the_policy`

## The Session reloads in one order, and keeps in force what a failed read could not read

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a reload whose load of the connections fails puts its list in force all the same, so an account whose credential was never read is held
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReloadConnectsWhatItReadAndKeepsWhatWasOnAFailedRead`
- **Break (2):** a reload whose read of the accounts fails puts an empty list in force, as if no account were listed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestAFailedReloadKeepsWhatItCouldNotRead`, `TestOnceLoadedAFailedReloadWaitsForTheNextInterval`
- **Break (3):** a reload whose policy load fails keeps the list of the reload before it in force and tells no one, so an account no longer listed stays held
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestAFailedReloadKeepsWhatItCouldNotRead`
- **Break (4):** those who asked are told after a reload whose read of the accounts failed, though nothing new is in force
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestAFailedReloadKeepsWhatItCouldNotRead`

## The Session's reload is tried again within seconds while no reload has loaded everything, and at the next interval once one has

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a reload that fails before any has loaded everything is tried again only at the next interval
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestTheSessionIsReadyOnceAReloadHasLoadedEverything`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything/the_list`, `TestTheSessionIsReadyOnceAReloadHasLoadedEverything/the_policy`
- **Break (2):** a reload that fails once one has loaded everything is answered as failed, so the scheduler tries it again after its backoff rather than at the next interval
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/sessions`:** `TestOnceLoadedAFailedReloadWaitsForTheNextInterval`

## The time a job waits for a slot of its kind's limit does not count toward its lateness

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), at RAPID_SEED=1, and every break again on 2026-10-11 at RAPID_SEED=1, after the performance measurements changed the code, tests or patches the row rests on · [pull request #341](https://github.com/ppat/mediated-mailbox-mcp/pull/341), and the breaks that edit the rules file again on 2026-10-12, after it moved under `packaging/chart/rules/` · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394), and breaks 1, 2 and 3 again on 2026-11-17 at RAPID_SEED=1, after the provider library and the credential family moved the code, tests or patches the row rests on · [pull request #450](https://github.com/ppat/mediated-mailbox-mcp/pull/450), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453)
- **Break (1):** the late rule counts the waits that ended, so a job that waited for a slot fires sooner after its wait than its bound
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAlertingRules`
- **Break (2):** the late rule counts the wait in progress, so a job waiting for a slot ages while it waits
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAlertingRules`
- **Break (3):** the late rule counts the time a job waits for a slot, so a job queued behind its kind's long runs fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAlertingRules`
- **Break (4):** the scheduler reports no wait for a slot, so a job queued behind its kind's runs ages as if it were late
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobsWaitForASlotIsReported`
- **Break (5):** a success leaves the time waited before it counted, so the job's lateness after the success loses that time
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/schedule`:** `TestAJobsWaitForASlotIsReported`

## Two loads in one process share nothing with each other or with the defaults

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** the result starts as the defaults, so it holds their maps and a load writes into them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestTwoConfigurationsShareNothing`
- **Break (2):** a value is copied without its lists and maps, so a list in the result is the one in the defaults
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestTwoConfigurationsShareNothing`

## Units of work begun through every caller of a process share each account's token

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break:** each unit of work makes a token of its own from the record's credential, so a rotation one unit sees is lost to the others
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAFailedWriteBackIsHeldLoggedAndKeptByAReload`, `TestAReSealAndAnEndWhoseWritesInterleaveBothLand`, `TestARotatedCredentialSurvivesARestart`, `TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed`, `TestAStoredCredentialSomeoneElseReplacedWins`, `TestATokenServesItsAccountUntilItsCredentialOrClientChanges`, `TestAnAdoptionBetweenAnEndsReadAndItsWriteDiscardsTheWriteBack`, `TestAnEndAfterAnAdoptionLeavesTheOperatorsCredential`, `TestOverlappingRotationsEachLand`, `TestTheEndRecordsTheTokensLatestAttempt`, `TestTheSameCredentialStoredUnderNewBytesKeepsTheToken`, `TestTwoEndsWhoseWritesInterleaveBothLand`, `TestUnitsOfWorkThroughTwoCallersShareTheAccountsToken`

## Upkeep is asked for once per shared thing, while the process is built, and never by a program that runs one piece of work

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a second upkeep for one shared thing is taken, so its first is never run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestUpkeepIsRefusedOutsideABuildOfAProcessThatServes`
- **Break (2):** upkeep asked for once the process was built is taken and never run
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestUpkeepIsRefusedOutsideABuildOfAProcessThatServes`
- **Break (3):** a program that does one piece of work and exits takes upkeep it never runs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestUpkeepIsRefusedOutsideABuildOfAProcessThatServes`

## Values layer per value in the order defaults, file, environment, flags

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-12 at RAPID_SEED=1, after the start banner, `--version` and the release version changed the code, tests or patches the row rests on · [pull request #356](https://github.com/ppat/mediated-mailbox-mcp/pull/356), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384), and every break again on 2026-11-17 at RAPID_SEED=1, after the configuration types moved into `core/config` changed the code, tests or patches the row rests on · [pull request #452](https://github.com/ppat/mediated-mailbox-mcp/pull/452)
- **Break (1):** the environment is applied after the flags, so it overrides them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEachValueRecordsTheLayerThatSetIt`, `TestLayersApplyPerValueInOrder`
- **Break (2):** the file is applied after the environment, so it overrides it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAListFromALaterLayerReplacesTheWholeList`, `TestLayersApplyPerValueInOrder`
- **Break (3):** the last layer that names any key of a map sets the whole map, so the keys earlier layers set are dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestEachValueRecordsTheLayerThatSetIt`, `TestLayersApplyPerValueInOrder`, `TestTwoConfigurationsShareNothing`
- **Break (4):** a list from a later layer is appended to the earlier layer's list rather than replacing it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestADurationHoldsNanosecondsAndIsWrittenAsTheText`, `TestAFileValueThatIsNotANull`, `TestAListFromALaterLayerReplacesTheWholeList`, `TestAValueFromTheEnvironmentOrAFlag`, `TestAValueFromTheEnvironmentOrAFlag/a_list_is_decoded_as_YAML`, `TestEachValueRecordsTheLayerThatSetIt`, `TestLayersApplyPerValueInOrder`
- **Break (5):** a value keeps the source of the first layer that set it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/settings`:** `TestAnInlineSectionsValuesSitBesideTheSectionsOwn`, `TestEachValueRecordsTheLayerThatSetIt`

## What the slots hold is closed last built first, once the servers have drained and every job and goroutine has returned

- **Date · evidence:** 2026-11-17 · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), at RAPID_SEED=1, and every break again on 2026-11-17 at RAPID_SEED=1, after the Session, the connections and the unit of work in the process family changed the code, tests or patches the row rests on · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455)
- **Break (1):** the slots are closed as the process starts to stop, before its requests have drained and its jobs and goroutines have returned
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProcessStopsInOrder`
- **Break (2):** the slots are closed in the order they were built, so a thing is closed before what was built over it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process`:** `TestTheProcessStopsInOrder`

## Write-backs, reloads, re-reads and the re-seal take turns under one lock of the connections, so the process's own writes each land and none is read back as someone else's

- **Date · evidence:** 2026-11-17 · [pull request #455](https://github.com/ppat/mediated-mailbox-mcp/pull/455), at RAPID_SEED=1
- **Break (1):** a reload reads the rows before it takes the connections' lock, so a read made while a write-back waits at the database is adopted after it as someone else's value and the token is made again
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReloadDuringAWriteBackWaitsForIt`
- **Break (2):** the re-seal of the accounts runs outside the connections' lock, so an end whose compare-and-set waits behind the re-seal adopts the re-sealed bytes and loses its rotation
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReSealAndAnEndWhoseWritesInterleaveBothLand`
- **Break (3):** a write-back no longer takes the connections' lock, so one whose compare-and-set waits behind another's reads the other's bytes back as someone else's and its rotation is lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/connections`:** `TestAReSealAndAnEndWhoseWritesInterleaveBothLand`, `TestTwoEndsWhoseWritesInterleaveBothLand`
