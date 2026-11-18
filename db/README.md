# db

The data-access library, published as `mediated-mailbox-db`
([ADR-0047](../docs/adr/data/0047-schema-first-data-access.md)). The conventions it shares with
every component are [CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

| Path | Holds |
| --- | --- |
| `db/migrations/` | The migration chain ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)). A baseline of three files, the one extension the schema needs, which the migrator's user creates since it is trusted, every table in [ADR-0016](../docs/adr/data/0016-schema.md)'s order, and the grants in one section, the application user's ([ADR-0143](../docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)), then the migrations after it, each tested over rows the chain before it wrote. The schema's file ends with the compatibility floor, and every migration after the baseline declares that every chain the floor admits runs on it, in the line `-- compatibility: every chain the floor admits runs on this migration`, or raises the floor ([ADR-0140](../docs/adr/data/0140-each-migration-declares-the-oldest-chain-that-runs-on-it.md)). Beside the SQL files sit the Go files, which embed them, so the migrator's binary carries the chain and the integration runs apply the same files ([ADR-0139](../docs/adr/data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md)), and which give the chain's version, its newest migration's, and the start check's comparison of it with the schema's, which `process/database` runs on each pool of a process that runs until it is told to stop ([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md), [ADR-0140](../docs/adr/data/0140-each-migration-declares-the-oldest-chain-that-runs-on-it.md)). The generator reads only the SQL files as its schema |
| `db/migrate/` | The application of a migration chain with goose linked as a library, the one package goose is admitted to. It applies every migration the database does not yet hold, after taking goose's PostgreSQL advisory lock on a connection of its own, so a second run at the same time waits and then applies nothing. It reads the chain with the application user's name it is given, quoted as an identifier, in place of the placeholder every grant names ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)). The migrator applies the embedded chain through it, and so does every integration run ([ADR-0139](../docs/adr/data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md)) |
| `db/bootstrap/` | `roles.sql`, run once per cluster before the chain by a role allowed to create roles. It creates the two users, the application user and the migrator's user, and nothing else. Creating them and the database are the privileged steps taken before the chain, since nothing runs inside the application database before the chain. The test harness runs `roles.sql` and creates the database. In a deployment the platform creates the application database, owned by the migrator's user ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)), and creates the same two users by its own means, under names the deployment chooses, since the project names no database user, and configures the API server, the UI, the worker and the migrator with them ([ADR-0143](../docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)). `roles.sql` grants nothing on the schema, so confining schema change to the migrator's user rests on PostgreSQL 15 or later, where PUBLIC holds no CREATE on the public schema. PUBLIC keeps TEMP on the database on every version, so the application user can create temporary objects in its own session |
| `db/<subsection>/` | One subsection per table or closely related group of tables of [ADR-0016](../docs/adr/data/0016-schema.md), each holding its statement files and the package generated from them ([ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)). A table's statements sit in the table's subsection, and where a subsection sits one directory further down, or folds into its table, is the rule [ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md) states, so no subsection sits there only so one database user could be granted fewer columns ([ADR-0143](../docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)). They are listed under [Subsections one directory down](#subsections-one-directory-down). `db/policychanges` is the policy history's own subsection, which only the UI admits. How each subsection's package is named is under [Package names](#package-names) |
| `db/tx/` | The shared transaction helper that sets and verifies the account, which every statement reads as `current_setting('app.account')` ([ADR-0047](../docs/adr/data/0047-schema-first-data-access.md), [ADR-0135](../docs/adr/data/0135-every-statement-reads-its-account-from-the-transaction.md)). `Run` opens its transaction at the database's default isolation level, where each statement reads the state committed when it starts. A unit whose statements must read one state passes `Run` the source `Snapshot` wraps, whose transactions are repeatable read and read-only, as the client surface's index reads do ([ADR-0109](../docs/adr/operability/0109-the-index-is-read-through-search-count-and-the-sender-listing.md)) and the UI's reads that classify senders ([ADR-0136](../docs/adr/redaction/0136-the-index-stores-no-sender-class-and-every-reader-classifies-when-it-reads.md)). The wrapper takes only a pool or a connection, never a transaction, so it cannot hide one from `Run`'s refusal. The decision whether a transaction may go on once its setting is read back is the pure `db/tx/core`'s, and both entry points run one unexported shell helper over it. `RunBase` opens the base-policy transaction, which sets the account empty and reads it back, the one unit of data access that names no account ([ADR-0112](../docs/adr/data/0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)). Each statement it runs starts with a `-- name:` comment, as a generated statement does, so its latency is measured under that name ([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)) |
| `db/check/` | The checks over the library's own files, run as Go tests, listed under [The checks](#the-checks). Their test inputs, including a small library of their own and the SQL violation files, sit under `db/check/testdata/`. Each check runs over the real library, which must be clean, and over that test library, which must produce exactly its listed problems, a SQL violation file's each in the statement its want annotation stands in, so no check passes over an empty input |

The generator's configuration sits at `db/sqlc.yaml`, which joins the library with its first
statement file because the generator refuses a configuration with no statements. CI always diffs the
test library's generated code, whose configuration also reads the real migration chain, and diffs
the library's own generated code whenever `db/sqlc.yaml` exists. No `Querier` interface is
generated, since nothing uses one. Statement and migration files are linted by sqlfluff,
configured to read the generator's parameters as placeholders.

## Subsections one directory down

These are the subsections the `db/<subsection>/` row places one directory further down, each named
for what it holds.

| Path | Holds |
| --- | --- |
| `db/accountstate/credential` | The statements that read and write an account's sealed credential, which is a secret |
| `db/ratestate/limiter` | The rate limiter's lease statements, which write the rate state |
| `db/senders/classification` | The UI's reads of the sender statistics, which it classifies under the policy in force, its policy screens counting rules against them, Home summing the messages of the restricted ones and its sender picker searching them with their prior scan hits. It sits below `db/senders` because the API server's list admits `db/senders` and [ADR-0109](../docs/adr/operability/0109-the-index-is-read-through-search-count-and-the-sender-listing.md) keeps the prior scan hits from the API server |

The reads of a run's failures, which join each item's message with its sender's domain for the UI
to classify under the policy in force
([ADR-0136](../docs/adr/redaction/0136-the-index-stores-no-sender-class-and-every-reader-classifies-when-it-reads.md)),
sit in `db/jobruns` itself. The count of masking events under the domain of each event's message's
sender sits in `db/maskingevents` itself. No record forbids their statements to code whose import
list admits their table's subsection, and a subsection of their own would exist only to grant a
database user fewer columns ([ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)).

The writes sit apart by the same rule of
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md), each one a write that the records bounding the
other programs' writes forbid them, such as
[ADR-0084](../docs/adr/mutation/0084-ui-writes-decisions-and-account-setup.md)'s list of the UI's
writes.

| Path | Holds |
| --- | --- |
| `db/messages/ingest` | Adding a message's metadata to the index with its attachment media, and masking a stored subject again |
| `db/maskingevents/record` | Recording a mask applied to a subject |
| `db/auditlog/record` | The API server's recording of each body it serves or denies |
| `db/jobruns/record` | A job kind recording its own runs, their timelines and failed items with the read it resumes from, and `RecordedSuccess`, the read of the account's latest recorded success and earliest run's start that each job's latest success starts from at its ensure ([ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) |
| `db/accountstate/completion` | Setting backfill's completion flags and clearing them when a pass is due again, and setting, reading and clearing the mark that starts the second pass over |
| `db/senders/statistics` | The rebuild of a sender's statistics from the stored messages and the counts of prior scan hits. Delta sync's removal of a sender with no stored message sits in `db/messages/change` |
| `db/messages/scan` | A scanning job kind's writes of a scan verdict, a skip, the delisting transition over the messages skipped as restricted ([ADR-0037](../docs/adr/redaction/0037-delisting-transition.md)), the return to pending of what another scanner decided and of each gate skip the gate no longer decides the same way, with the reads they need |
| `db/scangatedecisions/record` | Recording the scan gate's decisions |
| `db/messages/change` | Delta sync's application of the provider's changes, setting a stored message's labels and flags, removing a message the provider no longer holds and removing a sender's statistics once none of its messages is stored, with the read of the stored messages dated in a gap recovery's window |
| `db/accountstate/cursor` | Delta sync's read and write of its change cursor with the time it was written |
| `db/oauthclients/secret` | The connections' re-seal of an OAuth client's secret by compare-and-set, which delta sync runs, the one write of a client secret beside the UI's setup |
| `db/oauthclients/setup` | The UI's OAuth client setup, listing each client's identity and adding, replacing and removing a client |
| `db/accounts/setup` | The UI's account setup across `accounts` and `account_state`, writing a connected account's two rows, replacing a re-authorized credential, moving an account to another client, and reading and writing what account settings shows, never the credential |
| `db/policyrules/manage` | The UI's policy management of an account's own rules, adding, editing and lifting them, and the read of the rules its policy screen lists |
| `db/policyrules/base` | The base policy's rules and history rows, read and written only in a base-policy transaction, each statement scoped by the null account alone, which the statement check holds it to ([ADR-0112](../docs/adr/data/0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)) |

Recording the last provider authentication attempt sits in `db/accountstate` itself, since every
importer of the table may record one.

## Package names

A subsection's package name ends with the last segment of its directory's path, and where that
segment names another subsection's package too, a word for its parent stands before it, so no two
packages of the library share a name and no importer needs an alias for one. Those are
`auditrecord`, `runrecord`, `maskingrecord` and `gaterecord` under `db/auditlog`, `db/jobruns`,
`db/maskingevents` and `db/scangatedecisions`, and `accountsetup` and `clientsetup` under
`db/accounts` and `db/oauthclients`. `senderclassification` under `db/senders` carries its parent's
word as well, which ending with its segment allows. `sqlc.yaml`'s `package:` sets each name, and
`db/check` holds every name to ending with its directory's last segment and to being unique in the
library.

## The checks

`db/check` runs these checks over the library's own files.

- The statement constraints and the suppression-annotation ban. The case-insensitive cast check and
  the paged-sort check follow a column a FROM-list subquery outputs by name to the table column it
  reads, so a read that groups the subquery's rows is checked as one over the table. The account
  checks never follow it.
- The check on folding a sender domain's case in SQL ([ADR-0016](../docs/adr/data/0016-schema.md)).
  It refuses `lower()`, `upper()`, `casefold()`, `initcap()` or a cast to `citext` over an
  expression holding a domain column, or in one direct operand of a comparison or a function call
  whose other direct operand holds one, a parameter cast to `citext` among them, and `ILIKE`,
  `NOT ILIKE`, `~*` or `!~*` with a domain column in either operand. It matches a domain column by
  its name, `domain` or `from_domain`, whatever table, alias or common table expression qualifies
  it. These forms are outside what it matches and held by review. They are a fold inside a nested
  query whose output is compared with a domain, a domain column read under another name, the `'i'`
  flag of the regular expression functions, an embedded `(?i)` in a pattern, a case-insensitive
  collation, and any other form that folds case or matches approximately, full-text search among
  them.
- The migration lint.
- The compatibility declaration check. Every migration after the baseline, the three files whose
  versions run to 3, holds the line saying every chain the floor admits runs on it, meaning no
  binary whose chain is at or above the current floor depends on what it removes or changes, or
  raises the compatibility floor as `UPDATE schema_compatibility SET oldest_chain_version =
  greatest(oldest_chain_version, E)`, with E an integer from 1 up to the migration's own version,
  which its file name starts with. A statement that names the floor's table anywhere in its parse
  tree is admitted only when it is that raise, a read, which is a select or a create table as a
  select holding no write and no row lock, or a grant or a revoke of `SELECT` alone. Every other is
  refused, whatever its kind, so no migration lowers the floor, writes it otherwise or grants the
  application user a write of it
  ([ADR-0140](../docs/adr/data/0140-each-migration-declares-the-oldest-chain-that-runs-on-it.md)).
- The grantee check over the chain `db/migrate` applies. Every grantee of a grant or a revoke, a
  default privilege's included, is the placeholder `application_user` spelled as `db/migrate`
  matches it. A grant of one role to another is refused, and so is every other role a statement
  names by name, such as a new owner, the role whose default privileges change, a policy's roles and
  a schema's authorization, the placeholder's included. A statement that creates, renames or drops a
  role, or that sets or resets the session's role or authorization, is refused, since the deployment
  creates and names both users. A role name inside a string a function reads, such as
  `set_config`'s, is left to review. The placeholder appears nowhere else in a migration's text, in
  no comment, string, quoted name or other identifier, matched in any ASCII letter case, so the
  replacement of it with the application user's configured name changes nothing but a grantee
  ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)). The test library's own migration
  grants to its user by name and is not held to it.
- The derived account-keyed table list and its exceptions, the accounts listing, the base policy's
  statements, which compare the account `IS NULL`, and the operation log, which carries no account
  and is reached through a join from its plan column to a plan tied to the transaction's account
  ([ADR-0135](../docs/adr/data/0135-every-statement-reads-its-account-from-the-transaction.md)).
  Every other statement against an account-keyed table compares `account_id` with
  `current_setting('app.account')` in a conjunction of equalities, and an insert takes it from
  there. An account column compared with a parameter is refused, and so is any update of an account
  column, an upsert's included, whatever the value. A reference the check cannot show tied to the
  transaction's account, such as one tied only under `OR`, through a subquery's comparison or
  through a common table expression's output, is refused rather than accepted. The check reads
  selects, inserts, updates and deletes and the queries nested in the clauses it reads, refuses
  every other kind of statement, `MERGE` and `TRUNCATE` among them, and refuses a reference to an
  account-keyed table, or to the operation log, in any clause it does not read. It reads the tables
  a statement names in its parse tree, so a table named only inside a string of SQL that a function
  runs, such as `query_to_xml`, is left to review.
- The no-table-types assertion.
- The check that every generated file has its statement file.
- The check that no hand-written non-test Go file other than a `doc.go` sits in a subsection's
  directory, in a directory above one, or in the library's root, which keeps a wrapper file out of a
  subsection.
- The check that no package of the data-access library imports a subsection unless it is itself a
  subsection, which keeps a wrapper package such as the transaction helper or the root from reaching
  a subsection's statements.
- The check that every SQL file is read by some check.
- The check of the generator configuration's layout.
- The check that no two statements of the library, `db/tx`'s own included, share the name in their
  `-- name:` comment, which labels each statement's latency series
  ([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
- The check that the test library's generated code refuses a column the schema does not hold.
- The check that a component's list admits the library only by naming `db/tx`, `db/tx/core`,
  `db/migrate`, `db/migrations` and generated subsections exactly, never the library's root package
  or another directory holding no statements. None of the four named packages holds a statement of
  its own, and the list of the migrator's task package names the last two.
- The integration tests that hold the application user to its grants, every account-keyed table
  to the transaction's account in the statements' form, and the log tables to their foreign keys.
- The grant check, which plans every statement of every subsection an import list admits, directly
  or through the lists of one or more shared libraries, under the application user, so the user's
  grants are proven enough for every admitted statement
  ([ADR-0143](../docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)).
  What one program's code may run apart from another's is the import lists' to tell, not the
  grants'.

A list admits a library when one of its entries admits a package of the library that runs the
library's statements, one that imports one of the library's subsections or, through the library's
own packages, reaches one that does, so admitting only a library's pure core does not count. No
list of the migrator names a subsection. Its task's list reaches the start check's subsection
through `process/database`, and the grant check plans that subsection under the application user
as it does for every other list that reaches it. The chain itself is applied as the migrator's
user, which owns the schema and which the grant check does not plan under
([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)).
