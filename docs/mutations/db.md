# Mutations: db

The demonstrations of the controls whose patches sit in `db/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A base-policy transaction names no account

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1
- **Break:** RunBase neither sets the account empty nor reads it back, so a base-policy transaction runs under whatever account the connection's session holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/tx`:** `TestRunBaseEmptiesAnAccountTheConnectionHeld`, `TestRunBaseWritesTheBasePolicyAndReadsNoAccount`

## A binary refuses a schema whose compatibility floor is above its chain or that has not applied its chain, and runs on every other

- **Date · evidence:** 2026-11-12 · [pull request #421](https://github.com/ppat/mediated-mailbox-mcp/pull/421), at RAPID_SEED=1 · and every break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), and every break again on 2026-11-17 at RAPID_SEED=1, after the process package, with the scheduler and the database code moved into the process family, changed the code, tests or patches the row rests on · [pull request #453](https://github.com/ppat/mediated-mailbox-mcp/pull/453), and every break again on 2026-11-17 at RAPID_SEED=1, after the ping report and its tests were added to `process/database`, which changed the tests the row rests on · [pull request #454](https://github.com/ppat/mediated-mailbox-mcp/pull/454)
- **Break (1):** a floor above the chain is not refused, so a binary runs on a schema a migration it cannot run on left
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrations`:** `TestABinaryRefusesASchemaItsChainCannotRunOn`, `TestABinaryRefusesASchemaItsChainCannotRunOn/a_newer_schema_whose_floor_is_above_its_chain`, `TestABinaryRefusesASchemaItsChainCannotRunOn/the_floor_one_above_its_chain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestTheSchemaCheckRefusesASchemaTheChainCannotRunOn`
- **Break (2):** a floor at the chain is refused as well, so a binary refuses the schema its own release raised the floor to
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrations`:** `TestABinaryRefusesASchemaItsChainCannotRunOn`, `TestABinaryRefusesASchemaItsChainCannotRunOn/a_newer_schema_whose_floor_its_chain_meets`, `TestABinaryRefusesASchemaItsChainCannotRunOn/the_floor_at_its_chain`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestTheSchemaCheckRefusesASchemaTheChainCannotRunOn`
- **Break (3):** a database that has not applied the chain is not refused, so a binary runs before its release's migration step
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrations`:** `TestABinaryRefusesASchemaItsChainCannotRunOn`, `TestABinaryRefusesASchemaItsChainCannotRunOn/nothing_applied`, `TestABinaryRefusesASchemaItsChainCannotRunOn/the_chain's_newest_migration_not_applied`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestGetChecksTheSchemaAndForMigrationDoesNot`, `TestTheSchemaCheckRefusesASchemaTheChainCannotRunOn`
- **Break (4):** a newer schema whose floor the chain meets is refused as well, so a rollback across migrations that left the floor alone does not serve
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrations`:** `TestABinaryRefusesASchemaItsChainCannotRunOn`, `TestABinaryRefusesASchemaItsChainCannotRunOn/a_newer_schema_whose_floor_its_chain_meets`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestTheSchemaCheckRefusesASchemaTheChainCannotRunOn`
- **Break (5):** the chain's version is its oldest migration rather than its newest, so a binary takes itself for older than it is
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrations`:** `TestABinaryRefusesASchemaItsChainCannotRunOn`, `TestABinaryRefusesASchemaItsChainCannotRunOn/a_newer_schema_whose_floor_its_chain_meets`, `TestABinaryRefusesASchemaItsChainCannotRunOn/the_chain's_newest_migration_not_applied`, `TestABinaryRefusesASchemaItsChainCannotRunOn/the_floor_at_its_chain`, `TestTheChainsVersionIsItsNewestMigration`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/process/database`:** `TestGetChecksTheSchemaAndForMigrationDoesNot`, `TestTheSchemaCheckRefusesASchemaTheChainCannotRunOn`

## A message is stored refused only with the size bound it was refused under, and in no other state with one

- **Date · evidence:** 2026-10-13 · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), at RAPID_SEED=1, and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
- **Break (1):** the check holds a bound to the refused state but lets a refused message be stored with none
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestARefusedMessageCarriesTheBoundItWasRefusedUnder`
- **Break (2):** the bound is unchecked, so a refused message is stored with none and a pending one with one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestARefusedMessageCarriesTheBoundItWasRefusedUnder`

## A sender domain is stored in the normalizer's form

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and break 2 again on 2026-10-13 at RAPID_SEED=1, its patch regenerated after the comment beside the sender address in the baseline schema changed · [pull request #373](https://github.com/ppat/mediated-mailbox-mcp/pull/373), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
- **Break (1):** a candidate's domain is refused only when every letter is a capital, so a domain holding one capital is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestASenderDomainIsStoredInTheNormalizersForm`
- **Break (2):** a message's sender domain is not checked, so a domain holding an ASCII capital is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestASenderDomainIsStoredInTheNormalizersForm`
- **Break (3):** the sender statistics' domain is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestASenderDomainIsStoredInTheNormalizersForm`

## A statement cannot read a column the schema does not hold

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
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

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow · and every break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451)
- **Break (1):** the account's reference pairs its provider with the client's name and its client with the client's provider, so it is refused for the account's own provider's client
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAnAccountCannotNameAnotherProvidersClient`, `TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt`
- **Break (2):** the account's reference names the client alone, so the client's provider is never compared with the account's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAnAccountCannotNameAnotherProvidersClient`

## An OAuth client an account connects through cannot be removed

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
- **Break (1):** the account's reference to its client is dropped, so nothing refuses removing a client an account connects through
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAnAccountCannotNameAnotherProvidersClient`
- **Break (2):** removing a client an account connects through clears the account's client rather than being refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`

## Every log table references what it belongs to

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1, and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow · and breaks (6) and (10) again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451)
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

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1, and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse every statement kind it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse an account-keyed table in any clause it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after a common table expression named like a table stopped hiding the table from the check · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the statement check came to follow a FROM-list subquery's output for its cast and paged-sort checks · [pull request #373](https://github.com/ppat/mediated-mailbox-mcp/pull/373), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
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

## One OAuth client is never stored twice

- **Date · evidence:** 2026-10-02 · [pull request #255](https://github.com/ppat/mediated-mailbox-mcp/pull/255), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow · and every break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451)
- **Break (1):** the uniqueness of a provider's client identifier is dropped, so one client may be stored under two names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`
- **Break (2):** a client identifier is unique across providers, so another provider's client holding the same identifier is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`, `TestTheApplicationUserReadsAndWritesTheOAuthClients`
- **Break (3):** a client's name is unique only within its provider, so a client of another provider may take a name a client holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOneClientIsNeverStoredTwice`

## Only a scanned message carries content flags

- **Date · evidence:** 2026-10-13 · [pull request #372](https://github.com/ppat/mediated-mailbox-mcp/pull/372), and every break again on 2026-10-13 at RAPID_SEED=1, with its patches regenerated after the baseline schema's index list changed · [pull request #373](https://github.com/ppat/mediated-mailbox-mcp/pull/373), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
- **Break (1):** the check admits flags on a pending message too, so a message returned to pending with its flags kept is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOnlyAScannedMessageCarriesContentFlags`
- **Break (2):** the check on messages is dropped, so a message returned to pending, or skipped, with its flags kept is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestOnlyAScannedMessageCarriesContentFlags`

## Schema change is confined to the migrator's user

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and break 1 again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394) · and break (1) again on 2026-11-12 at RAPID_SEED=1, after the compatibility floor's grants moved the patches they rest on · [pull request #421](https://github.com/ppat/mediated-mailbox-mcp/pull/421) · and every break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), and renamed from "Schema change is confined to the migration role" on 2026-11-17
- **Break (1):** the chain grants the application user the right to create objects in the schema
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheApplicationUserCannotChangeTheSchema`, `TestTheApplicationUserCannotChangeTheSchema/create_a_function`, `TestTheApplicationUserCannotChangeTheSchema/create_a_table`
- **Break (2):** the bootstrap makes the application user a member of the migrator's user, so it acts as the schema's owner
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheApplicationUserCannotAlterTheAuditLog`, `TestTheApplicationUserCannotAlterTheAuditLog/delete`, `TestTheApplicationUserCannotAlterTheAuditLog/truncate`, `TestTheApplicationUserCannotAlterTheAuditLog/update`, `TestTheApplicationUserCannotAlterThePolicyHistory`, `TestTheApplicationUserCannotAlterThePolicyHistory/delete`, `TestTheApplicationUserCannotAlterThePolicyHistory/truncate`, `TestTheApplicationUserCannotAlterThePolicyHistory/update`, `TestTheApplicationUserCannotChangeTheSchema`, `TestTheApplicationUserCannotChangeTheSchema/add_a_body_column`, `TestTheApplicationUserCannotChangeTheSchema/create_a_policy`, `TestTheApplicationUserCannotChangeTheSchema/drop_a_table`, `TestTheApplicationUserCannotChangeTheSchema/enable_row_security`, `TestTheApplicationUserReadsAndWritesTheOAuthClients`, `TestTheApplicationUserReadsAndWritesTheOAuthClients/changes_a_client's_name_or_provider`, `TestTheBootstrapCreatesTheTwoUsers`

## The application user cannot update or delete a row of the policy history

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and the break again on 2026-10-10 at RAPID_SEED=1, with its patch regenerated after the grants file's comments around it moved · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394) · and the break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), and renamed from "No runtime role can update or delete a row of the policy history" on 2026-11-17
- **Break:** the application user gains update and delete on the policy history, so a compromised process can rewrite who lifted a restriction
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheApplicationUserCannotAlterThePolicyHistory`, `TestTheApplicationUserCannotAlterThePolicyHistory/delete`, `TestTheApplicationUserCannotAlterThePolicyHistory/update`

## The application user cannot update or delete an audit row

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153), and every break again on 2026-10-09, over the schema baseline that flattened the migration chain · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394) · and breaks (1) and (2) again on 2026-11-12 at RAPID_SEED=1, after the compatibility floor's grants moved the patches they rest on · [pull request #421](https://github.com/ppat/mediated-mailbox-mcp/pull/421) · and every break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), and renamed from "No runtime role can update or delete an audit row" on 2026-11-17
- **Break (1):** the chain grants DELETE on the audit log to every role
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestMigrationAndBootstrapFiles`, `TestTheApplicationUserCannotAlterTheAuditLog`, `TestTheApplicationUserCannotAlterTheAuditLog/delete`
- **Break (2):** the chain grants the application user TRUNCATE on the audit log, an erasing operation other than update and delete
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheApplicationUserCannotAlterTheAuditLog`, `TestTheApplicationUserCannotAlterTheAuditLog/truncate`
- **Break (3):** the chain grants the application user UPDATE on the audit log
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheApplicationUserCannotAlterTheAuditLog`, `TestTheApplicationUserCannotAlterTheAuditLog/update`

## The application user lists every account and reads one account's state in the statements' form

- **Date · evidence:** 2026-11-17 at RAPID_SEED=1 · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), taking over the breaks that still hold under one application user of the retired rows "The listing roles read every account's identifier, provider and client, and the heuristics job's role reads none" and "Only the roles that call a provider read oauth_clients and their own account's state, and the UI reads its account's progress and never a credential"
- **Break (1):** the application user loses the write of an account's credential that the connections' write-back needs
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheApplicationUserListsEveryAccountAndReadsOneState`, `TestTheApplicationUserListsEveryAccountAndReadsOneState/the_credential's_write-back`, `TestTheChainGrantsTheNameTheMigratorIsGiven`
- **Break (2):** the application user loses the accounts listing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheApplicationUserListsEveryAccountAndReadsOneState`, `TestTheApplicationUserListsEveryAccountAndReadsOneState/the_listing`, `TestTheChainGrantsTheNameTheMigratorIsGiven`

## The application user reads every OAuth client and writes what the client setup and the re-seal write

- **Date · evidence:** 2026-11-17 at RAPID_SEED=1 · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), taking over the breaks that still hold under one application user of the retired rows "Only the roles that call a provider read oauth_clients and their own account's state, and the UI reads its account's progress and never a credential" and "the UI's role writes exactly what its decisions and setups write, and reads a client's secret only for its one opening part"
- **Break (1):** the application user loses the read of each client's name
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheApplicationUserReadsAndWritesTheOAuthClients`, `TestTheApplicationUserReadsAndWritesTheOAuthClients/reads_every_client`, `TestTheApplicationUserReadsAndWritesTheOAuthClients/sets_up,_re-seals_and_removes_a_client`, `TestTheChainGrantsTheNameTheMigratorIsGiven`
- **Break (2):** the application user's update on oauth_clients covers every column, so a client's name or provider can be changed with no statement that uses the grant
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheApplicationUserReadsAndWritesTheOAuthClients`, `TestTheApplicationUserReadsAndWritesTheOAuthClients/changes_a_client's_name_or_provider`
- **Break (3):** the application user loses the write of a client's secret that the client setup's replacement and the connections' re-seal need
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheApplicationUserReadsAndWritesTheOAuthClients`, `TestTheApplicationUserReadsAndWritesTheOAuthClients/sets_up,_re-seals_and_removes_a_client`, `TestTheChainGrantsTheNameTheMigratorIsGiven`
- **Break (4):** the application user loses the read of a client's sealed secret that the connections and the UI's one opening part need
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheApplicationUserReadsAndWritesTheOAuthClients`, `TestTheApplicationUserReadsAndWritesTheOAuthClients/reads_every_client`, `TestTheApplicationUserReadsAndWritesTheOAuthClients/sets_up,_re-seals_and_removes_a_client`, `TestTheChainGrantsTheNameTheMigratorIsGiven`

## The attachment media at rest hold only the normalized form

- **Date · evidence:** 2026-10-10 · [pull request #339](https://github.com/ppat/mediated-mailbox-mcp/pull/339), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
- **Break (1):** a stored extension is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAttachmentMediaHoldsOnlyTheNormalizedForm`
- **Break (2):** a stored media type is checked without regard to case, so one in capitals is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAttachmentMediaHoldsOnlyTheNormalizedForm`
- **Break (3):** a stored media type is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAttachmentMediaHoldsOnlyTheNormalizedForm`

## The audit row's closed columns refuse a value outside their sets, and a body decision's row names what it rests on

- **Date · evidence:** 2026-10-11 · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, over the index that stores no sender class and the body fetch by part, which changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow · and every break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451)
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
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestAClientAnAccountConnectsThroughCannotBeRemoved`, `TestAMessageOrRunRemovedTakesItsPartsWithIt`, `TestARefusedMessageCarriesTheBoundItWasRefusedUnder`, `TestARunRecordsOneOfTheBuiltJobKindsPairs`, `TestASenderDomainIsStoredInTheNormalizersForm`, `TestAnAccountCannotNameAnotherProvidersClient`, `TestAnAccountWithEvidenceCannotBeDeleted`, `TestAttachmentMediaHoldsOnlyTheNormalizedForm`, `TestEachCheckedColumnRefusesAValueOutsideItsSet`, `TestEveryLogTableRefusesAnOrphan`, `TestNoAccountIdentifierIsAPathSegmentThePathCannotHold`, `TestOneClientIsNeverStoredTwice`, `TestOnlyAScannedMessageCarriesContentFlags`, `TestTheApplicationUserCannotAlterTheAuditLog`, `TestTheApplicationUserCannotAlterThePolicyHistory`, `TestTheApplicationUserCannotChangeTheSchema`, `TestTheApplicationUserListsEveryAccountAndReadsOneState`, `TestTheApplicationUserReadsAndWritesTheOAuthClients`, `TestTheAuditRowsColumnsRefuseAValueOutsideTheirSets`, `TestTheOperationLogIsReachedThroughItsPlan`, `TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged`, `TestTheTransactionsAccountScopesEveryAccountTable`
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

## The chain the binary embeds is every SQL file of the chain's directory and nothing else

- **Date · evidence:** 2026-10-15 · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), at RAPID_SEED=1
- **Break (1):** every file of the directory is embedded, so files that are not migrations reach the chain goose reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrations`:** `TestTheEmbeddedChainIsTheDirectory`
- **Break (2):** only the baseline's three files are embedded, so the migrations after it never reach the binary
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrations`:** `TestTheEmbeddedChainIsTheDirectory`

## The checked vocabularies refuse a value outside their closed set

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-10, after the test's messages lost the attachment types column the schema no longer holds · [pull request #339](https://github.com/ppat/mediated-mailbox-mcp/pull/339), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), with the breaks of the message's and the sender statistics' stored class retired, since neither column exists, and every remaining break again on 2026-10-13 at RAPID_SEED=1, after the index stopped storing a sender's class changed the tests the row rests on · [pull request #372](https://github.com/ppat/mediated-mailbox-mcp/pull/372), and every break again on 2026-10-13 at RAPID_SEED=1, after the body fetch by part and the refused scan state changed the code, tests or patches the row rests on · [pull request #369](https://github.com/ppat/mediated-mailbox-mcp/pull/369), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow · and breaks (1) and (2) again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451)
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
- **Break (10):** a plan's status is checked without regard to case, so a status spelled in lowercase is stored
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (11):** the policy history's action is not checked, so a wrong spelling is stored in an append-only table
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (12):** a policy rule's class is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (13):** a policy rule's source is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (14):** a run's state is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`
- **Break (15):** the scan state's set leaves out skipped_gate, so a state the gate writes is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestEachCheckedColumnRefusesAValueOutsideItsSet`, `TestOnlyAScannedMessageCarriesContentFlags`

## The grant check plans every statement an import list admits under the application user

- **Date · evidence:** 2026-11-17 at RAPID_SEED=1 · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451), taking over the breaks that still hold under one application user of the retired rows "A shared library's statements are planned under the role of each deployable and job kind that admits it", "Each deployable's and each job kind's import list names only subsections its role is granted" and "The UI's role writes exactly the policy writes policy management makes, each with its history row"
- **Break (1):** the application user may read and write the rate limiter's grants but not delete them, which the rate limiter's statements, admitted by a shared library's list, do
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheChainGrantsTheNameTheMigratorIsGiven`
- **Break (2):** the application user loses the delete on policy_rules, so policy management's lift of a rule, which the UI's list admits, is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheChainGrantsTheNameTheMigratorIsGiven`
- **Break (3):** the application user is not granted the read of the runs' progress events, which the latest recorded success backfill's list admits reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestGrantsCoverAdmittedSubsections`, `TestTheChainGrantsTheNameTheMigratorIsGiven`

## The identifier grammar's migration stops over a stored account outside it and rewrites none

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-15 at RAPID_SEED=1, after the migrator's configuration and goose's move into the migrator's binary changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397)
- **Break (1):** the grammar's check is added without validating the stored accounts, so the migration applies over an account outside it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt`
- **Break (2):** the migration removes each stored account outside the grammar before adding its check, so a system-of-record row is lost
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt`

## The operation log is reached only through a plan tied to the transaction's account

- **Date · evidence:** 2026-10-13 · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), at RAPID_SEED=1, and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse every statement kind it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the check came to refuse an account-keyed table in any clause it does not read · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after a common table expression named like a table stopped hiding the table from the check · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the statement check came to follow a FROM-list subquery's output for its cast and paged-sort checks · [pull request #373](https://github.com/ppat/mediated-mailbox-mcp/pull/373), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow
- **Break (1):** the operation log's count checks one plan against the transaction's account but reads the log of a second plan, named by another parameter, so a plan of the account opens any account's log
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`
- **Break (2):** the operation log's count reads the log by the plan's identifier alone, with no join to a plan tied to the transaction's account
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestStatementFiles`

## The recorded authentication outcome is the latest attempt, and recording an older one changes nothing

- **Date · evidence:** 2026-10-01 · [pull request #227](https://github.com/ppat/mediated-mailbox-mcp/pull/227), and every break again on 2026-10-07, after the account session and the entry packages moved the code, tests or patches the row rests on · [pull request #272](https://github.com/ppat/mediated-mailbox-mcp/pull/272), and every break again on 2026-10-08, after the families moved the code, tests or patches the row rests on · [pull request #317](https://github.com/ppat/mediated-mailbox-mcp/pull/317), and every break again on 2026-10-10, after the worker merged backfill and delta sync into one deployable, which moved the tests the row rests on · [pull request #328](https://github.com/ppat/mediated-mailbox-mcp/pull/328), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-14 and 2026-10-15 at RAPID_SEED=1, after one execution context per process changed the code, tests or patches the row rests on · [pull request #394](https://github.com/ppat/mediated-mailbox-mcp/pull/394) · and breaks (3) and (4) again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451)
- **Break (1):** the statement replaces the stored attempt only with an older one, so a later attempt never replaces an earlier one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestOnlyALaterAttemptIsRecorded`
- **Break (2):** the statement drops its predicate on the stored attempt's time, so the last write wins and an older attempt replaces a later one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestOnlyALaterAttemptIsRecorded`
- **Break (3):** the statement replaces the stored attempt only with an older one, so a later attempt never replaces an earlier one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestOnlyALaterAttemptIsRecorded`
- **Break (4):** the statement drops its predicate on the stored attempt's time, so the last write wins and an older attempt replaces a later one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill`:** `TestOnlyALaterAttemptIsRecorded`

## The run and audit indexes' migration leaves stored rows unchanged and builds every index

- **Date · evidence:** 2026-10-09 · [pull request #326](https://github.com/ppat/mediated-mailbox-mcp/pull/326), and every break again on 2026-10-11, after the typed audit row changed the code, tests, test fixtures or patch context they rest on · [pull request #347](https://github.com/ppat/mediated-mailbox-mcp/pull/347), and every break again on 2026-10-13 at RAPID_SEED=1, after every statement came to read its account from the transaction and the log tables gained their foreign keys · [pull request #370](https://github.com/ppat/mediated-mailbox-mcp/pull/370), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-15 at RAPID_SEED=1, after the migrator's configuration and goose's move into the migrator's binary changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397)
- **Break (1):** the migration marks every running run failed while it builds the indexes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged`
- **Break (2):** the migration leaves out the index of a job kind's runs by pass
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/check`:** `TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged`

## Two runs of the migration chain at once apply each migration once

- **Date · evidence:** 2026-10-15 · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), at RAPID_SEED=1 · and every break again on 2026-11-17 at RAPID_SEED=1, after the two database users changed the code, tests or patches the row rests on · [pull request #451](https://github.com/ppat/mediated-mailbox-mcp/pull/451)
- **Break (1):** the chain is applied with no session locker, so nothing stops a second run reading what is applied while the first applies it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrate`:** `TestTwoRunsAtOnceApplyAMigrationOnce`
- **Break (2):** each run takes an advisory lock of its own key, so two runs never wait for each other
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/db/migrate`:** `TestTwoRunsAtOnceApplyAMigrationOnce`
