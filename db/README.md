# db

The data-access library, published as `mediated-mailbox-db`
([ADR-0047](../docs/adr/data/0047-schema-first-data-access.md)). The conventions it shares with
every component are [CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

| Path | Holds |
| --- | --- |
| `db/migrations/` | The migration chain ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)). A baseline of four files, the one extension the schema needs, which the migration role creates since it is trusted, every table in [ADR-0016](../docs/adr/data/0016-schema.md)'s order, the row-level security policies, and the grants with one section per runtime role followed by one per shared library, then the migrations after it, each tested over rows the chain before it wrote |
| `db/bootstrap/` | `roles.sql`, run once per cluster before the chain by a role allowed to create roles, the one privileged step a deployment takes, since nothing runs inside the application database before the chain ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)). It is also the contract a deployment platform follows. It grants nothing on the schema, so confining schema change to the migration role rests on PostgreSQL 15 or later, where PUBLIC holds no CREATE on the public schema. PUBLIC keeps TEMP on the database on every version, so a runtime role can create temporary objects in its own session |
| `db/<subsection>/` | One subsection per table or closely related group of tables of [ADR-0016](../docs/adr/data/0016-schema.md), each holding its statement files and the package generated from them ([ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)). A table's statements sit in the table's subsection. A statement that a role admitted to that subsection may not be granted under [ADR-0118](../docs/adr/data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)'s three lines sits one directory further down, in a subsection named for what it holds, which only the roles that may run it admit. They are listed under [Subsections one directory down](#subsections-one-directory-down). `db/policychanges` is the policy history's own subsection, which only the UI admits. How each subsection's package is named is under [Package names](#package-names) |
| `db/tx/` | The shared transaction helper that sets and verifies the account ([ADR-0047](../docs/adr/data/0047-schema-first-data-access.md)). `Run` opens its transaction at the database's default isolation level, where each statement reads the state committed when it starts. A unit whose statements must read one state passes `Run` the source `Snapshot` wraps, whose transactions are repeatable read and read-only, as the client surface's index reads do ([ADR-0109](../docs/adr/operability/0109-the-index-is-read-through-search-count-and-the-sender-listing.md)). The wrapper takes only a pool or a connection, never a transaction, so it cannot hide one from `Run`'s refusal. `RunBase` opens the base-policy transaction, which sets the account empty and `app.base` on and reads both back, the one unit of data access that names no account ([ADR-0112](../docs/adr/data/0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)). Each statement it runs starts with a `-- name:` comment, as a generated statement does, so its latency is measured under that name ([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)) |
| `db/check/` | The checks over the library's own files, run as Go tests, listed under [The checks](#the-checks). Their test inputs, including a small library of their own and the SQL violation files, sit under `db/check/testdata/`. Each check runs over the real library, which must be clean, and over that test library, which must produce exactly its listed problems, so no check passes over an empty input |

The generator's configuration sits at `db/sqlc.yaml`, which joins the library with its first
statement file because the generator refuses a configuration with no statements. CI always diffs the
test library's generated code, whose configuration also reads the real migration chain, and diffs
the library's own generated code whenever `db/sqlc.yaml` exists. Statement and migration files are
linted by sqlfluff, configured to read the generator's parameters as placeholders.

## Subsections one directory down

These are the subsections the `db/<subsection>/` row places one directory further down, each named
for what it holds.

| Path | Holds |
| --- | --- |
| `db/accountstate/credential` | The statements that read and write an account's sealed credential, which is a secret |
| `db/ratestate/limiter` | The rate limiter's lease statements, which write the rate state |
| `db/messages/classification` | The statements that read the stored sender class, which a record forbids the mediator to act on ([ADR-0002](../docs/adr/redaction/0002-fetch-time-re-evaluation.md)) |
| `db/jobruns/classification` | The reads of a run's failures, which join each item's message and read its stored sender class for the same reason |
| `db/maskingevents/senders` | The count of masking events under the domain of each event's message's sender, a column the mediator's role is not granted |
| `db/senders/classification` | The UI's reads of the sender statistics with their stored class, which its policy screens count rules against and its sender picker searches |

The writes the tables' readers are not granted sit apart the same way.

| Path | Holds |
| --- | --- |
| `db/messages/ingest` | Adding a message's metadata to the index and masking a stored subject again |
| `db/maskingevents/record` | Recording a mask applied to a subject |
| `db/auditlog/record` | The mediator's recording of each body it serves or denies |
| `db/jobruns/record` | A job kind recording its own runs, their timelines and failed items with the read it resumes from, and `RecordedSuccess`, the read of the account's latest recorded success and earliest run's start that each job's latest success starts from at its ensure ([ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) |
| `db/accountstate/completion` | Setting backfill's completion flags and clearing them when a pass is due again, and setting, reading and clearing the mark that starts the second pass over |
| `db/accountstate/authentication` | Recording the last provider authentication attempt |
| `db/senders/statistics` | The rebuild of a sender's statistics from the stored messages and the counts of prior scan hits. Delta sync's removal of a sender with no stored message sits in `db/messages/change` |
| `db/messages/scan` | A scanning job kind's writes of a scan verdict, a skip, the delisting transition and its counterpart restricting the stored classes a rule added since restricts ([ADR-0113](../docs/adr/redaction/0113-an-added-rule-reaches-the-stored-classes-by-its-effect.md)), the return to pending of what another scanner decided and of each gate skip the gate no longer decides the same way, with the reads they need, the stored sender class among them |
| `db/scangatedecisions/record` | Recording the scan gate's decisions |
| `db/messages/change` | Delta sync's application of the provider's changes, setting a stored message's labels and flags, removing a message the provider no longer holds and removing a sender's statistics once none of its messages is stored, with the read of the stored messages dated in a gap recovery's window |
| `db/accountstate/cursor` | Delta sync's read and write of its change cursor with the time it was written |
| `db/oauthclients/secret` | Delta sync's re-seal of an OAuth client's secret by compare-and-set, the one write of a client secret beside the UI's setup |
| `db/oauthclients/setup` | The UI's OAuth client setup, listing each client's identity and adding, replacing and removing a client |
| `db/accounts/setup` | The UI's account setup across `accounts` and `account_state`, writing a connected account's two rows, replacing a re-authorized credential, moving an account to another client, and reading and writing what account settings shows, never the credential |
| `db/policyrules/manage` | The UI's policy management of an account's own rules, adding, editing and lifting them, and the read of the rules its policy screen lists |
| `db/policyrules/base` | The base policy's rules and history rows, read and written only in a base-policy transaction, each statement scoped by the null account alone, which the statement check holds it to ([ADR-0112](../docs/adr/data/0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)) |

## Package names

A subsection's package takes the last segment of its directory's path, and where that segment names
another subsection's package too, a word for its parent before it, so no two packages of the library
share a name and no importer needs an alias for one. They are `auditrecord`, `runrecord`,
`maskingrecord` and `gaterecord` under `db/auditlog`, `db/jobruns`, `db/maskingevents` and
`db/scangatedecisions`, `runclassification`, `messageclassification` and `senderclassification`
under `db/jobruns`, `db/messages` and `db/senders`, `maskingsenders` under `db/maskingevents`, and
`accountsetup` and `clientsetup` under `db/accounts` and `db/oauthclients`. `sqlc.yaml`'s `package:`
sets each name, and `db/check` holds every name to ending with its directory's last segment and to
being unique in the library.

## The checks

`db/check` runs these checks over the library's own files.

- The statement constraints and the suppression-annotation ban.
- The check on folding a sender domain's case in SQL ([ADR-0016](../docs/adr/data/0016-schema.md)).
  It refuses `lower()`, `upper()`, `casefold()`, `initcap()` or a cast to `citext` over an
  expression holding a domain column, or in one direct operand of a comparison or a function call
  whose other direct operand holds one, a parameter cast to `citext` among them, and `ILIKE`,
  `NOT ILIKE`, `~*` or `!~*` with a domain column in either operand. It matches a domain column by
  its name, `domain` or `from_domain`, whatever table, alias or common table expression qualifies
  it. These forms are outside what it matches and held by review: a fold inside a nested query whose
  output is compared with a domain, a domain column read under another name, the `'i'` flag of the
  regular expression functions, an embedded `(?i)` in a pattern, a case-insensitive collation, and
  any other form that folds case or matches approximately, full-text search among them.
- The migration lint.
- The derived account-keyed table list and its exceptions.
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
- The check that a component's list admits the library only by naming `db/tx` and generated
  subsections exactly, never the library's root package or a directory holding no statements.
- The check that every list naming data-access subsections has a database role to be tested under,
  its own or that of a deployable or job kind whose list admits it, directly or through the lists of
  other shared libraries that admit it, as the account session's list admits the account snapshot's
  package, and every role names a list that names such subsections or admits a library's list that
  does.
- The integration tests that hold each runtime role and the operation log's policy to their grants.
- The grant check, which plans each statement under the role of every deployable or job kind whose
  list admits its subsection, directly or through the lists of one or more shared libraries.

A list admits a library when one of its entries admits a package of the library that runs the
library's statements, one that imports one of the library's subsections or, through the library's
own packages, reaches one that does, so admitting only a library's pure core does not count. The
map from import list to database role those checks read is held in `db/check`, keyed by the
mediator's and the UI's import lists, by each narrower list governing part of their code that admits
code naming subsections, and by each list of a job kind of the worker
([ADR-0118](../docs/adr/data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)). The
worker's own list names no subsection and admits no shared library that runs statements, so it has
no entry, and a shared library has none either, since it connects as no role of its own. The
worker's list admits the driver only so its composition root can open each job kind's pool, and a
statement the worker's own code runs on a pool is left to review.
