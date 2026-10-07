# Mutations: policyload

The demonstrations of the controls whose patches sit in `policyload/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A base edit landing between two accounts' reads is read again, and fails the reload only when edits land through both reads

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a reload whose accounts read different base rules fails at once, so one base edit landing mid-reload pages
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`
- **Break (2):** a reload reads once more for each disagreement, up to two more times, so edits landing through two reads no longer fail it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`

## A read of the policy tables the loader cannot trust never replaces the active policy

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the base rules are taken from the first account's read without comparing the others', so accounts are composed from different base policies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`
- **Break (2):** a read that found no rules is taken as a failed read, so a policy with no rules read whole never takes effect
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAnEmptyPolicyReadWholeIsAccepted`
- **Break (3):** a read that failed is taken as a whole read that found no rules, so the policy with no rules replaces the active one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`
- **Break (4):** a read that fails for one account keeps the rows read before it, so the accounts read earlier replace the active policy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`

## A reload its caller cancelled keeps the active policy and raises no alarm

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a reload its caller cancelled is counted as a failed read, so a process shutting down mid-reload raises the alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAReloadItsCallerCancelledRaisesNoAlarm`
- **Break (2):** a reload whose deadline passed is taken as one its caller cancelled, so reloads that keep timing out raise no alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAReloadItsCallerCancelledRaisesNoAlarm`
- **Break (3):** every failed read is taken as a reload its caller cancelled, so no failed read raises the alarm
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`

## An account whose own rules the loader did not read is held to the policy that restricts every sender

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the accounts the loader read are held to the policy that restricts every sender, and the ones it did not read get the base rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`, `TestSetAccountsChangesWhatTheNextReloadReads`, `TestTheLoaderComposesEachAccountsPolicy`
- **Break (2):** an account the loader did not read gets the base rules alone, without the restrictions its own rules add
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestSetAccountsChangesWhatTheNextReloadReads`, `TestTheLoaderComposesEachAccountsPolicy`

## The policy loader reads the accounts it is set to from its next reload on

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** SetAccounts leaves the accounts the loader reads as they were
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestSetAccountsChangesWhatTheNextReloadReads`
- **Break (2):** SetAccounts takes no account or an empty name, as New refuses
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestSetAccountsChangesWhatTheNextReloadReads`

## The policy reload rule fires while a process's latest reload failed, and only then

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the rule fires for every process that loads policy, whether its latest reload failed or succeeded
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAlertingRules`
- **Break (2):** the rule compares the series with a value it never exceeds, so it never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAlertingRules`
- **Break (3):** the rule fires only after the failure has lasted five minutes, so a failure is not raised at once
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAlertingRules`

## The reload-failure series reads 1 while the latest policy reload failed and 0 once one succeeds

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** a reload that succeeds leaves the series as it was, so it reads 1 after the first failure for as long as the process runs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (2):** a reload that succeeds sets the series to 1, as a failure does
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestABaseEditBetweenTwoAccountsReadsIsReadAgain`, `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`, `TestAnEmptyPolicyReadWholeIsAccepted`, `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (3):** a reload whose rows do not validate leaves the series as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAnInvalidUpdateNeverDisplacesTheActivePolicy`
- **Break (4):** a reload whose read failed leaves the series as it was
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/policyload`:** `TestAFailedReadNeverReplacesTheActivePolicy`, `TestAFailedReadNeverReplacesTheActivePolicy/a_base_rule_is_added_between_two_accounts'_reads,_twice`, `TestAFailedReadNeverReplacesTheActivePolicy/an_account's_rule_cannot_be_decoded,_after_its_other_rules`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_first_account's_read_fails_before_any_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_role_cannot_read_the_table`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_after_one_row`, `TestAFailedReadNeverReplacesTheActivePolicy/the_second_account's_read_fails_before_any_row`, `TestAReloadItsCallerCancelledRaisesNoAlarm`
