# Mutations: db

The demonstrations of the controls whose patches sit in `db/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A policy-rule write names the transaction's account

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the base-rule policy admits inserting base rules rather than reading them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAPolicyRuleWriteNamesTheTransactionsAccount`, `TestAPolicyRuleWriteNamesTheTransactionsAccount/the_UI_inserts_a_base_rule`, `TestAPolicyRuleWriteNamesTheTransactionsAccount/the_UI_reads_the_base_rule`, `TestRowLevelSecurityScopesEveryAccountTable`, `TestRowLevelSecurityScopesEveryAccountTable/policy_rules/read_a_base_rule`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/a_transaction_naming_an_account_and_the_base_policy_writes_no_base_row`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/no_other_role_writes_the_base_policy`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/the_UI_writes_no_base_row_in_an_account's_transaction`
- **Break (2):** the base-rule policy covers every command rather than reads, so a runtime role writes base rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAPolicyRuleWriteNamesTheTransactionsAccount`, `TestAPolicyRuleWriteNamesTheTransactionsAccount/a_role_updates_a_base_rule`, `TestAPolicyRuleWriteNamesTheTransactionsAccount/the_UI_inserts_a_base_rule`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/a_transaction_naming_an_account_and_the_base_policy_writes_no_base_row`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/no_other_role_writes_the_base_policy`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/the_UI_writes_no_base_row_in_an_account's_transaction`

## A shared library's statements are planned under the role of each deployable that admits it

- **Date · evidence:** 2026-09-24 · [pull request #157](https://github.com/ppat/mediated-mailbox-mcp/pull/157)
- **Break (1):** the spenders' roles may read and write grants but not delete them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`
- **Break (2):** the reorganization workload's role gets no grant on the rate limiter's tables
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`

## A statement cannot read a column the schema does not hold

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153)
- **Break (1):** the chain gives messages a body column, so the schema no longer withholds it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGenerationRefusesAColumnTheSchemaDoesNotHold`, `TestGenerationRefusesAColumnTheSchemaDoesNotHold/body_column_violation.sql`
- **Break (2):** a later migration renames messages.message_id to body, so the schema holds a body column in an identifier's place
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGenerationRefusesAColumnTheSchemaDoesNotHold`, `TestGenerationRefusesAColumnTheSchemaDoesNotHold/body_column_violation.sql`
- **Break (3):** a later migration renames messages.subject to body, so the schema holds a body column in the subject's place
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGenerationRefusesAColumnTheSchemaDoesNotHold`, `TestGenerationRefusesAColumnTheSchemaDoesNotHold/body_column_violation.sql`

## A transaction that did not set the account fails before any data access

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153)
- **Break (1):** the helper sets the account for the session rather than the transaction, so it outlives the transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage`, `TestTheAccountEndsWithItsTransaction`
- **Break (2):** the helper refuses a connection as the Beginner rather than a transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage`, `TestRunRefusesATransaction`, `TestRunScopesTheTransactionToTheAccount`, `TestTheAccountEndsWithItsTransaction`
- **Break (3):** the helper no longer refuses a transaction as the Beginner, so a savepoint's account outlives it in the outer transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestRunRefusesATransaction`
- **Break (4):** the helper no longer compares the setting it reads back with the account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage`

## An account cannot name another provider's OAuth client

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** the account's reference pairs its provider with the client's name and its client with the client's provider, so it is refused for the account's own provider's client
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAnAccountCannotNameAnotherProvidersClient`
- **Break (2):** the account's reference names the client alone, so the client's provider is never compared with the account's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAnAccountCannotNameAnotherProvidersClient`

## An OAuth client an account connects through cannot be removed

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** the account's reference to its client is dropped, so nothing refuses removing a client an account connects through
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAnAccountCannotNameAnotherProvidersClient`
- **Break (2):** removing a client an account connects through clears the account's client rather than being refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`

## No runtime role can update or delete a row of the policy history

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break:** the UI's role gains update and delete on the policy history, so a compromised UI can rewrite who lifted a restriction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterThePolicyHistory`, `TestNoRuntimeRoleCanAlterThePolicyHistory/mediated_mailbox_ui/delete`, `TestNoRuntimeRoleCanAlterThePolicyHistory/mediated_mailbox_ui/update`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/delete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_action`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_actor`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_rule_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_suffixes_after`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_suffixes_before`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_ts`

## No runtime role can update or delete an audit row

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153)
- **Break (1):** the chain grants DELETE on the audit log to every role
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_backfill/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_organize/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_propose/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_sync/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_ui/delete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/audit_log/delete`
- **Break (2):** the chain grants the reorganization workload's role TRUNCATE on the audit log, an erasing operation other than update and delete
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_organize/truncate`
- **Break (3):** the chain grants the mediator's role UPDATE on the audit log
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/update`

## One OAuth client is never stored twice

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255)
- **Break (1):** the uniqueness of a provider's client identifier is dropped, so one client may be stored under two names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`
- **Break (2):** a client identifier is unique across providers, so another provider's client holding the same identifier is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`
- **Break (3):** a client's name is unique only within its provider, so a client of another provider may take a name a client holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`

## Only the roles that call a provider read oauth_clients and their own account's state, and the UI reads its account's progress and never a credential

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** delta sync's role loses the read and write of its account's credential that its snapshot's statements need
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_sync/reads_its_own_account's_state`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_sync/updates_another_account's_credential`
- **Break (2):** the UI's role reads and writes every account's credential column
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheProviderCallingRolesReadOnlyTheirAccountsState`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_ui/reads_a_credential`
- **Break (3):** the four roles that call a provider lose the read of each client's name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_backfill/reads_every_client`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_mediate/reads_every_client`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_organize/reads_every_client`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_sync/reads_every_client`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_sync/writes_a_client`
- **Break (4):** the heuristics job's role, which calls no provider, reads every OAuth client
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_propose/reads_a_client`
- **Break (5):** the four roles that call a provider write every client's secret with no statement that uses the grant
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_backfill/writes_a_client`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_mediate/writes_a_client`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_organize/writes_a_client`
- **Break (6):** delta sync's role writes a client's identifier as well as its secret, with no statement that uses the grant
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_sync/writes_a_client`
- **Break (7):** delta sync's role loses the write of a client's secret its re-seal's statement needs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_sync/writes_a_client`
- **Break (8):** the UI's read of its account's progress also covers the sealed credential
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheProviderCallingRolesReadOnlyTheirAccountsState`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_ui/reads_a_credential`
- **Break (9):** the UI's role loses the read of the progress columns its statement in db/accountstate needs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_ui/reads_its_own_account's_progress`

## Row-level security scopes every account-keyed table to the transaction's account

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** row-level security is not enabled on account_state, so its policy never applies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestRowLevelSecurityScopesEveryAccountTable`, `TestRowLevelSecurityScopesEveryAccountTable/account_state/insert_by_another_account`, `TestRowLevelSecurityScopesEveryAccountTable/account_state/read_by_another_account`, `TestRowLevelSecurityScopesEveryAccountTable/account_state/update_by_another_account`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_backfill/reads_its_own_account's_state`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_backfill/updates_another_account's_credential`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_mediate/reads_its_own_account's_state`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_mediate/updates_another_account's_credential`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_organize/reads_its_own_account's_state`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_organize/updates_another_account's_credential`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_sync/reads_its_own_account's_state`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_sync/updates_another_account's_credential`, `TestTheProviderCallingRolesReadOnlyTheirAccountsState/mediated_mailbox_ui/reads_its_own_account's_progress`
- **Break (2):** the audit log's policy compares the actor column with the account rather than account_id
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestRowLevelSecurityScopesEveryAccountTable`, `TestRowLevelSecurityScopesEveryAccountTable/audit_log/insert_by_the_owning_account`, `TestRowLevelSecurityScopesEveryAccountTable/audit_log/read_by_the_owning_account`, `TestRowLevelSecurityScopesEveryAccountTable/audit_log/update_by_the_owning_account`
- **Break (3):** no policy lets an account read the base rules, which carry no account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAPolicyRuleWriteNamesTheTransactionsAccount`, `TestAPolicyRuleWriteNamesTheTransactionsAccount/the_UI_reads_the_base_rule`, `TestRowLevelSecurityScopesEveryAccountTable`, `TestRowLevelSecurityScopesEveryAccountTable/policy_rules/read_a_base_rule`
- **Break (4):** row-level security is not enabled on messages, so its policy never applies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestRowLevelSecurityScopesEveryAccountTable`, `TestRowLevelSecurityScopesEveryAccountTable/messages/insert_by_another_account`, `TestRowLevelSecurityScopesEveryAccountTable/messages/read_by_another_account`, `TestRowLevelSecurityScopesEveryAccountTable/messages/update_by_another_account`
- **Break (5):** the senders policy reads a setting no transaction sets, so it compares the account with nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestRowLevelSecurityScopesEveryAccountTable`, `TestRowLevelSecurityScopesEveryAccountTable/senders/insert_by_the_owning_account`, `TestRowLevelSecurityScopesEveryAccountTable/senders/read_by_the_owning_account`, `TestRowLevelSecurityScopesEveryAccountTable/senders/update_by_the_owning_account`

## Schema change is confined to the migration role

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153)
- **Break (1):** the chain grants a runtime role the right to create objects in the schema
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestRuntimeRolesCannotChangeTheSchema`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_backfill/create_a_function`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_backfill/create_a_table`
- **Break (2):** the bootstrap makes a runtime role a member of the migration role, so it acts as the schema's owner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/truncate`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/update`, `TestRuntimeRolesCannotChangeTheSchema`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/add_a_body_column`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/change_a_table_owner`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/disable_a_policy`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/drop_a_policy`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/drop_a_table`, `TestTheBootstrapCreatesOneRuntimeRolePerDeployable`

## The base policy is written only in a base-policy transaction

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the base history's insert policy is dropped, so a base rule's change cannot be recorded and its write fails whole
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/the_UI_writes_the_base_policy_in_a_base-policy_transaction`
- **Break (2):** the base rules' write policy applies to every role rather than the UI's, so another runtime role writes base rules in a base-policy transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/no_other_role_writes_the_base_policy`
- **Break (3):** the base rules' write policy stops requiring an empty account, so a transaction naming an account and the base policy writes base rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/a_transaction_naming_an_account_and_the_base_policy_writes_no_base_row`
- **Break (4):** RunBase sets app.base off rather than on and stops checking it, so a base-policy transaction writes no base rule and raises nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestRunBaseWritesTheBasePolicyAndReadsNoAccount`
- **Break (5):** RunBase sets app.base for the session rather than the transaction, so a later transaction on the connection that never named the base policy writes base rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestTheBasePolicyEndsWithItsTransaction`

## The listing roles read the accounts table in full and write only their own rows

- **Date · evidence:** 2026-09-26 · [pull request #178](https://github.com/ppat/mediated-mailbox-mcp/pull/178), (1) repeated with its patch regenerated in [pull request #181](https://github.com/ppat/mediated-mailbox-mcp/pull/181)
- **Break (1):** the listing policy is dropped, so a listing role reads only its own row of the accounts table and a listing with no account set raises
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAListingNeedsNoAccountSet`, `TestAListingNeedsNoAccountSet/mediated_mailbox_backfill`, `TestAListingNeedsNoAccountSet/mediated_mailbox_mediate`, `TestAListingNeedsNoAccountSet/mediated_mailbox_organize`, `TestAListingNeedsNoAccountSet/mediated_mailbox_sync`, `TestAListingNeedsNoAccountSet/mediated_mailbox_ui`, `TestTheListingRolesReadEveryAccount`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_backfill/lists_every_account`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_mediate/lists_every_account`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_organize/lists_every_account`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_sync/lists_every_account`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_ui/lists_every_account`
- **Break (2):** the listing policy names no role, so every role granted a read of accounts lists every account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestRowLevelSecurityScopesEveryAccountTable`, `TestRowLevelSecurityScopesEveryAccountTable/accounts/read_by_another_account`, `TestTheListingRolesReadEveryAccount`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_propose/lists_only_its_own_account`
- **Break (3):** the listing policy covers every command rather than reads alone, so a listing role writes every account's row
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheListingRolesReadEveryAccount`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_backfill/inserts_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_backfill/updates_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_mediate/inserts_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_mediate/updates_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_organize/inserts_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_organize/updates_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_sync/inserts_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_sync/updates_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_ui/inserts_another_account's_row`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_ui/updates_another_account's_row`

## The operation log is scoped through its plan's account

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153)
- **Break (1):** row-level security is not enabled on the operation log, so its policy never applies
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheOperationLogIsScopedThroughItsPlan`, `TestTheOperationLogIsScopedThroughItsPlan/insert_by_another_account`, `TestTheOperationLogIsScopedThroughItsPlan/read_by_another_account`, `TestTheOperationLogIsScopedThroughItsPlan/update_by_another_account`
- **Break (2):** the policy admits the plans of every account but the transaction's own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheOperationLogIsScopedThroughItsPlan`, `TestTheOperationLogIsScopedThroughItsPlan/insert_by_the_plan's_account`, `TestTheOperationLogIsScopedThroughItsPlan/read_by_the_plan's_account`, `TestTheOperationLogIsScopedThroughItsPlan/update_by_the_plan's_account`

## The recorded authentication outcome is the latest attempt, and recording an older one changes nothing

- **Date · evidence:** 2026-10-01 · [pull request #227](https://github.com/ppat/mediated-mailbox-mcp/pull/227)
- **Break (1):** the statement replaces the stored attempt only with an older one, so a later attempt never replaces an earlier one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill`:** `TestOnlyALaterAttemptIsRecorded`
- **Break (2):** the statement drops its predicate on the stored attempt's time, so the last write wins and an older attempt replaces a later one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/backfill`:** `TestOnlyALaterAttemptIsRecorded`

## The UI's role writes exactly the policy writes policy management makes, each with its history row

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the UI's role loses the delete on policy_rules, so lifting a rule is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/a_base-policy_transaction_reaches_no_account's_rows`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/the_UI_writes_its_account's_rule_in_its_account's_transaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/the_UI_writes_no_base_row_in_an_account's_transaction`, `TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction/the_UI_writes_the_base_policy_in_a_base-policy_transaction`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/add,_edit_and_lift_an_account's_rule_with_its_history`
- **Break (2):** the UI's update on policy_rules covers every column rather than the domain suffixes, so an edit could move a rule between scopes or rename it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_class`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_created_at`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_created_by`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_rule_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_source`

## The UI's role writes exactly what its decisions and setups write, and reads a client's secret only for its one opening part

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the UI's role loses the read of a client's sealed secret its opening part needs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_ui/reads_every_client`
- **Break (2):** the UI's update grant on account_state covers every column rather than the setups' five
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_backfill_pass1_complete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_backfill_pass2_complete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_sync_cursor_at`

## The UI's role writes only its decision columns and policy rules

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** the UI's insert grant lands on policy_candidates rather than policy_rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAPolicyRuleWriteNamesTheTransactionsAccount`, `TestAPolicyRuleWriteNamesTheTransactionsAccount/the_UI_inserts_a_base_rule`, `TestAPolicyRuleWriteNamesTheTransactionsAccount/the_UI_inserts_its_account's_rule`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/confirm_a_candidate`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_candidates/insert`
- **Break (2):** the UI's update grant on reorg_plans covers every column rather than the three its verb sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_apply_validation`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_created_at`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_description`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_plan`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_plan_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_proposer`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_refusal_reason`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_validation`
