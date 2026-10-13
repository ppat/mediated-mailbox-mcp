# Mutations: db

The demonstrations of the controls whose patches sit in `db/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A base-policy transaction names no account

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1
- **Break:** RunBase neither sets the account empty nor reads it back, so a base-policy transaction runs under whatever account the connection's session holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestRunBaseEmptiesAnAccountTheConnectionHeld`, `TestRunBaseWritesTheBasePolicyAndReadsNoAccount`

## A sender domain is stored in the normalizer's form

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** a candidate's domain is refused only when every letter is a capital, so a domain holding one capital is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestASenderDomainIsStoredInTheNormalizersForm`
- **Break (2):** a message's sender domain is not checked, so a domain holding an ASCII capital is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestASenderDomainIsStoredInTheNormalizersForm`
- **Break (3):** the sender statistics' domain is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestASenderDomainIsStoredInTheNormalizersForm`

## A shared library's statements are planned under the role of each deployable and job kind that admits it

- **Date · evidence:** 2026-09-24 · [pull request #157](https://github.com/ppat/mediated-mailbox-mcp/pull/157), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and break (2) again on 2026-10-10 at RAPID_SEED=1, retargeted from the reorganization role, which no import list maps to, to backfill's job kind's role · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and renamed from "A shared library's statements are planned under the role of each deployable that admits it" on 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the spenders' roles may read and write grants but not delete them
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`
- **Break (2):** backfill's job kind's role gets no grant on the rate limiter's tables, which its import list admits
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`

## A statement cannot read a column the schema does not hold

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the chain gives messages a body column, so the schema no longer withholds it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGenerationRefusesAColumnTheSchemaDoesNotHold`, `TestGenerationRefusesAColumnTheSchemaDoesNotHold/body_column_violation.sql`
- **Break (2):** a later migration renames messages.message_id to body, so the schema holds a body column in an identifier's place
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGenerationRefusesAColumnTheSchemaDoesNotHold`, `TestGenerationRefusesAColumnTheSchemaDoesNotHold/body_column_violation.sql`, `TestStatementFiles`
- **Break (3):** a later migration renames messages.subject to body, so the schema holds a body column in the subject's place
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGenerationRefusesAColumnTheSchemaDoesNotHold`, `TestGenerationRefusesAColumnTheSchemaDoesNotHold/body_column_violation.sql`

## A transaction that did not set the account fails before any data access

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153), and every break again on 2026-10-11 at RAPID_SEED=1, after the performance measurements changed the code, tests or patches the row rests on · [pull request #341](https://github.com/ppat/mediated-mailbox-mcp/pull/341), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the helper sets the account for the session rather than the transaction, so it outlives the transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage`, `TestTheAccountEndsWithItsTransaction`
- **Break (2):** the helper refuses a connection as the Beginner rather than a transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage`, `TestRunBaseWritesTheBasePolicyAndReadsNoAccount`, `TestRunRefusesATransaction`, `TestRunScopesTheTransactionToTheAccount`, `TestTheAccountEndsWithItsTransaction`
- **Break (3):** the helper no longer refuses a transaction as the Beginner, so a savepoint's account outlives it in the outer transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestRunRefusesATransaction`
- **Break (4):** the helper no longer compares the setting it reads back with the account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage`

## An account cannot name another provider's OAuth client

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the account's reference pairs its provider with the client's name and its client with the client's provider, so it is refused for the account's own provider's client
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAnAccountCannotNameAnotherProvidersClient`, `TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/set_up_a_client,_connect_an_account_and_re-authorize_it`
- **Break (2):** the account's reference names the client alone, so the client's provider is never compared with the account's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAnAccountCannotNameAnotherProvidersClient`

## An OAuth client an account connects through cannot be removed

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the account's reference to its client is dropped, so nothing refuses removing a client an account connects through
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAnAccountCannotNameAnotherProvidersClient`
- **Break (2):** removing a client an account connects through clears the account's client rather than being refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`

## Each deployable's and each job kind's import list names only subsections its role is granted

- **Date · evidence:** 2026-10-10 · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and break (2) on 2026-10-10 at RAPID_SEED=1, added with the grant of the read of its runs that a job's latest recorded success needs · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** backfill's job kind's import list admits the re-seal of a client's secret, which backfill's role is not granted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`
- **Break (2):** backfill's role is not granted the read of its runs' progress events, which the latest recorded success its list admits reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`

## Every log table references what it belongs to

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1
- **Break (1):** the masking events lose their key to accounts, so an event of an account that does not exist is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEveryLogTableRefusesAnOrphan`, `TestEveryLogTableRefusesAnOrphan/a_masking_event`
- **Break (2):** a run's events' key refuses the run's removal rather than removing its events
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAMessageOrRunRemovedTakesItsPartsWithIt`
- **Break (3):** the audit log's key removes an account's audit rows with the account rather than refusing its deletion
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAnAccountWithEvidenceCannotBeDeleted`
- **Break (4):** the candidates lose their key to accounts, so a candidate of an account that does not exist is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEveryLogTableRefusesAnOrphan`, `TestEveryLogTableRefusesAnOrphan/a_candidate`
- **Break (5):** the gate decisions lose their key to messages, so a decision for a message the index does not hold is stored and a removed message leaves its decision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAMessageOrRunRemovedTakesItsPartsWithIt`, `TestEveryLogTableRefusesAnOrphan`, `TestEveryLogTableRefusesAnOrphan/a_gate_decision`, `TestEveryLogTableRefusesAnOrphan/a_gate_decision_for_another_account's_message`
- **Break (6):** a run's failures lose their key to the run, so a failure of a run never recorded is stored and a removed run leaves its failures
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAMessageOrRunRemovedTakesItsPartsWithIt`, `TestEveryLogTableRefusesAnOrphan`, `TestEveryLogTableRefusesAnOrphan/a_run's_failure`
- **Break (7):** the audit log loses its key to accounts, so an audit row of an account that does not exist is stored and an account with audit rows is deleted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAnAccountWithEvidenceCannotBeDeleted`, `TestEveryLogTableRefusesAnOrphan`, `TestEveryLogTableRefusesAnOrphan/an_audit_row`
- **Break (8):** the gate decisions' key to messages refuses the message's removal rather than removing its decision
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAMessageOrRunRemovedTakesItsPartsWithIt`
- **Break (9):** a run's events lose their key to the run, so an event of a run never recorded is stored and a removed run leaves its events
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAMessageOrRunRemovedTakesItsPartsWithIt`, `TestEveryLogTableRefusesAnOrphan`, `TestEveryLogTableRefusesAnOrphan/a_run's_event`
- **Break (10):** a run's failures' key refuses the run's removal rather than removing its failures
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAMessageOrRunRemovedTakesItsPartsWithIt`

## Every statement takes its account from the transaction

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1, and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse every statement kind it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse an account-keyed table in any clause it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after a common table expression named like a table stopped hiding the table from the check · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the gate decision's insert takes its account from a parameter instead of the transaction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`
- **Break (2):** a statement of the sender listing reads the account with the missing-ok argument, so an unset account reads as null rather than raising
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`
- **Break (3):** a statement of the sender listing compares the account with a parameter instead of the transaction's account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`
- **Break (4):** delta sync's update of a stored message's labels and flags also sets its account column, so a statement could move a row to another account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`
- **Break (5):** delta sync's update of a stored message's labels and flags is written as a merge matching the message by its identifier alone, a statement kind the check does not read, so every account's message of that identifier is updated
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`
- **Break (6):** the sender listing's page size is capped by a count of every account's messages in its LIMIT, a clause whose queries the check does not otherwise read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`

## No runtime role can update or delete a row of the policy history

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and the break again on 2026-10-10 at RAPID_SEED=1, with its patch regenerated after the grants file's comments around it moved · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break:** the UI's role gains update and delete on the policy history, so a compromised UI can rewrite who lifted a restriction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterThePolicyHistory`, `TestNoRuntimeRoleCanAlterThePolicyHistory/mediated_mailbox_ui/delete`, `TestNoRuntimeRoleCanAlterThePolicyHistory/mediated_mailbox_ui/update`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/delete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_action`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_actor`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_rule_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_suffixes_after`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_suffixes_before`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_changes/update_ts`

## No runtime role can update or delete an audit row

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the chain grants DELETE on the audit log to every role
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_backfill/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_organize/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_propose/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_sync/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_ui/delete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/audit_log/delete`
- **Break (2):** the chain grants the reorganization workload's role TRUNCATE on the audit log, an erasing operation other than update and delete
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_organize/truncate`
- **Break (3):** the chain grants the mediator's role UPDATE on the audit log
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/update`

## One OAuth client is never stored twice

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the uniqueness of a provider's client identifier is dropped, so one client may be stored under two names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`
- **Break (2):** a client identifier is unique across providers, so another provider's client holding the same identifier is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`
- **Break (3):** a client's name is unique only within its provider, so a client of another provider may take a name a client holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`

## Only the roles that call a provider read oauth_clients and their own account's state, and the UI reads its account's progress and never a credential

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** delta sync's role loses the read and write of its account's credential that its snapshot's statements need
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheProviderCallingRolesReadTheirAccountsState`, `TestTheProviderCallingRolesReadTheirAccountsState/mediated_mailbox_sync/reads_its_own_account's_state`, `TestTheProviderCallingRolesReadTheirAccountsState/mediated_mailbox_sync/updates_its_own_account's_credential`
- **Break (2):** the UI's role reads and writes every account's credential column
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheProviderCallingRolesReadTheirAccountsState`, `TestTheProviderCallingRolesReadTheirAccountsState/mediated_mailbox_ui/reads_a_credential`
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
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheProviderCallingRolesReadTheirAccountsState`, `TestTheProviderCallingRolesReadTheirAccountsState/mediated_mailbox_ui/reads_a_credential`
- **Break (9):** the UI's role loses the read of the progress columns its statement in db/accountstate needs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheProviderCallingRolesReadTheirAccountsState`, `TestTheProviderCallingRolesReadTheirAccountsState/mediated_mailbox_ui/reads_its_own_account's_progress`

## Only the UI's role writes the policy

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1
- **Break (1):** the heuristics job's role is granted the insert on the policy history, so it could record a change to the base policy
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOnlyTheUIWritesThePolicy`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_propose/record_a_base_change`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_propose/record_an_account_change`
- **Break (2):** the roles that load policy are granted every write of policy_rules beside their read, so backfill, delta sync and the mediator could write base rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOnlyTheUIWritesThePolicy`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_backfill/edit_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_backfill/insert_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_backfill/insert_an_account's_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_backfill/lift_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/edit_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/insert_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/insert_an_account's_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/lift_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_sync/edit_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_sync/insert_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_sync/insert_an_account's_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_sync/lift_a_base_rule`

## Schema change is confined to the migration role

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the chain grants a runtime role the right to create objects in the schema
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestRuntimeRolesCannotChangeTheSchema`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_backfill/create_a_function`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_backfill/create_a_table`
- **Break (2):** the bootstrap makes a runtime role a member of the migration role, so it acts as the schema's owner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/delete`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/truncate`, `TestNoRuntimeRoleCanAlterTheAuditLog/mediated_mailbox_mediate/update`, `TestNoRuntimeRoleCanAlterThePolicyHistory`, `TestNoRuntimeRoleCanAlterThePolicyHistory/mediated_mailbox_mediate/delete`, `TestNoRuntimeRoleCanAlterThePolicyHistory/mediated_mailbox_mediate/truncate`, `TestNoRuntimeRoleCanAlterThePolicyHistory/mediated_mailbox_mediate/update`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_mediate/writes_a_client`, `TestOnlyTheUIWritesThePolicy`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/edit_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/insert_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/insert_an_account's_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/lift_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/record_a_base_change`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_mediate/record_an_account_change`, `TestRuntimeRolesCannotChangeTheSchema`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/add_a_body_column`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/change_a_table_owner`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/create_a_policy`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/drop_a_table`, `TestRuntimeRolesCannotChangeTheSchema/mediated_mailbox_mediate/enable_row_security`, `TestTheBootstrapCreatesOneRuntimeRolePerDeployable`

## The attachment media at rest hold only the normalized form

- **Date · evidence:** 2026-10-10 · [pull request #339](https://github.com/ppat/mediated-mailbox-mcp/pull/339), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** a stored extension is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAttachmentMediaHoldsOnlyTheNormalizedForm`
- **Break (2):** a stored media type is checked without regard to case, so one in capitals is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAttachmentMediaHoldsOnlyTheNormalizedForm`
- **Break (3):** a stored media type is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAttachmentMediaHoldsOnlyTheNormalizedForm`

## The audit row's closed columns refuse a value outside their sets, and a body decision's row names what it rests on

- **Date · evidence:** 2026-10-11 · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the audit row's action is unchecked, so a spelling outside its set is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (2):** every row needs a reason, sender class and scan state, so a mutation's row is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (3):** a body decision's row is stored with no reason, sender class or scan state
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (4):** a body decision's row is stored with no scan state
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (5):** a class rule's identifier is stored without its scope and a scope without an identifier
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (6):** the audit row's content flags are checked to overlap the set rather than to sit inside it, so an unknown flag beside a known one is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (7):** the audit row's reason set leaves out released, so a release's row is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAMessageOrRunRemovedTakesItsPartsWithIt`, `TestARunRecordsOneOfTheBuiltJobKindsPairs`, `TestASenderDomainIsStoredInTheNormalizersForm`, `TestAnAccountCannotNameAnotherProvidersClient`, `TestAnAccountWithEvidenceCannotBeDeleted`, `TestAttachmentMediaHoldsOnlyTheNormalizedForm`, `TestEachCheckedColumnRefusesAValueOutsideItsSet`, `TestEveryLogTableRefusesAnOrphan`, `TestNoAccountIdentifierIsAPathSegmentThePathCannotHold`, `TestNoRuntimeRoleCanAlterTheAuditLog`, `TestNoRuntimeRoleCanAlterThePolicyHistory`, `TestOneClientIsNeverStoredTwice`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheUIWritesThePolicy`, `TestRuntimeRolesCannotChangeTheSchema`, `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`, `TestTheOperationLogIsReachedThroughItsPlan`, `TestTheProviderCallingRolesReadTheirAccountsState`, `TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged`, `TestTheTransactionsAccountScopesEveryAccountTable`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`
- **Break (8):** the audit row's reason is unchecked, so a spelling no token names is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (9):** the audit row's scan state set leaves out skipped_gate, so a gate-skipped message's row is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`, `TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged`
- **Break (10):** the class rule's scope is unchecked, so a scope outside base and account is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (11):** a scope is stored without a class rule's identifier, while an identifier still needs its scope
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (12):** the audit row's sender class is unchecked, so a class outside normal and restricted is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`
- **Break (13):** the audit row's stage is unchecked, so a stage outside gate and serve is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`

## The checked vocabularies refuse a value outside their closed set

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-10, after the test's messages lost the attachment types column the schema no longer holds · [pull request #339](https://github.com/ppat/mediated-mailbox-mcp/pull/339), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the job kind and the pass are each checked against the words alone, so a pair no job kind records is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestARunRecordsOneOfTheBuiltJobKindsPairs`
- **Break (2):** a run's job kind and pass are not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestARunRecordsOneOfTheBuiltJobKindsPairs`
- **Break (3):** a run's pass may be null
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestARunRecordsOneOfTheBuiltJobKindsPairs`
- **Break (4):** the last authentication's outcome is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (5):** a candidate's status is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (6):** the content flags are checked to overlap the set rather than to sit inside it, so an unknown flag beside a known one is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (7):** the gate's decision is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (8):** a rate grant's class is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (9):** a masking event's field is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (10):** a message's stored sender class is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (11):** a plan's status is checked without regard to case, so a status spelled in lowercase is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (12):** the policy history's action is not checked, so a wrong spelling is stored in an append-only table
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (13):** a policy rule's class is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (14):** a policy rule's source is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (15):** a run's state is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (16):** the scan state's set leaves out skipped_gate, so a state the gate writes is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (17):** the sender statistics' class is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`

## The identifier grammar's migration stops over a stored account outside it and rewrites none

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the grammar's check is added without validating the stored accounts, so the migration applies over an account outside it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt`
- **Break (2):** the migration removes each stored account outside the grammar before adding its check, so a system-of-record row is lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt`

## The listing roles read every account's identifier, provider and client, and the heuristics job's role reads none

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1
- **Break (1):** the heuristics job's role is granted the accounts listing, which no decided consumer of it needs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheListingRolesReadEveryAccount`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_propose`
- **Break (2):** backfill's role loses the accounts listing its account snapshot reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheListingRolesReadEveryAccount`, `TestTheListingRolesReadEveryAccount/mediated_mailbox_backfill`

## The operation log is reached only through a plan tied to the transaction's account

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1, and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse every statement kind it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse an account-keyed table in any clause it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after a common table expression named like a table stopped hiding the table from the check · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the operation log's count checks one plan against the transaction's account but reads the log of a second plan, named by another parameter, so a plan of the account opens any account's log
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`
- **Break (2):** the operation log's count reads the log by the plan's identifier alone, with no join to a plan tied to the transaction's account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`

## The recorded authentication outcome is the latest attempt, and recording an older one changes nothing

- **Date · evidence:** 2026-10-01 · [pull request #227](https://github.com/ppat/mediated-mailbox-mcp/pull/227), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10, after the worker merged backfill and delta sync into one deployable, which moved the tests the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the statement replaces the stored attempt only with an older one, so a later attempt never replaces an earlier one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestOnlyALaterAttemptIsRecorded`
- **Break (2):** the statement drops its predicate on the stored attempt's time, so the last write wins and an older attempt replaces a later one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestOnlyALaterAttemptIsRecorded`

## The run and audit indexes' migration leaves stored rows unchanged and builds every index

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the migration marks every running run failed while it builds the indexes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged`
- **Break (2):** the migration leaves out the index of a job kind's runs by pass
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged`

## The UI's role writes exactly the policy writes policy management makes, each with its history row

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the UI's role loses the delete on policy_rules, so lifting a rule is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestOnlyTheUIWritesThePolicy`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_ui/lift_a_base_rule`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/add,_edit_and_lift_an_account's_rule_with_its_history`
- **Break (2):** the UI's update on policy_rules covers every column rather than the domain suffixes, so an edit could move a rule between scopes or rename it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_class`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_created_at`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_created_by`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_rule_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_rules/update_source`

## The UI's role writes exactly what its decisions and setups write, and reads a client's secret only for its one opening part

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the UI's role loses the read of a client's sealed secret its opening part needs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients`, `TestOnlyTheProviderCallingRolesReadTheOAuthClients/mediated_mailbox_ui/reads_every_client`
- **Break (2):** the UI's update grant on account_state covers every column rather than the setups' five
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_backfill_pass1_complete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_backfill_pass2_complete`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/account_state/update_sync_cursor_at`

## The UI's role writes only its decision columns and policy rules

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370)
- **Break (1):** the UI's insert grant lands on policy_candidates rather than policy_rules
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestOnlyTheUIWritesThePolicy`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_ui/insert_a_base_rule`, `TestOnlyTheUIWritesThePolicy/mediated_mailbox_ui/insert_an_account's_rule`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/add,_edit_and_lift_an_account's_rule_with_its_history`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/confirm_a_candidate`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/policy_candidates/insert`
- **Break (2):** the UI's update grant on reorg_plans covers every column rather than the three its verb sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_account_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_apply_validation`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_created_at`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_description`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_plan`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_plan_id`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_proposer`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_refusal_reason`, `TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules/reorg_plans/update_validation`
